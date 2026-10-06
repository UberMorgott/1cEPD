package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// BackfillReader — попытки догрузки истории биллинга.
type BackfillReader interface {
	Attempts(ctx context.Context) (map[string]store.BackfillAttempt, error)
}

// WithHistory включает помесячную динамику биллинга.
func (h *Registry) WithHistory(attempts BackfillReader) *Registry {
	h.backfill = attempts
	return h
}

// BillingHistory отдаёт биллинг за 12 закрытых месяцев: итоги и расход по идентификаторам.
func (h *Registry) BillingHistory(w http.ResponseWriter, r *http.Request) {
	if h.backfill == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "История биллинга не настроена.")
		return
	}
	history, err := h.billingHistory(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать историю биллинга.")
		return
	}
	writeJSON(w, http.StatusOK, history)
}

// billingHistory сводит снимки 12 закрытых месяцев. Без WithHistory месяцы без
// снимка идут как «догружается»: попыток догрузки взять неоткуда.
func (h *Registry) billingHistory(ctx context.Context) (service.BillingHistory, error) {
	attempts := map[string]store.BackfillAttempt{}
	if h.backfill != nil {
		found, err := h.backfill.Attempts(ctx)
		if err != nil {
			return service.BillingHistory{}, err
		}
		attempts = found
	}

	months := service.ClosedMonths(time.Now())
	periods := make([]string, len(months))
	rows := make(map[string][]partner.EDOBillingRow, len(months))
	for i, month := range months {
		periods[i] = month.Format("2006-01")
		if h.snapshots == nil {
			continue
		}
		_, monthRows, err := h.snapshots.Latest(ctx, periods[i])
		if errors.Is(err, store.ErrNoSnapshot) {
			continue
		}
		if err != nil {
			return service.BillingHistory{}, err
		}
		rows[periods[i]] = monthRows
	}
	return service.BuildBillingHistory(periods, rows, attempts), nil
}
