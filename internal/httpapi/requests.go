package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/mailer"
	"partnerops/internal/money"
	"partnerops/internal/settings"
	"partnerops/internal/store"
)

// RequestStore — то, что нужно обработчикам от хранилища заявок.
type RequestStore interface {
	Save(ctx context.Context, draft store.RequestDraft, at time.Time) (int64, error)
	Update(ctx context.Context, draft store.RequestDraft, at time.Time) error
	Get(ctx context.Context, id int64) (store.RequestDraft, error)
	List(ctx context.Context) ([]store.RequestDraft, error)
	Payloads(ctx context.Context) ([]string, error)
	MarkExported(ctx context.Context, id int64, at time.Time) error
	MarkSent(ctx context.Context, id int64, at time.Time) error
}

// FileBuilder собирает файл-заявку.
type FileBuilder interface {
	Build(ctx context.Context, request itsreq.Request, outDir string) (string, error)
}

// Requests обслуживает экран заявок.
type Requests struct {
	store   RequestStore
	builder FileBuilder
	// settings нужен только отправке письма; nil означает, что маршрут отправки
	// не работает, а остальной экран заявок работает как раньше.
	settings *settings.Store
	// portal нужен проверкам в 1С; nil означает, что они не выполняются,
	// а заявка при этом остаётся выпускаемой.
	portal itsreq.Portal
	// directory — справочные данные 1С для сверки и предзаполнения; nil — сверки нет.
	directory itsreq.Directory
	// clients и traffic — реестр ЭДО и отчёты трафика: наши клиенты для подбора
	// и сверки заявки; nil — подбора нет, сверка знает только базу абонентов.
	clients ClientSource
	traffic []EPDUsageReader
	// programs и checker — программы клиента по проверке в 1С; nil — не показываются.
	programs ProgramStore
	checker  ProgramChecker
	// subscribers — сохранённая база абонентов 1С: ФИО абонента-владельца
	// подставляется ответственным клиента; nil — не подставляется.
	subscribers SubscriberNames
	// epdImport — загруженная выгрузка биллинга ЭПД: клиенты ЭПД, которых нет
	// в отчётах API; nil — подбор знает только API.
	epdImport EPDImportReader
}

// NewRequests создаёт обработчики.
func NewRequests(store RequestStore, builder FileBuilder) *Requests {
	return &Requests{store: store, builder: builder}
}

// WithMailer включает отправку заявки письмом.
func (h *Requests) WithMailer(store *settings.Store) *Requests {
	h.settings = store
	return h
}

// WithPortal включает проверки заявки в партнёрском API 1С.
func (h *Requests) WithPortal(portal itsreq.Portal) *Requests {
	h.portal = portal
	return h
}

// WithDirectory включает сверку заявки с данными 1С и подсказки для пустых полей.
func (h *Requests) WithDirectory(directory itsreq.Directory) *Requests {
	h.directory = directory
	return h
}

