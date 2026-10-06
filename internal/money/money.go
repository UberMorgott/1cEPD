// Package money хранит денежные суммы целым числом, чтобы не терять точность.
//
// Единица хранения — миллирубль, тысячная доля рубля. Так выбрано потому, что
// отчёты 1С в одном файле смешивают два и три знака после запятой: в колонке
// «Сумма для клиента» встречается 710,00, а в «Сумма для партнера» — 355,000.
package money

import (
	"fmt"
	"math"
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
	s = strings.ReplaceAll(s, " ", "") // неразрывный пробел
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, nil
	}

	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	// Один знак или одна запятая — не ноль, а испорченное поле.
	if !strings.ContainsAny(s, "0123456789") {
		return 0, fmt.Errorf("money: в %q нет цифр", s)
	}

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

	// Границу проверяем до умножения: иначе переполнение меняет знак суммы.
	if rubles > (math.MaxInt64-fraction)/1000 {
		return 0, fmt.Errorf("money: сумма слишком велика: %q", s)
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
	// Модуль считаем в uint64: у math.MinInt64 нет положительной пары в int64.
	value := uint64(a) //nolint:gosec // G115: переполнение здесь и есть цель — из него берётся модуль math.MinInt64
	if a < 0 {
		sign = "-"
		value = -value
	}

	// Округление до копеек по правилу «половина вверх».
	kopeks := (value + 5) / 10
	return fmt.Sprintf("%s%d,%02d", sign, kopeks/100, kopeks%100)
}

// Precise печатает сумму со всеми тремя знаками: 0,125. В отличие от String
// ничего не округляет, поэтому Parse(a.Precise()) возвращает исходное значение.
func (a Amount) Precise() string {
	sign := ""
	// Модуль считаем в uint64: у math.MinInt64 нет положительной пары в int64.
	value := uint64(a) //nolint:gosec // G115: переполнение здесь и есть цель — из него берётся модуль math.MinInt64
	if a < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d,%03d", sign, value/1000, value%1000)
}
