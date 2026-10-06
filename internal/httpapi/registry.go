package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// RegistryStore — то, что нужно обработчикам от хранилища реестра.
type RegistryStore interface {
	Events(ctx context.Context, includeAcknowledged bool) ([]store.AnomalyEvent, error)
	All(ctx context.Context) ([]store.IdentifierRecord, error)
	Acknowledge(ctx context.Context, edoID, fingerprint, reason, author string, at time.Time) error
}

// SnapshotStore — источник строк последнего снимка для расчёта биллинга.
type SnapshotStore interface {
	Latest(ctx context.Context, period string) (store.Snapshot, []partner.EDOBillingRow, error)
}

// Registry обслуживает экраны реестра, аномалий и биллинга.
type Registry struct {
	registry  RegistryStore
	snapshots SnapshotStore
	// itsChecks и itsRefresher — сроки договоров 1С:ИТС, см. WithITS.
	itsChecks    ITSChecksStore
	itsRefresher ITSRefresher
	// industryChecks — нужда в ИТС Отраслевом, см. WithIndustry.
	industryChecks IndustryChecksStore
	// epdUsage — расход ЭПД для подсказки тарифа, см. WithEPDUsage.
	epdUsage EPDUsageReader
	// backfill — догрузка истории биллинга, см. WithHistory.
	backfill BackfillReader
	// monthTraffic — прогноз перерасхода, см. WithForecast.
	monthTraffic EPDUsageReader
	// subscribers — база абонентов, см. WithSubscribers.
	subscribers          SubscribersReader
	subscribersRefresher ITSRefresher
	// reviewAcks и topology — пересмотр скрытых находок, см. WithReview.
	reviewAcks AckStore
	topology   TopologyStore
	// optionReports — лицензии сервисов, см. WithLicenses.
	optionReports OptionReportsStore
	// requests и programs — заявки и программы в карточке клиента, см. WithClientCards.
	requests RequestLister
	programs ProgramReader
	// epdImport — выгрузка биллинга ЭПД, загруженная руками, см. WithEPDImport.
	epdImport EPDImportStore
}

// NewRegistry создаёт обработчики.
func NewRegistry(registry RegistryStore, snapshots SnapshotStore) *Registry {
	return &Registry{registry: registry, snapshots: snapshots}
}

// Anomalies отдаёт находки. Параметр all=1 показывает и скрытые: помеченные
// законными и погашенные реестром топологии — для пересмотра.
func (h *Registry) Anomalies(w http.ResponseWriter, r *http.Request) {
	includeHidden := r.URL.Query().Get("all") == "1"
	all, err := h.anomalyItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать находки.")
		return
	}
	list := []anomalyItem{}
	for _, item := range all {
		if item.hidden() && !includeHidden {
			continue
		}
		list = append(list, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"anomalies": list})
}

// anomalyItems — все находки с пометками и погашением топологией, скрытые тоже.
// Скрытые отбираются здесь, а не в базе: топология живёт в другой таблице.
func (h *Registry) anomalyItems(ctx context.Context) ([]anomalyItem, error) {
	events, err := h.registry.Events(ctx, true)
	if err != nil {
		return nil, err
	}
	acks, topology, err := h.reviewData(ctx)
	if err != nil {
		return nil, err
	}

	// Прогноз перерасхода сюда не входит: он в биллинге. Сохранённые пометки
	// «это нормально» по прогнозу остаются в базе и никому не мешают.
	now := time.Now().UTC()
	list := make([]anomalyItem, 0, len(events))
	for _, e := range events {
		item := anomalyItem{
			ID: e.ID, EDOID: e.EDOID, Kind: e.Kind, Confidence: e.Confidence,
			Fingerprint: e.StateFingerprint, INN: e.INN, KPP: e.KPP,
			ClientName: e.ClientName, Login: e.Login, Details: e.Details,
			DetectedAt: e.DetectedAt.Format(time.RFC3339), Acked: e.Acknowledged,
		}
		if ack, ok := acks[item.EDOID+"/"+item.Fingerprint]; ok {
			item.Acked, item.AckReason, item.AckAuthor = true, ack.Reason, ack.Author
			item.AckedAt = ack.AckedAt.Format(time.RFC3339)
			if !ack.ReviewAt.IsZero() {
				item.ReviewAt = ack.ReviewAt.Format(time.RFC3339)
				item.ReviewDue = !now.Before(ack.ReviewAt)
			}
		}
		if purpose, ok := service.SuppressedByTopology(item.Kind, item.INN, item.KPP, item.EDOID, topology); ok {
			item.Suppressed, item.TopologyPurpose = true, purpose
		}
		list = append(list, item)
	}
	return list, nil
}

