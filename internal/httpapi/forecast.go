package httpapi

import (
	"context"
	"net/http"
	"time"

	"partnerops/internal/service"
)

// WithForecast включает прогноз перерасхода по трафику текущего месяца. Прогноз
// живёт в биллинге (/api/billing/forecast); находкой он больше не бывает:
// Находки — только о связях идентификаторов ЭДО.
func (h *Registry) WithForecast(month EPDUsageReader) *Registry {
	h.monthTraffic = month
	return h
}

// monthForecast считает прогноз; ok false — отчёта за текущий месяц ещё нет.
func (h *Registry) monthForecast(ctx context.Context, now time.Time) (service.MonthForecast, bool, error) {
	if h.monthTraffic == nil {
		return service.MonthForecast{}, false, nil
	}
	run, ok, err := h.monthTraffic.Run(ctx)
	if err != nil || !ok || run.PeriodFrom != now.UTC().Format("2006-01") {
		// Отчёт за прошлый месяц текущим не выдаём: прогноз по нему бессмыслен.
		return service.MonthForecast{}, false, err
	}
	rows, err := h.monthTraffic.Rows(ctx)
	if err != nil {
		return service.MonthForecast{}, false, err
	}
	records, err := h.registry.All(ctx)
	if err != nil {
		return service.MonthForecast{}, false, err
	}
	return service.BuildForecast(run, rows, records), true, nil
}

// BillingForecast отдаёт прогноз расхода лимитов к концу текущего месяца.
func (h *Registry) BillingForecast(w http.ResponseWriter, r *http.Request) {
	forecast, ok, err := h.monthForecast(r.Context(), time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось посчитать прогноз.")
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"clients": []any{}, "empty": true})
		return
	}
	writeJSON(w, http.StatusOK, forecast)
}
