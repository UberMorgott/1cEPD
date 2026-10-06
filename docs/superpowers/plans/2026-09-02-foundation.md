# Фундамент 1С Partner Ops — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Рабочий бэкенд, который каждый день сам забирает отчёт биллинга ЭДО из партнёрского API 1С, разбирает его и складывает в SQLite с историей, отдаёт данные по HTTP за логином и шлёт живые события на фронт.

**Architecture:** Один бинарник Go. Слой `partner` знает только про HTTP и форматы 1С, слой `store` — только про SQLite, слой `service` соединяет их и содержит смысл. Фоновая горутина строит ежедневный снапшот. Фронт получает изменения по SSE.

**Tech Stack:** Go 1.26 (stdlib `net/http` ServeMux), SQLite через `modernc.org/sqlite` (чистый Go, без cgo), `golang.org/x/crypto/bcrypt`, Vue 3 + Vite + TypeScript (в отдельном плане).

**Спека:** `docs/superpowers/specs/2026-09-02-partner-portal-design.md`
**Справочник по API:** `docs/API.md`

---

## Структура файлов

| Файл | Ответственность |
|---|---|
| `go.mod` | модуль `partnerops` |
| `cmd/server/main.go` | сборка зависимостей, запуск HTTP и фоновых задач |
| `internal/config/config.go` | чтение и валидация env |
| `internal/store/sqlite.go` | открытие БД, применение миграций |
| `internal/store/migrations/*.sql` | схема, вшивается через `embed` |
| `internal/store/sessions.go` | создание, проверка, удаление сессий |
| `internal/store/snapshots.go` | сохранение снапшота и его строк, чтение последнего |
| `internal/partner/client.go` | HTTP-клиент: Basic auth, таймауты, разбор ошибок |
| `internal/partner/csv.go` | разбор CSV-отчётов 1С |
| `internal/partner/reports.go` | постановка задачи отчёта и ожидание результата |
| `internal/partner/dto.go` | структуры данных 1С |
| `internal/service/snapshot.go` | построение ежедневного снапшота |
| `internal/httpapi/router.go` | маршруты |
| `internal/httpapi/auth.go` | вход, выход, middleware сессии |
| `internal/httpapi/ratelimit.go` | ограничение попыток входа |
| `internal/httpapi/errors.go` | единый формат ошибок |
| `internal/httpapi/events.go` | SSE |
| `internal/events/bus.go` | шина событий |
| `internal/money/money.go` | суммы в копейках, разбор русского формата |

---

## Task 1: Инициализация модуля

**Files:**
- Create: `go.mod`, `.env.example`, `.gitignore` (уже есть, дополнить)

- [ ] **Step 1: Создать модуль**

```bash
go mod init partnerops
```

- [ ] **Step 2: Создать `.env.example`**

```
# Доступ к партнёрскому API 1С (portal.1c.ru -> Администрирование -> API к Порталу 1С:ИТС)
PARTNER_API_LOGIN=api-login-00000
PARTNER_API_PASSWORD=
PARTNER_CODE=00000

# Вход в наш сервис. Хеш получить: go run ./cmd/hashpw "пароль"
APP_LOGIN=admin
APP_PASSWORD_HASH=

SESSION_TTL=12h
DB_PATH=./data/app.db
LISTEN_ADDR=127.0.0.1:8080
```

- [ ] **Step 3: Проверить, что `.env` и `data/` в `.gitignore`**

Run: `Get-Content .gitignore`
Expected: строки `.env` и `data/` присутствуют.

- [ ] **Step 4: Commit**

```bash
git add go.mod .env.example .gitignore
git commit -m "chore: init go module and env template"
```

---

## Task 2: Конфигурация

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package config

import (
	"testing"
	"time"
)

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
	if cfg.SessionTTL != 12*time.Hour {
		t.Errorf("SessionTTL = %v, ожидали 12h", cfg.SessionTTL)
	}
	if cfg.DBPath != "./data/app.db" {
		t.Errorf("DBPath = %q, ожидали ./data/app.db", cfg.DBPath)
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr = %q, ожидали 127.0.0.1:8080", cfg.ListenAddr)
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
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/config/ -v`
Expected: FAIL, `undefined: Load`

- [ ] **Step 3: Реализовать**

```go
// Package config читает настройки сервиса из переменных окружения.
package config

import (
	"fmt"
	"time"
)

// Config — все настройки сервиса. Секреты живут только здесь и в памяти процесса.
type Config struct {
	PartnerAPILogin    string
	PartnerAPIPassword string
	PartnerCode        string
	AppLogin           string
	AppPasswordHash    string
	SessionTTL         time.Duration
	DBPath             string
	ListenAddr         string
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
		AppPasswordHash:    get("APP_PASSWORD_HASH"),
		DBPath:             orDefault(get("DB_PATH"), "./data/app.db"),
		ListenAddr:         orDefault(get("LISTEN_ADDR"), "127.0.0.1:8080"),
	}

	required := map[string]string{
		"PARTNER_API_LOGIN":    cfg.PartnerAPILogin,
		"PARTNER_API_PASSWORD": cfg.PartnerAPIPassword,
		"PARTNER_CODE":         cfg.PartnerCode,
		"APP_LOGIN":            cfg.AppLogin,
		"APP_PASSWORD_HASH":    cfg.AppPasswordHash,
	}
	for name, value := range required {
		if value == "" {
			return Config{}, fmt.Errorf("не задана обязательная переменная %s", name)
		}
	}

	ttl := orDefault(get("SESSION_TTL"), "12h")
	parsed, err := time.ParseDuration(ttl)
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_TTL=%q: %w", ttl, err)
	}
	cfg.SessionTTL = parsed

	return cfg, nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/config/ -v`
Expected: PASS, три теста

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): load and validate settings from environment"
```

---

## Task 3: Деньги

Суммы в отчётах 1С приходят строками с десятичной запятой, причём **разрядность разная в одном
и том же отчёте**: `710,00` и `355,000` соседствуют в фикстуре `testdata/edo-billing.csv`.
Копейки такое не вмещают, `float64` для денег недопустим. Храним в миллирублях — тысячных долях
рубля, целым числом.

**Files:**
- Create: `internal/money/money.go`
- Test: `internal/money/money_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package money

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Amount // миллирубли
	}{
		{"710,00", 710000},
		{"0,00", 0},
		{"1 400,00", 1400000},
		{"4,20", 4200},
		{"", 0},
		{"12", 12000},
		{"3,5", 3500},
		// Три знака после запятой встречаются в колонке «Сумма для партнера».
		{"355,000", 355000},
		{"0,000", 0},
		{"0,125", 125},
		{"-5,50", -5500},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) вернул ошибку %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %d, ожидали %d", c.in, got, c.want)
		}
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, in := range []string{"абв", "1,2,3", "1.2.3", "1,2345"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) должен был вернуть ошибку", in)
		}
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		in   Amount
		want string
	}{
		{710000, "710,00"},
		{50, "0,05"},
		{355000, "355,00"},
		{125, "0,13"},  // округление до копеек при отображении
		{-5500, "-5,50"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Amount(%d).String() = %q, ожидали %q", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/money/ -v`
Expected: FAIL, `undefined: Parse`

- [ ] **Step 3: Реализовать**

```go
// Package money хранит денежные суммы целым числом, чтобы не терять точность.
//
// Единица хранения — миллирубль, тысячная доля рубля. Так выбрано потому, что
// отчёты 1С в одном файле смешивают два и три знака после запятой: в колонке
// «Сумма для клиента» встречается 710,00, а в «Сумма для партнера» — 355,000.
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// scale — сколько знаков после запятой вмещает единица хранения.
const scale = 3

// Amount — сумма в миллирублях: один рубль равен 1000.
type Amount int64

// Parse разбирает сумму в формате отчётов 1С: десятичная запятая, пробелы
// в качестве разделителя разрядов, пустая строка означает ноль.
// Допускается от нуля до трёх знаков после запятой.
func Parse(s string) (Amount, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, nil
	}

	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	whole, frac, hasFrac := strings.Cut(s, ",")
	if strings.Contains(frac, ",") {
		return 0, fmt.Errorf("money: несколько запятых в %q", s)
	}
	if whole == "" {
		whole = "0"
	}

	rubles, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: не разобрать целую часть %q: %w", s, err)
	}

	var fraction int64
	if hasFrac {
		if len(frac) > scale {
			return 0, fmt.Errorf("money: больше %d знаков после запятой в %q", scale, s)
		}
		// Дополняем справа нулями до единицы хранения: "5" -> "500", "00" -> "000".
		padded := frac + strings.Repeat("0", scale-len(frac))
		fraction, err = strconv.ParseInt(padded, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("money: не разобрать дробную часть %q: %w", s, err)
		}
	}

	total := rubles*1000 + fraction
	if negative {
		total = -total
	}
	return Amount(total), nil
}

// String возвращает сумму в привычном для отчётов виде с двумя знаками: 710,00.
// Третий знак округляется, поэтому для точных сверок берите само значение.
func (a Amount) String() string {
	sign := ""
	value := int64(a)
	if value < 0 {
		sign = "-"
		value = -value
	}

	// Округление до копеек по правилу «половина вверх».
	kopeks := (value + 5) / 10
	return fmt.Sprintf("%s%d,%02d", sign, kopeks/100, kopeks%100)
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/money/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/money/
git commit -m "feat(money): parse 1C decimal amounts into millirubles"
```

