// Package updater ведёт самообновление Пульта ЭПД: проверка раз в шесть часов,
// установка по кнопке или сама (если так настроено) и перезапуск. Порт
// internal/app/update.go из agent-link.
package updater

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"partnerops/internal/selfupdate"
	"partnerops/internal/settings"
)

// Release — релиз, который можно поставить; в работе это *selfupdate.Release.
type Release interface {
	Version() string
	Install(ctx context.Context, exePath string, progress selfupdate.Progress) error
}

// Prefs читает и пишет настройки самообновления; в работе это *settings.Store.
type Prefs interface {
	UpdatePrefs(ctx context.Context) (settings.UpdatePrefs, error)
	SaveUpdatePrefs(ctx context.Context, p settings.UpdatePrefs) error
}

// Status — состояние обновления для шапки, настроек и трея.
type Status struct {
	Current string `json:"current"`
	Latest  string `json:"latest,omitempty"`
	// Available: Latest новее Current и его можно поставить.
	Available bool `json:"available"`
	Busy      bool `json:"busy"`
	// Restarting: новая версия на месте, программа перезапускается.
	Restarting bool `json:"restarting,omitempty"`
	// Enabled: у сборки есть версия и известен её exe ("dev" не обновляется).
	Enabled bool   `json:"enabled"`
	Text    string `json:"text,omitempty"`
	Failed  bool   `json:"failed,omitempty"`
	// Installing: идёт загрузка, Downloaded байт из Size (0 — неизвестно).
	Installing bool  `json:"installing,omitempty"`
	Downloaded int64 `json:"downloaded,omitempty"`
	Size       int64 `json:"size,omitempty"`
	// AutoCheck и AutoInstall — настройки самообновления.
	AutoCheck   bool `json:"autoCheck"`
	AutoInstall bool `json:"autoInstall"`
}

// Расписание проверок: первая вскоре после старта, дальше раз в updateEvery
// с разбросом ±10%, чтобы машины одного офиса не ходили в GitHub разом.
const (
	updateFirst = time.Minute
	updateEvery = 6 * time.Hour
)

// LatestFunc находит последний релиз; newer — он новее current.
type LatestFunc func(ctx context.Context, current string) (rel Release, newer bool, err error)

// Updater — состояние самообновления. Нулевое значение не годится: New.
type Updater struct {
	version  string
	exe      string
	prefs    Prefs
	latest   LatestFunc
	relaunch func() error
	quit     func()

	mu          sync.Mutex
	rel         Release // новый релиз, найденный последней проверкой
	latestVer   string
	state       string // "", "check", "apply", "restart"
	text        string
	failed      bool
	done, total int64
}

// New собирает обновлятель. exe — путь собственного exe, relaunch запускает
// его заново, quit останавливает этот процесс штатным путём (как «Выход»).
func New(version, exe string, prefs Prefs, relaunch func() error, quit func()) *Updater {
	return &Updater{
		version: version, exe: exe, prefs: prefs, relaunch: relaunch, quit: quit,
		latest: func(ctx context.Context, current string) (Release, bool, error) {
			rel, newer, err := selfupdate.Check(ctx, current)
			if rel == nil {
				return nil, false, err
			}
			return rel, newer, err
		},
	}
}

// WithLatest подменяет поиск релиза (тесты).
func (u *Updater) WithLatest(f LatestFunc) *Updater {
	u.latest = f
	return u
}

// Enabled — сборка умеет обновляться: есть версия и exe.
func (u *Updater) Enabled() bool {
	return selfupdate.Valid(u.version) && u.exe != "" && u.relaunch != nil
}

// Version — версия этой сборки.
func (u *Updater) Version() string { return u.version }

func (u *Updater) prefsOrDefault(ctx context.Context) settings.UpdatePrefs {
	if u.prefs == nil {
		return settings.UpdatePrefs{Check: true, Install: true}
	}
	p, err := u.prefs.UpdatePrefs(ctx)
	if err != nil {
		slog.Warn("настройки обновления не прочитаны", "err", err)
		return settings.UpdatePrefs{Check: true, Install: true}
	}
	return p
}

// Status возвращает состояние обновления.
func (u *Updater) Status(ctx context.Context) Status {
	p := u.prefsOrDefault(ctx)
	u.mu.Lock()
	defer u.mu.Unlock()
	st := Status{
		Current: u.version, Latest: u.latestVer, Available: u.rel != nil && u.state == "",
		Busy: u.state != "", Restarting: u.state == "restart", Enabled: u.Enabled(),
		Text: u.text, Failed: u.failed, AutoCheck: p.Check, AutoInstall: p.Install,
	}
	if u.state == "apply" {
		st.Installing, st.Downloaded, st.Size = true, u.done, max(u.total, 0)
	}
	if !st.Enabled {
		st.Text, st.Available = "Сборка без версии (dev) не обновляется.", false
	}
	return st
}

