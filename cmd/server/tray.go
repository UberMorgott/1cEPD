// Значок в трее и то, что заменяет исчезнувшую консоль: журнал рядом с exe и
// окно Windows с причиной, если старт не удался.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"fyne.io/systray"
	"golang.org/x/sys/windows"

	"partnerops/internal/selfupdate"
	"partnerops/internal/updater"
)

// Тот же icon.ico -- значок самого exe: rsrc_windows_amd64.syso собран из него
// и подхватывается go build сам. Поменял icon.ico -- перегенерируй syso.
//
//go:generate go-winres simply --arch amd64 --manifest none --icon icon.ico
//go:embed icon.ico
var trayIcon []byte

// logFileName — журнал рядом с exe. Сборка идёт с -H=windowsgui, консоли нет,
// и это единственное место, где видно, что произошло.
const logFileName = "pult-epd.log"

// logPath считает путь журнала от собственного exe, а не от рабочей папки:
// ярлык может запустить программу откуда угодно.
func logPath() string {
	exe, err := os.Executable()
	if err != nil {
		return logFileName
	}
	return filepath.Join(filepath.Dir(exe), logFileName)
}

// setupLog уводит slog и стандартный log в файл журнала. Файл обнуляется на
// каждом старте: ротация тут лишняя, а расти без предела он не должен.
// Если файл не открылся — остаёмся на stderr: из-за журнала падать нельзя.
func setupLog() {
	path := logPath()
	//nolint:gosec // путь считается от собственного exe, пользовательского ввода тут нет
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		slog.Warn("журнал не открыт, пишем в stderr", "path", path, "err", err)
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(file, nil)))
	log.SetOutput(file) // systray сообщает о своих бедах через стандартный log
}

// fatal показывает причину и выходит. Без консоли текст ошибки читать негде,
// поэтому он идёт и в журнал, и в окно Windows.
func fatal(err error) {
	slog.Error("сервис остановлен с ошибкой", "err", err)
	messageBox("Пульт ЭПД не запустился", fmt.Sprintf("%v\n\nПодробности: %s", err, logPath()))
	os.Exit(1)
}

// messageBox показывает окно с ошибкой средствами Windows: cgo для этого не нужен.
func messageBox(caption, text string) {
	showBox(caption, text, windows.MB_ICONERROR)
}

// infoBox показывает окно с сообщением.
func infoBox(caption, text string) {
	showBox(caption, text, windows.MB_ICONINFORMATION)
}

// versionLabel — версия для людей: «v0.1.0», а у сборки без версии — «dev».
func versionLabel(v string) string {
	if selfupdate.Valid(v) {
		return "v" + v
	}
	return v
}

// checkFromTray — пункт «Проверить обновления»: новая версия ставится сразу
// и программа перезапускается, иначе окно с итогом проверки.
func checkFromTray(ctx context.Context, updates *updater.Updater) {
	st := updates.Check(ctx)
	if st.Available {
		st = updates.Install(ctx)
		if st.Restarting {
			return
		}
	}
	if st.Failed {
		messageBox("Пульт ЭПД: обновление", st.Text)
		return
	}
	infoBox("Пульт ЭПД: обновление", "Версия "+versionLabel(st.Current)+". "+st.Text)
}

func showBox(caption, text string, icon uint32) {
	captionPtr, err := windows.UTF16PtrFromString(caption)
	if err != nil {
		return
	}
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	if _, err := windows.MessageBox(0, textPtr, captionPtr,
		windows.MB_OK|icon|windows.MB_SETFOREGROUND|windows.MB_TOPMOST); err != nil {
		slog.Error("окно с ошибкой не показано", "err", err)
	}
}

// browseURL превращает адрес прослушивания в ссылку для браузера. LISTEN_ADDR
// может быть без хоста (":8080") или с 0.0.0.0 — по таким адресам браузер не
// ходит, подставляем петлю.
func browseURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// openBrowser открывает ссылку в браузере по умолчанию. Блокирует до возврата
// rundll32, поэтому вызывается из отдельной горутины. Контекст обрывает сам
// rundll32, а не открытый им браузер, — на уже открытую вкладку выход не влияет.
func openBrowser(ctx context.Context, url string) {
	//nolint:gosec // адрес собран из нашей же конфигурации, а не из запроса
	cmd := exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", url)
	if err := cmd.Run(); err != nil {
		slog.Error("браузер не открылся", "url", url, "err", err)
	}
}

// runTray показывает значок в трее и блокирует, пока тот жив.
//
// Пункт «Выход» не останавливает ничего сам, а зовёт quit — ту же отмену
// контекста, что дёргает обработчик сигналов, так что путь остановки один.
//
// Вызывается из отдельной горутины, прибитой к своему потоку ОС: окно и цикл
// сообщений Windows должны жить на одном потоке, а главную горутину занимать
// нельзя. Без рабочего стола (служба, сеанс без оболочки) systray не
// поднимается и остаётся в своём цикле навсегда — сервер это переживает
// только потому, что ждёт остановки не здесь. Причину неудачи systray пишет
// в тот же журнал через стандартный log.
func runTray(ctx context.Context, url string, quit context.CancelFunc, updates *updater.Updater) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	version := versionLabel(updates.Version())
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("Пульт ЭПД")
		systray.SetTooltip("Пульт ЭПД " + version + " — " + url)
		systray.SetOnTapped(func() { go openBrowser(ctx, url) })

		open := systray.AddMenuItem("Открыть Пульт ЭПД", url)
		systray.AddSeparator()
		address := systray.AddMenuItem(url, "адрес сервиса")
		address.Disable()
		versionItem := systray.AddMenuItem("Версия "+version, "версия программы")
		versionItem.Disable()
		check := systray.AddMenuItem("Проверить обновления", "Проверить новую версию на GitHub и установить её")
		systray.AddSeparator()
		exit := systray.AddMenuItem("Выход", "Остановить Пульт ЭПД")

		go func() {
			for {
				select {
				case <-open.ClickedCh:
					go openBrowser(ctx, url)
				case <-check.ClickedCh:
					go checkFromTray(ctx, updates)
				case <-exit.ClickedCh:
					quit()
					systray.Quit()
					return
				case <-ctx.Done(): // остановили сигналом — значок убираем следом
					systray.Quit()
					return
				}
			}
		}()
	}, nil)

	slog.Info("значок в трее убран")
}
