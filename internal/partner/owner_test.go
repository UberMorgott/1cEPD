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