// begin отмечает начало шага; false — уже идёт другой.
func (u *Updater) begin(state, text string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state != "" {
		return false
	}
	u.state, u.text, u.failed = state, text, false
	u.done, u.total = 0, 0
	return true
}

func (u *Updater) failLocked(prefix string, err error) {
	u.state, u.failed = "", true
	if rl, ok := errors.AsType[*selfupdate.RateLimitError](err); ok {
		u.text = rl.Error()
		return
	}
	u.text = prefix + ": " + err.Error()
}

// Check спрашивает GitHub о последнем релизе.
func (u *Updater) Check(ctx context.Context) Status {
	if !u.Enabled() || !u.begin("check", "Проверяем обновления…") {
		return u.Status(ctx)
	}
	rel, newer, err := u.latest(ctx, u.version)
	u.mu.Lock()
	u.state, u.rel, u.latestVer = "", nil, ""
	switch {
	case err != nil:
		slog.Warn("проверка обновлений", "err", err)
		u.failLocked("Не удалось проверить обновления", err)
	case rel == nil:
		u.text = "Опубликованных версий пока нет."
	case newer:
		u.rel, u.latestVer = rel, rel.Version()
		u.text = "Доступна версия v" + rel.Version() + "."
	default:
		u.latestVer = rel.Version()
		u.text = "Установлена последняя версия."
	}
	u.mu.Unlock()
	return u.Status(ctx)
}

// Install ставит найденный проверкой релиз (проверяя сначала, если нужно) и
// перезапускает программу: relaunch запускает новый exe, который ждёт выхода
// этого, а quit останавливает этот штатно.
func (u *Updater) Install(ctx context.Context) Status {
	if !u.Enabled() {
		return u.Status(ctx)
	}
	u.mu.Lock()
	rel := u.rel
	u.mu.Unlock()
	if rel == nil {
		if st := u.Check(ctx); !st.Available {
			return st
		}
		u.mu.Lock()
		rel = u.rel
		u.mu.Unlock()
	}
	if !u.begin("apply", "Устанавливаем v"+rel.Version()+"…") {
		return u.Status(ctx)
	}
	err := rel.Install(ctx, u.exe, u.progress)
	u.mu.Lock()
	if err != nil {
		slog.Error("обновление не установлено", "version", rel.Version(), "err", err)
		u.failLocked("Не удалось установить обновление", err)
		u.mu.Unlock()
		return u.Status(ctx)
	}
	slog.Info("обновление установлено", "version", rel.Version())
	u.state, u.text = "restart", "Версия v"+rel.Version()+" установлена, перезапускаемся…"
	u.mu.Unlock()
	go u.restart()
	return u.Status(ctx)
}

func (u *Updater) progress(done, total int64) {
	u.mu.Lock()
	u.done, u.total = done, total
	u.mu.Unlock()
}

// restart запускает новый exe и останавливает этот процесс. Короткая пауза
// даёт ответу «перезапускаемся» уйти в браузер до остановки сервера.
func (u *Updater) restart() {
	time.Sleep(500 * time.Millisecond)
	if err := u.relaunch(); err != nil {
		slog.Error("перезапуск после обновления не удался", "err", err)
		u.mu.Lock()
		u.rel = nil
		u.state, u.text, u.failed = "", "Новая версия установлена, но не запустилась: перезапустите программу вручную.", true
		u.mu.Unlock()
		return
	}
	slog.Info("новая версия запущена, выходим")
	if u.quit != nil {
		u.quit()
	}
}

// SetPrefs сохраняет настройки самообновления.
func (u *Updater) SetPrefs(ctx context.Context, p settings.UpdatePrefs) error {
	if u.prefs == nil {
		return errors.New("updater: настройки не подключены")
	}
	return u.prefs.SaveUpdatePrefs(ctx, p)
}

// Run проверяет обновления вскоре после старта и дальше раз в несколько
// часов, пока включена автопроверка, и ставит найденное, пока включена
// автоустановка. Возвращается с концом ctx или сразу для сборки без версии.
func (u *Updater) Run(ctx context.Context) {
	if !u.Enabled() {
		return
	}
	t := time.NewTimer(updateFirst)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		p := u.prefsOrDefault(ctx)
		if p.Check && u.Check(ctx).Available && p.Install {
			if u.Install(ctx).Restarting {
				return
			}
		}
		jitter := updateEvery / 10
		t.Reset(updateEvery - jitter + rand.N(2*jitter+1)) // #nosec G404 -- разброс опроса, не секрет
	}
}
