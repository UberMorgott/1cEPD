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