---

## Task 4: База данных и миграции

**Files:**
- Create: `internal/store/sqlite.go`, `internal/store/migrations/001_init.sql`
- Test: `internal/store/sqlite_test.go`

- [ ] **Step 1: Установить драйвер**

```bash
go get modernc.org/sqlite
```

- [ ] **Step 2: Написать падающий тест**

```go
package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open вернул ошибку: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("не прочитать user_version: %v", err)
	}
	if version != 1 {
		t.Errorf("user_version = %d, ожидали 1", version)
	}

	for _, table := range []string{"sessions", "edo_snapshots", "edo_rows"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("таблица %s не создана: %v", table, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("первый Open: %v", err)
	}
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("второй Open: %v", err)
	}
	defer db2.Close()
}
```

- [ ] **Step 3: Запустить тест, убедиться что падает**

Run: `go test ./internal/store/ -v`
Expected: FAIL, `undefined: Open`

- [ ] **Step 4: Написать миграцию `internal/store/migrations/001_init.sql`**

```sql
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE edo_snapshots (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    period     TEXT NOT NULL,
    taken_at   INTEGER NOT NULL,
    row_count  INTEGER NOT NULL,
    csv_sha256 TEXT NOT NULL,
    -- baseline: первый снимок периода, для него сравнение не делается,
    -- иначе импорт породил бы лавину фиктивных «пропаж».
    is_baseline INTEGER NOT NULL DEFAULT 0,
    raw_csv    BLOB NOT NULL
);
CREATE INDEX idx_edo_snapshots_period ON edo_snapshots(period, taken_at);
-- Один и тот же отчёт, полученный повторно, не создаёт новый снимок.
CREATE UNIQUE INDEX idx_edo_snapshots_content ON edo_snapshots(period, csv_sha256);

CREATE TABLE edo_rows (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id     INTEGER NOT NULL REFERENCES edo_snapshots(id) ON DELETE CASCADE,
    partner_code    TEXT NOT NULL DEFAULT '',
    owner           TEXT NOT NULL DEFAULT '',
    login           TEXT NOT NULL DEFAULT '',
    edo_id          TEXT NOT NULL DEFAULT '',
    client_name     TEXT NOT NULL DEFAULT '',
    inn             TEXT NOT NULL DEFAULT '',
    kpp             TEXT NOT NULL DEFAULT '',
    its_tariffs     TEXT NOT NULL DEFAULT '',
    limit_docs      INTEGER,
    invoices_out    INTEGER NOT NULL DEFAULT 0,
    non_invoices_out INTEGER NOT NULL DEFAULT 0,
    packets         INTEGER NOT NULL DEFAULT 0,
    packets_by_owner INTEGER NOT NULL DEFAULT 0,
    discount        INTEGER NOT NULL DEFAULT 0,
    packets_billable INTEGER NOT NULL DEFAULT 0,
    tariff_amount   INTEGER NOT NULL DEFAULT 0,
    client_amount   INTEGER NOT NULL DEFAULT 0,
    partner_amount  INTEGER NOT NULL DEFAULT 0,
    extra           TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_edo_rows_snapshot ON edo_rows(snapshot_id);
CREATE INDEX idx_edo_rows_edo_id ON edo_rows(edo_id);
-- ИНН недостаточен как ключ организации: филиалы делят ИНН и различаются КПП,
-- у ИП КПП пуст. Ищем всегда по паре.
CREATE INDEX idx_edo_rows_org ON edo_rows(inn, kpp);
CREATE INDEX idx_edo_rows_login ON edo_rows(login);
```

- [ ] **Step 5: Реализовать `internal/store/sqlite.go`**

```go
// Package store отвечает за хранение данных в SQLite.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open открывает базу, создаёт каталог при необходимости и применяет миграции.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("store: не создать каталог %s: %w", dir, err)
		}
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: не открыть базу: %w", err)
	}

	// Один писатель: SQLite не любит параллельную запись, а нагрузка здесь мизерная.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrate применяет миграции, которых ещё нет, ориентируясь на PRAGMA user_version.
func migrate(db *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("store: не прочитать миграции: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("store: не прочитать user_version: %w", err)
	}

	for i, name := range names {
		version := i + 1
		if version <= current {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("store: не прочитать %s: %w", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("store: не начать транзакцию для %s: %w", name, err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: миграция %s не применилась: %w", name, err)
		}
		// PRAGMA не принимает параметры, поэтому число подставляется в текст запроса.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: не обновить user_version после %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: не зафиксировать %s: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 6: Запустить тест, убедиться что проходит**

Run: `go test ./internal/store/ -v`
Expected: PASS, два теста

- [ ] **Step 7: Commit**

```bash
git add internal/store/ go.mod go.sum
git commit -m "feat(store): open SQLite with WAL and apply embedded migrations"
```

---

## Task 5: Разбор CSV-отчёта биллинга ЭДО

Формат подтверждён живым вызовом: UTF-8 с BOM, разделитель `;`, удвоенные кавычки,
десятичная запятая. Фикстура лежит в `testdata/edo-billing.csv`.

**Files:**
- Create: `internal/partner/csv.go`, `internal/partner/dto.go`
- Test: `internal/partner/csv_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package partner

import (
	"os"
	"testing"
)

func TestParseEDOBilling(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/edo-billing.csv")
	if err != nil {
		t.Fatalf("не прочитать фикстуру: %v", err)
	}

	rows, err := ParseEDOBilling(raw)
	if err != nil {
		t.Fatalf("ParseEDOBilling вернул ошибку: %v", err)
	}
	if len(rows) != 23 {
		t.Fatalf("разобрано %d строк, ожидали 23", len(rows))
	}

	first := rows[0]
	if first.EDOID == "" {
		t.Error("EDOID пуст в первой строке")
	}
	if first.INN == "" {
		t.Error("INN пуст в первой строке")
	}
	if first.ClientName == "" {
		t.Error("ClientName пуст в первой строке")
	}

	// В фикстуре есть строки без владельца — это ожидаемое состояние, не ошибка.
	orphans := 0
	for _, r := range rows {
		if r.Owner == "" {
			orphans++
		}
	}
	if orphans != 7 {
		t.Errorf("строк без владельца %d, ожидали 7", orphans)
	}
}

