package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"partnerops/internal/settings"
	"partnerops/internal/updater"
)

// Update обслуживает версию в шапке и настройки самообновления.
type Update struct {
	updater *updater.Updater
}

// NewUpdate создаёт обработчики.
func NewUpdate(u *updater.Updater) *Update {
	return &Update{updater: u}
}

// Status отдаёт состояние обновления.
func (h *Update) Status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.updater.Status(r.Context()))
}

// Check проверяет GitHub сейчас.
func (h *Update) Check(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.updater.Check(r.Context()))
}

// Apply ставит новую версию и перезапускает программу. Загрузка идёт в
// контексте программы, а не запроса: закрытая вкладка её не обрывает.
func (h *Update) Apply(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.updater.Install(context.WithoutCancel(r.Context())))
}

// updatePrefsBody — настройки самообновления.
type updatePrefsBody struct {
	AutoCheck   bool `json:"autoCheck"`
	AutoInstall bool `json:"autoInstall"`
}

// PutPrefs сохраняет настройки самообновления и отдаёт состояние.
func (h *Update) PutPrefs(w http.ResponseWriter, r *http.Request) {
	var body updatePrefsBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}
	if err := h.updater.SetPrefs(r.Context(), settings.UpdatePrefs{Check: body.AutoCheck, Install: body.AutoInstall}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить настройки обновления.")
		return
	}
	h.Status(w, r)
}
