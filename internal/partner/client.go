package partner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL — адрес партнёрского API 1С.
const DefaultBaseURL = "https://partner-api.1c.ru/api"

// Client обращается к партнёрскому API. Учётные данные не покидают этот тип.
type Client struct {
	baseURL  string
	login    string
	password string
	http     *http.Client
	// pollInterval — пауза между опросами готовности отчёта. Тесты уменьшают её.
	pollInterval time.Duration
	// maxResponseBytes — потолок размера ответа. Отчёты в разы меньше, а вот
	// бесконечный поток от сервера съел бы всю память. Тесты уменьшают лимит.
	maxResponseBytes int64
}

// New создаёт клиента. Пустой baseURL означает боевой адрес.
func New(baseURL, login, password string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		login:    login,
		password: password,
		http: &http.Client{
			Timeout: 60 * time.Second,
			// Редиректы запрещены: на 301/302 POST выродится в GET, а заголовок
			// Authorization уйдёт на посторонний хост.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return fmt.Errorf("редирект на %s запрещён", req.URL.Redacted())
			},
		},
		pollInterval:     5 * time.Second,
		maxResponseBytes: 64 << 20,
	}
}

// APIError — ошибка, вернувшаяся от 1С.
type APIError struct {
	StatusCode int
	Code       string // прикладной код, например MAX_TASKS_PER_HOUR_LIMIT_REACHED
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("1С вернула %d: %s (%s)", e.StatusCode, e.Message, e.Code)
	}
	return fmt.Sprintf("1С вернула %d: %s", e.StatusCode, e.Message)
}

// Коды, означающие, что задача не провалилась, а упёрлась в лимит и её надо повторить позже.
var rateLimitCodes = map[string]bool{
	"MAX_TASKS_PER_HOUR_LIMIT_REACHED":    true,
	"MAX_NOT_HANDLED_TASKS_COUNT_REACHED": true,
}

// IsRateLimited сообщает, что 1С отказала из-за лимита построения отчётов.
func IsRateLimited(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return rateLimitCodes[apiErr.Code]
}

// IsNoBilling сообщает, что биллинга за запрошенный месяц у 1С нет
// (BILLING_DOES_NOT_EXIST). Ошибка предметная: повтор не поможет. Код приходит
// либо кодом ответа 400, либо текстом в поле error постановки задачи.
func IsNoBilling(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == "BILLING_DOES_NOT_EXIST" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "BILLING_DOES_NOT_EXIST")
}

// IsUnauthorized сообщает, что учётные данные не приняты.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}

// IsLoginNotFound сообщает, что логина нет на Портале ИТС: поиск программ по
// логину отвечает на это 404 со статусом LoginNotFound, а не пустым списком.
func IsLoginNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound &&
		strings.Contains(apiErr.Message, "LoginNotFound")
}

// response — сырой ответ, тело может быть как JSON, так и CSV.
type response struct {
	StatusCode int
	Body       []byte
}

func (c *Client) post(ctx context.Context, path string, body []byte) (*response, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

func (c *Client) get(ctx context.Context, path string) (*response, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (*response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать запрос: %w", err)
	}
	req.SetBasicAuth(c.login, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("partner: запрос %s %s не выполнен: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем на байт больше лимита: так видно, что поток не кончился.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("partner: не прочитать ответ: %w", err)
	}
	if int64(len(raw)) > c.maxResponseBytes {
		return nil, fmt.Errorf("partner: ответ на %s %s больше %d байт", method, path, c.maxResponseBytes)
	}

	if resp.StatusCode >= 400 {
		return nil, apiErrorFrom(resp.StatusCode, raw)
	}
	return &response{StatusCode: resp.StatusCode, Body: raw}, nil
}

// apiErrorFrom вытаскивает прикладной код из тела, если он там есть.
func apiErrorFrom(status int, raw []byte) error {
	apiErr := &APIError{StatusCode: status, Message: strings.TrimSpace(string(raw))}

	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if payload.Message != "" {
			apiErr.Message = payload.Message
		}
		// Поле error у Spring содержит текст статуса — как "Bad Request", так и
		// "BAD_REQUEST". Отсекаем его, а кодом считаем оставшийся верхний регистр.
		if payload.Error != "" &&
			!sameAsStatusText(payload.Error, status) &&
			strings.ToUpper(payload.Error) == payload.Error {
			apiErr.Code = payload.Error
		}
	}
	return apiErr
}

// statusTextNoise убирает разделители, которыми отличаются "Bad Request",
// "BAD_REQUEST" и "BadRequest".
var statusTextNoise = strings.NewReplacer(" ", "", "_", "", "-", "")

// sameAsStatusText сообщает, что значение — это просто текст HTTP-статуса.
func sameAsStatusText(value string, status int) bool {
	normalize := func(s string) string {
		return strings.ToUpper(statusTextNoise.Replace(s))
	}
	return normalize(value) == normalize(http.StatusText(status))
}
