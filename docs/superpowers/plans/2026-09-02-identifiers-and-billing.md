# Реестр, аномалии и биллинг — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сервис накапливает собственный реестр идентификаторов ЭДО, находит в нём поломки, считает остатки квот и кто вышел за лимит, и отдаёт это по HTTP.

**Architecture:** Снапшоты уже сохраняются. Поверх них появляется накопительный реестр `edo_identifiers`, сравнение состояний рождает `identifier_events` с уровнем уверенности, а расчёт по последнему снапшоту даёт биллинг. Всё читается через REST под сессией.

**Tech Stack:** Go 1.26, SQLite, stdlib `net/http`.

**Предыдущий план:** `2026-09-02-foundation.md` (выполнен)
**Спека:** `2026-09-02-partner-portal-design.md`, разделы 4.1 и 4.2

---

## Что уже есть

| Пакет | Что предоставляет |
|---|---|
| `internal/partner` | `Client.EDOBillingReport(ctx, date) ([]byte, error)`, `ParseEDOBilling([]byte) ([]EDOBillingRow, error)`, тип `EDOBillingRow` |
| `internal/store` | `Open(path)`, `Snapshots.Save(...) (SaveResult, error)`, `Snapshots.Latest(ctx, period)`, `Sessions` |
| `internal/service` | `Snapshot.Take(ctx, date) error` |
| `internal/events` | `Bus.Publish/Subscribe` |
| `internal/httpapi` | `Auth.RequireSession`, `writeJSON`, `writeError`, `NewRouter(auth, sse)` |
| `internal/money` | `Amount`, `Parse`, `String`, `Precise` |

`EDOBillingRow` содержит: `PartnerCode, PartnerName, Owner, Login, EDOID, ClientName, INN, KPP,
ITSTariffs, Limit *int64, InvoicesOut, NonInvoicesOut, Packets, PacketsByOwner, Discount,
PacketsBillable, TariffAmount, ClientAmount, PartnerAmount, Extra`.

---

## Структура файлов

| Файл | Ответственность |
|---|---|
| `internal/store/migrations/002_registry.sql` | таблицы реестра, событий, подтверждений |
| `internal/store/identifiers.go` | чтение и обновление реестра, запись событий |
| `internal/service/registry.go` | наполнение реестра из строк снапшота |
| `internal/service/anomalies.go` | правила поиска аномалий |
| `internal/service/billing.go` | остатки квот, кто за лимитом |
| `internal/httpapi/identifiers.go` | `GET /api/identifiers`, `GET /api/anomalies`, `POST /api/anomalies/{id}/ack` |
| `internal/httpapi/billing.go` | `GET /api/billing` |

---

## Task 1: Схема реестра

**Files:**
- Create: `internal/store/migrations/002_registry.sql`
- Test: `internal/store/sqlite_test.go` (дополнить)

- [ ] **Step 1: Дописать тест в `internal/store/sqlite_test.go`**

```go
func TestMigrationsCreateRegistryTables(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != 2 {
		t.Errorf("user_version = %d, ожидали 2", version)
	}

	for _, table := range []string{"edo_identifiers", "identifier_events", "identifier_acks"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("таблица %s не создана: %v", table, err)
		}
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/store/ -run TestMigrationsCreateRegistry -v`
Expected: FAIL, `user_version = 1, ожидали 2`

- [ ] **Step 3: Создать `internal/store/migrations/002_registry.sql`**

```sql
-- Накопительный реестр идентификаторов ЭДО.
-- Реестра связей в партнёрском API нет, поэтому сервис собирает его сам
-- из отчётов и хранит как последнее известное состояние.
CREATE TABLE edo_identifiers (
    edo_id          TEXT PRIMARY KEY,
    inn             TEXT NOT NULL DEFAULT '',
    kpp             TEXT NOT NULL DEFAULT '',
    client_name     TEXT NOT NULL DEFAULT '',
    login           TEXT NOT NULL DEFAULT '',
    owner_raw       TEXT NOT NULL DEFAULT '',
    owner_code      TEXT NOT NULL DEFAULT '',
    its_tariffs     TEXT NOT NULL DEFAULT '',
    limit_docs      INTEGER,
    packets         INTEGER NOT NULL DEFAULT 0,
    first_seen_at   INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    last_period     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_edo_identifiers_org ON edo_identifiers(inn, kpp);
CREATE INDEX idx_edo_identifiers_login ON edo_identifiers(login);

-- Найденные аномалии. state_fingerprint — отпечаток состояния, на основании
-- которого возникла находка: подтверждение «это законно» гасит только его,
-- изменившееся состояние поднимет сигнал заново.
CREATE TABLE identifier_events (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    edo_id            TEXT NOT NULL,
    kind              TEXT NOT NULL,
    confidence        TEXT NOT NULL,
    state_fingerprint TEXT NOT NULL,
    inn               TEXT NOT NULL DEFAULT '',
    kpp               TEXT NOT NULL DEFAULT '',
    client_name       TEXT NOT NULL DEFAULT '',
    login             TEXT NOT NULL DEFAULT '',
    details           TEXT NOT NULL DEFAULT '',
    detected_at       INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_identifier_events_unique
    ON identifier_events(edo_id, kind, state_fingerprint);
CREATE INDEX idx_identifier_events_detected ON identifier_events(detected_at);

-- Пометки «это законно», привязанные к отпечатку состояния.
CREATE TABLE identifier_acks (
    edo_id            TEXT NOT NULL,
    state_fingerprint TEXT NOT NULL,
    reason            TEXT NOT NULL DEFAULT '',
    author            TEXT NOT NULL DEFAULT '',
    acked_at          INTEGER NOT NULL,
    PRIMARY KEY (edo_id, state_fingerprint)
);
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): add registry, anomaly and acknowledgement tables"
```

---

## Task 2: Разбор поля «Владелец»

Значение приходит составным: `FR-FR-600361 - Евгений Ковалев`. Нужен код абонента отдельно.

