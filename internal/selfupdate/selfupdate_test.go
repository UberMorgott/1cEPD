package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func digestOf(b []byte) string {
	h := sha256.Sum256(b)
	return digestPrefix + hex.EncodeToString(h[:])
}

// fake — поддельные github.com и api.github.com на одном сервере.
type fake struct {
	location string // редирект releases/latest
	bin      []byte // файл релиза для этой платформы (nil — нет)
	digest   string // его digest ("" — нет)
	limited  bool   // API отвечает 403 по лимиту
}

func fakeGitHub(t *testing.T, tag string, f *fake) {
	t.Helper()
	asset := AssetName(Program, runtime.GOOS, runtime.GOARCH)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + Repo + "/releases/latest":
			w.Header().Set("Location", f.location)
			w.WriteHeader(http.StatusFound)
		case "/repos/" + Repo + "/releases/tags/" + tag:
			if f.limited {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", "2000000000")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			list := []string{fmt.Sprintf(`{"name":"other.zip","digest":%q}`, digestOf([]byte("zip")))}
			if f.bin != nil {
				list = append(list, fmt.Sprintf(`{"name":%q,"digest":%q}`, asset, f.digest))
			}
			_, _ = fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, tag, strings.Join(list, ","))
		case "/" + Repo + "/releases/download/" + tag + "/" + asset:
			if f.bin == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(f.bin)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	savedWeb, savedAPI := webBase, apiBase
	webBase, apiBase = srv.URL, srv.URL
	t.Cleanup(func() { webBase, apiBase = savedWeb, savedAPI })
}

var newBin = []byte("new pult-epd")

func install(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), fileName(Program))
	if err := os.WriteFile(exe, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	return exe
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) // #nosec G304 -- временный файл теста
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCompareVer(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.0.1", 1},
		{"v0.1.0", "0.1.0", 0},
		{"0.1.0", "0.2.0", -1},
		{"1.10.0", "1.9.9", 1},
		{"0.1.0", "0.1.0-rc1", 1},
		{"0.1.0-rc2", "0.1.0-rc1", 1},
		{"0.1.0", "dev", 0},
		{"dev", "0.1.0", 0},
		{"1.2", "1.2.0", 0},
	}
	for _, c := range cases {
		if got := compareVer(c.a, c.b); got != c.want {
			t.Errorf("compareVer(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if (&Release{version: "0.1.0"}).Newer("dev") {
		t.Error("dev-сборка не должна считать релиз новее: она не обновляется")
	}
}

func TestAssetName(t *testing.T) {
	cases := map[[3]string]string{
		{"pult-epd", "windows", "amd64"}: "pult-epd.exe",
		{"pult-epd", "windows", "arm64"}: "pult-epd-windows-arm64.exe",
		{"pult-epd", "linux", "amd64"}:   "pult-epd-linux-amd64",
	}
	for in, want := range cases {
		if got := AssetName(in[0], in[1], in[2]); got != want {
			t.Errorf("AssetName%v = %q, want %q", in, got, want)
		}
	}
}

func TestParseLatest(t *testing.T) {
	from := "https://github.com/" + Repo + "/releases/latest"
	rel, err := parseLatest(from, "https://github.com/"+Repo+"/releases/tag/v0.1.0")
	if err != nil || rel == nil || rel.Version() != "0.1.0" {
		t.Fatalf("parseLatest = %v, %v", rel, err)
	}
	if rel, err := parseLatest(from, "https://github.com/"+Repo+"/releases"); rel != nil || err != nil {
		t.Errorf("без релизов: %v, %v", rel, err)
	}
	for _, bad := range []string{"https://evil.example/" + Repo + "/releases/tag/v0.1.0", "/" + Repo + "/releases/tag/latest", ""} {
		if _, err := parseLatest(from, bad); err == nil {
			t.Errorf("parseLatest(%q) принят", bad)
		}
	}
}

func TestCheckFindsNewer(t *testing.T) {
	fakeGitHub(t, "v0.1.0", &fake{location: "/" + Repo + "/releases/tag/v0.1.0"})
	rel, newer, err := Check(context.Background(), "0.0.1")
	if err != nil || rel == nil || !newer || rel.Version() != "0.1.0" {
		t.Fatalf("Check(0.0.1) = %v, %v, %v", rel, newer, err)
	}
	if _, newer, _ := Check(context.Background(), "0.1.0"); newer {
		t.Error("та же версия не новее")
	}
}

func TestInstallVerifiesDigest(t *testing.T) {
	fakeGitHub(t, "v0.1.0", &fake{location: "/" + Repo + "/releases/tag/v0.1.0", bin: newBin, digest: digestOf(newBin)})
	exe := install(t)
	rel := &Release{version: "0.1.0", tag: "v0.1.0"}
	var reported int64
	if err := rel.Install(context.Background(), exe, func(done, _ int64) { reported = done }); err != nil {
		t.Fatal(err)
	}
	if got := read(t, exe); got != string(newBin) {
		t.Errorf("exe = %q, want new", got)
	}
	if reported != int64(len(newBin)) {
		t.Errorf("progress = %d, want %d", reported, len(newBin))
	}
	if err := Cleanup(exe); err != nil {
		t.Errorf("Cleanup: %v", err)
	}
	if _, err := os.Stat(OldPath(exe)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("старый exe остался: %v", err)
	}
}

func TestInstallRejectsBadDigest(t *testing.T) {
	cases := map[string]*fake{
		"чужая сумма": {bin: newBin, digest: digestOf([]byte("something else"))},
		"нет суммы":   {bin: newBin, digest: ""},
		"нет файла":   {},
		"лимит":       {bin: newBin, digest: digestOf(newBin), limited: true},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			f.location = "/" + Repo + "/releases/tag/v0.1.0"
			fakeGitHub(t, "v0.1.0", f)
			exe := install(t)
			err := (&Release{version: "0.1.0", tag: "v0.1.0"}).Install(context.Background(), exe, nil)
			if err == nil {
				t.Fatal("непроверенное обновление установлено")
			}
			if f.limited && !errors.Is(err, ErrRateLimited) {
				t.Errorf("err = %v, want rate limit", err)
			}
			if got := read(t, exe); got != "old" {
				t.Errorf("exe тронут: %q", got)
			}
			if _, err := os.Stat(newPath(exe)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf(".new остался: %v", err)
			}
		})
	}
}

func TestInstallRefusesRenamedExe(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "other.exe")
	if err := (&Release{version: "0.1.0", tag: "v0.1.0"}).Install(context.Background(), exe, nil); err == nil {
		t.Fatal("чужой exe обновлён")
	}
}

func TestReplaceRollsBack(t *testing.T) {
	exe := install(t)
	saved := rename
	t.Cleanup(func() { rename = saved })
	calls := 0
	rename = func(from, to string) error {
		calls++
		if calls == 2 { // живой exe убран, новый не встал
			return errors.New("boom")
		}
		return saved(from, to)
	}
	if err := replace(exe, newBin); err == nil {
		t.Fatal("ошибка подмены потеряна")
	}
	if got := read(t, exe); got != "old" {
		t.Errorf("старый exe не вернулся: %q", got)
	}
}
