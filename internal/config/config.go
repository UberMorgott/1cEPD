// Package config читает настройки сервиса из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Config — все настройки сервиса. Секреты живут только здесь и в памяти процесса.
type Config struct {
	PartnerAPILogin    string
	PartnerAPIPassword string
	PartnerCode        string
	AppLogin           string
	// AppPasswordHash — всегда bcrypt-хеш: открытый APP_PASSWORD хешируется
	// при старте и в конфигурации не остаётся.
	AppPasswordHash string
	SessionTTL      time.Duration
	DBPath          string
	ListenAddr      string
	// CookieSecure выключается только для локального запуска по http:
	// браузер не сохраняет cookie с флагом Secure на незащищённом соединении.
	CookieSecure bool
	// PrimeUILicense — ключ Community-лицензии PrimeUI (PRIMEUI_LICENSE).
	// Необязателен: без него интерфейс рисует плашку о лицензии. Живёт в .env,
	// а не в сборке, чтобы ключ не попадал в репозиторий и в релизы.
	PrimeUILicense string
}

// Getter возвращает значение переменной окружения. Отдельный тип, чтобы тесты
// не трогали настоящее окружение процесса.
type Getter func(key string) string

// Load читает конфигурацию и проверяет, что обязательное заполнено.
func Load(get Getter) (Config, error) {
	cfg := Config{
		PartnerAPILogin:    get("PARTNER_API_LOGIN"),
		PartnerAPIPassword: get("PARTNER_API_PASSWORD"),
		PartnerCode:        get("PARTNER_CODE"),
		AppLogin:           get("APP_LOGIN"),
		DBPath:             orDefault(get("DB_PATH"), "./data/app.db"),
		ListenAddr:         orDefault(get("LISTEN_ADDR"), "127.0.0.1:8080"),
		PrimeUILicense:     strings.TrimSpace(get("PRIMEUI_LICENSE")),
	}

	required := map[string]string{
		"PARTNER_API_LOGIN":    cfg.PartnerAPILogin,
		"PARTNER_API_PASSWORD": cfg.PartnerAPIPassword,
		"PARTNER_CODE":         cfg.PartnerCode,
		"APP_LOGIN":            cfg.AppLogin,
	}
	for name, value := range required {
		if value == "" {
			return Config{}, fmt.Errorf("не задана обязательная переменная %s", name)
		}
	}

	hash, err := appPasswordHash(get)
	if err != nil {
		return Config{}, err
	}
	cfg.AppPasswordHash = hash

	ttl := orDefault(get("SESSION_TTL"), "12h")
	parsed, err := time.ParseDuration(ttl)
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_TTL=%q: %w", ttl, err)
	}
	// Ноль и отрицательное время означали бы сессию, истёкшую в момент выдачи.
	if parsed <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL=%q: должно быть положительным", ttl)
	}
	cfg.SessionTTL = parsed

	// Значение по умолчанию — true: небезопасный режим должен включаться осознанно.
	cfg.CookieSecure = !strings.EqualFold(get("COOKIE_SECURE"), "false")

	return cfg, nil
}

// appPasswordHash возвращает bcrypt-хеш пароля входа.
//
// Пароль задаётся одним из двух способов: APP_PASSWORD_HASH (хеш из cmd/hashpw)
// или APP_PASSWORD (обычный пароль, который хешируется здесь же). Второй способ
// удобнее тем, кто разворачивает сервис у себя, и остаётся вторым: если заданы
// оба, побеждает хеш. Сам пароль никуда не сохраняется и не логируется.
func appPasswordHash(get Getter) (string, error) {
	hash := get("APP_PASSWORD_HASH")
	plain := get("APP_PASSWORD")

	switch {
	case hash != "" && plain != "":
		slog.Warn("заданы и APP_PASSWORD_HASH, и APP_PASSWORD: используется хеш")
		return hash, nil
	case hash != "":
		return hash, nil
	case plain != "":
		generated, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
		if err != nil {
			// Текст ошибки bcrypt пароля не содержит — длину он называет, значение нет.
			return "", fmt.Errorf("не захешировать APP_PASSWORD: %w", err)
		}
		return string(generated), nil
	}
	return "", errors.New(
		"не задан пароль входа: заполните APP_PASSWORD (обычный пароль) " +
			"или APP_PASSWORD_HASH (хеш из `go run ./cmd/hashpw`)")
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