**Files:**
- Create: `internal/partner/owner.go`
- Test: `internal/partner/owner_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package partner

import "testing"

func TestParseOwner(t *testing.T) {
	cases := []struct {
		in       string
		wantCode string
		wantName string
	}{
		{"FR-FR-600361 - Евгений Ковалев", "FR-FR-600361", "Евгений Ковалев"},
		{"CL-1000530 - Кравец Юлия Владимировна", "CL-1000530", "Кравец Юлия Владимировна"},
		{"CL-7000582: Мельничук Вера", "CL-7000582", "Мельничук Вера"},
		{"FR-FR-800918", "FR-FR-800918", ""},
		{"", "", ""},
		{"  CL-100 - Имя  ", "CL-100", "Имя"},
		{"нечто без кода", "", "нечто без кода"},
	}
	for _, c := range cases {
		code, name := ParseOwner(c.in)
		if code != c.wantCode || name != c.wantName {
			t.Errorf("ParseOwner(%q) = (%q, %q), ожидали (%q, %q)",
				c.in, code, name, c.wantCode, c.wantName)
		}
	}
}

func TestIsFreshOwner(t *testing.T) {
	// Тариф ЭПД, оформленный файлом-заявкой, действует только на владельцев CL-.
	// Для FR- (облако Фреш) заявка уйдёт в брак, см. docs/REQUESTS.md.
	if !IsFreshOwner("FR-FR-600361") {
		t.Error("FR-FR-600361 должен опознаваться как облачный")
	}
	if IsFreshOwner("CL-1000530") {
		t.Error("CL-1000530 не облачный")
	}
	if IsFreshOwner("") {
		t.Error("пустой код не облачный")
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/partner/ -run TestParseOwner -v`
Expected: FAIL, `undefined: ParseOwner`

- [ ] **Step 3: Реализовать**

```go
package partner

import (
	"regexp"
	"strings"
)

// ownerCodePattern описывает код абонента: CL-<цифры> для локальных программ
// и FR-FR-<цифры> для облака Фреш. Различие важно: тариф ЭПД, оформленный
// файлом-заявкой, действует только на владельцев CL-.
var ownerCodePattern = regexp.MustCompile(`^(CL-\d+|FR-FR-\d+|FR-\d+)`)

// ParseOwner разбирает составное поле «Владелец» вида «FR-FR-600361 - Евгений Ковалев».
// Разделителем бывает как « - », так и «: ». Если код не распознан, он пуст,
// а вся строка возвращается как имя — данные не теряются.
func ParseOwner(raw string) (code, name string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	match := ownerCodePattern.FindString(raw)
	if match == "" {
		return "", raw
	}

	rest := strings.TrimSpace(raw[len(match):])
	rest = strings.TrimLeft(rest, " -:")
	return match, strings.TrimSpace(rest)
}

// IsFreshOwner сообщает, что абонент работает в облаке Фреш.
func IsFreshOwner(code string) bool {
	return strings.HasPrefix(code, "FR-")
}
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/partner/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/partner/
git commit -m "feat(partner): split composite owner field into code and name"
```

---

## Task 3: Хранилище реестра

**Files:**
- Create: `internal/store/identifiers.go`
- Test: `internal/store/identifiers_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func testIdentifiers(t *testing.T) *Identifiers {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewIdentifiers(db)
}

func TestUpsertInsertsAndUpdates(t *testing.T) {
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	rec := IdentifierRecord{
		EDOID: "2AE-A", INN: "7700000001", KPP: "770001001",
		ClientName: "ООО Тест", Login: "user@example.ru",
		OwnerRaw: "CL-1 - Иванов", OwnerCode: "CL-1", Packets: 5,
	}
	if err := s.Upsert(ctx, []IdentifierRecord{rec}, "2026-08", now); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("записей %d, ожидали 1", len(all))
	}
	if !all[0].FirstSeenAt.Equal(now) || !all[0].LastSeenAt.Equal(now) {
		t.Errorf("даты первого и последнего появления должны совпадать при вставке")
	}

	later := now.Add(24 * time.Hour)
	rec.Packets = 9
	if err := s.Upsert(ctx, []IdentifierRecord{rec}, "2026-08", later); err != nil {
		t.Fatalf("повторный Upsert: %v", err)
	}

	all, _ = s.All(ctx)
	if len(all) != 1 {
		t.Fatalf("записей %d, ожидали 1 после обновления", len(all))
	}
	if !all[0].FirstSeenAt.Equal(now) {
		t.Errorf("FirstSeenAt = %v, должен остаться прежним", all[0].FirstSeenAt)
	}
	if !all[0].LastSeenAt.Equal(later) {
		t.Errorf("LastSeenAt = %v, ожидали %v", all[0].LastSeenAt, later)
	}
	if all[0].Packets != 9 {
		t.Errorf("Packets = %d, ожидали 9", all[0].Packets)
	}
}

func TestRecordEventIsIdempotent(t *testing.T) {
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Now().UTC()

	event := AnomalyEvent{
		EDOID: "2AE-A", Kind: "orphan_with_traffic", Confidence: "high",
		StateFingerprint: "abc123", INN: "7700000001", Details: "56 пакетов",
	}
	for i := 0; i < 3; i++ {
		if err := s.RecordEvent(ctx, event, now); err != nil {
			t.Fatalf("RecordEvent %d: %v", i, err)
		}
	}

	events, err := s.Events(ctx, false)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("событий %d, ожидали 1: повтор с тем же отпечатком не должен дублироваться", len(events))
	}
}

func TestAckHidesEvent(t *testing.T) {
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Now().UTC()

	event := AnomalyEvent{
		EDOID: "2AE-A", Kind: "duplicate", Confidence: "low", StateFingerprint: "fp-1",
	}
	if err := s.RecordEvent(ctx, event, now); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	if err := s.Acknowledge(ctx, "2AE-A", "fp-1", "разные виды деятельности", "admin", now); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}

	active, err := s.Events(ctx, false)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("активных событий %d, подтверждённое показываться не должно", len(active))
	}

	all, err := s.Events(ctx, true)
	if err != nil {
		t.Fatalf("Events(all): %v", err)
	}
	if len(all) != 1 {
		t.Errorf("всего событий %d, ожидали 1", len(all))
	}
	if !all[0].Acknowledged {
		t.Error("событие должно быть помечено подтверждённым")
	}
}

func TestAckDoesNotHideChangedState(t *testing.T) {
	// Подтверждение гасит конкретное состояние. Если состояние изменилось,
	// сигнал обязан появиться снова, иначе одна пометка скроет будущую поломку.
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := s.RecordEvent(ctx, AnomalyEvent{
		EDOID: "2AE-A", Kind: "duplicate", Confidence: "low", StateFingerprint: "fp-1",
	}, now); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	if err := s.Acknowledge(ctx, "2AE-A", "fp-1", "законно", "admin", now); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if err := s.RecordEvent(ctx, AnomalyEvent{
		EDOID: "2AE-A", Kind: "duplicate", Confidence: "low", StateFingerprint: "fp-2",
	}, now); err != nil {
		t.Fatalf("RecordEvent с новым отпечатком: %v", err)
	}

	active, _ := s.Events(ctx, false)
	if len(active) != 1 {
		t.Errorf("активных событий %d, ожидали 1: изменившееся состояние должно всплыть", len(active))
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/store/ -run TestUpsert -v`
Expected: FAIL, `undefined: NewIdentifiers`

