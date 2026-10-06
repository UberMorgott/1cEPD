package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// EPDImportReader — загруженная выгрузка «Детализация биллинга» ЭПД.
type EPDImportReader interface {
	Rows(ctx context.Context) ([]partner.EPDBillingRow, error)
}

// EPDImportStore — хранение выгрузки; *store.EPDBillingImport ей удовлетворяет.
type EPDImportStore interface {
	EPDImportReader
	Replace(ctx context.Context, fileName string, rows []partner.EPDBillingRow, at time.Time) error
	Last(ctx context.Context) (store.EPDImportInfo, bool, error)
}

// WithEPDImport включает загрузку выгрузки биллинга ЭПД: в партнёрском API
// отчёта биллинга ЭПД нет, и клиентов, у которых только ЭПД, реестр без неё
// не видит (docs/API.md §8.4).
func (h *Registry) WithEPDImport(imports EPDImportStore) *Registry {
	h.epdImport = imports
	return h
}

// WithEPDImport дополняет подбор клиента в заявке клиентами из выгрузки
// биллинга ЭПД. Поля выгрузки идут последними: последняя заявка, затем API.
func (h *Requests) WithEPDImport(imports EPDImportReader) *Requests {
	h.epdImport = imports
	return h
}

// maxEPDImportSize — выгрузка — десятки килобайт; мегабайты — не она.
const maxEPDImportSize = 8 << 20

func epdImportJSON(info store.EPDImportInfo) map[string]any {
	return map[string]any{"importedAt": info.ImportedAt.Format(time.RFC3339),
		"fileName": info.FileName, "rows": info.Total}
}

// ImportEPDBilling принимает выгрузку «Детализация биллинга» ЭПД (multipart,
// поле file) и заменяет ею прежнюю. Отвечает, сколько организаций выгрузки
// реестр клиентов видит впервые (added), а сколько уже знал (updated).
func (h *Registry) ImportEPDBilling(w http.ResponseWriter, r *http.Request) {
	if h.epdImport == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Загрузка выгрузки биллинга не настроена.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxEPDImportSize+64<<10)
	file, header, err := r.FormFile("file")
	if err != nil {
		if _, tooBig := errors.AsType[*http.MaxBytesError](err); tooBig {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "Файл слишком большой для выгрузки биллинга.")
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Приложите CSV-файл выгрузки биллинга.")
		return
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, maxEPDImportSize+1))
	if err != nil || len(raw) > maxEPDImportSize {
		writeError(w, http.StatusBadRequest, "bad_request", "Не удалось прочитать файл.")
		return
	}
	rows, err := partner.ParseEPDBilling(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_csv", "Файл не похож на выгрузку «Детализация биллинга» ЭПД: "+err.Error())
		return
	}

	ctx := r.Context()
	before, err := h.loadClientData(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать данные клиентов.")
		return
	}
	added, updated := 0, 0
	seen := map[string]bool{}
	for _, row := range rows {
		key := clientKey(row.INN, row.KPP)
		if key == "" {
			key = "edo-" + row.EDOID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if before.index.forRow(row.EDOID, row.INN, row.KPP) != nil {
			updated++
		} else {
			added++
		}
	}

	at := time.Now().UTC()
	name := filepath.Base(header.Filename)
	if err := h.epdImport.Replace(ctx, name, rows, at); err != nil {
		slog.Error("выгрузка биллинга ЭПД не сохранена", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить выгрузку.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": added, "updated": updated,
		"epdImport": epdImportJSON(store.EPDImportInfo{ImportedAt: at, FileName: name, Total: len(rows)})})
}
