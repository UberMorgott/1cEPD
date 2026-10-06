package httpapi

import (
	"context"
	"net/http"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// EPDUsageReader — сохранённый расход ЭПД за 12 месяцев.
type EPDUsageReader interface {
	Run(ctx context.Context) (store.EPDUsageRun, bool, error)
	Rows(ctx context.Context) ([]partner.EDOTrafficRow, error)
}

// WithEPDUsage включает подсказку выгодного тарифа ЭПД.
func (h *Registry) WithEPDUsage(usage EPDUsageReader) *Registry {
	h.epdUsage = usage
	return h
}

type epdTariffItem struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Volume    int    `json:"volume"`
	FreshCode string `json:"freshCode"`
	Count     int    `json:"count,omitempty"`
}

type epdAdviceItem struct {
	INN        string          `json:"inn"`
	KPP        string          `json:"kpp"`
	ClientName string          `json:"clientName"`
	Subscriber string          `json:"subscriber"`
	Fresh      bool            `json:"fresh"`
	EPDOut     int64           `json:"epdOut"`
	EPDIn      int64           `json:"epdIn"`
	Current    []epdTariffItem `json:"current"`
	// Стоимости — розница за год, строкой «1 400,00».
	CurrentCost  string `json:"currentCost"`
	PerPieceCost string `json:"perPieceCost"`
	// Best — выгоднейший тариф; null — выгоднее поштучно.
	Best       *epdTariffItem `json:"best"`
	BestCost   string         `json:"bestCost"`
	Savings    string         `json:"savings"`
	Individual bool           `json:"individual"`
	Optimal    bool           `json:"optimal"`
}

// EPDAdvice отдаёт подсказки тарифа ЭПД по годовому расходу клиентов.
func (h *Registry) EPDAdvice(w http.ResponseWriter, r *http.Request) {
	if h.epdUsage == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Расход ЭПД не настроен.")
		return
	}
	run, ok, err := h.epdUsage.Run(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать расход ЭПД.")
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"clients": []any{}, "empty": true})
		return
	}
	rows, err := h.epdUsage.Rows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать расход ЭПД.")
		return
	}
	records, err := h.registry.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр.")
		return
	}

	advice := service.BuildEPDAdvice(rows, records)
	list := make([]epdAdviceItem, 0, len(advice))
	for _, a := range advice {
		list = append(list, epdAdviceFrom(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"periodFrom": run.PeriodFrom, "periodTo": run.PeriodTo,
		"fetchedAt": run.FetchedAt.Format(time.RFC3339), "clients": list,
	})
}

func epdAdviceFrom(a service.EPDAdvice) epdAdviceItem {
	item := epdAdviceItem{
		INN: a.INN, KPP: a.KPP, ClientName: a.ClientName, Subscriber: a.Subscriber,
		Fresh: a.Fresh, EPDOut: a.EPDOut, EPDIn: a.EPDIn,
		Current:      make([]epdTariffItem, 0, len(a.Current)),
		CurrentCost:  formatKopeks(a.CurrentCostKopeks),
		PerPieceCost: formatKopeks(a.PerPieceCostKopeks),
		BestCost:     formatKopeks(a.BestCostKopeks),
		Savings:      formatKopeks(max(0, a.SavingsKopeks)),
		Individual:   a.Individual, Optimal: a.AlreadyOptimal,
	}
	for _, held := range a.Current {
		t := tariffItem(held.Tariff)
		t.Count = held.Count
		item.Current = append(item.Current, t)
	}
	if a.BestOK {
		best := tariffItem(a.Best)
		item.Best = &best
	}
	return item
}

func tariffItem(t itsreq.Tariff) epdTariffItem {
	return epdTariffItem{Code: t.Code, Name: t.Name, Volume: t.Volume, FreshCode: t.FreshCode}
}