- [ ] **Step 3: Реализовать `internal/store/identifiers.go`**

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// IdentifierRecord — последнее известное состояние идентификатора ЭДО.
type IdentifierRecord struct {
	EDOID       string
	INN         string
	KPP         string
	ClientName  string
	Login       string
	OwnerRaw    string
	OwnerCode   string
	ITSTariffs  string
	Limit       *int64
	Packets     int64
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	LastPeriod  string
}

// AnomalyEvent — найденная аномалия.
type AnomalyEvent struct {
	ID               int64
	EDOID            string
	Kind             string
	Confidence       string
	StateFingerprint string
	INN              string
	KPP              string
	ClientName       string
	Login            string
	Details          string
	DetectedAt       time.Time
	Acknowledged     bool
}

// Identifiers хранит накопленный реестр и найденные аномалии.
type Identifiers struct {
	db *sql.DB
}

// NewIdentifiers создаёт хранилище поверх открытой базы.
func NewIdentifiers(db *sql.DB) *Identifiers {
	return &Identifiers{db: db}
}

// Upsert добавляет новые идентификаторы и обновляет известные.
// Дата первого появления сохраняется, дата последнего сдвигается.
func (s *Identifiers) Upsert(
	ctx context.Context, records []IdentifierRecord, period string, seenAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию реестра: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO edo_identifiers (
			edo_id, inn, kpp, client_name, login, owner_raw, owner_code,
			its_tariffs, limit_docs, packets, first_seen_at, last_seen_at, last_period
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(edo_id) DO UPDATE SET
			inn = excluded.inn,
			kpp = excluded.kpp,
			client_name = excluded.client_name,
			login = excluded.login,
			owner_raw = excluded.owner_raw,
			owner_code = excluded.owner_code,
			its_tariffs = excluded.its_tariffs,
			limit_docs = excluded.limit_docs,
			packets = excluded.packets,
			last_seen_at = excluded.last_seen_at,
			last_period = excluded.last_period`)
	if err != nil {
		return fmt.Errorf("store: не подготовить запись реестра: %w", err)
	}
	defer stmt.Close()

	unix := seenAt.UTC().Unix()
	for _, r := range records {
		var limit any
		if r.Limit != nil {
			limit = *r.Limit
		}
		_, err := stmt.ExecContext(ctx,
			r.EDOID, r.INN, r.KPP, r.ClientName, r.Login, r.OwnerRaw, r.OwnerCode,
			r.ITSTariffs, limit, r.Packets, unix, unix, period)
		if err != nil {
			return fmt.Errorf("store: не записать идентификатор %s: %w", r.EDOID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать реестр: %w", err)
	}
	return nil
}

// All возвращает весь реестр.
func (s *Identifiers) All(ctx context.Context) ([]IdentifierRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT edo_id, inn, kpp, client_name, login, owner_raw, owner_code,
		       its_tariffs, limit_docs, packets, first_seen_at, last_seen_at, last_period
		FROM edo_identifiers ORDER BY client_name, edo_id`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать реестр: %w", err)
	}
	defer rows.Close()

	var result []IdentifierRecord
	for rows.Next() {
		var r IdentifierRecord
		var limit sql.NullInt64
		var first, last int64
		if err := rows.Scan(&r.EDOID, &r.INN, &r.KPP, &r.ClientName, &r.Login,
			&r.OwnerRaw, &r.OwnerCode, &r.ITSTariffs, &limit, &r.Packets,
			&first, &last, &r.LastPeriod); err != nil {
			return nil, fmt.Errorf("store: не разобрать запись реестра: %w", err)
		}
		if limit.Valid {
			value := limit.Int64
			r.Limit = &value
		}
		r.FirstSeenAt = time.Unix(first, 0).UTC()
		r.LastSeenAt = time.Unix(last, 0).UTC()
		result = append(result, r)
	}
	return result, rows.Err()
}

// RecordEvent сохраняет находку. Повтор с тем же отпечатком состояния игнорируется:
// одна и та же аномалия не должна плодить записи при каждом снапшоте.
func (s *Identifiers) RecordEvent(ctx context.Context, event AnomalyEvent, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO identifier_events (
			edo_id, kind, confidence, state_fingerprint, inn, kpp,
			client_name, login, details, detected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(edo_id, kind, state_fingerprint) DO NOTHING`,
		event.EDOID, event.Kind, event.Confidence, event.StateFingerprint,
		event.INN, event.KPP, event.ClientName, event.Login, event.Details,
		at.UTC().Unix())
	if err != nil {
		return fmt.Errorf("store: не записать находку по %s: %w", event.EDOID, err)
	}
	return nil
}

// Events возвращает находки. При includeAcknowledged = false подтверждённые скрываются.
func (s *Identifiers) Events(ctx context.Context, includeAcknowledged bool) ([]AnomalyEvent, error) {
	query := `
		SELECT e.id, e.edo_id, e.kind, e.confidence, e.state_fingerprint,
		       e.inn, e.kpp, e.client_name, e.login, e.details, e.detected_at,
		       a.edo_id IS NOT NULL AS acknowledged
		FROM identifier_events e
		LEFT JOIN identifier_acks a
			ON a.edo_id = e.edo_id AND a.state_fingerprint = e.state_fingerprint`
	if !includeAcknowledged {
		query += ` WHERE a.edo_id IS NULL`
	}
	query += ` ORDER BY e.detected_at DESC, e.id DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать находки: %w", err)
	}
	defer rows.Close()

	var result []AnomalyEvent
	for rows.Next() {
		var e AnomalyEvent
		var detected int64
		if err := rows.Scan(&e.ID, &e.EDOID, &e.Kind, &e.Confidence, &e.StateFingerprint,
			&e.INN, &e.KPP, &e.ClientName, &e.Login, &e.Details, &detected,
			&e.Acknowledged); err != nil {
			return nil, fmt.Errorf("store: не разобрать находку: %w", err)
		}
		e.DetectedAt = time.Unix(detected, 0).UTC()
		result = append(result, e)
	}
	return result, rows.Err()
}