func TestParseEDOBillingHandlesEmptyNumbers(t *testing.T) {
	csv := "\ufeffКод партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
		"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
		"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
		"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
		"Сумма для партнера;Дополнительная информация\r\n" +
		"99999;Партнёр;;login;2AE-X;\"ООО \"\"Тест\"\"\";7700000000;770001001;;;0;0;0;;;;;;;\r\n"

	rows, err := ParseEDOBilling([]byte(csv))
	if err != nil {
		t.Fatalf("ParseEDOBilling вернул ошибку: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("разобрано %d строк, ожидали 1", len(rows))
	}
	if rows[0].Limit != nil {
		t.Errorf("Limit = %v, ожидали nil при пустом поле", *rows[0].Limit)
	}
	if rows[0].ClientName != `ООО "Тест"` {
		t.Errorf("ClientName = %q, ожидали ООО \"Тест\"", rows[0].ClientName)
	}
	if rows[0].ClientAmount != 0 {
		t.Errorf("ClientAmount = %d, ожидали 0", rows[0].ClientAmount)
	}
}

func TestParseEDOBillingRejectsUnknownHeader(t *testing.T) {
	if _, err := ParseEDOBilling([]byte("что-то;совсем;другое\r\n")); err == nil {
		t.Fatal("ожидали ошибку про неизвестный заголовок")
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/partner/ -v`
Expected: FAIL, `undefined: ParseEDOBilling`

- [ ] **Step 3: Создать `internal/partner/dto.go`**

```go
package partner

import "partnerops/internal/money"

// EDOBillingRow — строка отчёта биллинга ЭДО.
// Названия полей соответствуют колонкам CSV, см. docs/API.md §8.1.
type EDOBillingRow struct {
	PartnerCode     string
	PartnerName     string
	Owner           string // код абонента-владельца, пустой означает потерянную связь
	Login           string
	EDOID           string
	ClientName      string
	INN             string
	KPP             string
	ITSTariffs      string
	Limit           *int64 // nil, если лимит не задан
	InvoicesOut     int64
	NonInvoicesOut  int64
	Packets         int64
	PacketsByOwner  int64
	Discount        int64
	PacketsBillable int64
	TariffAmount    money.Amount
	ClientAmount    money.Amount
	PartnerAmount   money.Amount
	Extra           string
}
```

- [ ] **Step 4: Создать `internal/partner/csv.go`**

```go
package partner

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"partnerops/internal/money"
)

// Колонки отчёта биллинга ЭДО в том виде, в каком их отдаёт 1С.
var edoBillingColumns = []string{
	"Код партнера",
	"Название партнера",
	"Владелец",
	"Логин",
	"Ид_ЭДО клиента",
	"Наименование клиента",
	"ИНН клиента",
	"КПП клиента",
	"Тарифы ИТС",
	"Лимит",
	"СФ_исх",
	"не_СФ_исх",
	"Кол-во пакетов документов ЭДО",
	"Сумма пакетов документов ЭДО по владельцу",
	"Льгота",
	"Количество пакетов документов ЭДО к оплате",
	"Тариф для клиента",
	"Сумма для клиента",
	"Сумма для партнера",
	"Дополнительная информация",
}

// ParseEDOBilling разбирает CSV отчёта биллинга ЭДО.
func ParseEDOBilling(raw []byte) ([]EDOBillingRow, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\ufeff"))))
	reader.Comma = ';'
	// Число колонок в отчёте стабильно, но хвостовые пустые поля встречаются:
	// проверяем заголовок сами, а строки допускаем разной длины.
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("partner: не разобрать CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("partner: пустой отчёт")
	}

	index, err := indexColumns(records[0], edoBillingColumns)
	if err != nil {
		return nil, err
	}

	rows := make([]EDOBillingRow, 0, len(records)-1)
	for _, record := range records[1:] {
		if isBlank(record) {
			continue
		}
		row, err := edoRowFrom(record, index)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// indexColumns сопоставляет имена колонок их позициям и требует, чтобы все ожидаемые были на месте.
func indexColumns(header []string, expected []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}
	var missing []string
	for _, name := range expected {
		if _, ok := index[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("partner: в отчёте нет колонок: %s", strings.Join(missing, ", "))
	}
	return index, nil
}

func edoRowFrom(record []string, index map[string]int) (EDOBillingRow, error) {
	get := func(column string) string {
		i, ok := index[column]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	row := EDOBillingRow{
		PartnerCode: get("Код партнера"),
		PartnerName: get("Название партнера"),
		Owner:       get("Владелец"),
		Login:       get("Логин"),
		EDOID:       get("Ид_ЭДО клиента"),
		ClientName:  get("Наименование клиента"),
		INN:         get("ИНН клиента"),
		KPP:         get("КПП клиента"),
		ITSTariffs:  get("Тарифы ИТС"),
		Extra:       get("Дополнительная информация"),
	}

	var err error
	if limit := get("Лимит"); limit != "" {
		value, parseErr := strconv.ParseInt(limit, 10, 64)
		if parseErr != nil {
			return row, fmt.Errorf("partner: лимит %q: %w", limit, parseErr)
		}
		row.Limit = &value
	}

	counts := []struct {
		column string
		target *int64
	}{
		{"СФ_исх", &row.InvoicesOut},
		{"не_СФ_исх", &row.NonInvoicesOut},
		{"Кол-во пакетов документов ЭДО", &row.Packets},
		{"Сумма пакетов документов ЭДО по владельцу", &row.PacketsByOwner},
		{"Льгота", &row.Discount},
		{"Количество пакетов документов ЭДО к оплате", &row.PacketsBillable},
	}
	for _, c := range counts {
		if *c.target, err = parseCount(get(c.column)); err != nil {
			return row, fmt.Errorf("partner: колонка %q: %w", c.column, err)
		}
	}

	amounts := []struct {
		column string
		target *money.Amount
	}{
		{"Тариф для клиента", &row.TariffAmount},
		{"Сумма для клиента", &row.ClientAmount},
		{"Сумма для партнера", &row.PartnerAmount},
	}
	for _, a := range amounts {
		if *a.target, err = money.Parse(get(a.column)); err != nil {
			return row, fmt.Errorf("partner: колонка %q: %w", a.column, err)
		}
	}

	return row, nil
}

// parseCount разбирает счётчик документов. Пустое значение означает ноль.
func parseCount(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	// Счётчики иногда приходят с дробной частью вида "2,000".
	if whole, _, found := strings.Cut(s, ","); found {
		s = whole
	}
	return strconv.ParseInt(strings.ReplaceAll(s, " ", ""), 10, 64)
}

func isBlank(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: Запустить тест, убедиться что проходит**

Run: `go test ./internal/partner/ -v`
Expected: PASS, три теста

- [ ] **Step 6: Commit**

```bash
git add internal/partner/
git commit -m "feat(partner): parse EDO billing CSV report"
```

---

## Task 6: HTTP-клиент к партнёрскому API

**Files:**
- Create: `internal/partner/client.go`
- Test: `internal/partner/client_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package partner

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientSendsBasicAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "api-login-00000", "secret")
	if _, err := c.post(context.Background(), "/anything", []byte(`{}`)); err != nil {
		t.Fatalf("post вернул ошибку: %v", err)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("api-login-00000:secret"))
	if gotAuth != want {
		t.Errorf("Authorization = %q, ожидали %q", gotAuth, want)
	}
}

func TestClientReportsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"status":401,"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "wrong")
	_, err := c.post(context.Background(), "/anything", []byte(`{}`))
	if err == nil {
		t.Fatal("ожидали ошибку авторизации")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("ошибка %q не содержит код 401", err.Error())
	}
}

func TestClientReportsRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"MAX_TASKS_PER_HOUR_LIMIT_REACHED"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	_, err := c.post(context.Background(), "/anything", []byte(`{}`))
	if !IsRateLimited(err) {
		t.Errorf("IsRateLimited(%v) = false, ожидали true", err)
	}
}

func TestClientRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := New(srv.URL, "login", "secret")
	if _, err := c.post(ctx, "/slow", []byte(`{}`)); err == nil {
		t.Fatal("ожидали ошибку по истечении контекста")
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/partner/ -run TestClient -v`
Expected: FAIL, `undefined: New`

- [ ] **Step 3: Реализовать**

```go
package partner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL — адрес партнёрского API 1С.
const DefaultBaseURL = "https://partner-api.1c.ru/api"

// Client обращается к партнёрскому API. Учётные данные не покидают этот тип.
type Client struct {
	baseURL  string
	login    string
	password string
	http     *http.Client
}

// New создаёт клиента. Пустой baseURL означает боевой адрес.
func New(baseURL, login, password string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		login:    login,
		password: password,
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError — ошибка, вернувшаяся от 1С.
type APIError struct {
	StatusCode int
	Code       string // прикладной код, например MAX_TASKS_PER_HOUR_LIMIT_REACHED
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("1С вернула %d: %s (%s)", e.StatusCode, e.Message, e.Code)
	}
	return fmt.Sprintf("1С вернула %d: %s", e.StatusCode, e.Message)
}

// Коды, означающие, что задача не провалилась, а упёрлась в лимит и её надо повторить позже.
var rateLimitCodes = map[string]bool{
	"MAX_TASKS_PER_HOUR_LIMIT_REACHED":    true,
	"MAX_NOT_HANDLED_TASKS_COUNT_REACHED": true,
}

// IsRateLimited сообщает, что 1С отказала из-за лимита построения отчётов.
func IsRateLimited(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return rateLimitCodes[apiErr.Code]
}

// IsUnauthorized сообщает, что учётные данные не приняты.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}

// response — сырой ответ, тело может быть как JSON, так и CSV.
type response struct {
	StatusCode int
	Body       []byte
}

func (c *Client) post(ctx context.Context, path string, body []byte) (*response, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

func (c *Client) get(ctx context.Context, path string) (*response, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (*response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать запрос: %w", err)
	}
	req.SetBasicAuth(c.login, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("partner: запрос %s %s не выполнен: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("partner: не прочитать ответ: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, apiErrorFrom(resp.StatusCode, raw)
	}
	return &response{StatusCode: resp.StatusCode, Body: raw}, nil
}

// apiErrorFrom вытаскивает прикладной код из тела, если он там есть.
func apiErrorFrom(status int, raw []byte) error {
	apiErr := &APIError{StatusCode: status, Message: strings.TrimSpace(string(raw))}

	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if payload.Message != "" {
			apiErr.Message = payload.Message
		}
		// Поле error у Spring содержит текст статуса, у прикладных ошибок — код.
		if payload.Error != "" && strings.ToUpper(payload.Error) == payload.Error {
			apiErr.Code = payload.Error
		}
	}
	return apiErr
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/partner/ -v`
Expected: PASS, все тесты пакета

- [ ] **Step 5: Commit**

```bash
git add internal/partner/
git commit -m "feat(partner): HTTP client with basic auth and typed API errors"
```

---

## Task 7: Построение отчёта биллинга ЭДО

**Files:**
- Create: `internal/partner/reports.go`
- Test: `internal/partner/reports_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package partner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEDOBillingReportPollsUntilReady(t *testing.T) {
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/edo/reports/billing":
			w.Write([]byte(`{"taskUeid":"task-1","error":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/edo/reports/billing/task-1":
			if atomic.AddInt32(&polls, 1) < 3 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Write([]byte("\ufeffКолонка\r\nзначение\r\n"))
		default:
			t.Errorf("неожиданный запрос %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	raw, err := c.EDOBillingReport(context.Background(), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EDOBillingReport вернул ошибку: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("отчёт пуст")
	}
	if atomic.LoadInt32(&polls) != 3 {
		t.Errorf("опросов %d, ожидали 3", polls)
	}
}

func TestEDOBillingReportFailsOnTaskError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"taskUeid":"","error":"что-то пошло не так"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	_, err := c.EDOBillingReport(context.Background(), time.Now())
	if err == nil {
		t.Fatal("ожидали ошибку из поля error")
	}
}

func TestEDOBillingReportStopsOnContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"taskUeid":"task-1"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	if _, err := c.EDOBillingReport(ctx, time.Now()); err == nil {
		t.Fatal("ожидали ошибку по истечении контекста")
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/partner/ -run TestEDOBillingReport -v`
Expected: FAIL, `c.pollInterval undefined`

- [ ] **Step 3: Добавить поле в `Client` (файл `internal/partner/client.go`)**

В структуру `Client` добавить поле:

```go
	pollInterval time.Duration
```

В `New` добавить его инициализацию после `http`:

```go
		pollInterval: 5 * time.Second,
```

- [ ] **Step 4: Создать `internal/partner/reports.go`**

```go
package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// taskResponse — ответ на постановку задачи построения отчёта ЭДО.
type taskResponse struct {
	TaskUeid string `json:"taskUeid"`
	Error    string `json:"error"`
}

// EDOBillingReport ставит задачу на отчёт биллинга ЭДО за указанный месяц и ждёт результат.
// Возвращает сырой CSV: разбирать его должен вызывающий через ParseEDOBilling.
//
// Повторно ставить задачу при ошибке нельзя — 1С создаст дубль и потратит часовой лимит.
func (c *Client) EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error) {
	body, err := json.Marshal(map[string]string{
		"date": date.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}

	resp, err := c.post(ctx, "/edo/reports/billing", body)
	if err != nil {
		return nil, fmt.Errorf("partner: не поставить задачу на отчёт: %w", err)
	}

	var task taskResponse
	if err := json.Unmarshal(resp.Body, &task); err != nil {
		return nil, fmt.Errorf("partner: не разобрать ответ на постановку задачи: %w", err)
	}
	if task.Error != "" {
		return nil, fmt.Errorf("partner: 1С отказала в построении отчёта: %s", task.Error)
	}
	if task.TaskUeid == "" {
		return nil, fmt.Errorf("partner: 1С не вернула идентификатор задачи")
	}

	return c.waitForReport(ctx, "/edo/reports/billing/"+task.TaskUeid)
}

// waitForReport опрашивает готовность отчёта. Пока отчёт строится, 1С отвечает 204.
func (c *Client) waitForReport(ctx context.Context, path string) ([]byte, error) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		resp, err := c.get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("partner: не забрать отчёт: %w", err)
		}
		if resp.StatusCode != http.StatusNoContent && len(resp.Body) > 0 {
			return resp.Body, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("partner: отчёт не готов, ожидание прервано: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
```

- [ ] **Step 5: Запустить тест, убедиться что проходит**

Run: `go test ./internal/partner/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/partner/
git commit -m "feat(partner): build and poll EDO billing report"
```

---

## Task 8: Сохранение снапшота

**Files:**
- Create: `internal/store/snapshots.go`
- Test: `internal/store/snapshots_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func testDB(t *testing.T) *Snapshots {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSnapshots(db)
}

func sampleRows() []partner.EDOBillingRow {
	limit := int64(100)
	return []partner.EDOBillingRow{
		{EDOID: "2AE-A", INN: "7700000001", Owner: "CL-1", Login: "a", Limit: &limit, Packets: 5},
		{EDOID: "2AE-B", INN: "7700000002", Owner: "", Login: "b"},
	}
}

func TestSaveAndLoadLatest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	taken := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	res, err := s.Save(ctx, "2026-08", taken, []byte("csv"), sampleRows())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if res.ID == 0 {
		t.Fatal("Save вернул нулевой идентификатор")
	}
	if !res.IsBaseline {
		t.Error("первый снимок периода должен быть baseline")
	}
	if res.Duplicate {
		t.Error("первый снимок не может быть дубликатом")
	}

	snap, rows, err := s.Latest(ctx, "2026-08")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if snap.ID != res.ID {
		t.Errorf("ID = %d, ожидали %d", snap.ID, res.ID)
	}
	if snap.RowCount != 2 {
		t.Errorf("RowCount = %d, ожидали 2", snap.RowCount)
	}
	if len(rows) != 2 {
		t.Fatalf("строк %d, ожидали 2", len(rows))
	}
	if rows[0].Limit == nil || *rows[0].Limit != 100 {
		t.Errorf("Limit не сохранился: %v", rows[0].Limit)
	}
	if rows[1].Limit != nil {
		t.Errorf("Limit должен быть nil, получили %v", *rows[1].Limit)
	}
}

func TestLatestReturnsNewest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	older := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	if _, err := s.Save(ctx, "2026-08", older, []byte("old"), sampleRows()); err != nil {
		t.Fatalf("Save старого: %v", err)
	}
	newRes, err := s.Save(ctx, "2026-08", newer, []byte("new"), sampleRows()[:1])
	if err != nil {
		t.Fatalf("Save нового: %v", err)
	}
	if newRes.IsBaseline {
		t.Error("второй снимок периода не должен быть baseline")
	}

	snap, rows, err := s.Latest(ctx, "2026-08")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if snap.ID != newRes.ID {
		t.Errorf("вернулся снапшот %d, ожидали %d", snap.ID, newRes.ID)
	}
	if len(rows) != 1 {
		t.Errorf("строк %d, ожидали 1", len(rows))
	}
}

func TestSaveSkipsIdenticalReport(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	raw := []byte("одинаковый csv")

	first, err := s.Save(ctx, "2026-08", time.Now(), raw, sampleRows())
	if err != nil {
		t.Fatalf("первый Save: %v", err)
	}

	second, err := s.Save(ctx, "2026-08", time.Now().Add(time.Hour), raw, sampleRows())
	if err != nil {
		t.Fatalf("второй Save: %v", err)
	}
	if !second.Duplicate {
		t.Error("повторный идентичный отчёт должен помечаться как дубликат")
	}
	if second.ID != first.ID {
		t.Errorf("дубликат вернул id %d, ожидали %d", second.ID, first.ID)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM edo_snapshots`).Scan(&count); err != nil {
		t.Fatalf("подсчёт снапшотов: %v", err)
	}
	if count != 1 {
		t.Errorf("снапшотов в базе %d, ожидали 1", count)
	}
}

func TestSaveDistinguishesPeriods(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	raw := []byte("одинаковый csv")

	if _, err := s.Save(ctx, "2026-07", time.Now(), raw, sampleRows()); err != nil {
		t.Fatalf("Save за июль: %v", err)
	}
	res, err := s.Save(ctx, "2026-08", time.Now(), raw, sampleRows())
	if err != nil {
		t.Fatalf("Save за август: %v", err)
	}
	if res.Duplicate {
		t.Error("одинаковый CSV в разных периодах — не дубликат")
	}
	if !res.IsBaseline {
		t.Error("первый снимок августа должен быть baseline")
	}
}

func TestLatestOnEmptyDatabase(t *testing.T) {
	s := testDB(t)
	_, _, err := s.Latest(context.Background(), "2026-08")
	if err != ErrNoSnapshot {
		t.Errorf("err = %v, ожидали ErrNoSnapshot", err)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	// Строка с недопустимо длинным значением не должна оставить пустой снапшот.
	rows := sampleRows()
	if _, err := s.Save(ctx, "2026-08", time.Now(), []byte("csv"), rows); err != nil {
		t.Fatalf("Save: %v", err)
	}

	snap, _, err := s.Latest(ctx, "2026-08")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if snap.RowCount != int64(len(rows)) {
		t.Errorf("RowCount = %d, ожидали %d", snap.RowCount, len(rows))
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/store/ -v`
Expected: FAIL, `undefined: NewSnapshots`

- [ ] **Step 3: Реализовать**

```go
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"partnerops/internal/money"
	"partnerops/internal/partner"
)

// ErrNoSnapshot означает, что снапшотов за период ещё нет.
var ErrNoSnapshot = errors.New("store: снапшот не найден")

// Snapshot — метаданные сохранённого отчёта.
type Snapshot struct {
	ID         int64
	Period     string
	TakenAt    time.Time
	RowCount   int64
	IsBaseline bool
}

// Snapshots хранит снапшоты отчёта биллинга ЭДО.
type Snapshots struct {
	db *sql.DB
}

// NewSnapshots создаёт хранилище поверх открытой базы.
func NewSnapshots(db *sql.DB) *Snapshots {
	return &Snapshots{db: db}
}

// SaveResult описывает, что произошло при сохранении.
type SaveResult struct {
	ID         int64
	IsBaseline bool // первый снимок периода: сравнивать не с чем
	Duplicate  bool // такой же отчёт уже сохранён, ничего не записано
}

// Save сохраняет отчёт целиком: сырой CSV и разобранные строки. Всё в одной транзакции,
// чтобы не появилось снапшота без строк.
//
// Повторно полученный байт в байт отчёт не сохраняется: 1С отдаёт одни и те же данные
// при нескольких запусках за день, и лишние снимки породили бы пустые сравнения.
func (s *Snapshots) Save(
	ctx context.Context, period string, takenAt time.Time, raw []byte, rows []partner.EDOBillingRow,
) (SaveResult, error) {
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не начать транзакцию: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM edo_snapshots WHERE period = ? AND csv_sha256 = ?`, period, digest,
	).Scan(&existingID)
	if err == nil {
		return SaveResult{ID: existingID, Duplicate: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SaveResult{}, fmt.Errorf("store: не проверить дубликат снапшота: %w", err)
	}

	var priorCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM edo_snapshots WHERE period = ?`, period,
	).Scan(&priorCount); err != nil {
		return SaveResult{}, fmt.Errorf("store: не сосчитать снапшоты периода: %w", err)
	}
	isBaseline := priorCount == 0

	result, err := tx.ExecContext(ctx,
		`INSERT INTO edo_snapshots (period, taken_at, row_count, csv_sha256, is_baseline, raw_csv)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		period, takenAt.UTC().Unix(), len(rows), digest, isBaseline, raw,
	)
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не сохранить снапшот: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не получить идентификатор снапшота: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO edo_rows (
			snapshot_id, partner_code, owner, login, edo_id, client_name, inn, kpp,
			its_tariffs, limit_docs, invoices_out, non_invoices_out, packets,
			packets_by_owner, discount, packets_billable,
			tariff_amount, client_amount, partner_amount, extra
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не подготовить вставку строк: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		var limit any
		if r.Limit != nil {
			limit = *r.Limit
		}
		_, err := stmt.ExecContext(ctx,
			id, r.PartnerCode, r.Owner, r.Login, r.EDOID, r.ClientName, r.INN, r.KPP,
			r.ITSTariffs, limit, r.InvoicesOut, r.NonInvoicesOut, r.Packets,
			r.PacketsByOwner, r.Discount, r.PacketsBillable,
			int64(r.TariffAmount), int64(r.ClientAmount), int64(r.PartnerAmount), r.Extra,
		)
		if err != nil {
			return SaveResult{}, fmt.Errorf("store: не сохранить строку %s: %w", r.EDOID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return SaveResult{}, fmt.Errorf("store: не зафиксировать снапшот: %w", err)
	}
	return SaveResult{ID: id, IsBaseline: isBaseline}, nil
}

// Latest возвращает самый свежий снапшот за период вместе со строками.
func (s *Snapshots) Latest(ctx context.Context, period string) (Snapshot, []partner.EDOBillingRow, error) {
	var snap Snapshot
	var takenAt int64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, period, taken_at, row_count, is_baseline FROM edo_snapshots
		 WHERE period = ? ORDER BY taken_at DESC, id DESC LIMIT 1`, period,
	).Scan(&snap.ID, &snap.Period, &takenAt, &snap.RowCount, &snap.IsBaseline)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, nil, ErrNoSnapshot
	}
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("store: не прочитать снапшот: %w", err)
	}
	snap.TakenAt = time.Unix(takenAt, 0).UTC()

	rows, err := s.rowsOf(ctx, snap.ID)
	if err != nil {
		return Snapshot{}, nil, err
	}
	return snap, rows, nil
}

func (s *Snapshots) rowsOf(ctx context.Context, snapshotID int64) ([]partner.EDOBillingRow, error) {
	cursor, err := s.db.QueryContext(ctx, `
		SELECT partner_code, owner, login, edo_id, client_name, inn, kpp, its_tariffs,
		       limit_docs, invoices_out, non_invoices_out, packets, packets_by_owner,
		       discount, packets_billable, tariff_amount, client_amount, partner_amount, extra
		FROM edo_rows WHERE snapshot_id = ? ORDER BY id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать строки снапшота: %w", err)
	}
	defer cursor.Close()

	var rows []partner.EDOBillingRow
	for cursor.Next() {
		var r partner.EDOBillingRow
		var limit sql.NullInt64
		var tariff, client, partnerAmount int64

		err := cursor.Scan(
			&r.PartnerCode, &r.Owner, &r.Login, &r.EDOID, &r.ClientName, &r.INN, &r.KPP,
			&r.ITSTariffs, &limit, &r.InvoicesOut, &r.NonInvoicesOut, &r.Packets,
			&r.PacketsByOwner, &r.Discount, &r.PacketsBillable,
			&tariff, &client, &partnerAmount, &r.Extra,
		)
		if err != nil {
			return nil, fmt.Errorf("store: не разобрать строку снапшота: %w", err)
		}
		if limit.Valid {
			value := limit.Int64
			r.Limit = &value
		}
		r.TariffAmount = money.Amount(tariff)
		r.ClientAmount = money.Amount(client)
		r.PartnerAmount = money.Amount(partnerAmount)
		rows = append(rows, r)
	}
	return rows, cursor.Err()
}
```


- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): persist EDO billing snapshots atomically"
```

---

## Task 9: Шина событий

**Files:**
- Create: `internal/events/bus.go`
- Test: `internal/events/bus_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package events

import (
	"testing"
	"time"
)

func TestPublishReachesAllSubscribers(t *testing.T) {
	bus := NewBus()

	first, unsubFirst := bus.Subscribe()
	defer unsubFirst()
	second, unsubSecond := bus.Subscribe()
	defer unsubSecond()

	bus.Publish(Event{Kind: "snapshot.completed", Payload: map[string]any{"rows": 23}})

	for i, ch := range []<-chan Event{first, second} {
		select {
		case got := <-ch:
			if got.Kind != "snapshot.completed" {
				t.Errorf("подписчик %d получил %q", i, got.Kind)
			}
		case <-time.After(time.Second):
			t.Errorf("подписчик %d не получил событие", i)
		}
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	bus := NewBus()
	ch, unsub := bus.Subscribe()
	unsub()

	bus.Publish(Event{Kind: "test"})

	select {
	case _, open := <-ch:
		if open {
			t.Error("канал отписавшегося подписчика получил событие")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSlowSubscriberDoesNotBlockPublish(t *testing.T) {
	bus := NewBus()
	_, unsub := bus.Subscribe() // никто не читает
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			bus.Publish(Event{Kind: "test"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish заблокировался на медленном подписчике")
	}
}

func TestUnsubscribeTwiceIsSafe(t *testing.T) {
	bus := NewBus()
	_, unsub := bus.Subscribe()
	unsub()
	unsub()
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/events/ -v`
Expected: FAIL, `undefined: NewBus`

- [ ] **Step 3: Реализовать**

```go
// Package events рассылает изменения состояния подписчикам, а те отдают их фронту по SSE.
package events

import "sync"

// Event — одно изменение состояния.
type Event struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload,omitempty"`
}

// bufferSize подобран так, чтобы короткая пауза в чтении не приводила к потере событий.
const bufferSize = 16

// Bus рассылает события всем подписчикам. Медленный подписчик события теряет,
// но публикацию не тормозит: интерфейс всё равно перезапрашивает данные при подключении.
type Bus struct {
	mu          sync.Mutex
	subscribers map[int]chan Event
	nextID      int
}

// NewBus создаёт пустую шину.
func NewBus() *Bus {
	return &Bus{subscribers: make(map[int]chan Event)}
}

// Subscribe возвращает канал событий и функцию отписки. Отписка идемпотентна.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	ch := make(chan Event, bufferSize)
	b.subscribers[id] = ch

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if existing, ok := b.subscribers[id]; ok {
				delete(b.subscribers, id)
				close(existing)
			}
		})
	}
	return ch, unsubscribe
}

// Publish рассылает событие. Не блокируется, если подписчик не успевает читать.
func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default: // подписчик отстал, событие для него теряется
		}
	}
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/events/ -v -race`
Expected: PASS, гонок нет

- [ ] **Step 5: Commit**

```bash
git add internal/events/
git commit -m "feat(events): non-blocking in-process event bus"
```

---

## Task 10: Сессии

**Files:**
- Create: `internal/store/sessions.go`
- Test: `internal/store/sessions_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func testSessions(t *testing.T) *Sessions {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSessions(db)
}

