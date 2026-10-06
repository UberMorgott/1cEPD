package main

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"partnerops/internal/selfupdate"
)

// afterUpdateFlag — аргумент, с которым обновлённый exe запускает себя:
// «-after-update <pid прежнего процесса>».
const afterUpdateFlag = "-after-update"

// afterUpdateWait — сколько ждать выхода прежней версии: ей нужно закрыть
// базу и освободить порт.
const afterUpdateWait = 30 * time.Second

// afterUpdate дожидается выхода прежней версии, если этот процесс запущен
// обновлением, и убирает оставленный обновлением старый exe.
func afterUpdate() {
	if len(os.Args) == 3 && os.Args[1] == afterUpdateFlag {
		if pid, err := strconv.Atoi(os.Args[2]); err == nil {
			slog.Info("запуск после обновления, ждём выхода прежней версии", "pid", pid, "version", selfupdate.Version)
			selfupdate.WaitExit(pid, afterUpdateWait)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if err := selfupdate.Cleanup(exe); err != nil {
		slog.Warn("старый exe после обновления не удалён", "err", err)
	}
}
