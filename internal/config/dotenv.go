package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// LoadDotEnv читает файл вида KEY=value и возвращает разобранные пары.
// Отсутствие файла ошибкой не считается: в проде настройки обычно приходят
// из окружения, а .env нужен для локального запуска.
//
// Поддерживается только то, что реально встречается в наших файлах:
// комментарии со знака #, пустые строки и значения в кавычках.
// Подстановки переменных и многострочных значений здесь нет намеренно.
func LoadDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path) //nolint:gosec // путь приходит из нашей же конфигурации, а не из запроса
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: не открыть %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		key, value, found := strings.Cut(text, "=")
		if !found {
			return nil, fmt.Errorf("config: %s строка %d: нет знака равенства", path, line)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("config: %s строка %d: пустое имя переменной", path, line)
		}

		values[key] = unquote(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("config: не прочитать %s: %w", path, err)
	}
	return values, nil
}

// GetterWith возвращает Getter, который сначала смотрит в окружение,
// а затем в значения из файла. Окружение приоритетнее: так запуск
// с временными настройками не требует правки файла.
func GetterWith(values map[string]string) Getter {
	return func(key string) string {
		if fromEnv := os.Getenv(key); fromEnv != "" {
			return fromEnv
		}
		return values[key]
	}
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	first, last := value[0], value[len(value)-1]
	if first == last && (first == '"' || first == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
