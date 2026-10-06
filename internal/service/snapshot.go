// Package service содержит прикладную логику: она соединяет партнёрское API и хранилище.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"partnerops/internal/events"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// ReportSource строит отчёт биллинга ЭДО. Отдельный интерфейс, чтобы тесты не ходили в сеть.
type ReportSource interface {
	EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error)
}

// SnapshotStore сохраняет снапшот.
type SnapshotStore interface {
	Save(ctx context.Context, period string, takenAt time.Time, raw []byte, rows []partner.EDOBillingRow) (store.SaveResult, error)
}

// Registry накапливает реестр идентификаторов и хранит находки.
type Registry interface {
	Upsert(ctx context.Context, records []store.IdentifierRecord, period string, seenAt time.Time) error
	RecordEvent(ctx context.Context, event store.AnomalyEvent, at time.Time) error
	OwnersByID(ctx context.Context) (map[string]string, error)
	All(ctx context.Context) ([]store.IdentifierRecord, error)
}

// Snapshot строит и сохраняет ежедневный снимок отчёта биллинга ЭДО.
type Snapshot struct {
	reports  ReportSource
	store    SnapshotStore
	bus      *events.Bus
	registry Registry
}

// NewSnapshot собирает сервис.
func NewSnapshot(reports ReportSource, store SnapshotStore, bus *events.Bus) *Snapshot {
	return &Snapshot{reports: reports, store: store, bus: bus}
}

// WithRegistry подключает реестр. Без него снапшоты просто сохраняются.
func (s *Snapshot) WithRegistry(registry Registry) *Snapshot {
	s.registry = registry
	return s
}

// SnapshotRefreshInterval — отчёт биллинга моложе этого суточная задача не
// строит заново: перезапуск сервиса не должен тратить часовой лимит задач 1С.
// Чуть меньше суток, как у ITSRefreshInterval: тик, пришедший на минуту раньше,
// иначе пропустил бы снимок ещё на сутки.
const SnapshotRefreshInterval = 20 * time.Hour

// FetchLog сообщает, когда отчёт за период строился в последний раз;
// *store.Snapshots ему удовлетворяет.
type FetchLog interface {
	LastFetched(ctx context.Context, period string) (time.Time, error)
}

// TakeIfStale делает Take, если отчёт за месяц даты не строился последние
// maxAge. Хранилище без FetchLog проверку не поддерживает — снимок снимается
// всегда. Возвращает, строился ли отчёт.
func (s *Snapshot) TakeIfStale(ctx context.Context, date time.Time, maxAge time.Duration) (bool, error) {
	if log, ok := s.store.(FetchLog); ok {
		period := date.UTC().Format("2006-01")
		last, err := log.LastFetched(ctx, period)
		if err != nil {
			return false, err
		}
		if !last.IsZero() && time.Since(last) < maxAge {
			slog.Info("отчёт биллинга свежий, повторно не строится", "period", period, "fetchedAt", last)
			return false, nil
		}
	}
	return true, s.Take(ctx, date)
}

// Take строит отчёт за месяц указанной даты, разбирает его и сохраняет.
//
// Пустой отчёт считается сбоем: у партнёра всегда есть клиенты, и затирать историю
// пустым снимком нельзя.
func (s *Snapshot) Take(ctx context.Context, date time.Time) error {
	period := date.UTC().Format("2006-01")

	raw, err := s.reports.EDOBillingReport(ctx, date)
	if err != nil {
		return fmt.Errorf("service: не построить отчёт за %s: %w", period, err)
	}

	rows, err := partner.ParseEDOBilling(raw)
	if err != nil {
		return fmt.Errorf("service: не разобрать отчёт за %s: %w", period, err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("service: отчёт за %s пуст, снапшот не сохранён", period)
	}

	takenAt := time.Now().UTC()
	res, err := s.store.Save(ctx, period, takenAt, raw, rows)
	if err != nil {
		return fmt.Errorf("service: не сохранить снапшот за %s: %w", period, err)
	}

	if res.Duplicate {
		// Данные не изменились с прошлого раза — сообщать не о чем.
		slog.Info("отчёт совпал с предыдущим, снапшот не создан", "period", period, "id", res.ID)
		return nil
	}

	if s.registry != nil {
		if err := s.updateRegistry(ctx, rows, period, takenAt); err != nil {
			return fmt.Errorf("service: не обновить реестр за %s: %w", period, err)
		}
	}

	slog.Info("снапшот сохранён",
		"period", period, "rows", len(rows), "id", res.ID, "baseline", res.IsBaseline)
	s.bus.Publish(events.Event{
		Kind: "snapshot.completed",
		Payload: map[string]any{
			"period":   period,
			"rows":     len(rows),
			"takenAt":  takenAt.Format(time.RFC3339),
			"baseline": res.IsBaseline,
		},
	})
	return nil
}

// updateRegistry пополняет реестр и записывает найденные аномалии.
// Владельцы читаются ДО обновления реестра: иначе сравнивать будет не с чем.
func (s *Snapshot) updateRegistry(
	ctx context.Context, rows []partner.EDOBillingRow, period string, at time.Time,
) error {
	previousOwners, err := s.registry.OwnersByID(ctx)
	if err != nil {
		return err
	}
	known, err := s.registry.All(ctx)
	if err != nil {
		return err
	}

	records := make([]store.IdentifierRecord, 0, len(rows))
	for _, r := range rows {
		if r.EDOID == "" {
			continue
		}
		code, _ := partner.ParseOwner(r.Owner)
		records = append(records, store.IdentifierRecord{
			EDOID: r.EDOID, INN: r.INN, KPP: r.KPP, ClientName: r.ClientName,
			Login: r.Login, OwnerRaw: r.Owner, OwnerCode: code,
			ITSTariffs: r.ITSTariffs, Limit: r.Limit, Packets: r.Packets,
		})
	}

	for _, anomaly := range append(FindAnomalies(rows, previousOwners), FindVanished(rows, known, period)...) {
		err := s.registry.RecordEvent(ctx, store.AnomalyEvent{
			EDOID: anomaly.EDOID, Kind: anomaly.Kind, Confidence: anomaly.Confidence,
			StateFingerprint: anomaly.StateFingerprint, INN: anomaly.INN, KPP: anomaly.KPP,
			ClientName: anomaly.ClientName, Login: anomaly.Login, Details: anomaly.Details,
		}, at)
		if err != nil {
			return err
		}
	}

	return s.registry.Upsert(ctx, records, period, at)
}
