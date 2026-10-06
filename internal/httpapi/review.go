package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"partnerops/internal/service"
	"partnerops/internal/store"
)

// AckStore — пометки «это законно» с подробностями и их снятие.
type AckStore interface {
	Acks(ctx context.Context) (map[string]store.Ack, error)
	Unacknowledge(ctx context.Context, edoID, fingerprint string) error
}

// TopologyStore — реестр ожидаемой топологии.
type TopologyStore interface {
	All(ctx context.Context) ([]store.TopologyEntry, error)
	Put(ctx context.Context, entry store.TopologyEntry) error
	Remove(ctx context.Context, inn, kpp, edoID string) error
}

// WithReview включает пересмотр скрытых находок и реестр ожидаемой топологии.
func (h *Registry) WithReview(acks AckStore, topology TopologyStore) *Registry {
	h.reviewAcks, h.topology = acks, topology
	return h
}

// reviewData — пометки и топология (ключ service.TopologyKey → назначение).
// Без WithReview обе пусты.
func (h *Registry) reviewData(ctx context.Context) (map[string]store.Ack, map[string]string, error) {
	acks := map[string]store.Ack{}
	topology := map[string]string{}
	if h.reviewAcks != nil {
		found, err := h.reviewAcks.Acks(ctx)
		if err != nil {
			return nil, nil, err
		}
		acks = found
	}
	if h.topology != nil {
		entries, err := h.topology.All(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range entries {
			topology[service.TopologyKey(e.INN, e.KPP, e.EDOID)] = e.Purpose
		}
	}
	return acks, topology, nil
}

// Unacknowledge снимает пометку «это законно»: находка возвращается в активные.
func (h *Registry) Unacknowledge(w http.ResponseWriter, r *http.Request) {
	if h.reviewAcks == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Пересмотр пометок не настроен.")
		return
	}
	var req ackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil ||
		req.EDOID == "" || req.Fingerprint == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Нужны идентификатор и отпечаток состояния.")
		return
	}
	if err := h.reviewAcks.Unacknowledge(r.Context(), req.EDOID, req.Fingerprint); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось снять пометку.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type topologyItem struct {
	INN       string `json:"inn"`
	KPP       string `json:"kpp"`
	EDOID     string `json:"edoId"`
	Purpose   string `json:"purpose"`
	Author    string `json:"author,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Topology отдаёт реестр ожидаемой топологии.
func (h *Registry) Topology(w http.ResponseWriter, r *http.Request) {
	if h.topology == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Реестр топологии не настроен.")
		return
	}
	entries, err := h.topology.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр топологии.")
		return
	}
	list := make([]topologyItem, 0, len(entries))
	for _, e := range entries {
		list = append(list, topologyItem{INN: e.INN, KPP: e.KPP, EDOID: e.EDOID, Purpose: e.Purpose,
			Author: e.Author, UpdatedAt: e.UpdatedAt.Format(time.RFC3339)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": list})
}

// decodeTopology читает связь; назначение обязательно только при записи.
func decodeTopology(w http.ResponseWriter, r *http.Request, needPurpose bool) (topologyItem, bool) {
	var req topologyItem
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return req, false
	}
	req.INN, req.KPP, req.EDOID = strings.TrimSpace(req.INN), strings.TrimSpace(req.KPP), strings.TrimSpace(req.EDOID)
	req.Purpose = strings.TrimSpace(req.Purpose)
	if req.INN == "" || req.EDOID == "" || (needPurpose && req.Purpose == "") {
		writeError(w, http.StatusBadRequest, "bad_request", "Нужны ИНН, идентификатор и назначение связи.")
		return req, false
	}
	return req, true
}

// PutTopology отмечает связь «организация → идентификатор» законной.
func (h *Registry) PutTopology(w http.ResponseWriter, r *http.Request) {
	if h.topology == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Реестр топологии не настроен.")
		return
	}
	req, ok := decodeTopology(w, r, true)
	if !ok {
		return
	}
	if err := h.topology.Put(r.Context(), store.TopologyEntry{INN: req.INN, KPP: req.KPP, EDOID: req.EDOID,
		Purpose: req.Purpose, Author: sessionLogin(r.Context()), UpdatedAt: time.Now().UTC()}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить связь.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// RemoveTopology убирает связь из реестра: её сигналы снова видны.
func (h *Registry) RemoveTopology(w http.ResponseWriter, r *http.Request) {
	if h.topology == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Реестр топологии не настроен.")
		return
	}
	req, ok := decodeTopology(w, r, false)
	if !ok {
		return
	}
	if err := h.topology.Remove(r.Context(), req.INN, req.KPP, req.EDOID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось удалить связь.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
