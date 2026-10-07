package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"partnerops/internal/mailer"
	"partnerops/internal/settings"
)

// emailLike — та же грубая проверка адреса, что и в заявках: полноценный
// разбор RFC 5322 здесь ничего не добавит.
var emailLike = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// smtpPorts — порты, которые умеет отправщик: 465 сразу по TLS, 587 через STARTTLS.
var smtpPorts = map[int]bool{465: true, 587: true}

// errMailNotConfigured означает, что письмо отправлять нечем.
var errMailNotConfigured = errors.New("httpapi: почта не настроена")

// Settings обслуживает экран настроек почты.
type Settings struct {
	store *settings.Store
}

// NewSettings создаёт обработчики.
func NewSettings(store *settings.Store) *Settings {
	return &Settings{store: store}
}

// settingsBody — тело запроса и ответа. Пароль уходит только в одну сторону.
type settingsBody struct {
	SMTPHost        string   `json:"smtpHost"`
	SMTPPort        int      `json:"smtpPort"`
	SMTPLogin       string   `json:"smtpLogin"`
	SMTPFrom        string   `json:"smtpFrom"`
	MailTo          []string `json:"mailTo"`
	SMTPPasswordSet bool     `json:"smtpPasswordSet"`
	// SMTPPassword разбирается вручную: нужно различать «ключа нет» (оставить
	// пароль), null (удалить) и строку (заменить).
	SMTPPassword json.RawMessage `json:"smtpPassword,omitempty"`
}

// Get отдаёт настройки без пароля.
func (h *Settings) Get(w http.ResponseWriter, r *http.Request) {
	current, err := h.store.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать настройки.")
		return
	}
	writeSettings(w, current)
}

// Put сохраняет настройки.
func (h *Settings) Put(w http.ResponseWriter, r *http.Request) {
	var body settingsBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}

	password, ok := decodePassword(w, body.SMTPPassword)
	if !ok {
		return
	}
	if message := validateSettings(body); message != "" {
		writeError(w, http.StatusBadRequest, "bad_request", message)
		return
	}

	next := settings.Settings{
		SMTPHost: body.SMTPHost, SMTPPort: body.SMTPPort, SMTPLogin: body.SMTPLogin,
		SMTPFrom: body.SMTPFrom, MailTo: body.MailTo,
	}
	if err := h.store.Save(r.Context(), next, password, time.Now()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить настройки.")
		return
	}

	current, err := h.store.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Настройки сохранены, но не читаются.")
		return
	}
	writeSettings(w, current)
}

// TestSMTP отправляет короткое письмо по текущим настройкам.
// Текст ошибки уходит клиенту как есть: ради него кнопку и нажимают.
func (h *Settings) TestSMTP(w http.ResponseWriter, r *http.Request) {
	cfg, err := mailConfig(r.Context(), h.store)
	if err != nil {
		writeMailConfigError(w, err)
		return
	}

	err = mailer.Send(r.Context(), cfg,
		"Проверка настроек SMTP",
		"Это тестовое письмо от сервиса заявок ЭПД. Настройки почты работают.",
		"", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "smtp_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// senderBody — отправитель заявки ИТС. Пароль ходит в обе стороны: форма
// заявки подставляет его сама, иначе его пришлось бы набирать каждый раз.
type senderBody struct {
	PartnerCode string `json:"partnerCode"`
	Responsible string `json:"responsible"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

// GetSender отдаёт отправителя заявки для подстановки в форму.
func (h *Settings) GetSender(w http.ResponseWriter, r *http.Request) {
	sender, err := h.store.Sender(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать отправителя заявки.")
		return
	}
	writeJSON(w, http.StatusOK, senderBody(sender))
}

// PutSender сохраняет отправителя заявки.
func (h *Settings) PutSender(w http.ResponseWriter, r *http.Request) {
	var body senderBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}
	body.PartnerCode = strings.TrimSpace(body.PartnerCode)
	body.Responsible = strings.TrimSpace(body.Responsible)
	body.Email = strings.TrimSpace(body.Email)
	if body.Email != "" && !emailLike.MatchString(body.Email) {
		writeError(w, http.StatusBadRequest, "bad_request", "E-mail для протокола должен быть адресом почты.")
		return
	}
	if err := h.store.SaveSender(r.Context(), settings.Sender(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить отправителя заявки.")
		return
	}
	h.GetSender(w, r)
}

// decodePassword переводит поле smtpPassword в «оставить / удалить / заменить».
func decodePassword(w http.ResponseWriter, raw json.RawMessage) (*string, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		empty := ""
		return &empty, true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Пароль должен быть строкой или null.")
		return nil, false
	}
	if value == "" {
		writeError(w, http.StatusBadRequest, "bad_request",
			"Пустой пароль не сохраняем: пришлите null, чтобы удалить сохранённый.")
		return nil, false
	}
	return &value, true
}

// validateSettings возвращает сообщение об ошибке или пустую строку.
func validateSettings(body settingsBody) string {
	if !smtpPorts[body.SMTPPort] {
		return "Порт SMTP должен быть 465 или 587."
	}
	if body.SMTPLogin != "" && !emailLike.MatchString(body.SMTPLogin) {
		return "Логин SMTP должен быть адресом почты."
	}
	if body.SMTPFrom != "" && !emailLike.MatchString(body.SMTPFrom) {
		return "Адрес отправителя должен быть адресом почты."
	}
	for _, address := range body.MailTo {
		if !emailLike.MatchString(address) {
			return "В списке получателей есть строка, не похожая на адрес почты: " + address
		}
	}
	if body.SMTPLogin != "" && len(body.MailTo) == 0 {
		return "Укажите хотя бы одного получателя."
	}
	return ""
}

func writeSettings(w http.ResponseWriter, current settings.Settings) {
	writeJSON(w, http.StatusOK, settingsBody{
		SMTPHost: current.SMTPHost, SMTPPort: current.SMTPPort, SMTPLogin: current.SMTPLogin,
		SMTPFrom: current.SMTPFrom, MailTo: current.MailTo, SMTPPasswordSet: current.PasswordSet,
	})
}

// mailConfig собирает настройки отправки и проверяет, что их хватает.
func mailConfig(ctx context.Context, store *settings.Store) (mailer.Config, error) {
	current, err := store.Get(ctx)
	if err != nil {
		return mailer.Config{}, err
	}
	password, err := store.Password(ctx)
	if err != nil {
		return mailer.Config{}, err
	}
	if strings.TrimSpace(current.SMTPHost) == "" {
		return mailer.Config{}, mailer.ErrNoServer
	}
	if current.SMTPLogin == "" || current.SMTPFrom == "" ||
		password == "" || len(current.MailTo) == 0 {
		return mailer.Config{}, errMailNotConfigured
	}
	return mailer.Config{
		Host: current.SMTPHost, Port: current.SMTPPort, Login: current.SMTPLogin,
		Password: password, From: current.SMTPFrom, To: current.MailTo,
	}, nil
}

func writeMailConfigError(w http.ResponseWriter, err error) {
	if errors.Is(err, mailer.ErrNoServer) {
		writeError(w, http.StatusBadRequest, "not_configured",
			"Почтовый сервер не настроен: укажите SMTP-хост в «Настройках».")
		return
	}
	if errors.Is(err, errMailNotConfigured) {
		writeError(w, http.StatusBadRequest, "not_configured",
			"Почта не настроена: заполните сервер, логин, пароль, адрес отправителя и получателей.")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать настройки почты.")
}
