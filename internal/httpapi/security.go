package httpapi

import "net/http"

// contentSecurityPolicy рассчитана на собранный SPA: скрипты и стили лежат
// отдельными файлами рядом с index.html, наружу приложение не ходит.
//
// 'unsafe-inline' оставлен только стилям: Vue раскладывает :style-привязки в
// атрибут style, и без него часть разметки поедет. Скриптам он не нужен —
// вшитый dist инлайновых <script> не содержит.
//
// Шрифт лежит в самом dist, поэтому внешних источников в политике нет вовсе:
// на закрытом контуре ходить в интернет всё равно некуда.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// SecurityHeaders добавляет заголовки, которые браузер применяет сам.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		// frame-ancestors из CSP старые браузеры не понимают, X-Frame-Options — да.
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// RejectCrossSite отбивает запросы, начатые с чужой страницы, — это и есть
// защита от CSRF, токена в приложении нет.
//
// Почему не токен. Cookie сессии выставлена с SameSite=Strict, и одного этого
// почти хватает. Почти — потому что «site» считается по домену без порта:
// страница на другом порту того же 127.0.0.1 (или того же хоста в локальной
// сети) для браузера свой сайт, и Strict её не остановит. Заголовок
// Sec-Fetch-Site такую подмену как раз видит: он ставится самим браузером,
// подделать его со страницы нельзя, и cross-origin запрос отличим от своего.
// Вдвоём это закрывает те же случаи, что и токен, без хранения состояния.
//
// Запросы без Sec-Fetch-Site (curl, старый браузер) проходят: CSRF без браузера
// не бывает, а браузеры, которые не ставят заголовок, ушли вместе с теми, что
// не понимают SameSite.
func RejectCrossSite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Читающие методы состояние не меняют.
		default:
			switch r.Header.Get("Sec-Fetch-Site") {
			case "", "same-origin", "none":
				// Свой же запрос или клиент без заголовка.
			default:
				writeError(w, http.StatusForbidden, "cross_site",
					"Запрос пришёл с чужой страницы и отклонён.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