// Tariffs отдаёт справочник тарифов ЭПД для выпадающего списка.
func (h *Requests) Tariffs(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		Volume    int    `json:"volume"`
		Retail    string `json:"retail"`
		Partner   string `json:"partner"`
		FreshCode string `json:"freshCode"`
	}

	list := make([]item, 0, len(itsreq.Tariffs))
	for _, tariff := range itsreq.Tariffs {
		list = append(list, item{
			Code: tariff.Code, Name: tariff.Name, Volume: tariff.Volume,
			Retail:    formatKopeks(tariff.RetailKopeks),
			Partner:   formatKopeks(tariff.PartnerKopeks),
			FreshCode: tariff.FreshCode,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tariffs": list})
}

// Validate проверяет заявку и возвращает замечания: сначала формат, затем —
// если он в порядке — проверки в 1С.
// Отвечает 200 даже при замечаниях: это не ошибка запроса, а результат проверки.
func (h *Requests) Validate(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	result := itsreq.Check(r.Context(), h.portal, h.checkDirectory(), request)
	issues := result.Issues
	if issues == nil {
		issues = []itsreq.Issue{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issues":      issues,
		"blocking":    itsreq.Blocking(issues),
		"unchecked":   itsreq.Unchecked(issues),
		"notes":       result.Notes,
		"suggestions": result.Suggestions,
	})
}

// precheck не даёт собрать файл по заявке, которую робот всё равно отвергнет.
// Возвращает false, если ответ уже отправлен.
func (h *Requests) precheck(w http.ResponseWriter, r *http.Request, request itsreq.Request) bool {
	issues := itsreq.Check(r.Context(), h.portal, h.checkDirectory(), request).Issues
	if !itsreq.Blocking(issues) {
		return true
	}
	writeError(w, http.StatusBadRequest, "precheck_failed", itsreq.FirstBlocking(issues))
	return false
}

// Download собирает файл и отдаёт его на скачивание.
func (h *Requests) Download(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	if !h.precheck(w, r, request) {
		return
	}

	dir, err := os.MkdirTemp("", "itsreq")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось подготовить файл.")
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path, err := h.builder.Build(r.Context(), request, dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, "build_failed", err.Error())
		return
	}

	file, err := os.Open(path) //nolint:gosec // путь вернул наш же сборщик внутри созданного здесь os.MkdirTemp, а не клиент
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Файл собран, но не читается.")
		return
	}
	defer func() { _ = file.Close() }()

	if request.ID != 0 {
		// Файл уже собран: неудачная отметка не повод его не отдать.
		if err := h.store.MarkExported(r.Context(), request.ID, time.Now().UTC()); err != nil {
			slog.Error("не отметить выгрузку заявки", "id", request.ID, "err", err)
		}
	}

	name := itsreq.FileName(request.PartnerCode)
	w.Header().Set("Content-Type", "application/vnd.ms-excel")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, time.Now(), file)
}