// Acknowledge помечает состояние законным. Гасится только этот отпечаток:
// если состояние изменится, сигнал появится снова.
func (s *Identifiers) Acknowledge(
	ctx context.Context, edoID, fingerprint, reason, author string, at time.Time,
) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO identifier_acks (edo_id, state_fingerprint, reason, author, acked_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(edo_id, state_fingerprint) DO UPDATE SET
			reason = excluded.reason, author = excluded.author, acked_at = excluded.acked_at`,
		edoID, fingerprint, reason, author, at.UTC().Unix())
	if err != nil {
		return fmt.Errorf("store: не сохранить подтверждение: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): accumulate identifier registry with fingerprinted anomalies"
```

---

## Task 4: Правила поиска аномалий

Порядок и смысл правил обоснованы в спеке, раздел 4.2. Главное: несколько идентификаторов
на одну организацию бывают законны, поэтому сам факт дубля сигналом не является.

**Files:**
- Create: `internal/service/anomalies.go`
- Test: `internal/service/anomalies_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package service

import (
	"testing"

	"partnerops/internal/partner"
)

func row(edoID, inn, owner, login string, packets int64, limit *int64) partner.EDOBillingRow {
	return partner.EDOBillingRow{
		EDOID: edoID, INN: inn, KPP: "770001001", ClientName: "ООО Тест",
		Owner: owner, Login: login, Packets: packets, Limit: limit,
	}
}

func limitOf(v int64) *int64 { return &v }

func TestOrphanWithTrafficIsHighConfidence(t *testing.T) {
	// Документы идут, а тарифа, к которому их отнести, нет — прямые деньги.
	rows := []partner.EDOBillingRow{
		row("2AE-A", "7700000001", "", "user", 56, nil),
	}

	found := FindAnomalies(rows, nil)

	if len(found) != 1 {
		t.Fatalf("находок %d, ожидали 1", len(found))
	}
	if found[0].Kind != KindOrphanWithTraffic {
		t.Errorf("kind = %q, ожидали %q", found[0].Kind, KindOrphanWithTraffic)
	}
	if found[0].Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, ожидали %q", found[0].Confidence, ConfidenceHigh)
	}
}

func TestOrphanWithoutTrafficIsLowConfidence(t *testing.T) {
	rows := []partner.EDOBillingRow{
		row("2AE-B", "7700000002", "", "user", 0, nil),
	}

	found := FindAnomalies(rows, nil)

	if len(found) != 1 {
		t.Fatalf("находок %d, ожидали 1", len(found))
	}
	if found[0].Kind != KindOrphanIdle {
		t.Errorf("kind = %q, ожидали %q", found[0].Kind, KindOrphanIdle)
	}
	if found[0].Confidence != ConfidenceLow {
		t.Errorf("confidence = %q, ожидали low", found[0].Confidence)
	}
}

func TestOwnerLostBetweenSnapshots(t *testing.T) {
	previous := map[string]string{"2AE-C": "CL-1 - Иванов"}
	rows := []partner.EDOBillingRow{
		row("2AE-C", "7700000003", "", "user", 0, nil),
	}

	found := FindAnomalies(rows, previous)

	var kinds []string
	for _, a := range found {
		kinds = append(kinds, a.Kind)
	}
	if !contains(kinds, KindOwnerLost) {
		t.Errorf("виды находок %v, ожидали среди них %q", kinds, KindOwnerLost)
	}
	for _, a := range found {
		if a.Kind == KindOwnerLost && a.Confidence != ConfidenceHigh {
			t.Errorf("потеря владельца должна быть высокой уверенности, получили %q", a.Confidence)
		}
	}
}

func TestHealthyRowsProduceNothing(t *testing.T) {
	rows := []partner.EDOBillingRow{
		row("2AE-D", "7700000004", "CL-1 - Иванов", "user", 10, limitOf(100)),
		row("2AE-E", "7700000005", "FR-FR-2 - Петров", "other", 0, limitOf(50)),
	}

	if found := FindAnomalies(rows, nil); len(found) != 0 {
		t.Errorf("находок %d, ожидали 0: здоровые строки сигналов не дают: %+v", len(found), found)
	}
}

func TestLegalDuplicateIsNotReported() func(*testing.T) {
	return func(t *testing.T) {}
}

func TestDuplicateWithOwnersIsNotAnomaly(t *testing.T) {
	// Один ИНН с двумя полноценными идентификаторами — законная ситуация:
	// у клиента разные виды деятельности. Сигналом это быть не должно.
	rows := []partner.EDOBillingRow{
		row("2AE-F", "7700000006", "CL-1 - Иванов", "user", 5, limitOf(50)),
		row("2AE-G", "7700000006", "CL-2 - Иванов", "user", 7, limitOf(50)),
	}

	if found := FindAnomalies(rows, nil); len(found) != 0 {
		t.Errorf("находок %d, ожидали 0: дубль с полноценными владельцами законен: %+v",
			len(found), found)
	}
}

func TestFingerprintChangesWithState(t *testing.T) {
	first := FindAnomalies([]partner.EDOBillingRow{
		row("2AE-H", "7700000007", "", "user", 10, nil),
	}, nil)
	second := FindAnomalies([]partner.EDOBillingRow{
		row("2AE-H", "7700000007", "", "user", 20, nil),
	}, nil)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("ожидали по одной находке, получили %d и %d", len(first), len(second))
	}
	if first[0].StateFingerprint == second[0].StateFingerprint {
		t.Error("отпечаток обязан меняться вместе с состоянием, иначе подтверждение скроет новую поломку")
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/service/ -run TestOrphan -v`
Expected: FAIL, `undefined: FindAnomalies`

- [ ] **Step 3: Реализовать `internal/service/anomalies.go`**

```go
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"partnerops/internal/partner"
)

// Виды находок.
const (
	// KindOrphanWithTraffic — трафик идёт, а тарифа, к которому его отнести, нет.
	KindOrphanWithTraffic = "orphan_with_traffic"
	// KindOwnerLost — владелец был в прошлом снимке и пропал.
	KindOwnerLost = "owner_lost"
	// KindOrphanIdle — идентификатор без владельца и без трафика.
	KindOrphanIdle = "orphan_idle"
)

// Уровни уверенности.
const (
	ConfidenceHigh = "high"
	ConfidenceLow  = "low"
)

// Anomaly — найденная аномалия до записи в базу.
type Anomaly struct {
	EDOID            string
	Kind             string
	Confidence       string
	StateFingerprint string
	INN              string
	KPP              string
	ClientName       string
	Login            string
	Details          string
}

// FindAnomalies ищет поломки в строках снапшота.
//
// previousOwners — владельцы из предыдущего снимка, ключ это идентификатор.
// Может быть nil: тогда переходы состояний не проверяются.
//
// Несколько идентификаторов на одну организацию сигналом НЕ считаются:
// проверено на живых данных, что это бывает законно (разные виды деятельности,
// один логин с двумя ИНН). Отличить законный дубль от поломки структурно нельзя.
func FindAnomalies(rows []partner.EDOBillingRow, previousOwners map[string]string) []Anomaly {
	var found []Anomaly

	for _, r := range rows {
		if r.EDOID == "" {
			continue
		}

		hadOwner := false
		if previousOwners != nil {
			previous, seen := previousOwners[r.EDOID]
			hadOwner = seen && previous != ""
		}

		switch {
		case r.Owner == "" && r.Packets > 0:
			found = append(found, Anomaly{
				EDOID: r.EDOID, Kind: KindOrphanWithTraffic, Confidence: ConfidenceHigh,
				StateFingerprint: fingerprint(r.EDOID, r.Owner, r.Login, r.Packets),
				INN:              r.INN, KPP: r.KPP, ClientName: r.ClientName, Login: r.Login,
				Details: fmt.Sprintf(
					"Трафик %d пакетов не привязан ни к какому тарифу: лимита и владельца нет.",
					r.Packets),
			})

		case r.Owner == "":
			found = append(found, Anomaly{
				EDOID: r.EDOID, Kind: KindOrphanIdle, Confidence: ConfidenceLow,
				StateFingerprint: fingerprint(r.EDOID, r.Owner, r.Login, r.Packets),
				INN:              r.INN, KPP: r.KPP, ClientName: r.ClientName, Login: r.Login,
				Details:          "Идентификатор без владельца и без трафика.",
			})
		}

		if hadOwner && r.Owner == "" {
			found = append(found, Anomaly{
				EDOID: r.EDOID, Kind: KindOwnerLost, Confidence: ConfidenceHigh,
				StateFingerprint: fingerprint(r.EDOID, previousOwners[r.EDOID], r.Login, r.Packets),
				INN:              r.INN, KPP: r.KPP, ClientName: r.ClientName, Login: r.Login,
				Details: fmt.Sprintf("Владелец был «%s», теперь поле пусто.",
					previousOwners[r.EDOID]),
			})
		}
	}

	return found
}

// fingerprint описывает состояние, породившее находку. Подтверждение «это законно»
// гасит только его: изменилось состояние — сигнал поднимется снова.
func fingerprint(parts ...any) string {
	sum := sha256.Sum256([]byte(fmt.Sprint(parts...)))
	return hex.EncodeToString(sum[:16])
}
```

- [ ] **Step 4: Удалить заглушку из теста**

Убрать функцию `TestLegalDuplicateIsNotReported` — она пустая и оставлена по ошибке.

- [ ] **Step 5: Запустить, убедиться что проходит**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/
git commit -m "feat(service): detect identifier anomalies ranked by confidence"
```

---

## Task 5: Наполнение реестра и запуск проверки

**Files:**
- Modify: `internal/service/snapshot.go`
- Test: `internal/service/snapshot_test.go` (дополнить)

- [ ] **Step 1: Дописать тест**

```go
func TestTakeFillsRegistryAndRecordsAnomalies(t *testing.T) {
	reports := &fakeReports{raw: []byte(sampleCSVWithOrphan)}
	st := &fakeStore{}
	reg := &fakeRegistry{}
	bus := events.NewBus()

	svc := NewSnapshot(reports, st, bus)
	svc.WithRegistry(reg)

	if err := svc.Take(context.Background(), time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Take: %v", err)
	}

	if len(reg.upserted) != 1 {
		t.Errorf("в реестр записано %d идентификаторов, ожидали 1", len(reg.upserted))
	}
	if len(reg.events) != 1 {
		t.Fatalf("находок записано %d, ожидали 1", len(reg.events))
	}
	if reg.events[0].Kind != KindOrphanWithTraffic {
		t.Errorf("kind = %q, ожидали %q", reg.events[0].Kind, KindOrphanWithTraffic)
	}
}
```

Дополнительно определить в тестовом файле:

```go
const sampleCSVWithOrphan = "﻿Код партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
	"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
	"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
	"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
	"Сумма для партнера;Дополнительная информация\r\n" +
	"00000;Партнёр;;login;2AE-ORPHAN;Клиент;7700000001;770001001;;;0;0;56;;;;;;;\r\n"

type fakeRegistry struct {
	upserted []store.IdentifierRecord
	events   []store.AnomalyEvent
	owners   map[string]string
}

func (f *fakeRegistry) Upsert(
	ctx context.Context, records []store.IdentifierRecord, period string, seenAt time.Time,
) error {
	f.upserted = append(f.upserted, records...)
	return nil
}

func (f *fakeRegistry) RecordEvent(ctx context.Context, event store.AnomalyEvent, at time.Time) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeRegistry) OwnersByID(ctx context.Context) (map[string]string, error) {
	return f.owners, nil
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/service/ -run TestTakeFillsRegistry -v`
Expected: FAIL, `svc.WithRegistry undefined`

- [ ] **Step 3: Добавить в `internal/store/identifiers.go` метод чтения владельцев**

```go
// OwnersByID возвращает владельцев из реестра, ключ — идентификатор.
// Нужен для сравнения состояний между снимками.
func (s *Identifiers) OwnersByID(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT edo_id, owner_raw FROM edo_identifiers`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать владельцев: %w", err)
	}
	defer rows.Close()

	owners := make(map[string]string)
	for rows.Next() {
		var id, owner string
		if err := rows.Scan(&id, &owner); err != nil {
			return nil, fmt.Errorf("store: не разобрать владельца: %w", err)
		}
		owners[id] = owner
	}
	return owners, rows.Err()
}
```

- [ ] **Step 4: Расширить `internal/service/snapshot.go`**

Добавить интерфейс и поле, затем вызов в конце `Take` перед публикацией события:

```go
// Registry накапливает реестр идентификаторов и хранит находки.
type Registry interface {
	Upsert(ctx context.Context, records []store.IdentifierRecord, period string, seenAt time.Time) error
	RecordEvent(ctx context.Context, event store.AnomalyEvent, at time.Time) error
	OwnersByID(ctx context.Context) (map[string]string, error)
}

// WithRegistry подключает реестр. Без него снапшоты просто сохраняются.
func (s *Snapshot) WithRegistry(registry Registry) *Snapshot {
	s.registry = registry
	return s
}

// updateRegistry пополняет реестр и записывает найденные аномалии.
// Владельцы читаются ДО обновления реестра: иначе сравнивать будет не с чем.
func (s *Snapshot) updateRegistry(
	ctx context.Context, rows []partner.EDOBillingRow, period string, at time.Time,
) error {
	previousOwners, err := s.registry.OwnersByID(ctx)
	if err != nil {
		return err
	}

	records := make([]store.IdentifierRecord, 0, len(rows))
	for _, r := range rows {
		if r.EDOID == "" {
			continue
		}
		code, _ := partner.ParseOwner(r.Owner)
		records = append(records, store.IdentifierRecord{
			EDOID: r.EDOID, INN: r.INN, KPP: r.KPP, ClientName: r.ClientName,
			Login: r.Login, OwnerRaw: r.Owner, OwnerCode: code,
			ITSTariffs: r.ITSTariffs, Limit: r.Limit, Packets: r.Packets,
		})
	}

	for _, anomaly := range FindAnomalies(rows, previousOwners) {
		err := s.registry.RecordEvent(ctx, store.AnomalyEvent{
			EDOID: anomaly.EDOID, Kind: anomaly.Kind, Confidence: anomaly.Confidence,
			StateFingerprint: anomaly.StateFingerprint, INN: anomaly.INN, KPP: anomaly.KPP,
			ClientName: anomaly.ClientName, Login: anomaly.Login, Details: anomaly.Details,
		}, at)
		if err != nil {
			return err
		}
	}

	return s.registry.Upsert(ctx, records, period, at)
}
```

В структуру `Snapshot` добавить поле `registry Registry`, а в `Take` после успешного сохранения
и до публикации события вставить:

```go
	if s.registry != nil {
		if err := s.updateRegistry(ctx, rows, period, takenAt); err != nil {
			return fmt.Errorf("service: не обновить реестр за %s: %w", period, err)
		}
	}
```

- [ ] **Step 5: Запустить, убедиться что проходит**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/ internal/store/
git commit -m "feat(service): fill identifier registry and record anomalies on snapshot"
```

---

## Task 6: Расчёт биллинга

**Files:**
- Create: `internal/service/billing.go`
- Test: `internal/service/billing_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package service

import (
	"testing"

	"partnerops/internal/money"
	"partnerops/internal/partner"
)

func TestBillingComputesRemainder(t *testing.T) {
	limit := int64(100)
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-A", ClientName: "ООО Тест", INN: "7700000001",
			Limit: &limit, Packets: 27, PacketsBillable: 0},
	}

	report := BuildBilling(rows)

	if len(report.Clients) != 1 {
		t.Fatalf("клиентов %d, ожидали 1", len(report.Clients))
	}
	client := report.Clients[0]
	if client.Limit == nil || *client.Limit != 100 {
		t.Errorf("лимит не сохранён: %v", client.Limit)
	}
	if client.Remaining == nil || *client.Remaining != 73 {
		t.Errorf("остаток = %v, ожидали 73", client.Remaining)
	}
	if client.OverLimit {
		t.Error("клиент в пределах лимита не должен быть отмечен как превысивший")
	}
}