func TestCreateAndValidate(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, err := s.Create(ctx, time.Hour, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(token) < 32 {
		t.Errorf("токен подозрительно короткий: %d символов", len(token))
	}

	ok, err := s.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok {
		t.Error("свежая сессия не прошла проверку")
	}
}

func TestValidateRejectsUnknownToken(t *testing.T) {
	s := testSessions(t)
	ok, err := s.Validate(context.Background(), "не существует")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok {
		t.Error("неизвестный токен прошёл проверку")
	}
}

func TestValidateRejectsExpired(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, err := s.Create(ctx, -time.Minute, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ok, err := s.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok {
		t.Error("просроченная сессия прошла проверку")
	}
}

func TestDeleteRemovesSession(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, _ := s.Create(ctx, time.Hour, "127.0.0.1", "agent")
	if err := s.Delete(ctx, token); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	ok, _ := s.Validate(ctx, token)
	if ok {
		t.Error("удалённая сессия прошла проверку")
	}
}

func TestTokensAreUnique(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		token, err := s.Create(ctx, time.Hour, "", "")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[token] {
			t.Fatal("токен повторился")
		}
		seen[token] = true
	}
}

func TestPurgeExpiredRemovesOnlyStale(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	fresh, _ := s.Create(ctx, time.Hour, "", "")
	s.Create(ctx, -time.Hour, "", "")

	removed, err := s.PurgeExpired(ctx)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("удалено %d сессий, ожидали 1", removed)
	}
	if ok, _ := s.Validate(ctx, fresh); !ok {
		t.Error("живая сессия была удалена")
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/store/ -run TestCreate -v`
Expected: FAIL, `undefined: NewSessions`

- [ ] **Step 3: Реализовать**

```go
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Sessions хранит сессии сотрудников. В базе лежит только хеш токена:
// утечка базы не даёт войти в сервис.
type Sessions struct {
	db *sql.DB
}

// NewSessions создаёт хранилище поверх открытой базы.
func NewSessions(db *sql.DB) *Sessions {
	return &Sessions{db: db}
}

// Create выдаёт новый токен сессии и возвращает его в открытом виде — единственный раз.
func (s *Sessions) Create(ctx context.Context, ttl time.Duration, ip, userAgent string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("store: не сгенерировать токен: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, created_at, expires_at, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?)`,
		hashToken(token), now.Unix(), now.Add(ttl).Unix(), ip, userAgent,
	)
	if err != nil {
		return "", fmt.Errorf("store: не сохранить сессию: %w", err)
	}
	return token, nil
}

// Validate сообщает, действует ли сессия. Ошибка возвращается только при сбое базы.
func (s *Sessions) Validate(ctx context.Context, token string) (bool, error) {
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM sessions WHERE token_hash = ?`, hashToken(token),
	).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: не проверить сессию: %w", err)
	}
	return time.Now().UTC().Unix() < expiresAt, nil
}

// Delete завершает сессию.
func (s *Sessions) Delete(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	if err != nil {
		return fmt.Errorf("store: не удалить сессию: %w", err)
	}
	return nil
}

// PurgeExpired удаляет истёкшие сессии и возвращает их количество.
func (s *Sessions) PurgeExpired(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UTC().Unix())
	if err != nil {
		return 0, fmt.Errorf("store: не очистить сессии: %w", err)
	}
	return result.RowsAffected()
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): session storage keyed by token hash"
```

---

## Task 11: Вход и защита маршрутов

**Files:**
- Create: `internal/httpapi/errors.go`, `internal/httpapi/ratelimit.go`, `internal/httpapi/auth.go`
- Test: `internal/httpapi/auth_test.go`, `internal/httpapi/ratelimit_test.go`

- [ ] **Step 1: Установить bcrypt**

```bash
go get golang.org/x/crypto/bcrypt
```

- [ ] **Step 2: Написать падающий тест ограничителя**

```go
package httpapi

import (
	"testing"
	"time"
)

func TestLimiterAllowsUpToLimit(t *testing.T) {
	l := newLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("попытка %d отклонена, ожидали разрешение", i+1)
		}
	}
	if l.allow("1.2.3.4") {
		t.Error("четвёртая попытка разрешена, ожидали отказ")
	}
}

func TestLimiterSeparatesKeys(t *testing.T) {
	l := newLimiter(1, time.Minute)
	if !l.allow("1.1.1.1") {
		t.Fatal("первый адрес отклонён")
	}
	if !l.allow("2.2.2.2") {
		t.Error("второй адрес отклонён, хотя лимит у каждого свой")
	}
}

func TestLimiterForgetsAfterWindow(t *testing.T) {
	l := newLimiter(1, 10*time.Millisecond)
	l.allow("1.1.1.1")
	time.Sleep(20 * time.Millisecond)
	if !l.allow("1.1.1.1") {
		t.Error("после окончания окна попытка должна снова разрешаться")
	}
}
```

- [ ] **Step 3: Запустить тест, убедиться что падает**

Run: `go test ./internal/httpapi/ -v`
Expected: FAIL, `undefined: newLimiter`

- [ ] **Step 4: Реализовать `internal/httpapi/ratelimit.go`**

```go
package httpapi

import (
	"sync"
	"time"
)

// limiter считает попытки по ключу в скользящем окне. Хранит всё в памяти:
// сервис однопроцессный, переживать рестарт этим счётчикам не нужно.
type limiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// allow регистрирует попытку и сообщает, укладывается ли она в лимит.
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	fresh := l.attempts[key][:0]
	for _, at := range l.attempts[key] {
		if at.After(cutoff) {
			fresh = append(fresh, at)
		}
	}

	if len(fresh) >= l.limit {
		l.attempts[key] = fresh
		return false
	}

	l.attempts[key] = append(fresh, now)
	return true
}
```

- [ ] **Step 5: Написать падающий тест входа**

```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"partnerops/internal/store"
)

func testAuth(t *testing.T) *Auth {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	hash, err := bcrypt.GenerateFromPassword([]byte("пароль"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return NewAuth(store.NewSessions(db), "admin", string(hash), time.Hour)
}

func TestLoginSucceeds(t *testing.T) {
	auth := testAuth(t)

	req := httptest.NewRequest(http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"пароль"}`))
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200, тело: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("cookie сессии не выставлена")
	}
	c := cookies[0]
	if !c.HttpOnly {
		t.Error("cookie должна быть HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Error("cookie должна быть SameSite=Lax")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	auth := testAuth(t)

	req := httptest.NewRequest(http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"неверный"}`))
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if _, ok := body["error"]; !ok {
		t.Error("в теле нет поля error")
	}
}

func TestLoginRateLimited(t *testing.T) {
	auth := testAuth(t)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login",
			strings.NewReader(`{"login":"admin","password":"неверный"}`))
		req.RemoteAddr = "10.0.0.1:1234"
		auth.Login(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"пароль"}`))
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("код %d, ожидали 429", rec.Code)
	}
}

