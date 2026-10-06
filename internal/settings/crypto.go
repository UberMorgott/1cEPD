package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// keySize — AES-256.
const keySize = 32

// KeyFileName — имя файла с ключом. Лежит рядом с базой: переносят их вместе,
// иначе сохранённый пароль расшифровать нечем.
const KeyFileName = "secret.key"

// LoadKey читает ключ рядом с базой, создавая его при первом запуске.
//
// Отдельной переменной окружения намеренно нет: на уже развёрнутых установках
// её никто не задаст, и сервис перестал бы стартовать.
func LoadKey(dbPath string) ([]byte, error) {
	path := filepath.Join(filepath.Dir(dbPath), KeyFileName)

	key, err := os.ReadFile(path) //nolint:gosec // путь собран из DB_PATH конфигурации, а не из запроса
	switch {
	case err == nil && len(key) == keySize:
		return key, nil
	case err == nil:
		return nil, fmt.Errorf("settings: файл ключа %s повреждён: нужно %d байт, а не %d", path, keySize, len(key))
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("settings: не прочитать ключ %s: %w", path, err)
	}

	key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("settings: не сгенерировать ключ: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("settings: не записать ключ %s: %w", path, err)
	}
	return key, nil
}

// encrypt шифрует пароль. Формат хранения — nonce||ciphertext, nonce свой на
// каждое сохранение.
func encrypt(key []byte, plain string) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("settings: не сгенерировать nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

// decrypt разбирает nonce||ciphertext обратно в пароль.
func decrypt(key, blob []byte) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(blob) < gcm.NonceSize() {
		return "", errors.New("settings: зашифрованный пароль короче nonce")
	}
	plain, err := gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("settings: не расшифровать пароль: %w", err)
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("settings: неверный ключ шифрования: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("settings: не собрать GCM: %w", err)
	}
	return gcm, nil
}