type anomalyItem struct {
	ID          int64  `json:"id"`
	EDOID       string `json:"edoId"`
	Kind        string `json:"kind"`
	Confidence  string `json:"confidence"`
	Fingerprint string `json:"fingerprint"`
	INN         string `json:"inn"`
	KPP         string `json:"kpp"`
	ClientName  string `json:"clientName"`
	Login       string `json:"login"`
	Details     string `json:"details"`
	DetectedAt  string `json:"detectedAt"`
	Acked       bool   `json:"acknowledged"`
	// Пометка «это законно»: кто, почему, когда и когда пересмотреть.
	AckReason string `json:"ackReason,omitempty"`
	AckAuthor string `json:"ackAuthor,omitempty"`
	AckedAt   string `json:"ackedAt,omitempty"`
	ReviewAt  string `json:"reviewAt,omitempty"`
	ReviewDue bool   `json:"reviewDue,omitempty"`
	// Suppressed — погашена реестром ожидаемой топологии, TopologyPurpose — назначение связи.
	Suppressed      bool   `json:"suppressed,omitempty"`
	TopologyPurpose string `json:"topologyPurpose,omitempty"`
}

// hidden — находка скрыта из активных: помечена законной или погашена топологией.
func (a anomalyItem) hidden() bool { return a.Acked || a.Suppressed }

type ackRequest struct {
	EDOID       string `json:"edoId"`
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
}

// Acknowledge помечает находку законной.
func (h *Registry) Acknowledge(w http.ResponseWriter, r *http.Request) {
	var req ackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}
	// Без отпечатка подтверждение погасило бы и будущие поломки этого идентификатора.
	if req.EDOID == "" || req.Fingerprint == "" {
		writeError(w, http.StatusBadRequest, "bad_request",
			"Нужны идентификатор и отпечаток состояния.")
		return
	}

	if err := h.registry.Acknowledge(
		r.Context(), req.EDOID, req.Fingerprint, req.Reason, sessionLogin(r.Context()), time.Now().UTC(),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить пометку.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Identifiers отдаёт накопленный реестр.
func (h *Registry) Identifiers(w http.ResponseWriter, r *http.Request) {
	records, err := h.registry.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр.")
		return
	}

	traffic, err := h.trafficByID(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать отчёт трафика.")
		return
	}

	list := make([]identifierItem, 0, len(records))
	for _, record := range records {
		list = append(list, identifierFrom(record, traffic))
	}
	writeJSON(w, http.StatusOK, map[string]any{"identifiers": list})
}

type identifierItem struct {
	EDOID      string `json:"edoId"`
	ClientName string `json:"clientName"`
	INN        string `json:"inn"`
	KPP        string `json:"kpp"`
	Login      string `json:"login"`
	Owner      string `json:"owner"`
	OwnerCode  string `json:"ownerCode"`
	Tariffs    string `json:"tariffs"`
	Limit      *int64 `json:"limit"`
	Packets    int64  `json:"packets"`
	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
	Period     string `json:"period"`
	// Traffic — оператор, даты связи и документооборот из отчёта трафика.
	Traffic *service.IdentifierTraffic `json:"traffic"`
}

func identifierFrom(r store.IdentifierRecord, traffic map[string]service.IdentifierTraffic) identifierItem {
	var t *service.IdentifierTraffic
	if found, ok := traffic[r.EDOID]; ok {
		t = &found
	}
	return identifierItem{
		Traffic: t,
		EDOID:   r.EDOID, ClientName: r.ClientName, INN: r.INN, KPP: r.KPP,
		Login: r.Login, Owner: r.OwnerRaw, OwnerCode: r.OwnerCode,
		Tariffs: r.ITSTariffs, Limit: r.Limit, Packets: r.Packets,
		FirstSeen: r.FirstSeenAt.Format(time.RFC3339),
		LastSeen:  r.LastSeenAt.Format(time.RFC3339),
		Period:    r.LastPeriod,
	}
}

// trafficByID сводит сохранённые отчёты трафика (год, затем текущий месяц).
func (h *Registry) trafficByID(ctx context.Context) (map[string]service.IdentifierTraffic, error) {
	var reports []service.TrafficReport
	for _, source := range []EPDUsageReader{h.epdUsage, h.monthTraffic} {
		if source == nil {
			continue
		}
		run, ok, err := source.Run(ctx)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		rows, err := source.Rows(ctx)
		if err != nil {
			return nil, err
		}
		reports = append(reports, service.TrafficReport{PeriodFrom: run.PeriodFrom, PeriodTo: run.PeriodTo, Rows: rows})
	}
	return service.TrafficByID(reports...), nil
}

// Billing отдаёт расчёт по последнему снимку указанного периода.
func (h *Registry) Billing(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = previousPeriod(time.Now())
	}

	snapshot, rows, err := h.snapshots.Latest(r.Context(), period)
	if errors.Is(err, store.ErrNoSnapshot) {
		writeJSON(w, http.StatusOK, map[string]any{
			"period":  period,
			"clients": []any{},
			"empty":   true,
		})
		return
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать снимок.")
		return
	}

	report := service.BuildBilling(rows)
	writeJSON(w, http.StatusOK, map[string]any{
		"period":      period,
		"takenAt":     snapshot.TakenAt.Format(time.RFC3339),
		"clients":     report.Clients,
		"needsAction": report.NeedsAction,
		"totalDue":    report.TotalText,
	})
}

// previousPeriod возвращает предыдущий месяц: биллинг существует только для закрытых.
func previousPeriod(now time.Time) string {
	year, month, _ := now.UTC().Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01")
}
