package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// defaultExpiryDays — окно напоминаний о продлении по умолчанию.
const defaultExpiryDays = 30

// maxExpiryDays — больше года вперёд напоминать бессмысленно.
const maxExpiryDays = 366

// moscow — договоры кончаются в московскую полночь; даты показываем по Москве
// (docs/API.md §12, грабля 8). Фиксированный сдвиг: tzdata на Windows нет.
var moscow = time.FixedZone("MSK", 3*60*60)

// ITSChecksStore — сохранённые проверки договоров 1С:ИТС.
type ITSChecksStore interface {
	All(ctx context.Context) ([]store.ITSCheck, error)
}

// ITSRefresher обновляет проверки из 1С; *service.ITSRefresher ему удовлетворяет.
type ITSRefresher interface {
	Refresh(ctx context.Context, maxAge time.Duration) (bool, error)
}

// WithITS включает сроки договоров 1С:ИТС. refresher nil — кнопка проверки недоступна.
func (h *Registry) WithITS(checks ITSChecksStore, refresher ITSRefresher) *Registry {
	h.itsChecks, h.itsRefresher = checks, refresher
	return h
}

type itsContractItem struct {
	Name        string `json:"name"`
	NameForUser string `json:"nameForUser"`
	TypeNumber  *int   `json:"typeNumber"`
	Description string `json:"description"`
	Start       string `json:"start"`
	End         string `json:"end"`
}

type itsClientItem struct {
	INN        string `json:"inn"`
	KPP        string `json:"kpp"`
	ClientName string `json:"clientName"`
	Tariffs    string `json:"tariffs"`
}