func TestBillingFlagsOverLimit(t *testing.T) {
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-B", ClientName: "ООО Вторма", INN: "4200000320",
			Packets: 71, PacketsBillable: 71, ClientAmount: money.Amount(710000)},
	}

	report := BuildBilling(rows)

	if len(report.NeedsAction) != 1 {
		t.Fatalf("в «требует действия» %d записей, ожидали 1", len(report.NeedsAction))
	}
	if !report.Clients[0].OverLimit {
		t.Error("клиент с пакетами к оплате должен быть отмечен")
	}
	if report.TotalDue != money.Amount(710000) {
		t.Errorf("сумма к выставлению = %d, ожидали 710000", report.TotalDue)
	}
}

func TestBillingWarnsOnLowRemainder(t *testing.T) {
	limit := int64(50)
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-C", ClientName: "ООО Почти", Limit: &limit, Packets: 47},
	}

	report := BuildBilling(rows)

	if !report.Clients[0].LowRemainder {
		t.Error("остаток 3 из 50 должен помечаться как низкий")
	}
	if len(report.NeedsAction) != 1 {
		t.Errorf("в «требует действия» %d записей, ожидали 1", len(report.NeedsAction))
	}
}

func TestBillingIgnoresRowsWithoutLimit(t *testing.T) {
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-D", ClientName: "ООО Без лимита", Packets: 5},
	}

	report := BuildBilling(rows)

	if report.Clients[0].Remaining != nil {
		t.Error("без лимита остаток посчитать нельзя, ожидали nil")
	}
	if report.Clients[0].LowRemainder {
		t.Error("без лимита предупреждать не о чем")
	}
}

