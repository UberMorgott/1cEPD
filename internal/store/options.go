package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"partnerops/internal/partner"
)

// Состояния сохранённого отчёта по опциям.
const (
	OptionStateOK = "ok"
	// OptionStateNone — 1С ответила BILLING_DOES_NOT_EXIST: у партнёра нет
	// тарифов этого вида. Повторять раньше суток бессмысленно.
	OptionStateNone = "none"
)

// OptionReport — сохранённый отчёт по опциям одного вида.
type OptionReport struct {
	Type      string
	State     string
	Entries   []partner.OptionEntry
	FetchedAt time.Time
}

// Формат колонки entries. Отдельные типы, чтобы хранение не зависело от
// полей partner.
type optionJSON struct {
	Name         string   `json:"name"`
	Quantitative bool     `json:"quantitative"`
	MaxVolume    *float64 `json:"maxVolume"`
	UsedVolume   *float64 `json:"usedVolume"`
}

type optionTariffJSON struct {
	Name       string       `json:"name"`
	TypeNumber *int         `json:"typeNumber"`
	BuyDate    time.Time    `json:"buyDate"`
	OrgName    string       `json:"orgName"`
	OrgINN     string       `json:"orgInn"`
	OrgKPP     string       `json:"orgKpp"`
	Start      time.Time    `json:"start"`
	End        time.Time    `json:"end"`
	Options    []optionJSON `json:"options"`
}

type optionEntryJSON struct {
	SubscriberCode string             `json:"subscriberCode"`
	Tariffs        []optionTariffJSON `json:"tariffs"`
}

// OptionReports хранит последние отчёты по опциям.
type OptionReports struct {
	db *sql.DB
}

// NewOptionReports создаёт хранилище поверх открытой базы.
func NewOptionReports(db *sql.DB) *OptionReports {
	return &OptionReports{db: db}
}

// Save заменяет отчёт вида reportType.
func (s *OptionReports) Save(ctx context.Context, report OptionReport) error {
	entries := make([]optionEntryJSON, 0, len(report.Entries))
	for _, e := range report.Entries {
		entry := optionEntryJSON{SubscriberCode: e.SubscriberCode, Tariffs: make([]optionTariffJSON, 0, len(e.Tariffs))}
		for _, t := range e.Tariffs {
			tariff := optionTariffJSON{Name: t.Name, TypeNumber: t.TypeNumber, BuyDate: t.BuyDate,
				OrgName: t.OrgName, OrgINN: t.OrgINN, OrgKPP: t.OrgKPP, Start: t.Start, End: t.End,
				Options: make([]optionJSON, 0, len(t.Options))}
			for _, o := range t.Options {
				tariff.Options = append(tariff.Options, optionJSON(o))
			}
			entry.Tariffs = append(entry.Tariffs, tariff)
		}
		entries = append(entries, entry)
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("store: не сохранить отчёт по опциям: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO option_reports (report_type, state, entries, fetched_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(report_type) DO UPDATE SET state = excluded.state, entries = excluded.entries,
			fetched_at = excluded.fetched_at`,
		report.Type, report.State, string(raw), report.FetchedAt.UTC().Unix()); err != nil {
		return fmt.Errorf("store: не сохранить отчёт по опциям %s: %w", report.Type, err)
	}
	return nil
}

// Fetched возвращает время последнего отчёта по каждому виду.
func (s *OptionReports) Fetched(ctx context.Context) (map[string]time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT report_type, fetched_at FROM option_reports`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать отчёты по опциям: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := map[string]time.Time{}
	for rows.Next() {
		var kind string
		var fetched int64
		if err := rows.Scan(&kind, &fetched); err != nil {
			return nil, fmt.Errorf("store: не разобрать отчёт по опциям: %w", err)
		}
		result[kind] = time.Unix(fetched, 0).UTC()
	}
	return result, rows.Err()
}

// All возвращает сохранённые отчёты, упорядоченные по виду.
func (s *OptionReports) All(ctx context.Context) ([]OptionReport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT report_type, state, entries, fetched_at FROM option_reports ORDER BY report_type`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать отчёты по опциям: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []OptionReport
	for rows.Next() {
		var report OptionReport
		var raw string
		var fetched int64
		if err := rows.Scan(&report.Type, &report.State, &raw, &fetched); err != nil {
			return nil, fmt.Errorf("store: не разобрать отчёт по опциям: %w", err)
		}
		var entries []optionEntryJSON
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return nil, fmt.Errorf("store: не разобрать абонентов отчёта по опциям %s: %w", report.Type, err)
		}
		for _, e := range entries {
			entry := partner.OptionEntry{SubscriberCode: e.SubscriberCode}
			for _, t := range e.Tariffs {
				tariff := partner.OptionTariff{Name: t.Name, TypeNumber: t.TypeNumber, BuyDate: t.BuyDate,
					OrgName: t.OrgName, OrgINN: t.OrgINN, OrgKPP: t.OrgKPP, Start: t.Start, End: t.End}
				for _, o := range t.Options {
					tariff.Options = append(tariff.Options, partner.Option(o))
				}
				entry.Tariffs = append(entry.Tariffs, tariff)
			}
			report.Entries = append(report.Entries, entry)
		}
		report.FetchedAt = time.Unix(fetched, 0).UTC()
		list = append(list, report)
	}
	return list, rows.Err()
}
