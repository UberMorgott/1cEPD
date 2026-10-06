// Команда server поднимает HTTP-сервис и фоновые задачи.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"partnerops/internal/config"
	"partnerops/internal/events"
	"partnerops/internal/httpapi"
	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
	"partnerops/internal/selfupdate"
	"partnerops/internal/service"
	"partnerops/internal/settings"
	"partnerops/internal/store"
	"partnerops/internal/updater"
	"partnerops/web"
)

func main() {
	setupLog()
	afterUpdate()
	if err := run(); err != nil {
		fatal(err)
	}
}

func run() error {
	// Настройки берутся из окружения, а .env рядом с бинарником служит запасным
	// источником для локального запуска. Окружение имеет приоритет.
	dotenv, err := config.LoadDotEnv(".env")
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.GetterWith(dotenv))
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("база не закрыта", "err", err)
		}
	}()

	bus := events.NewBus()
	sessions := store.NewSessions(db)
	snapshots := store.NewSnapshots(db)
	client := partner.New("", cfg.PartnerAPILogin, cfg.PartnerAPIPassword)
	identifiers := store.NewIdentifiers(db)
	snapshotService := service.NewSnapshot(client, snapshots, bus).WithRegistry(identifiers)
	itsChecks := store.NewITSChecks(db)
	industryChecks := store.NewIndustryChecks(db)
	itsRefresher := service.NewITSRefresher(client, itsChecks, identifiers).WithIndustry(client, industryChecks)
	epdUsage := store.NewEPDUsage(db)
	epdRefresher := service.NewEPDUsageRefresher(client, epdUsage)
	monthTraffic := store.NewMonthTraffic(db)
	monthRefresher := service.NewMonthTrafficRefresher(client, monthTraffic)
	backfillStore := store.NewBackfill(db)
	backfill := service.NewBillingBackfill(client, snapshots, backfillStore, bus)
	// Проверка заявки идёт на каждое изменение формы: 1С спрашиваем через кэш.
	// Через него же суточная задача сохраняет базу абонентов.
	directory := service.NewRequestDirectory(client)
	subscribers := store.NewSubscribers(db)
	subscriberRefresher := service.NewSubscriberRefresher(directory, subscribers)
	registryAPI := httpapi.NewRegistry(identifiers, snapshots).WithITS(itsChecks, itsRefresher).
		WithEPDUsage(epdUsage).WithIndustry(industryChecks).WithHistory(backfillStore).
		WithForecast(monthTraffic).WithSubscribers(subscribers, subscriberRefresher).
		WithReview(identifiers, store.NewTopology(db))
	optionReports := store.NewOptionReports(db)
	requestStore := store.NewRequests(db)
	clientPrograms := store.NewClientPrograms(db)
	epdImport := store.NewEPDBillingImport(db)
	registryAPI.WithLicenses(optionReports).WithClientCards(requestStore, clientPrograms).WithEPDImport(epdImport)
	optionsRefresher := service.NewOptionsRefresher(client, optionReports)

	auth := httpapi.NewAuth(sessions, cfg.AppLogin, cfg.AppPasswordHash, cfg.SessionTTL, cfg.CookieSecure)

	frontend, err := web.Handler()
	if err != nil {
		return err
	}
	// Ключ шифрования пароля SMTP лежит рядом с базой и заводится при первом старте:
	// обязательной переменной окружения тут быть не должно, иначе уже развёрнутые
	// установки перестанут подниматься.
	secretKey, err := settings.LoadKey(cfg.DBPath)
	if err != nil {
		return err
	}
	settingsStore := settings.New(db, secretKey)

	requestsAPI := httpapi.NewRequests(
		requestStore,
		itsreq.NewBuilder(),
	).WithMailer(settingsStore).WithPortal(directory).WithDirectory(directory).WithClients(identifiers, monthTraffic, epdUsage).
		WithPrograms(clientPrograms, client).WithSubscribers(subscribers).WithEPDImport(epdImport)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Отмена поверх сигнальной: пункт «Выход» в трее останавливает сервис тем же
	// путём, что и Ctrl+C, так что вторая копия логики остановки не нужна.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Самообновление: новая версия стартует с -after-update и ждёт, пока этот
	// процесс освободит порт и базу; этот уходит тем же путём, что «Выход».
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("путь exe не найден, самообновление выключено", "err", err)
		exe = ""
	}
	updates := updater.New(selfupdate.Version, exe, settingsStore, func() error {
		return selfupdate.Start(exe, []string{afterUpdateFlag, strconv.Itoa(os.Getpid())}, []string{"NO_BROWSER=1"})
	}, cancel)

	router := httpapi.NewRouter(auth, httpapi.NewEvents(bus), registryAPI, frontend,
		requestsAPI, httpapi.NewSettings(settingsStore), httpapi.NewUpdate(updates))

	go runDailyJobs(ctx, snapshotService, sessions, itsRefresher, epdRefresher, monthRefresher, subscriberRefresher)
	go runBackfill(ctx, backfill)
	go runOptions(ctx, optionsRefresher)
	go updates.Run(ctx)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Общий таймаут записи оборвал бы поток SSE, поэтому обработчик потока
		// снимает дедлайн с себя сам (см. httpapi.Events.Stream).
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 16,
	}

	go func() {
		<-ctx.Done()
		// Родительский контекст уже отменён сигналом, поэтому отмену снимаем:
		// иначе Shutdown оборвётся сразу, не дождавшись живых соединений.
		shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("сервер не остановлен штатно", "err", err)
		}
	}()

	// Порт занимаем до трея: иначе значок появился бы у сервиса, который не
	// слушает, а ошибка всплыла бы позже и не окном.
	var listenCfg net.ListenConfig
	listener, err := listenCfg.Listen(ctx, "tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("не занять адрес %s: %w", cfg.ListenAddr, err)
	}

	served := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		served <- err
		cancel() // сервер упал сам — значок в трее уводим следом
	}()

	slog.Info("сервис запущен", "addr", cfg.ListenAddr, "version", selfupdate.Version)
	url := browseURL(cfg.ListenAddr)
	// NO_BROWSER=1 нужен проверке сборки: она поднимает exe пачкой и окно
	// браузера ей ни к чему.
	if os.Getenv("NO_BROWSER") == "" {
		go openBrowser(ctx, url)
	}

	// Значок живёт в своей горутине: без рабочего стола цикл сообщений
	// зависает навсегда, и остановка не должна от него зависеть.
	go runTray(ctx, url, cancel, updates)

	<-ctx.Done()
	return <-served
}