func TestBillingDoesNotSumAggregateColumn(t *testing.T) {
	// «Сумма пакетов документов ЭДО по владельцу» относится к владельцу и повторяется
	// в его строках. Складывать её построчно нельзя — получится двойной счёт.
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-E", ClientName: "Клиент", Owner: "CL-1 - Иванов",
			Packets: 5, PacketsByOwner: 10, PacketsBillable: 5, ClientAmount: money.Amount(50000)},
		{EDOID: "2AE-F", ClientName: "Клиент", Owner: "CL-1 - Иванов",
			Packets: 5, PacketsByOwner: 10, PacketsBillable: 0},
	}

	report := BuildBilling(rows)

	if report.TotalDue != money.Amount(50000) {
		t.Errorf("сумма = %d, ожидали 50000: агрегатную колонку складывать нельзя", report.TotalDue)
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/service/ -run TestBilling -v`
Expected: FAIL, `undefined: BuildBilling`

- [ ] **Step 3: Реализовать**

```go
package service

import (
	"sort"

	"partnerops/internal/money"
	"partnerops/internal/partner"
)

// lowRemainderShare — доля лимита, ниже которой остаток считается низким.
const lowRemainderShare = 0.1

// BillingClient — строка отчёта по клиенту.
type BillingClient struct {
	EDOID        string       `json:"edoId"`
	ClientName   string       `json:"clientName"`
	INN          string       `json:"inn"`
	KPP          string       `json:"kpp"`
	Login        string       `json:"login"`
	Owner        string       `json:"owner"`
	Tariffs      string       `json:"tariffs"`
	Limit        *int64       `json:"limit"`
	Used         int64        `json:"used"`
	Remaining    *int64       `json:"remaining"`
	Billable     int64        `json:"billable"`
	ClientAmount money.Amount `json:"-"`
	AmountText   string       `json:"amount"`
	OverLimit    bool         `json:"overLimit"`
	LowRemainder bool         `json:"lowRemainder"`
}

// BillingReport — то, что показывается на экране биллинга.
type BillingReport struct {
	Clients     []BillingClient `json:"clients"`
	NeedsAction []BillingClient `json:"needsAction"`
	TotalDue    money.Amount    `json:"-"`
	TotalText   string          `json:"totalDue"`
}

// BuildBilling считает остатки квот и собирает список тех, кем надо заняться.
//
// Складывается только «Сумма для клиента»: колонка «по владельцу» агрегатная
// и повторяется в строках одного владельца, суммирование дало бы двойной счёт.
func BuildBilling(rows []partner.EDOBillingRow) BillingReport {
	report := BillingReport{Clients: make([]BillingClient, 0, len(rows))}

	for _, r := range rows {
		client := BillingClient{
			EDOID: r.EDOID, ClientName: r.ClientName, INN: r.INN, KPP: r.KPP,
			Login: r.Login, Owner: r.Owner, Tariffs: r.ITSTariffs,
			Limit: r.Limit, Used: r.Packets, Billable: r.PacketsBillable,
			ClientAmount: r.ClientAmount, AmountText: r.ClientAmount.String(),
			OverLimit: r.PacketsBillable > 0,
		}

		if r.Limit != nil {
			remaining := *r.Limit - r.Packets
			client.Remaining = &remaining
			client.LowRemainder = *r.Limit > 0 &&
				float64(remaining) <= float64(*r.Limit)*lowRemainderShare
		}

		report.TotalDue += r.ClientAmount
		report.Clients = append(report.Clients, client)

		if client.OverLimit || client.LowRemainder {
			report.NeedsAction = append(report.NeedsAction, client)
		}
	}

	// Сначала те, у кого больше сумма к выставлению, затем по остатку.
	sort.SliceStable(report.NeedsAction, func(i, j int) bool {
		return report.NeedsAction[i].ClientAmount > report.NeedsAction[j].ClientAmount
	})

	report.TotalText = report.TotalDue.String()
	return report
}
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat(service): compute quota remainders and who needs invoicing"
```

---

## Task 7: HTTP-эндпоинты

**Files:**
- Create: `internal/httpapi/registry.go`
- Modify: `internal/httpapi/router.go`, `cmd/server/main.go`
- Test: `internal/httpapi/registry_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"partnerops/internal/store"
)

