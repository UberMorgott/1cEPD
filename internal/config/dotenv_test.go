package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("не записать файл: %v", err)
	}
	return path
}

func TestLoadDotEnvParsesFile(t *testing.T) {
	path := writeFile(t, "# комментарий\n\nAPP_LOGIN=admin\nDB_PATH= ./data/app.db \nQUOTED=\"с пробелом\"\n")

	values, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}

	want := map[string]string{
		"APP_LOGIN": "admin",
		"DB_PATH":   "./data/app.db",
		"QUOTED":    "с пробелом",
	}
	for key, expected := range want {
		if values[key] != expected {
			t.Errorf("%s = %q, ожидали %q", key, values[key], expected)
		}
	}
	if len(values) != len(want) {
		t.Errorf("прочитано %d переменных, ожидали %d", len(values), len(want))
	}
}

func TestLoadDotEnvKeepsEqualsInValue(t *testing.T) {
	// Хеш bcrypt содержит символы, а base64-значения могут содержать "=".
	path := writeFile(t, "APP_PASSWORD_HASH=$2a$10$abc=def=\n")

	values, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := values["APP_PASSWORD_HASH"]; got != "$2a$10$abc=def=" {
		t.Errorf("значение = %q, ожидали сохранения знаков равенства", got)
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	values, err := LoadDotEnv(filepath.Join(t.TempDir(), "нет-такого"))
	if err != nil {
		t.Fatalf("отсутствие файла не должно быть ошибкой, получили %v", err)
	}
	if values != nil {
		t.Errorf("ожидали nil, получили %v", values)
	}
}

func TestLoadDotEnvRejectsBrokenLine(t *testing.T) {
	path := writeFile(t, "APP_LOGIN=admin\nмусор без равенства\n")

	if _, err := LoadDotEnv(path); err == nil {
		t.Fatal("ожидали ошибку на строке без знака равенства")
	}
}

func TestGetterWithPrefersEnvironment(t *testing.T) {
	t.Setenv("APP_LOGIN", "из-окружения")

	get := GetterWith(map[string]string{"APP_LOGIN": "из-файла", "DB_PATH": "из-файла"})

	if got := get("APP_LOGIN"); got != "из-окружения" {
		t.Errorf("APP_LOGIN = %q, окружение должно быть приоритетнее", got)
	}
	if got := get("DB_PATH"); got != "из-файла" {
		t.Errorf("DB_PATH = %q, ожидали значение из файла", got)
	}
	if got := get("НЕТ_ТАКОЙ"); got != "" {
		t.Errorf("неизвестный ключ вернул %q, ожидали пустую строку", got)
	}
}