func TestRequireSessionBlocksAnonymous(t *testing.T) {
	auth := testAuth(t)
	handler := auth.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatever", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401", rec.Code)
	}
}

func TestRequireSessionAllowsValidCookie(t *testing.T) {
	auth := testAuth(t)

	token, err := auth.sessions.Create(context.Background(), time.Hour, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	called := false
	handler := auth.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("обработчик не вызван при действующей сессии")
	}
}
```

- [ ] **Step 6: Реализовать `internal/httpapi/errors.go`**

```go
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorBody — единый формат ошибок API.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeError отвечает ошибкой в едином формате.
func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("не отправить тело ошибки", "err", err)
	}
}

// writeJSON отвечает успешным телом.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("не отправить тело ответа", "err", err)
	}
}
```

- [ ] **Step 7: Реализовать `internal/httpapi/auth.go`**

```go
package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"partnerops/internal/store"
)

const sessionCookieName = "session"

// Auth отвечает за вход, выход и защиту маршрутов.
type Auth struct {
	sessions     *store.Sessions
	login        string
	passwordHash string
	ttl          time.Duration
	limiter      *limiter
}

// NewAuth создаёт обработчик входа. Пароль хранится только как bcrypt-хеш из конфигурации.
func NewAuth(sessions *store.Sessions, login, passwordHash string, ttl time.Duration) *Auth {
	return &Auth{
		sessions:     sessions,
		login:        login,
		passwordHash: passwordHash,
		ttl:          ttl,
		limiter:      newLimiter(5, time.Minute),
	}
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Login проверяет учётные данные и выдаёт cookie сессии.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts",
			"Слишком много попыток входа. Попробуйте через минуту.")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}

	loginOK := subtle.ConstantTimeCompare([]byte(req.Login), []byte(a.login)) == 1
	passwordOK := bcrypt.CompareHashAndPassword([]byte(a.passwordHash), []byte(req.Password)) == nil
	if !loginOK || !passwordOK {
		writeError(w, http.StatusUnauthorized, "bad_credentials", "Неверный логин или пароль.")
		return
	}

	token, err := a.sessions.Create(r.Context(), a.ttl, clientIP(r), r.UserAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось создать сессию.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.ttl.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Logout завершает сессию и стирает cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.sessions.Delete(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: true, MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// RequireSession пропускает дальше только запросы с действующей сессией.
func (a *Auth) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход.")
			return
		}
		ok, err := a.sessions.Validate(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось проверить сессию.")
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Сессия истекла, войдите заново.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP возвращает адрес клиента без порта.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 8: Запустить тесты, убедиться что проходят**

Run: `go test ./internal/httpapi/ -v`
Expected: PASS, восемь тестов

- [ ] **Step 9: Commit**

```bash
git add internal/httpapi/ go.mod go.sum
git commit -m "feat(httpapi): login, session middleware and attempt rate limiting"
```

---

## Task 12: Утилита для хеша пароля

Без неё нечем заполнить `APP_PASSWORD_HASH`.

**Files:**
- Create: `cmd/hashpw/main.go`

- [ ] **Step 1: Реализовать**

```go
// Команда hashpw печатает bcrypt-хеш пароля для переменной APP_PASSWORD_HASH.
//
//	go run ./cmd/hashpw "мой пароль"
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "использование: hashpw <пароль>")
		os.Exit(2)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "не удалось посчитать хеш:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
```

- [ ] **Step 2: Проверить работу**

Run: `go run ./cmd/hashpw "тест"`
Expected: строка вида `$2a$10$...`, длиной 60 символов

- [ ] **Step 3: Commit**

```bash
git add cmd/hashpw/
git commit -m "feat(cmd): add hashpw helper for APP_PASSWORD_HASH"
```

---

## Task 13: Сервис снапшота

**Files:**
- Create: `internal/service/snapshot.go`
- Test: `internal/service/snapshot_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"partnerops/internal/events"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

type fakeReports struct {
	raw  []byte
	err  error
	took int
}

func (f *fakeReports) EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error) {
	f.took++
	return f.raw, f.err
}

type fakeStore struct {
	period    string
	rows      []partner.EDOBillingRow
	saved     int
	duplicate bool
}

func (f *fakeStore) Save(
	ctx context.Context, period string, takenAt time.Time, raw []byte, rows []partner.EDOBillingRow,
) (store.SaveResult, error) {
	f.period = period
	f.rows = rows
	f.saved++
	return store.SaveResult{
		ID:         int64(f.saved),
		IsBaseline: f.saved == 1,
		Duplicate:  f.duplicate,
	}, nil
}

const sampleCSV = "\ufeffКод партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
	"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
	"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
	"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
	"Сумма для партнера;Дополнительная информация\r\n" +
	"00000;Партнёр;CL-1;login;2AE-A;Клиент;7700000001;770001001;тариф;100;2;0;2;2;2;0;0,00;0,00;0,00;\r\n"

func TestTakeSavesAndPublishes(t *testing.T) {
	reports := &fakeReports{raw: []byte(sampleCSV)}
	st := &fakeStore{}
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	svc := NewSnapshot(reports, st, bus)
	err := svc.Take(context.Background(), time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Take: %v", err)
	}

	if st.saved != 1 {
		t.Errorf("сохранений %d, ожидали 1", st.saved)
	}
	if st.period != "2026-08" {
		t.Errorf("период %q, ожидали 2026-08", st.period)
	}
	if len(st.rows) != 1 {
		t.Errorf("строк %d, ожидали 1", len(st.rows))
	}

	select {
	case event := <-ch:
		if event.Kind != "snapshot.completed" {
			t.Errorf("событие %q, ожидали snapshot.completed", event.Kind)
		}
	case <-time.After(time.Second):
		t.Error("событие не опубликовано")
	}
}

func TestTakeDoesNotSaveOnReportError(t *testing.T) {
	reports := &fakeReports{err: errors.New("1С недоступна")}
	st := &fakeStore{}

	svc := NewSnapshot(reports, st, events.NewBus())
	if err := svc.Take(context.Background(), time.Now()); err == nil {
		t.Fatal("ожидали ошибку")
	}
	if st.saved != 0 {
		t.Error("при ошибке отчёта ничего сохранять нельзя")
	}
}

func TestTakeDoesNotSaveEmptyReport(t *testing.T) {
	// Отчёт с заголовком, но без строк, — признак сбоя на стороне 1С,
	// а не признак того, что клиентов не осталось. Затирать историю нельзя.
	header := sampleCSV[:len(sampleCSV)-len(
		"00000;Партнёр;CL-1;login;2AE-A;Клиент;7700000001;770001001;тариф;100;2;0;2;2;2;0;0,00;0,00;0,00;\r\n")]
	reports := &fakeReports{raw: []byte(header)}
	st := &fakeStore{}

	svc := NewSnapshot(reports, st, events.NewBus())
	if err := svc.Take(context.Background(), time.Now()); err == nil {
		t.Fatal("ожидали ошибку на пустом отчёте")
	}
	if st.saved != 0 {
		t.Error("пустой отчёт сохранять нельзя")
	}
}

func TestTakeStaysSilentOnDuplicate(t *testing.T) {
	// 1С отдаёт одни и те же данные при повторных запусках за день.
	// Событие в таком случае публиковать нельзя: интерфейс мигнёт без причины.
	reports := &fakeReports{raw: []byte(sampleCSV)}
	st := &fakeStore{duplicate: true}
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	svc := NewSnapshot(reports, st, bus)
	if err := svc.Take(context.Background(), time.Now()); err != nil {
		t.Fatalf("Take: %v", err)
	}

	select {
	case event := <-ch:
		t.Errorf("на дубликате опубликовано событие %q", event.Kind)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTakeMarksBaseline(t *testing.T) {
	// Первый снимок сравнивать не с чем, событие должно это сообщать,
	// иначе последующий разбор сочтёт весь реестр «появившимся».
	reports := &fakeReports{raw: []byte(sampleCSV)}
	st := &fakeStore{}
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	svc := NewSnapshot(reports, st, bus)
	if err := svc.Take(context.Background(), time.Now()); err != nil {
		t.Fatalf("Take: %v", err)
	}

	select {
	case event := <-ch:
		if event.Payload["baseline"] != true {
			t.Errorf("payload = %v, ожидали baseline=true", event.Payload)
		}
	case <-time.After(time.Second):
		t.Error("событие не опубликовано")
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/service/ -v`
Expected: FAIL, `undefined: NewSnapshot`

- [ ] **Step 3: Реализовать**

```go
// Package service содержит прикладную логику: она соединяет партнёрское API и хранилище.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"partnerops/internal/events"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// ReportSource строит отчёт биллинга ЭДО. Отдельный интерфейс, чтобы тесты не ходили в сеть.
type ReportSource interface {
	EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error)
}

// SnapshotStore сохраняет снапшот.
type SnapshotStore interface {
	Save(ctx context.Context, period string, takenAt time.Time, raw []byte, rows []partner.EDOBillingRow) (store.SaveResult, error)
}

// Snapshot строит и сохраняет ежедневный снимок отчёта биллинга ЭДО.
type Snapshot struct {
	reports ReportSource
	store   SnapshotStore
	bus     *events.Bus
}

// NewSnapshot собирает сервис.
func NewSnapshot(reports ReportSource, store SnapshotStore, bus *events.Bus) *Snapshot {
	return &Snapshot{reports: reports, store: store, bus: bus}
}

// Take строит отчёт за месяц указанной даты, разбирает его и сохраняет.
//
// Пустой отчёт считается сбоем: у партнёра всегда есть клиенты, и затирать историю
// пустым снимком нельзя.
func (s *Snapshot) Take(ctx context.Context, date time.Time) error {
	period := date.UTC().Format("2006-01")

	raw, err := s.reports.EDOBillingReport(ctx, date)
	if err != nil {
		return fmt.Errorf("service: не построить отчёт за %s: %w", period, err)
	}

	rows, err := partner.ParseEDOBilling(raw)
	if err != nil {
		return fmt.Errorf("service: не разобрать отчёт за %s: %w", period, err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("service: отчёт за %s пуст, снапшот не сохранён", period)
	}

	takenAt := time.Now().UTC()
	res, err := s.store.Save(ctx, period, takenAt, raw, rows)
	if err != nil {
		return fmt.Errorf("service: не сохранить снапшот за %s: %w", period, err)
	}

	if res.Duplicate {
		// Данные не изменились с прошлого раза — сообщать не о чем.
		slog.Info("отчёт совпал с предыдущим, снапшот не создан", "period", period, "id", res.ID)
		return nil
	}

	slog.Info("снапшот сохранён",
		"period", period, "rows", len(rows), "id", res.ID, "baseline", res.IsBaseline)
	s.bus.Publish(events.Event{
		Kind: "snapshot.completed",
		Payload: map[string]any{
			"period":   period,
			"rows":     len(rows),
			"takenAt":  takenAt.Format(time.RFC3339),
			"baseline": res.IsBaseline,
		},
	})
	return nil
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/service/ -v`
Expected: PASS, три теста

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat(service): build, parse and store daily EDO snapshot"
```

---

## Task 14: SSE

**Files:**
- Create: `internal/httpapi/events.go`
- Test: `internal/httpapi/events_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"partnerops/internal/events"
)

func TestStreamSendsEvent(t *testing.T) {
	bus := events.NewBus()
	handler := NewEvents(bus)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.Stream(rec, req)
		close(done)
	}()

	// Даём обработчику подписаться до публикации.
	time.Sleep(50 * time.Millisecond)
	bus.Publish(events.Event{Kind: "snapshot.completed", Payload: map[string]any{"rows": 23}})
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("обработчик не завершился после отмены контекста")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: snapshot.completed") {
		t.Errorf("в потоке нет имени события, тело: %q", body)
	}
	if !strings.Contains(body, `"rows":23`) {
		t.Errorf("в потоке нет полезной нагрузки, тело: %q", body)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, ожидали text/event-stream", got)
	}
}