type stubRegistry struct {
	events []store.AnomalyEvent
	acked  []string
}

func (s *stubRegistry) Events(ctx context.Context, includeAcknowledged bool) ([]store.AnomalyEvent, error) {
	return s.events, nil
}

func (s *stubRegistry) All(ctx context.Context) ([]store.IdentifierRecord, error) {
	return []store.IdentifierRecord{{EDOID: "2AE-A", ClientName: "ООО Тест"}}, nil
}

func (s *stubRegistry) Acknowledge(ctx context.Context, edoID, fingerprint, reason, author string, at time.Time) error {
	s.acked = append(s.acked, edoID+"/"+fingerprint)
	return nil
}

func TestAnomaliesReturnsList(t *testing.T) {
	reg := &stubRegistry{events: []store.AnomalyEvent{
		{ID: 1, EDOID: "2AE-A", Kind: "orphan_with_traffic", Confidence: "high"},
	}}
	h := NewRegistry(reg, nil)

	rec := httptest.NewRecorder()
	h.Anomalies(rec, httptest.NewRequest(http.MethodGet, "/api/anomalies", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	var body struct {
		Anomalies []struct {
			EDOID string `json:"edoId"`
			Kind  string `json:"kind"`
		} `json:"anomalies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("не разобрать ответ: %v", err)
	}
	if len(body.Anomalies) != 1 || body.Anomalies[0].EDOID != "2AE-A" {
		t.Errorf("в ответе %+v", body.Anomalies)
	}
}

func TestAcknowledgeMarksEvent(t *testing.T) {
	reg := &stubRegistry{}
	h := NewRegistry(reg, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/anomalies/ack",
		strings.NewReader(`{"edoId":"2AE-A","fingerprint":"fp-1","reason":"законно"}`))
	rec := httptest.NewRecorder()
	h.Acknowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200, тело %s", rec.Code, rec.Body.String())
	}
	if len(reg.acked) != 1 || reg.acked[0] != "2AE-A/fp-1" {
		t.Errorf("подтверждено %v", reg.acked)
	}
}

func TestAcknowledgeRejectsEmptyFingerprint(t *testing.T) {
	// Без отпечатка подтверждение погасило бы находку навсегда, включая будущие поломки.
	h := NewRegistry(&stubRegistry{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/anomalies/ack",
		strings.NewReader(`{"edoId":"2AE-A","fingerprint":""}`))
	rec := httptest.NewRecorder()
	h.Acknowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("код %d, ожидали 400", rec.Code)
	}
}

func TestIdentifiersReturnsRegistry(t *testing.T) {
	h := NewRegistry(&stubRegistry{}, nil)

	rec := httptest.NewRecorder()
	h.Identifiers(rec, httptest.NewRequest(http.MethodGet, "/api/identifiers", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "2AE-A") {
		t.Errorf("в ответе нет записи реестра: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/httpapi/ -run TestAnomalies -v`
Expected: FAIL, `undefined: NewRegistry`

- [ ] **Step 3: Реализовать `internal/httpapi/registry.go`**

```go
package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// RegistryStore — то, что нужно обработчикам от хранилища реестра.
type RegistryStore interface {
	Events(ctx context.Context, includeAcknowledged bool) ([]store.AnomalyEvent, error)
	All(ctx context.Context) ([]store.IdentifierRecord, error)
	Acknowledge(ctx context.Context, edoID, fingerprint, reason, author string, at time.Time) error
}

// SnapshotStore — источник строк последнего снимка для расчёта биллинга.
type SnapshotStore interface {
	Latest(ctx context.Context, period string) (store.Snapshot, []partner.EDOBillingRow, error)
}

// Registry обслуживает экраны реестра, аномалий и биллинга.
type Registry struct {
	registry  RegistryStore
	snapshots SnapshotStore
}

// NewRegistry создаёт обработчики.
func NewRegistry(registry RegistryStore, snapshots SnapshotStore) *Registry {
	return &Registry{registry: registry, snapshots: snapshots}
}

// Anomalies отдаёт находки. Параметр all=1 показывает и подтверждённые.
func (h *Registry) Anomalies(w http.ResponseWriter, r *http.Request) {
	includeAcked := r.URL.Query().Get("all") == "1"

	events, err := h.registry.Events(r.Context(), includeAcked)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать находки.")
		return
	}

	type item struct {
		ID          int64  `json:"id"`
		EDOID       string `json:"edoId"`
		Kind        string `json:"kind"`
		Confidence  string `json:"confidence"`
		Fingerprint string `json:"fingerprint"`
		INN         string `json:"inn"`
		KPP         string `json:"kpp"`
		ClientName  string `json:"clientName"`
		Login       string `json:"login"`
		Details     string `json:"details"`
		DetectedAt  string `json:"detectedAt"`
		Acked       bool   `json:"acknowledged"`
	}

	list := make([]item, 0, len(events))
	for _, e := range events {
		list = append(list, item{
			ID: e.ID, EDOID: e.EDOID, Kind: e.Kind, Confidence: e.Confidence,
			Fingerprint: e.StateFingerprint, INN: e.INN, KPP: e.KPP,
			ClientName: e.ClientName, Login: e.Login, Details: e.Details,
			DetectedAt: e.DetectedAt.Format(time.RFC3339), Acked: e.Acknowledged,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"anomalies": list})
}

type ackRequest struct {
	EDOID       string `json:"edoId"`
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
}

// Acknowledge помечает находку законной.
func (h *Registry) Acknowledge(w http.ResponseWriter, r *http.Request) {
	var req ackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}
	// Без отпечатка подтверждение погасило бы и будущие поломки этого идентификатора.
	if req.EDOID == "" || req.Fingerprint == "" {
		writeError(w, http.StatusBadRequest, "bad_request",
			"Нужны идентификатор и отпечаток состояния.")
		return
	}

	if err := h.registry.Acknowledge(
		r.Context(), req.EDOID, req.Fingerprint, req.Reason, "admin", time.Now().UTC(),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить пометку.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Identifiers отдаёт накопленный реестр.
func (h *Registry) Identifiers(w http.ResponseWriter, r *http.Request) {
	records, err := h.registry.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр.")
		return
	}

	type item struct {
		EDOID      string `json:"edoId"`
		ClientName string `json:"clientName"`
		INN        string `json:"inn"`
		KPP        string `json:"kpp"`
		Login      string `json:"login"`
		Owner      string `json:"owner"`
		OwnerCode  string `json:"ownerCode"`
		Tariffs    string `json:"tariffs"`
		Limit      *int64 `json:"limit"`
		Packets    int64  `json:"packets"`
		FirstSeen  string `json:"firstSeen"`
		LastSeen   string `json:"lastSeen"`
		Period     string `json:"period"`
	}

	list := make([]item, 0, len(records))
	for _, r := range records {
		list = append(list, item{
			EDOID: r.EDOID, ClientName: r.ClientName, INN: r.INN, KPP: r.KPP,
			Login: r.Login, Owner: r.OwnerRaw, OwnerCode: r.OwnerCode,
			Tariffs: r.ITSTariffs, Limit: r.Limit, Packets: r.Packets,
			FirstSeen: r.FirstSeenAt.Format(time.RFC3339),
			LastSeen:  r.LastSeenAt.Format(time.RFC3339),
			Period:    r.LastPeriod,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"identifiers": list})
}

// Billing отдаёт расчёт по последнему снимку указанного периода.
func (h *Registry) Billing(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = previousPeriod(time.Now())
	}

	snapshot, rows, err := h.snapshots.Latest(r.Context(), period)
	if err == store.ErrNoSnapshot {
		writeJSON(w, http.StatusOK, map[string]any{
			"period":  period,
			"clients": []any{},
			"empty":   true,
		})
		return
	}
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать снимок.")
		return
	}

	report := service.BuildBilling(rows)
	writeJSON(w, http.StatusOK, map[string]any{
		"period":      period,
		"takenAt":     snapshot.TakenAt.Format(time.RFC3339),
		"clients":     report.Clients,
		"needsAction": report.NeedsAction,
		"totalDue":    report.TotalText,
	})
}

// previousPeriod возвращает предыдущий месяц: биллинг существует только для закрытых.
func previousPeriod(now time.Time) string {
	year, month, _ := now.UTC().Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01")
}
```

- [ ] **Step 4: Подключить маршруты в `internal/httpapi/router.go`**

Изменить сигнатуру и добавить маршруты:

```go
// NewRouter собирает маршруты сервиса.
func NewRouter(auth *Auth, sse *Events, registry *Registry) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/login", auth.Login)
	mux.HandleFunc("POST /api/logout", auth.Logout)

	mux.Handle("GET /api/events", auth.RequireSession(http.HandlerFunc(sse.Stream)))

	if registry != nil {
		mux.Handle("GET /api/identifiers", auth.RequireSession(http.HandlerFunc(registry.Identifiers)))
		mux.Handle("GET /api/anomalies", auth.RequireSession(http.HandlerFunc(registry.Anomalies)))
		mux.Handle("POST /api/anomalies/ack", auth.RequireSession(http.HandlerFunc(registry.Acknowledge)))
		mux.Handle("GET /api/billing", auth.RequireSession(http.HandlerFunc(registry.Billing)))
	}

	return mux
}
```

В существующих тестах роутера передавать `nil` третьим аргументом.

- [ ] **Step 5: Подключить в `cmd/server/main.go`**

После создания `snapshots` добавить:

```go
	identifiers := store.NewIdentifiers(db)
	snapshotService := service.NewSnapshot(client, snapshots, bus).WithRegistry(identifiers)
	registryAPI := httpapi.NewRegistry(identifiers, snapshots)
```

и передать `registryAPI` третьим аргументом в `httpapi.NewRouter`.

- [ ] **Step 6: Запустить всё**

Run: `go build ./... && go test ./... -race`
Expected: всё зелёное

- [ ] **Step 7: Commit**

```bash
git add internal/httpapi/ cmd/server/
git commit -m "feat(httpapi): expose registry, anomalies and billing endpoints"
```

---

## Task 8: Проверка на живых данных

- [ ] **Step 1: Удалить базу и запустить сервис**

```powershell
Remove-Item data -Recurse -Force -ErrorAction SilentlyContinue
go run ./cmd/server
```

Expected: в логе `снапшот сохранён` с `rows=23`

- [ ] **Step 2: Войти и проверить аномалии**

```powershell
$s=$null
Invoke-WebRequest "http://127.0.0.1:8080/api/login" -Method Post -ContentType "application/json" -Body '{"login":"admin","password":"<пароль из .env>"}' -SessionVariable s | Out-Null
(Invoke-WebRequest "http://127.0.0.1:8080/api/anomalies" -WebSession $s).Content
```

Expected: ровно 7 находок — одна `orphan_with_traffic` с высокой уверенностью и шесть `orphan_idle`

- [ ] **Step 3: Проверить биллинг**

```powershell
(Invoke-WebRequest "http://127.0.0.1:8080/api/billing" -WebSession $s).Content
```

Expected: 23 клиента, в `needsAction` минимум один с суммой `710,00`

- [ ] **Step 4: Проверить подтверждение находки**

Взять `edoId` и `fingerprint` любой находки `orphan_idle`, отправить подтверждение и убедиться,
что она исчезла из списка, а с параметром `all=1` осталась.

- [ ] **Step 5: Commit**

Коммитить нечего, зафиксировать результат в следующем плане.

---

## Самопроверка плана

**Покрытие спеки.** Раздел 4.2 (мониторинг идентификаторов) закрывают задачи 1–5,
раздел 4.1 (биллинг и квоты) — задачи 6–7.

**Не входит и требует отдельных планов:** снапшоты отчёта трафика с полем `kind`,
экран заявок и генератор `.xls`, справочник программ, фронтенд на Vue.

**Согласованность типов.** `store.IdentifierRecord` и `store.AnomalyEvent` определены в Task 3
и используются в Task 5 и 7. `service.Anomaly` определён в Task 4, преобразуется в
`store.AnomalyEvent` в Task 5. `partner.ParseOwner` из Task 2 используется в Task 5.
Интерфейс `service.Registry` в Task 5 совпадает по сигнатурам с методами `store.Identifiers`
из Task 3 и с методом `OwnersByID`, добавленным в том же Task 5.
`httpapi.SnapshotStore` совпадает с `store.Snapshots.Latest` из плана фундамента.
