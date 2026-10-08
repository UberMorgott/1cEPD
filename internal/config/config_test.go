package config

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// base — минимальное окружение, к которому тесты добавляют своё.
func base() map[string]string {
	return map[string]string{
		"PARTNER_API_LOGIN":    "api-login-00000",
		"PARTNER_API_PASSWORD": "secret",
		"PARTNER_CODE":         "00000",
		"APP_LOGIN":            "admin",
	}
}

func TestLoadHashesPlainPassword(t *testing.T) {
	env := base()
	env["APP_PASSWORD"] = "открытый пароль"

	cfg, err := Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("не ожидали ошибку: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.AppPasswordHash), []byte("открытый пароль")); err != nil {
		t.Errorf("хеш не подходит к APP_PASSWORD: %v", err)
	}
}

// Уже развёрнутые установки задают хеш: он должен побеждать.
func TestLoadPrefersHashOverPlainPassword(t *testing.T) {
	env := base()
	env["APP_PASSWORD"] = "открытый пароль"
	env["APP_PASSWORD_HASH"] = "$2a$10$abcdefghijklmnopqrstuv"

	cfg, err := Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("не ожидали ошибку: %v", err)
	}
	if cfg.AppPasswordHash != "$2a$10$abcdefghijklmnopqrstuv" {
		t.Errorf("AppPasswordHash = %q, ожидали значение APP_PASSWORD_HASH", cfg.AppPasswordHash)
	}
}

func TestLoadRequiresSomePassword(t *testing.T) {
	env := base()
	_, err := Load(func(key string) string { return env[key] })
	if err == nil {
		t.Fatal("ожидали ошибку об отсутствии пароля, получили nil")
	}
	for _, name := range []string{"APP_PASSWORD", "APP_PASSWORD_HASH"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("в тексте ошибки нет %s: %v", name, err)
		}
	}
}

func TestLoadRequiresPartnerCredentials(t *testing.T) {
	_, err := Load(func(key string) string {
		return map[string]string{
			"APP_LOGIN":         "admin",
			"APP_PASSWORD_HASH": "$2a$10$abcdefghijklmnopqrstuv",
		}[key]
	})
	if err == nil {
		t.Fatal("ожидали ошибку об отсутствии PARTNER_API_LOGIN, получили nil")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	env := map[string]string{
		"PARTNER_API_LOGIN":    "api-login-00000",
		"PARTNER_API_PASSWORD": "secret",
		"PARTNER_CODE":         "00000",
		"APP_LOGIN":            "admin",
		"APP_PASSWORD_HASH":    "$2a$10$abcdefghijklmnopqrstuv",
	}
	cfg, err := Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("не ожидали ошибку: %v", err)
	}
	if cfg.SessionTTL != 720*time.Hour {
		t.Errorf("SessionTTL = %v, ожидали 720h", cfg.SessionTTL)
	}
	if cfg.DBPath != "./data/app.db" {
		t.Errorf("DBPath = %q, ожидали ./data/app.db", cfg.DBPath)
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr = %q, ожидали 127.0.0.1:8080", cfg.ListenAddr)
	}
}

// Нулевой или отрицательный TTL создаёт сессии, истёкшие в момент выдачи.
func TestLoadRejectsNonPositiveTTL(t *testing.T) {
	for _, ttl := range []string{"0", "0s", "-1h"} {
		env := map[string]string{
			"PARTNER_API_LOGIN":    "api-login-00000",
			"PARTNER_API_PASSWORD": "secret",
			"PARTNER_CODE":         "00000",
			"APP_LOGIN":            "admin",
			"APP_PASSWORD_HASH":    "$2a$10$abcdefghijklmnopqrstuv",
			"SESSION_TTL":          ttl,
		}
		if _, err := Load(func(key string) string { return env[key] }); err == nil {
			t.Errorf("SESSION_TTL=%q принят, ожидали ошибку", ttl)
		}
	}
}

func TestLoadRejectsBadTTL(t *testing.T) {
	env := map[string]string{
		"PARTNER_API_LOGIN":    "api-login-00000",
		"PARTNER_API_PASSWORD": "secret",
		"PARTNER_CODE":         "00000",
		"APP_LOGIN":            "admin",
		"APP_PASSWORD_HASH":    "$2a$10$abcdefghijklmnopqrstuv",
		"SESSION_TTL":          "не время",
	}
	if _, err := Load(func(key string) string { return env[key] }); err == nil {
		t.Fatal("ожидали ошибку разбора SESSION_TTL")
	}
}
