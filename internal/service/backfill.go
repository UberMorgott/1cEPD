package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"partnerops/internal/events"
	"partnerops/internal/money"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// BackfillInterval — не чаще одного отчёта догрузки истории за этот срок:
// каждый отчёт тратит часовой лимит 1С, который нужен и ежедневным задачам.
const BackfillInterval = time.Hour

// backfillMaxFailures — после стольких сбоев месяц бросаем: иначе битый месяц
// тратил бы по отчёту в час бесконечно.
const backfillMaxFailures = 3

// BackfillStore хранит попытки догрузки.
type BackfillStore interface {
	Record(ctx context.Context, period, status, errText string, at time.Time) error
	Attempts(ctx context.Context) (map[string]store.BackfillAttempt, error)
}

// SnapshotHistory — снапшоты, в которые догрузка кладёт прошлые месяцы.
type SnapshotHistory interface {
	SnapshotStore
	Periods(ctx context.Context) (map[string]bool, error)
}

// BillingBackfill догружает биллинг ЭДО за 12 закрытых месяцев, по одному
// месяцу за интервал. Реестр и аномалии не трогает: старый месяц не должен
// откатывать владельцев и даты последнего появления.
type BillingBackfill struct {
	reports   ReportSource
	snapshots SnapshotHistory
	attempts  BackfillStore
	bus       *events.Bus
	now       func() time.Time
	interval  time.Duration
}

// NewBillingBackfill собирает сервис.
func NewBillingBackfill(
	reports ReportSource, snapshots SnapshotHistory, attempts BackfillStore, bus *events.Bus,
) *BillingBackfill {
	return &BillingBackfill{
		reports: reports, snapshots: snapshots, attempts: attempts, bus: bus,
		now: time.Now, interval: BackfillInterval,
	}
}

// ClosedMonths возвращает первые числа 12 закрытых месяцев перед now, по возрастанию.
func ClosedMonths(now time.Time) []time.Time {
	from, _ := lastTwelveMonths(now)
	months := make([]time.Time, 12)
	for i := range months {
		months[i] = from.AddDate(0, i, 0)
	}
	return months
}

// Step строит не больше одного отчёта: за самый свежий недостающий месяц, и
// только если с прошлой попытки прошёл интервал. Время попытки хранится в базе,
// поэтому перезапуск не сбрасывает паузу. Возвращает месяц, за которым ходили.
func (b *BillingBackfill) Step(ctx context.Context) (string, error) {
	now := b.now().UTC()
	attempts, err := b.attempts.Attempts(ctx)
	if err != nil {
		return "", err
	}
	for _, a := range attempts {
		if now.Sub(a.AttemptedAt) < b.interval {
			return "", nil
		}
	}
	have, err := b.snapshots.Periods(ctx)
	if err != nil {
		return "", err
	}

	month, ok := nextBackfillMonth(ClosedMonths(now), have, attempts)
	if !ok {
		return "", nil
	}
	period := month.Format("2006-01")
	status, errText := b.fetch(ctx, month, period, now)
	// Попытку записываем и при остановке сервиса: задача в 1С уже поставлена.
	if err := b.attempts.Record(context.WithoutCancel(ctx), period, status, errText, now); err != nil {
		return period, err
	}
	if status == store.BackfillFailed || status == store.BackfillLimited {
		return period, fmt.Errorf("service: биллинг за %s не догружен: %s", period, errText)
	}
	return period, nil
}

func (b *BillingBackfill) fetch(ctx context.Context, month time.Time, period string, now time.Time) (string, string) {
	raw, err := b.reports.EDOBillingReport(ctx, month)
	switch {
	case partner.IsNoBilling(err):
		return store.BackfillNone, ""
	case partner.IsRateLimited(err):
		return store.BackfillLimited, err.Error()
	case err != nil:
		return store.BackfillFailed, err.Error()
	}
	rows, err := partner.ParseEDOBilling(raw)
	if err != nil {
		return store.BackfillFailed, err.Error()
	}
	if len(rows) == 0 {
		return store.BackfillNone, ""
	}
	if _, err := b.snapshots.Save(ctx, period, now, raw, rows); err != nil {
		return store.BackfillFailed, err.Error()
	}
	slog.Info("биллинг прошлого месяца догружен", "period", period, "rows", len(rows))
	if b.bus != nil {
		b.bus.Publish(events.Event{Kind: "snapshot.completed", Payload: map[string]any{
			"period": period, "rows": len(rows), "backfill": true,
		}})
	}
	return store.BackfillDone, ""
}

