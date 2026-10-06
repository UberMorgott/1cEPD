package selfupdate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// hideFile ставит FILE_ATTRIBUTE_HIDDEN, чтобы «.<base>.old» не мозолил глаза
// в папке пакета до следующего старта: Windows держит его запертым, пока жив
// его процесс.
func hideFile(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_HIDDEN)
}

const createNoWindow = 0x08000000

// Start запускает exe с args процессом, который переживает этот: своя группа
// процессов и, если можно, вне job-объекта. Рабочая папка — папка exe (там
// лежит .env, как при запуске через start.bat), к окружению добавляется env.
func Start(exe string, args, env []string) error {
	start := func(breakaway bool) error {
		flags := uint32(createNoWindow | windows.CREATE_NEW_PROCESS_GROUP)
		if breakaway {
			flags |= windows.CREATE_BREAKAWAY_FROM_JOB
		}
		// Перезапущенный процесс переживает этот, контекст ему не нужен.
		cmd := exec.CommandContext(context.Background(), exe, args...) // #nosec G204 -- собственный путь и аргументы
		cmd.Dir = filepath.Dir(exe)
		cmd.Env = append(os.Environ(), env...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: flags}
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	}
	err := start(true)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) { // job запрещает breakaway
		err = start(false)
	}
	return err
}

// WaitExit ждёт завершения процесса pid не дольше timeout. Процесса уже нет
// (или его не открыть) — возвращается сразу.
func WaitExit(pid int, timeout time.Duration) {
	if pid <= 0 || pid > 1<<31-1 {
		return
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer func() { _ = windows.CloseHandle(h) }()
	ms := uint32(windows.INFINITE - 1)
	if t := timeout.Milliseconds(); t >= 0 && t < int64(ms) {
		ms = uint32(t) //nolint:gosec // границы проверены строкой выше
	}
	_, _ = windows.WaitForSingleObject(h, ms)
}
