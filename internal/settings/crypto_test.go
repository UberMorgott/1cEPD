package settings

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("ключ: %v", err)
	}

	const plain = "секретная строка 1234"
	blob, err := encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if string(blob) == plain {
		t.Fatal("пароль лежит в открытом виде")
	}

	got, err := decrypt(key, blob)
	if err != nil || got != plain {
		t.Fatalf("decrypt = %q, %v; хотим %q", got, err, plain)
	}

	// Другой nonce на каждое сохранение: одинаковый пароль не даёт одинаковый шифротекст.
	again, err := encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if string(again) == string(blob) {
		t.Fatal("nonce не меняется между сохранениями")
	}

	// Чужой ключ должен давать ошибку, а не мусор.
	other := make([]byte, keySize)
	if _, err := rand.Read(other); err != nil {
		t.Fatalf("ключ: %v", err)
	}
	if _, err := decrypt(other, blob); err == nil {
		t.Fatal("чужой ключ расшифровал пароль")
	}
}

func TestLoadKeyCreatesOnceNextToDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")

	first, err := LoadKey(dbPath)
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}
	if len(first) != keySize {
		t.Fatalf("длина ключа = %d, хотим %d", len(first), keySize)
	}
	if _, err := os.Stat(filepath.Join(dir, KeyFileName)); err != nil {
		t.Fatalf("файл ключа не создан: %v", err)
	}

	second, err := LoadKey(dbPath)
	if err != nil {
		t.Fatalf("LoadKey повторно: %v", err)
	}
	if string(second) != string(first) {
		t.Fatal("второй запуск перезаписал ключ")
	}
}