// Send собирает тот же файл, что и Download, и отправляет его письмом.
func (h *Requests) Send(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		writeError(w, http.StatusInternalServerError, "internal", "Отправка почты недоступна.")
		return
	}

	request, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	if !h.precheck(w, r, request) {
		return
	}

	cfg, err := mailConfig(r.Context(), h.settings)
	if err != nil {
		writeMailConfigError(w, err)
		return
	}

	dir, err := os.MkdirTemp("", "itsreq")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось подготовить файл.")
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path, err := h.builder.Build(r.Context(), request, dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, "build_failed", err.Error())
		return
	}
	file, err := os.ReadFile(path) //nolint:gosec // путь вернул наш же сборщик внутри созданного здесь os.MkdirTemp, а не клиент
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Файл собран, но не читается.")
		return
	}

	name := itsreq.FileName(request.PartnerCode)
	subject := "Заявка ЭПД, партнёр " + request.PartnerCode
	body := "Во вложении заявка ЭПД на " + strconv.Itoa(len(request.Rows)) + " поз. Файл: " + name +
		".\r\nОтветственный: " + request.Responsible + ", " + request.Email + "."
	if err := mailer.Send(r.Context(), cfg, subject, body, name, file); err != nil {
		writeError(w, http.StatusBadGateway, "smtp_failed", err.Error())
		return
	}

	sentAt := time.Now().UTC()
	if request.ID != 0 {
		// Письмо уже ушло: неудачная отметка не повод отвечать ошибкой.
		if err := h.store.MarkSent(r.Context(), request.ID, sentAt); err != nil {
			slog.Error("не отметить отправку заявки", "id", request.ID, "err", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sentAt": sentAt.Format(time.RFC3339)})
}

// List отдаёт сохранённые заявки.
func (h *Requests) List(w http.ResponseWriter, r *http.Request) {
	drafts, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать заявки.")
		return
	}

	list := make([]requestListItem, 0, len(drafts))
	for _, draft := range drafts {
		entry, _ := requestItemFrom(draft)
		list = append(list, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": list})
}

// requestListItem — строка списка заявок.
type requestListItem struct {
	ID int64 `json:"id"`
	// Number — порядковый номер заявки, CreatedAt — дата её создания.
	Number     int64  `json:"number"`
	CreatedAt  string `json:"createdAt"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Revision   int64  `json:"revision"`
	UpdatedAt  string `json:"updatedAt"`
	ExportedAt string `json:"exportedAt,omitempty"`
	SentAt     string `json:"sentAt,omitempty"`
	// Клиент первой строки: список заявок показывает, на кого она.
	CompanyName string `json:"companyName"`
	INN         string `json:"inn"`
	KPP         string `json:"kpp"`
	Rows        int    `json:"rows"`
}

// requestItemFrom собирает строку списка и отдаёт строки заявки — по ним заявка
// находит своих клиентов. Повреждённый черновик не прячет заявку из списка:
// клиент просто пуст.
func requestItemFrom(draft store.RequestDraft) (requestListItem, []itsreq.Row) {
	entry := requestListItem{
		ID: draft.ID, Number: draft.Number, CreatedAt: draft.CreatedAt.Format(time.RFC3339),
		Title: draft.Title, Status: draft.Status, Revision: draft.Revision, UpdatedAt: draft.UpdatedAt.Format(time.RFC3339),
	}
	var request itsreq.Request
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &request); err == nil {
		entry.Rows = len(request.Rows)
		if len(request.Rows) > 0 {
			first := request.Rows[0]
			entry.CompanyName, entry.INN, entry.KPP = first.CompanyName, first.INN, first.KPP
		}
	}
	if draft.ExportedAt != nil {
		entry.ExportedAt = draft.ExportedAt.Format(time.RFC3339)
	}
	if draft.SentAt != nil {
		entry.SentAt = draft.SentAt.Format(time.RFC3339)
	}
	return entry, request.Rows
}

// Save создаёт или обновляет черновик.
func (h *Requests) Save(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       int64          `json:"id"`
		Title    string         `json:"title"`
		Revision int64          `json:"revision"`
		Request  itsreq.Request `json:"request"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}

	// Пароль подлинности заявки — секрет: в черновик он не попадает,
	// перед выпуском файла его вводят заново.
	saved := body.Request
	saved.Password, saved.NewPassword = "", ""

	payload, err := json.Marshal(saved) //nolint:gosec // G117: пароли из заявки уже стёрты выше
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Не удалось сохранить данные заявки.")
		return
	}

	now := time.Now().UTC()
	draft := store.RequestDraft{
		ID: body.ID, Title: body.Title, Status: "draft", SchemaVer: "3.09",
		Revision: body.Revision, PayloadJSON: string(payload),
	}

	if body.ID == 0 {
		id, err := h.store.Save(r.Context(), draft, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить заявку.")
			return
		}
		var number int64
		if created, err := h.store.Get(r.Context(), id); err == nil {
			number = created.Number
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "revision": 1, "number": number})
		return
	}

	if err := h.store.Update(r.Context(), draft, now); err != nil {
		if errors.Is(err, store.ErrStaleRevision) {
			writeError(w, http.StatusConflict, "stale",
				"Заявку изменили в другом окне. Обновите страницу.")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить заявку.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": body.ID, "revision": body.Revision + 1})
}

// Get отдаёт один черновик.
func (h *Requests) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный идентификатор заявки.")
		return
	}

	draft, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена.")
		return
	}

	var request itsreq.Request
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &request); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Черновик заявки повреждён.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id": draft.ID, "number": draft.Number, "title": draft.Title, "status": draft.Status,
		"revision": draft.Revision, "request": request,
	})
}

func decodeRequest(w http.ResponseWriter, r *http.Request) (itsreq.Request, bool) {
	var request itsreq.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат заявки.")
		return itsreq.Request{}, false
	}
	return request, true
}

// formatKopeks печатает цену так же, как остальные суммы приложения.
// Копейка — десять миллирублей, единицы хранения money.
func formatKopeks(value int64) string {
	return money.Amount(value * 10).String()
}
