//go:build !windows

package selfupdate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// hideFile вне Windows не нужен: заменённый exe удаляется сразу.
func hideFile(string) error { return nil }

// Start запускает exe с args в своём сеансе, чтобы он пережил этот процесс.
func Start(exe string, args, env []string) error {
	cmd := exec.CommandContext(context.Background(), exe, args...) // #nosec G204 -- собственный путь и аргументы
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// WaitExit ждёт, пока процесс pid не исчезнет, но не дольше timeout.
func WaitExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for pid > 0 && time.Now().Before(deadline) {
		p, err := os.FindProcess(pid)
		if err != nil || p.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