// runDailyJobs раз в сутки снимает снапшот, проверяет договоры 1С:ИТС, при смене
// месяца обновляет расход ЭПД и чистит истёкшие сессии. Первый прогон выполняется сразу при старте, чтобы не ждать
// сутки на пустой базе.
func runDailyJobs(
	ctx context.Context, snapshots *service.Snapshot, sessions *store.Sessions,
	its *service.ITSRefresher, epd *service.EPDUsageRefresher, month *service.MonthTrafficRefresher,
	subscribers *service.SubscriberRefresher,
) {
	tick := func() {
		// Биллинг ЭДО содержит начисления за закрытый месяц: на запрос текущего
		// 1С отвечает BILLING_DOES_NOT_EXIST. Поэтому снимаем предыдущий месяц.
		// Свежий отчёт (перезапуск сервиса) повторно не строится.
		if _, err := snapshots.TakeIfStale(ctx, previousMonth(time.Now()), service.SnapshotRefreshInterval); err != nil {
			slog.Error("снапшот не снят", "err", err)
		}
		// После снапшота: коды абонентов берутся из только что пополненного реестра.
		if _, err := its.Refresh(ctx, service.ITSRefreshInterval); err != nil {
			slog.Error("договоры 1С:ИТС не проверены", "err", err)
		}
		// Отчёт трафика тратит часовой лимит: строится только при смене месяца.
		if _, err := epd.Refresh(ctx); err != nil {
			slog.Error("расход ЭПД не обновлён", "err", err)
		}
		// Трафик текущего месяца — для прогноза перерасхода лимита, раз в сутки.
		if _, err := month.Refresh(ctx); err != nil {
			slog.Error("трафик текущего месяца не обновлён", "err", err)
		}
		// База абонентов — справочник и сверка с биллингом, отчётов не тратит.
		if _, err := subscribers.Refresh(ctx, service.SubscribersRefreshInterval); err != nil {
			slog.Error("база абонентов не выгружена", "err", err)
		}
		if removed, err := sessions.PurgeExpired(ctx); err != nil {
			slog.Error("не очистить сессии", "err", err)
		} else if removed > 0 {
			slog.Info("истёкшие сессии удалены", "count", removed)
		}
	}

	tick()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

// Догрузка истории биллинга ждёт после старта, пока ежедневные задачи
// потратят свои отчёты, а затем проверяет очередь. Сам шаг строит не больше
// одного отчёта за service.BackfillInterval.
const (
	backfillStartDelay = 5 * time.Minute
	backfillCheckEvery = 10 * time.Minute
)

// runBackfill по одному месяцу догружает биллинг за 12 закрытых месяцев.
func runBackfill(ctx context.Context, backfill *service.BillingBackfill) {
	wait := backfillStartDelay
	for {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if _, err := backfill.Step(ctx); err != nil {
			slog.Error("история биллинга не догружена", "err", err)
		}
		wait = backfillCheckEvery
	}
}

// Отчёты по опциям сервисов строятся по одному виду за шаг: видов одиннадцать,
// и все разом съели бы часовой лимит задач 1С, нужный биллингу.
const (
	optionsStartDelay = 5 * time.Minute
	optionsCheckEvery = 10 * time.Minute
)

// runOptions держит свежими остатки и сроки лицензий сервисов (1С-Отчетность,
// 1С:Подпись…): каждый вид перестраивается раз в сутки.
func runOptions(ctx context.Context, options *service.OptionsRefresher) {
	wait := optionsStartDelay
	for {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if _, err := options.Step(ctx); err != nil {
			slog.Error("лицензии сервисов не обновлены", "err", err)
		}
		wait = optionsCheckEvery
	}
}

// previousMonth возвращает первое число предыдущего месяца.
// Через AddDate(0, -1, 0) так делать нельзя: 31 марта минус месяц даёт 3 марта.
func previousMonth(now time.Time) time.Time {
	year, month, _ := now.UTC().Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
}