func TestStreamStopsOnDisconnect(t *testing.T) {
	bus := events.NewBus()
	handler := NewEvents(bus)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // клиент отключился сразу

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		handler.Stream(httptest.NewRecorder(), req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("обработчик не завершился при отключённом клиенте")
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/httpapi/ -run TestStream -v`
Expected: FAIL, `undefined: NewEvents`

- [ ] **Step 3: Реализовать**

```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"partnerops/internal/events"
)

// keepAliveInterval не даёт обратному прокси разорвать простаивающее соединение.
const keepAliveInterval = 25 * time.Second

// Events отдаёт поток изменений состояния по Server-Sent Events.
type Events struct {
	bus *events.Bus
}

// NewEvents создаёт обработчик потока.
func NewEvents(bus *events.Bus) *Events {
	return &Events{bus: bus}
}

// Stream держит соединение и шлёт события, пока клиент не отключится.
func (e *Events) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "no_streaming",
			"Сервер не умеет отдавать поток событий.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Отключает буферизацию в nginx, иначе события копятся и приходят пачкой.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	stream, unsubscribe := e.bus.Subscribe()
	defer unsubscribe()

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case event, open := <-stream:
			if !open {
				return
			}
			payload, err := json.Marshal(event.Payload)
			if err != nil {
				slog.Error("не сериализовать событие", "kind", event.Kind, "err", err)
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Kind, payload)
			flusher.Flush()

		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/httpapi/ -v -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/
git commit -m "feat(httpapi): server-sent events stream"
```

---

## Task 15: Маршруты и запуск

**Files:**
- Create: `internal/httpapi/router.go`, `cmd/server/main.go`
- Test: `internal/httpapi/router_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"partnerops/internal/events"
	"partnerops/internal/store"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	hash, _ := bcrypt.GenerateFromPassword([]byte("пароль"), bcrypt.DefaultCost)
	auth := NewAuth(store.NewSessions(db), "admin", string(hash), time.Hour)
	return NewRouter(auth, NewEvents(events.NewBus()))
}

func TestHealthIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("код %d, ожидали 200", rec.Code)
	}
}

func TestEventsRequireSession(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/events", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401", rec.Code)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("код %d, ожидали 404", rec.Code)
	}
}
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `go test ./internal/httpapi/ -run TestHealth -v`
Expected: FAIL, `undefined: NewRouter`

- [ ] **Step 3: Реализовать `internal/httpapi/router.go`**

```go
package httpapi

import "net/http"

// NewRouter собирает маршруты сервиса.
func NewRouter(auth *Auth, sse *Events) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/login", auth.Login)
	mux.HandleFunc("POST /api/logout", auth.Logout)

	mux.Handle("GET /api/events", auth.RequireSession(http.HandlerFunc(sse.Stream)))

	return mux
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `go test ./internal/httpapi/ -v`
Expected: PASS

- [ ] **Step 5: Реализовать `cmd/server/main.go`**

```go
// Команда server поднимает HTTP-сервис и фоновые задачи.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"partnerops/internal/config"
	"partnerops/internal/events"
	"partnerops/internal/httpapi"
	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	bus := events.NewBus()
	sessions := store.NewSessions(db)
	snapshots := store.NewSnapshots(db)
	client := partner.New("", cfg.PartnerAPILogin, cfg.PartnerAPIPassword)
	snapshotService := service.NewSnapshot(client, snapshots, bus)

	auth := httpapi.NewAuth(sessions, cfg.AppLogin, cfg.AppPasswordHash, cfg.SessionTTL)
	router := httpapi.NewRouter(auth, httpapi.NewEvents(bus))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go runDailyJobs(ctx, snapshotService, sessions)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// Таймаут записи не ставим: он обрывает поток SSE.
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	slog.Info("сервис запущен", "addr", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// runDailyJobs раз в сутки снимает снапшот и чистит истёкшие сессии.
// Первый прогон выполняется сразу при старте, чтобы не ждать сутки на пустой базе.
func runDailyJobs(ctx context.Context, snapshots *service.Snapshot, sessions *store.Sessions) {
	tick := func() {
		if err := snapshots.Take(ctx, time.Now()); err != nil {
			slog.Error("снапшот не снят", "err", err)
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
```

- [ ] **Step 6: Проверить сборку и все тесты**

Run: `go build ./... && go test ./... -race`
Expected: сборка без ошибок, все тесты PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/ internal/httpapi/
git commit -m "feat(server): wire router, background jobs and graceful shutdown"
```

---

## Task 16: Проверка на живом API

Первый запуск против настоящего 1С. Тесты этого не покрывают — нужен человек.

**Files:**
- Create: `.env` (не коммитится)

- [ ] **Step 1: Заполнить `.env`**

Скопировать `.env.example` в `.env`, подставить настоящие значения.
Хеш пароля получить командой:

Run: `go run ./cmd/hashpw "выбранный пароль"`

- [ ] **Step 2: Запустить сервис**

Run: `go run ./cmd/server`
Expected: в логе `сервис запущен`, затем `снапшот сохранён` с ненулевым `rows`

- [ ] **Step 3: Проверить health**

Run: `curl.exe http://127.0.0.1:8080/api/health`
Expected: `{"status":"ok"}`

- [ ] **Step 4: Проверить, что закрытые маршруты закрыты**

Run: `curl.exe -i http://127.0.0.1:8080/api/events`
Expected: `HTTP/1.1 401 Unauthorized`

- [ ] **Step 5: Проверить содержимое базы**

Run: `sqlite3 data/app.db "SELECT period, row_count FROM edo_snapshots;"`
Expected: одна строка с текущим периодом и числом строк больше нуля

- [ ] **Step 6: Commit**

Коммитить нечего — `.env` и `data/` в `.gitignore`. Зафиксировать результат проверки в описании
следующего плана.

---

## Самопроверка плана

**Покрытие спеки.** Этот план закрывает разделы спеки: конфигурация (Task 2), хранение и миграции
(Task 4), сессии и вход (Task 10, 11), клиент партнёрского API и разбор CSV (Task 5, 6, 7),
снапшоты (Task 8, 13), шина событий и SSE (Task 9, 14), сборка и фоновые задачи (Task 15).

**Сознательно не входит в этот план** и требует отдельных планов:
- сравнение снапшотов и детекция аномалий идентификаторов (§4.2 спеки);
- экран биллинга и квот, отчёт опций (§4.1);
- заявки 1С:ИТС и работа с шаблоном Excel (§4.3);
- абоненты, проверка, справочник программ (§4.4);
- фронтенд на Vue целиком и вшивание его через `embed`;
- воркер отчётов с очередью `report_jobs` — в фундаменте отчёт строится синхронно раз в сутки,
  очередь понадобится, когда отчёты начнут запускать руками из интерфейса.

**Согласованность типов.** `partner.EDOBillingRow` определён в Task 5 и используется без изменений
в Task 8 и 13. `money.Amount` определён в Task 3, применяется в Task 5 и 8.
`events.Event` определён в Task 9, используется в Task 13 и 14.
`store.Sessions` определён в Task 10, используется в Task 11 и 15.
Интерфейсы `service.ReportSource` и `service.SnapshotStore` в Task 13 совпадают по сигнатурам
с методами `partner.Client.EDOBillingReport` (Task 7) и `store.Snapshots.Save` (Task 8).
