package httpapi

import "net/http"

// NewRouter собирает маршруты сервиса.
//
// frontend — собранный интерфейс; nil означает «фронт не отдаём»,
// так роутер поднимается в тестах без вшитой сборки.
func NewRouter(auth *Auth, sse *Events, registry *Registry, frontend http.Handler,
	requests *Requests, settings *Settings, update *Update, uiLicense string,
) http.Handler {
	mux := http.NewServeMux()

	// Ключ лицензии PrimeUI нужен интерфейсу до входа (страница входа тоже
	// на PrimeVue), поэтому без сессии. Он и так виден любому, кто открыл
	// страницу: раньше он лежал прямо в бандле.
	mux.HandleFunc("GET /api/ui-config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"primeuiLicense": uiLicense})
	})

	// Версия открыта без сессии: по ней страница после обновления ждёт, пока
	// поднимется новая сборка.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{"status": "ok"}
		if update != nil {
			body["version"] = update.updater.Version()
		}
		writeJSON(w, http.StatusOK, body)
	})

	if update != nil {
		mux.Handle("GET /api/update", auth.RequireSession(http.HandlerFunc(update.Status)))
		mux.Handle("POST /api/update/check", auth.RequireSession(http.HandlerFunc(update.Check)))
		mux.Handle("POST /api/update/apply", auth.RequireSession(http.HandlerFunc(update.Apply)))
		mux.Handle("PUT /api/update/settings", auth.RequireSession(http.HandlerFunc(update.PutPrefs)))
	}

	mux.HandleFunc("POST /api/login", auth.Login)
	mux.HandleFunc("POST /api/logout", auth.Logout)

	mux.Handle("GET /api/events", auth.RequireSession(http.HandlerFunc(sse.Stream)))

	if registry != nil {
		mux.Handle("GET /api/identifiers", auth.RequireSession(http.HandlerFunc(registry.Identifiers)))
		mux.Handle("GET /api/anomalies", auth.RequireSession(http.HandlerFunc(registry.Anomalies)))
		mux.Handle("POST /api/anomalies/ack", auth.RequireSession(http.HandlerFunc(registry.Acknowledge)))
		mux.Handle("POST /api/anomalies/unack", auth.RequireSession(http.HandlerFunc(registry.Unacknowledge)))
		mux.Handle("GET /api/topology", auth.RequireSession(http.HandlerFunc(registry.Topology)))
		mux.Handle("POST /api/topology", auth.RequireSession(http.HandlerFunc(registry.PutTopology)))
		mux.Handle("POST /api/topology/remove", auth.RequireSession(http.HandlerFunc(registry.RemoveTopology)))
		mux.Handle("GET /api/billing", auth.RequireSession(http.HandlerFunc(registry.Billing)))
		mux.Handle("GET /api/billing/history", auth.RequireSession(http.HandlerFunc(registry.BillingHistory)))
		mux.Handle("GET /api/billing/forecast", auth.RequireSession(http.HandlerFunc(registry.BillingForecast)))
		mux.Handle("GET /api/its/contracts", auth.RequireSession(http.HandlerFunc(registry.ITSContracts)))
		mux.Handle("POST /api/its/contracts/refresh", auth.RequireSession(http.HandlerFunc(registry.RefreshITSContracts)))
		mux.Handle("GET /api/licenses", auth.RequireSession(http.HandlerFunc(registry.Licenses)))
		mux.Handle("GET /api/its/epd-advice", auth.RequireSession(http.HandlerFunc(registry.EPDAdvice)))
		mux.Handle("GET /api/subscribers", auth.RequireSession(http.HandlerFunc(registry.Subscribers)))
		mux.Handle("POST /api/subscribers/refresh", auth.RequireSession(http.HandlerFunc(registry.RefreshSubscribers)))
		mux.Handle("GET /api/clients", auth.RequireSession(http.HandlerFunc(registry.ClientList)))
		mux.Handle("POST /api/clients/epd-import", auth.RequireSession(http.HandlerFunc(registry.ImportEPDBilling)))
		mux.Handle("GET /api/clients/{key}", auth.RequireSession(http.HandlerFunc(registry.ClientCard)))
		mux.Handle("GET /api/dashboard", auth.RequireSession(http.HandlerFunc(registry.Dashboard)))
	}

	if requests != nil {
		mux.Handle("GET /api/its/tariffs", auth.RequireSession(http.HandlerFunc(requests.Tariffs)))
		mux.Handle("POST /api/its/validate", auth.RequireSession(http.HandlerFunc(requests.Validate)))
		mux.Handle("POST /api/its/download", auth.RequireSession(http.HandlerFunc(requests.Download)))
		mux.Handle("POST /api/its/send", auth.RequireSession(http.HandlerFunc(requests.Send)))
		mux.Handle("GET /api/its/requests", auth.RequireSession(http.HandlerFunc(requests.List)))
		mux.Handle("POST /api/its/requests", auth.RequireSession(http.HandlerFunc(requests.Save)))
		mux.Handle("GET /api/its/requests/{id}", auth.RequireSession(http.HandlerFunc(requests.Get)))
		mux.Handle("GET /api/its/clients", auth.RequireSession(http.HandlerFunc(requests.Clients)))
		mux.Handle("POST /api/its/clients/programs", auth.RequireSession(http.HandlerFunc(requests.CheckPrograms)))
	}

	if settings != nil {
		mux.Handle("GET /api/settings", auth.RequireSession(http.HandlerFunc(settings.Get)))
		mux.Handle("PUT /api/settings", auth.RequireSession(http.HandlerFunc(settings.Put)))
		mux.Handle("POST /api/settings/smtp/test", auth.RequireSession(http.HandlerFunc(settings.TestSMTP)))
		mux.Handle("GET /api/settings/its-sender", auth.RequireSession(http.HandlerFunc(settings.GetSender)))
		mux.Handle("PUT /api/settings/its-sender", auth.RequireSession(http.HandlerFunc(settings.PutSender)))
	}

	if frontend != nil {
		mux.Handle("/", frontend)
	}

	return SecurityHeaders(RejectCrossSite(mux))
}
