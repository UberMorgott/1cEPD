package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"partnerops/internal/store"
)

const sessionCookieName = "session"

const (
	// loginAttempts и loginLockout задают блокировку по адресу клиента: после
	// стольких неудач подряд адрес не пускают до конца окна. То же окно служит
	// сроком давности для счётчика по логину, который вместо блокировки даёт
	// растущую задержку ответа.
	loginAttempts = 5
	loginLockout  = 5 * time.Minute
)

// Auth отвечает за вход, выход и защиту маршрутов.
type Auth struct {
	sessions     *store.Sessions
	login        string
	passwordHash string
	ttl          time.Duration
	limiter      *limiter
	cookieSecure bool
}

// NewAuth создаёт обработчик входа. Пароль хранится только как bcrypt-хеш из конфигурации.
//
// cookieSecure выключается только для локального запуска по http: браузер не сохраняет
// cookie с флагом Secure на незащищённом соединении, и вход выглядит успешным,
// но сессия не прилипает.
func NewAuth(sessions *store.Sessions, login, passwordHash string, ttl time.Duration, cookieSecure bool) *Auth {
	return &Auth{
		sessions:     sessions,
		login:        login,
		passwordHash: passwordHash,
		ttl:          ttl,
		limiter:      newLimiter(loginAttempts, loginLockout),
		cookieSecure: cookieSecure,
	}
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Login проверяет учётные данные и выдаёт cookie сессии.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}

	ip := clientIP(r)
	ipKey, loginKey := "ip:"+ip, "login:"+req.Login
	if a.limiter.blocked(ipKey) {
		slog.Warn("вход заблокирован после серии неудач", "ip", ip)
		writeError(w, http.StatusTooManyRequests, "too_many_attempts",
			"Слишком много попыток входа. Попробуйте через несколько минут.")
		return
	}

	// Логины сравниваются в виде дайджестов: у ConstantTimeCompare на строках
	// разной длины ответ мгновенный, и длина настоящего логина утекала бы.
	// bcrypt считается всегда, даже когда логин уже не подошёл: иначе неизвестный
	// логин отвечал бы заметно быстрее неверного пароля.
	wantLogin := sha256.Sum256([]byte(a.login))
	gotLogin := sha256.Sum256([]byte(req.Login))
	loginOK := subtle.ConstantTimeCompare(gotLogin[:], wantLogin[:]) == 1
	passwordOK := bcrypt.CompareHashAndPassword([]byte(a.passwordHash), []byte(req.Password)) == nil
	if !loginOK || !passwordOK {
		// Неудача считается по обоим ключам, и по логину — независимо от того,
		// существует он или нет. Поэтому задержка зависит только от числа
		// неудач по введённой строке и не подсказывает, угадан ли логин.
		a.limiter.fail(ipKey, loginKey)
		if !a.limiter.delay(r.Context(), loginKey) {
			slog.Warn("вход отклонён: слишком много задержанных попыток", "ip", ip)
			writeError(w, http.StatusTooManyRequests, "too_many_attempts",
				"Слишком много попыток входа. Попробуйте через несколько минут.")
			return
		}
		// Пароль в лог не попадает — ни целиком, ни длиной.
		slog.Warn("неудачная попытка входа", "ip", ip)
		writeError(w, http.StatusUnauthorized, "bad_credentials", "Неверный логин или пароль.")
		return
	}
	a.limiter.reset(ipKey, loginKey)

	// Смена идентификатора сессии: старый, если он был предъявлен, гасится,
	// чтобы навязанный до входа токен не стал после входа рабочим.
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := a.sessions.Delete(r.Context(), cookie.Value); err != nil {
			slog.Error("прежняя сессия не удалена при входе", "err", err)
		}
	}

	token, err := a.sessions.Create(r.Context(), a.ttl, ip, r.UserAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось создать сессию.")
		return
	}

	setSessionCookie(w, token, a.cookieSecure, int(a.ttl.Seconds()))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// setSessionCookie выставляет cookie сессии одинаково при входе и выходе.
//
// SameSite=Strict — она же защита от CSRF: cookie не уезжает ни в один запрос,
// начатый с чужой страницы. Подробности выбора — в security.go.
func setSessionCookie(w http.ResponseWriter, token string, secure bool, maxAge int) {
	//nolint:gosec // G124 засчитывает только литерал Secure: true, а флаг настраиваемый: локальный запуск идёт по http
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})
}

// Logout завершает сессию и стирает cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := a.sessions.Delete(r.Context(), cookie.Value); err != nil {
			slog.Error("сессия не удалена при выходе", "err", err)
		}
	}
	setSessionCookie(w, "", a.cookieSecure, -1)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// RequireSession пропускает дальше только запросы с действующей сессией.
func (a *Auth) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход.")
			return
		}
		ok, extended, err := a.sessions.Extend(r.Context(), cookie.Value, a.ttl, sessionRefreshEvery(a.ttl))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось проверить сессию.")
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Сессия истекла, войдите заново.")
			return
		}
		if extended {
			// Срок в базе сдвинут — cookie живёт столько же, иначе браузер
			// выбросил бы её раньше, чем истечёт сессия.
			setSessionCookie(w, cookie.Value, a.cookieSecure, int(a.ttl.Seconds()))
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionLoginKey{}, a.login)))
	})
}

// sessionRefreshEvery — как часто активная сессия продлевается: раз в 10 минут,
// а при коротком сроке — раз в десятую его часть, чтобы продление вообще успевало.
func sessionRefreshEvery(ttl time.Duration) time.Duration {
	return min(10*time.Minute, ttl/10)
}

type sessionLoginKey struct{}

// sessionLogin — логин, под которым вошли: им подписываются пометки и связи
// реестра. Вне RequireSession пусто.
func sessionLogin(ctx context.Context) string {
	login, _ := ctx.Value(sessionLoginKey{}).(string)
	return login
}

// clientIP возвращает адрес клиента без порта.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
