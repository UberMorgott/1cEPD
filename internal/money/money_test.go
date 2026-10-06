package money

import (
	"math"
	"testing"
)

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

// Целая часть, помещающаяся в int64, после умножения на 1000 переполняет его
// и меняет знак — это должно быть ошибкой, а не тихо неверной суммой.
func TestParseRejectsOverflow(t *testing.T) {
	for _, in := range []string{
		"99999999999999999",
		"9223372036854775,808",
		"-99999999999999999",
	} {
		got, err := Parse(in)
		if err == nil {
			t.Errorf("Parse(%q) = %d, ожидали ошибку о слишком большой сумме", in, got)
		}
	}
}

// Граница диапазона должна разбираться без ошибки.
func TestParseAcceptsMaxAmount(t *testing.T) {
	got, err := Parse("9223372036854775,807")
	if err != nil {
		t.Fatalf("Parse вернул ошибку: %v", err)
	}
	if got != Amount(math.MaxInt64) {
		t.Errorf("Parse = %d, ожидали %d", got, int64(math.MaxInt64))
	}
}

// Строка без единой цифры — не ноль, а ошибка.
func TestParseRejectsSignOnly(t *testing.T) {
	for _, in := range []string{"-", ",", "-,"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %d, ожидали ошибку", in, got)
		}
	}
}

// String не должен переполняться на math.MinInt64.
func TestStringMinInt64(t *testing.T) {
	want := "-9223372036854775,81"
	if got := Amount(math.MinInt64).String(); got != want {
		t.Errorf("Amount(MinInt64).String() = %q, ожидали %q", got, want)
	}
}

// Precise печатает все три знака, поэтому round-trip через него не теряет точность.
func TestPreciseRoundTrip(t *testing.T) {
	if got := Amount(125).Precise(); got != "0,125" {
		t.Errorf("Amount(125).Precise() = %q, ожидали \"0,125\"", got)
	}
	for _, want := range []Amount{0, 125, 710000, 355000, -5500, -1, math.MaxInt64} {
		got, err := Parse(want.Precise())
		if err != nil {
			t.Errorf("Parse(%q) вернул ошибку %v", want.Precise(), err)
			continue
		}
		if got != want {
			t.Errorf("round-trip %d -> %q -> %d", want, want.Precise(), got)
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
		{125, "0,13"}, // округление до копеек при отображении
		{-5500, "-5,50"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Amount(%d).String() = %q, ожидали %q", c.in, got, c.want)
		}
	}
}