// nextBackfillMonth — самый свежий месяц без снапшота, который ещё стоит просить.
func nextBackfillMonth(
	months []time.Time, have map[string]bool, attempts map[string]store.BackfillAttempt,
) (time.Time, bool) {
	for _, month := range slices.Backward(months) {
		period := month.Format("2006-01")
		if have[period] {
			continue
		}
		a, tried := attempts[period]
		if tried && (a.Status == store.BackfillNone || a.Status == store.BackfillDone ||
			a.Failures >= backfillMaxFailures) {
			continue
		}
		return month, true
	}
	return time.Time{}, false
}

// Состояние месяца в истории.
const (
	HistoryOK      = "ok"
	HistoryPending = "pending"
	HistoryNone    = "none"
	HistoryFailed  = "failed"
)

// HistoryMonth — итоги месяца по всем клиентам.
type HistoryMonth struct {
	Period       string `json:"period"`
	Status       string `json:"status"`
	Clients      int    `json:"clients"`
	Packets      int64  `json:"packets"`
	Billable     int64  `json:"billable"`
	TotalDue     string `json:"totalDue"`
	PartnerTotal string `json:"partnerTotal"`
}

// HistoryClient — помесячный расход идентификатора; позиции совпадают с месяцами,
// null — строки за месяц нет.
type HistoryClient struct {
	EDOID    string   `json:"edoId"`
	Packets  []*int64 `json:"packets"`
	Limits   []*int64 `json:"limits"`
	Billable []int64  `json:"billable"`
}

// BillingHistory — помесячная динамика биллинга.
type BillingHistory struct {
	Months  []HistoryMonth  `json:"months"`
	Clients []HistoryClient `json:"clients"`
}

// BuildBillingHistory сводит снимки месяцев в помесячную динамику.
// rows — строки последнего снимка месяца; нет ключа — снимка нет.
func BuildBillingHistory(
	periods []string, rows map[string][]partner.EDOBillingRow, attempts map[string]store.BackfillAttempt,
) BillingHistory {
	history := BillingHistory{Months: make([]HistoryMonth, len(periods)), Clients: []HistoryClient{}}
	byID := map[string]*HistoryClient{}
	var order []string
	for i, period := range periods {
		month := HistoryMonth{Period: period, Status: HistoryPending}
		monthRows, ok := rows[period]
		switch a := attempts[period]; {
		case ok:
			month.Status = HistoryOK
		case a.Status == store.BackfillNone:
			month.Status = HistoryNone
		case a.Failures >= backfillMaxFailures:
			month.Status = HistoryFailed
		}
		var due, partnerDue money.Amount
		for _, r := range monthRows {
			month.Clients++
			month.Packets += r.Packets
			month.Billable += r.PacketsBillable
			due += r.ClientAmount
			partnerDue += r.PartnerAmount
			if r.EDOID == "" {
				continue
			}
			client := byID[r.EDOID]
			if client == nil {
				client = &HistoryClient{
					EDOID: r.EDOID, Packets: make([]*int64, len(periods)),
					Limits: make([]*int64, len(periods)), Billable: make([]int64, len(periods)),
				}
				byID[r.EDOID] = client
				order = append(order, r.EDOID)
			}
			packets := r.Packets
			client.Packets[i] = &packets
			client.Limits[i] = r.Limit
			client.Billable[i] += r.PacketsBillable
		}
		if ok {
			month.TotalDue, month.PartnerTotal = due.String(), partnerDue.String()
		}
		history.Months[i] = month
	}
	for _, id := range order {
		history.Clients = append(history.Clients, *byID[id])
	}
	return history
}