type itsSubscriberItem struct {
	Code        string            `json:"code"`
	StatusCode  int               `json:"statusCode"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Contracts   []itsContractItem `json:"contracts"`
	Clients     []itsClientItem   `json:"clients"`
	CheckedAt   string            `json:"checkedAt"`
	// Industry — проверка ИТС Отраслевого; nil, если её ещё не было.
	Industry *industryItem `json:"industry,omitempty"`
}

// IndustryChecksStore — сохранённые проверки ИТС Отраслевого.
type IndustryChecksStore interface {
	All(ctx context.Context) ([]store.IndustryCheck, error)
}

// WithIndustry включает сведения об ИТС Отраслевом в ответ о договорах.
func (h *Registry) WithIndustry(checks IndustryChecksStore) *Registry {
	h.industryChecks = checks
	return h
}

type industrySubscriptionItem struct {
	Name  string `json:"name"`
	Begin string `json:"begin"`
	End   string `json:"end"`
}

type industryProgramItem struct {
	Name          string                     `json:"name"`
	Subscriptions []industrySubscriptionItem `json:"subscriptions"`
}

type industryItem struct {
	StatusCode  int                   `json:"statusCode"`
	Description string                `json:"description"`
	Programs    []industryProgramItem `json:"programs"`
	// Missing — конфигурации, которым нужен ИТС Отраслевой, а он не оформлен.
	Missing []string `json:"missing"`
}

type industryMissingItem struct {
	SubscriberCode string          `json:"subscriberCode"`
	Programs       []string        `json:"programs"`
	Clients        []itsClientItem `json:"clients"`
}

func industryFrom(check store.IndustryCheck) *industryItem {
	item := &industryItem{StatusCode: check.Code, Description: check.Description,
		Programs: make([]industryProgramItem, 0, len(check.Programs)), Missing: []string{}}
	for _, p := range check.Programs {
		program := industryProgramItem{Name: p.Name,
			Subscriptions: make([]industrySubscriptionItem, 0, len(p.Subscriptions))}
		for _, s := range p.Subscriptions {
			sub := industrySubscriptionItem{Name: firstNonEmpty(s.NomenclatureName, s.TypeDescription)}
			if !s.Begin.IsZero() {
				sub.Begin = s.Begin.Format(time.RFC3339)
			}
			if !s.End.IsZero() {
				sub.End = s.End.Format(time.RFC3339)
			}
			program.Subscriptions = append(program.Subscriptions, sub)
		}
		item.Programs = append(item.Programs, program)
	}
	for _, p := range check.MissingIndustry() {
		item.Missing = append(item.Missing, p.Name)
	}
	return item
}

// renewalDraft — чем предзаполнить заявку на продление.
type renewalDraft struct {
	// StartDate — день после окончания договора, ДД.ММ.ГГ, как в заявке на ЭПД.
	StartDate string `json:"startDate"`
	// TariffCode — код тарифа ЭПД, если он известен: вид договора совпал с кодом
	// или тариф ЭПД виден у клиента в биллинге. Пусто — выбирает человек.
	TariffCode string `json:"tariffCode"`
}

type itsExpiringItem struct {
	SubscriberCode string          `json:"subscriberCode"`
	Contract       itsContractItem `json:"contract"`
	DaysLeft       int             `json:"daysLeft"`
	Clients        []itsClientItem `json:"clients"`
	Renewal        renewalDraft    `json:"renewal"`
}

// ITSContracts отдаёт договоры 1С:ИТС клиентов и тех, чей договор кончается
// в ближайшие days суток (по умолчанию 30).
func (h *Registry) ITSContracts(w http.ResponseWriter, r *http.Request) {
	if h.itsChecks == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Проверка договоров 1С:ИТС не настроена.")
		return
	}
	days := defaultExpiryDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxExpiryDays {
			writeError(w, http.StatusBadRequest, "bad_request", "Окно напоминаний — от 1 до 366 дней.")
			return
		}
		days = value
	}

	checks, err := h.itsChecks.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать договоры 1С:ИТС.")
		return
	}
	records, err := h.registry.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр.")
		return
	}
	clients := clientsByOwner(records)

	industry := map[string]*industryItem{}
	industryMissing := []industryMissingItem{}
	if h.industryChecks != nil {
		found, err := h.industryChecks.All(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать проверки ИТС Отраслевого.")
			return
		}
		for _, check := range found {
			item := industryFrom(check)
			industry[check.SubscriberCode] = item
			if len(item.Missing) > 0 {
				industryMissing = append(industryMissing, industryMissingItem{
					SubscriberCode: check.SubscriberCode, Programs: item.Missing,
					Clients: nonNilClients(clients[check.SubscriberCode]),
				})
			}
		}
	}

	var checkedAt time.Time
	subscribers := make([]itsSubscriberItem, 0, len(checks))
	for _, check := range checks {
		if check.CheckedAt.After(checkedAt) {
			checkedAt = check.CheckedAt
		}
		item := itsSubscriberItem{
			Code: check.SubscriberCode, StatusCode: check.Code, Status: check.Status,
			Description: check.Description, Contracts: make([]itsContractItem, 0, len(check.Contracts)),
			Clients: nonNilClients(clients[check.SubscriberCode]), CheckedAt: check.CheckedAt.Format(time.RFC3339),
			Industry: industry[check.SubscriberCode],
		}
		for _, contract := range check.Contracts {
			item.Contracts = append(item.Contracts, contractItem(contract))
		}
		subscribers = append(subscribers, item)
	}

	found := service.ExpiringContracts(checks, time.Now().UTC(), days)
	expiring := make([]itsExpiringItem, 0, len(found))
	for _, e := range found {
		owned := nonNilClients(clients[e.SubscriberCode])
		expiring = append(expiring, itsExpiringItem{
			SubscriberCode: e.SubscriberCode, Contract: contractItem(e.Contract),
			DaysLeft: e.DaysLeft, Clients: owned, Renewal: renewal(e.Contract, owned),
		})
	}

	response := map[string]any{"days": days, "subscribers": subscribers, "expiring": expiring,
		"industryMissing": industryMissing}
	if !checkedAt.IsZero() {
		response["checkedAt"] = checkedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, response)
}

// RefreshITSContracts проверяет договоры в 1С сейчас. Проверку моложе
// service.ITSManualInterval не повторяет — отвечает «уже свежие».
func (h *Registry) RefreshITSContracts(w http.ResponseWriter, r *http.Request) {
	if h.itsRefresher == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Проверка договоров 1С:ИТС не настроена.")
		return
	}
	called, err := h.itsRefresher.Refresh(r.Context(), service.ITSManualInterval)
	if err != nil {
		slog.Warn("договоры 1С:ИТС не проверены", "err", err)
		message := "1С не ответила на проверку договоров. Попробуйте позже."
		if partner.IsUnauthorized(err) {
			message = "1С не приняла логин и пароль партнёрского API."
		}
		writeError(w, http.StatusBadGateway, "portal_failed", message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "checked": called})
}

// clientsByOwner группирует организации реестра по коду владельца.
// Одна организация (ИНН/КПП) с несколькими идентификаторами идёт один раз.
func clientsByOwner(records []store.IdentifierRecord) map[string][]itsClientItem {
	result := map[string][]itsClientItem{}
	seen := map[string]bool{}
	for _, record := range records {
		code := strings.TrimSpace(record.OwnerCode)
		if code == "" {
			continue
		}
		key := code + "|" + record.INN + "/" + record.KPP
		if seen[key] {
			continue
		}
		seen[key] = true
		result[code] = append(result[code], itsClientItem{
			INN: record.INN, KPP: record.KPP, ClientName: record.ClientName, Tariffs: record.ITSTariffs,
		})
	}
	for code := range result {
		sort.SliceStable(result[code], func(i, j int) bool {
			return result[code][i].ClientName < result[code][j].ClientName
		})
	}
	return result
}

func nonNilClients(list []itsClientItem) []itsClientItem {
	if list == nil {
		return []itsClientItem{}
	}
	return list
}

func contractItem(c partner.ITSContract) itsContractItem {
	item := itsContractItem{
		Name: c.TypeName, NameForUser: c.TypeNameForUser, TypeNumber: c.TypeNumber,
		Description: c.Description,
	}
	if !c.Start.IsZero() {
		item.Start = c.Start.Format(time.RFC3339)
	}
	if !c.End.IsZero() {
		item.End = c.End.Format(time.RFC3339)
	}
	return item
}

// renewal собирает предзаполнение заявки на продление: начало — следующие
// московские сутки после окончания договора.
func renewal(c partner.ITSContract, clients []itsClientItem) renewalDraft {
	draft := renewalDraft{StartDate: c.End.Add(time.Second).In(moscow).Format("02.01.06")}
	if c.TypeNumber != nil {
		if tariff, ok := itsreq.TariffByCode(strconv.Itoa(*c.TypeNumber)); ok {
			draft.TariffCode = tariff.Code
			return draft
		}
	}
	for _, client := range clients {
		if held := itsreq.EPDTariffsInText(client.Tariffs); len(held) > 0 {
			draft.TariffCode = held[0].Tariff.Code
			break
		}
	}
	return draft
}
