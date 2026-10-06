// Package selfupdate заменяет exe Пульта ЭПД последним релизом с GitHub,
// сверяя загрузку с SHA-256, который GitHub сам считает для файла релиза.
//
// Порт агента обновлений agent-link. Проверка не ходит в REST API (без токена
// там 60 запросов в час на IP, общие для всего офиса за одним NAT): тег берётся
// из редиректа github.com/<repo>/releases/latest. Только установка делает один
// запрос к API — список файлов релиза с полем digest ("sha256:<hex>"); без
// него ничего не ставится. Дальше загрузка с github.com, сверка SHA-256 и
// переименования, которые работают и с запущенным exe Windows.
//
// Загрузка проверяется до того, как тронут хоть один файл на диске, а
// неудачная подмена возвращает старый exe на место.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Version — версия этой сборки без «v». Релизная сборка задаёт её так:
//
//	-ldflags "-X partnerops/internal/selfupdate.Version=0.1.0"
//
// "dev" (обычный go build) версией не считается: такая сборка не обновляется.
var Version = "dev"

// Repo — репозиторий GitHub, откуда берутся релизы.
const Repo = "UberMorgott/1cEPD"

// webBase — корень сайта GitHub (редирект latest и загрузки), apiBase — корень
// REST. Переменные, чтобы тесты подставили локальный сервер.
var (
	webBase = "https://github.com"
	apiBase = "https://api.github.com"
)

// Program — exe релиза: тот, что лежит в распакованной папке и запускается
// через start.bat.
const Program = "pult-epd"

const digestPrefix = "sha256:"

const (
	maxJSONSize  = 4 << 20   // 4 МиБ
	maxAssetSize = 256 << 20 // 256 МиБ, с огромным запасом над ~10 МБ сборки
	maxOldSlots  = 16        // сколько старых exe может ждать удаления
)

// client ограничивает каждый запрос: зависшее соединение не должно вешать обновление.
var client = &http.Client{Timeout: 5 * time.Minute}

// noRedirect нужен для releases/latest: ответ — сам редирект.
var noRedirect = &http.Client{
	Timeout:       time.Minute,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// ErrRateLimited — то, чему соответствует *RateLimitError в errors.Is.
var ErrRateLimited = errors.New("лимит запросов GitHub")

// RateLimitError — GitHub отказал по лимиту запросов. Ничего не установлено;
// Reset — когда GitHub снова начнёт отвечать.
type RateLimitError struct{ Reset time.Time }

func (e *RateLimitError) Error() string {
	return "GitHub ограничил запросы из этой сети; повторите после " + e.Reset.Local().Format("15:04")
}

// Is делает errors.Is(err, ErrRateLimited) истинным.
func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// rateLimit распознаёт 403/429, которые GitHub помечает как лимит.
func rateLimit(r *response) error {
	if r.code != http.StatusForbidden && r.code != http.StatusTooManyRequests {
		return nil
	}
	retry := strings.TrimSpace(r.header.Get("Retry-After"))
	if retry == "" && strings.TrimSpace(r.header.Get("X-RateLimit-Remaining")) != "0" {
		return nil
	}
	now := time.Now()
	reset := now.Add(time.Hour)
	if s, err := strconv.ParseInt(retry, 10, 64); err == nil && s >= 0 {
		reset = now.Add(time.Duration(s) * time.Second)
	} else if u, err := strconv.ParseInt(strings.TrimSpace(r.header.Get("X-RateLimit-Reset")), 10, 64); err == nil && u > 0 {
		reset = time.Unix(u, 0)
	}
	return &RateLimitError{Reset: reset}
}

// rename — os.Rename; тесты подменяют его, чтобы уронить шаг подмены.
var rename = os.Rename

// Valid говорит, можно ли сравнивать версию v ("dev" — нельзя).
func Valid(v string) bool {
	_, _, ok := parseVer(v)
	return ok
}

// AssetName — имя файла релиза для goos/goarch. Для windows/amd64 — то же
// имя, что у exe в папке пакета («pult-epd.exe»), иначе
// «<program>-<goos>-<goarch>[.exe]».
func AssetName(program, goos, goarch string) string {
	if goos == "windows" && goarch == "amd64" {
		return program + ".exe"
	}
	name := program + "-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// fileName — имя exe программы в этой системе.
func fileName(program string) string {
	if runtime.GOOS == "windows" {
		return program + ".exe"
	}
	return program
}

// OldPath — куда обновление убирает заменённый exe. Запущенный exe Windows
// удалить нельзя, только переименовать, и он заперт, пока жив его процесс;
// следующий старт (Cleanup) его удаляет.
func OldPath(exePath string) string {
	dir, base := filepath.Split(exePath)
	return filepath.Join(dir, "."+base+".old")
}

func oldPathSlot(exePath string, slot int) string {
	if slot == 0 {
		return OldPath(exePath)
	}
	return OldPath(exePath) + "." + strconv.Itoa(slot)
}

func newPath(exePath string) string {
	dir, base := filepath.Split(exePath)
	return filepath.Join(dir, "."+base+".new")
}

// Release — последний опубликованный релиз.
type Release struct {
	version string // без «v»
	tag     string // как опубликован, «v0.1.0»
}

// Version — версия релиза без «v».
func (r *Release) Version() string { return r.version }

// Newer — релиз строго новее current: защита от отката. Нечитаемая версия с
// любой стороны («dev», кривой тег) новее не бывает.
func (r *Release) Newer(current string) bool { return compareVer(r.version, current) > 0 }

// Latest находит последний релиз по редиректу github.com/<repo>/releases/latest.
// Репозиторий без релизов — (nil, nil): обновляться не на что.
func Latest(ctx context.Context) (*Release, error) {
	u := webBase + "/" + Repo + "/releases/latest"
	resp, err := fetch(ctx, noRedirect, u, "text/html", maxJSONSize, nil)
	if err != nil {
		return nil, err
	}
	if err := rateLimit(resp); err != nil {
		return nil, err
	}
	switch resp.code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("github releases/latest: HTTP %d вместо редиректа на тег", resp.code)
	}
	return parseLatest(u, resp.header.Get("Location"))
}

// parseLatest читает релиз из Location редиректа releases/latest на том же
// хосте: «.../<repo>/releases/tag/<tag>». Редирект на «.../<repo>/releases»
// значит, что релизов нет.
func parseLatest(from, location string) (*Release, error) {
	base, err := url.Parse(from)
	if err != nil {
		return nil, err
	}
	loc, err := base.Parse(strings.TrimSpace(location))
	if err != nil || location == "" {
		return nil, fmt.Errorf("github releases/latest: плохой редирект %q", location)
	}
	if loc.Host != base.Host {
		return nil, fmt.Errorf("github releases/latest: редирект на другой хост %q", location)
	}
	prefix := "/" + Repo + "/releases"
	if strings.TrimSuffix(loc.Path, "/") == prefix {
		return nil, nil
	}
	tag, ok := strings.CutPrefix(loc.Path, prefix+"/tag/")
	if !ok || !validTag(tag) {
		return nil, fmt.Errorf("github releases/latest: редирект %q — не тег релиза", location)
	}
	return &Release{version: strings.TrimPrefix(tag, "v"), tag: tag}, nil
}

// validTag — тег версии («v0.1.0») только из символов, безопасных в пути URL.
func validTag(tag string) bool {
	if !Valid(tag) {
		return false
	}
	for _, c := range tag {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '.' && c != '-' && c != '+' {
			return false
		}
	}
	return true
}

// Check возвращает последний релиз и то, новее ли он current.
func Check(ctx context.Context, current string) (rel *Release, newer bool, err error) {
	rel, err = Latest(ctx)
	if err != nil || rel == nil {
		return nil, false, err
	}
	return rel, rel.Newer(current), nil
}

// digest спрашивает у API (один запрос) SHA-256 файла name в этом релизе.
// Нет файла или нет digest — ошибка: непроверенное не ставим.
func (r *Release) digest(ctx context.Context, name string) (string, error) {
	resp, err := fetch(ctx, client, apiBase+"/repos/"+Repo+"/releases/tags/"+url.PathEscape(r.tag),
		"application/vnd.github+json", maxJSONSize, nil)
	if err != nil {
		return "", err
	}
	if err := rateLimit(resp); err != nil {
		return "", err
	}
	if resp.code != http.StatusOK {
		return "", fmt.Errorf("github releases API: HTTP %d", resp.code)
	}
	var gh struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(resp.body, &gh); err != nil {
		return "", fmt.Errorf("ответ releases API не разобран: %w", err)
	}
	for _, a := range gh.Assets {
		if a.Name == name {
			if d := parseDigest(a.Digest); d != "" {
				return d, nil
			}
			return "", fmt.Errorf("у релиза v%s нет sha256 для %s: непроверяемое обновление не ставим", r.version, name)
		}
	}
	return "", fmt.Errorf("в релизе v%s нет файла %s для этой платформы", r.version, name)
}

// Progress сообщает о загрузке: done байт из total (-1 — неизвестно).
type Progress func(done, total int64)

// Install берёт у API digest файла, качает exe релиза с github.com (сообщая
// progress, если он не nil), сверяет SHA-256 и подменяет им exePath. Лимит,
// отсутствующий или неверный digest прерывают до того, как тронут диск.
func (r *Release) Install(ctx context.Context, exePath string, progress Progress) error {
	if base := filepath.Base(exePath); !strings.EqualFold(base, fileName(Program)) {
		return fmt.Errorf("%s — не exe Пульта ЭПД (%s): переименованный файл не обновляем", base, fileName(Program))
	}
	name := AssetName(Program, runtime.GOOS, runtime.GOARCH)
	want, err := r.digest(ctx, name)
	if err != nil {
		return err
	}
	dl := webBase + "/" + Repo + "/releases/download/" + url.PathEscape(r.tag) + "/" + name
	if strings.HasPrefix(webBase, "https://") && !strings.HasPrefix(dl, "https://") {
		return fmt.Errorf("адрес файла релиза не HTTPS: %q", dl)
	}
	resp, err := fetch(ctx, client, dl, "application/octet-stream", maxAssetSize, progress)
	if err != nil {
		return fmt.Errorf("загрузка %s: %w", name, err)
	}
	if err := rateLimit(resp); err != nil {
		return err
	}
	if resp.code != http.StatusOK {
		return fmt.Errorf("загрузка %s: HTTP %d", name, resp.code)
	}
	sum := sha256.Sum256(resp.body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("контрольная сумма %s не сошлась: sha256 %s, в релизе %s", name, got, want)
	}
	return replace(exePath, resp.body)
}

// parseDigest возвращает hex из "sha256:<hex>" в нижнем регистре или "".
func parseDigest(d string) string {
	hexSum, ok := strings.CutPrefix(strings.TrimSpace(d), digestPrefix)
	if !ok || len(hexSum) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(hexSum); err != nil {
		return ""
	}
	return strings.ToLower(hexSum)
}

// replace подменяет exe переименованиями, которые работают и с запущенным
// файлом (перезаписать живой .exe Windows не даёт, переименовать — даёт):
// новый пишется в «.<base>.new», живой уезжает в «.<base>.old», новый встаёт
// на место. Любая ошибка возвращает старый файл.
func replace(path string, data []byte) error {
	mode := os.FileMode(0o755)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	np := newPath(path)
	err := os.WriteFile(np, data, mode) // #nosec G306 -- exe должен быть исполняемым
	if err == nil {
		err = os.Chmod(np, mode)
	}
	if err != nil {
		_ = os.Remove(np)
		return fmt.Errorf("не записать новый %s: %w", filepath.Base(path), err)
	}
	old, err := parkOld(path)
	if err != nil {
		_ = os.Remove(np)
		return fmt.Errorf("не убрать в сторону %s: %w", filepath.Base(path), err)
	}
	if err := rename(np, path); err != nil {
		_ = rename(old, path)
		_ = os.Remove(np)
		return fmt.Errorf("не поставить новый %s: %w", filepath.Base(path), err)
	}
	// Запущенный exe заперт: прячем его до Cleanup на следующем старте.
	if err := os.Remove(old); err != nil {
		_ = hideFile(old)
	}
	return nil
}

// parkOld находит свободное имя для старого exe, даже если предыдущий старый
// ещё работает. Удаляются только обычные файлы ровно с нашими именами.
func parkOld(path string) (string, error) {
	var occupied error
	for slot := 0; slot <= maxOldSlots; slot++ {
		old := oldPathSlot(path, slot)
		if st, err := os.Lstat(old); err == nil && !st.Mode().IsRegular() {
			continue
		}
		if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
			occupied = err
			continue
		}
		if err := rename(path, old); err != nil {
			return "", err
		}
		return old, nil
	}
	if occupied != nil {
		return "", fmt.Errorf("все %d мест для старого exe заняты: %w", maxOldSlots+1, occupied)
	}
	return "", fmt.Errorf("все %d мест для старого exe заняты", maxOldSlots+1)
}

// Cleanup удаляет то, что обновление оставило рядом с exePath: заменённый
// exe (заперт, пока жил его процесс) и недописанный «.new». Возвращает первую
// ошибку: файл, который ещё не удалить.
func Cleanup(exePath string) error {
	var first error
	remove := func(path string) {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
			first = err
		}
	}
	for slot := 0; slot <= maxOldSlots; slot++ {
		old := oldPathSlot(exePath, slot)
		if st, err := os.Lstat(old); err == nil && !st.Mode().IsRegular() {
			continue
		}
		remove(old)
	}
	remove(newPath(exePath))
	return first
}

// response — то, что прочитал fetch.
type response struct {
	code   int
	header http.Header
	body   []byte
}

// fetch скачивает u через c: статус, заголовки и тело (не больше limit байт).
// Тело ответа 200 по мере поступления сообщается progress (если не nil).
func fetch(ctx context.Context, c *http.Client, u, accept string, limit int64, progress Progress) (*response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "pult-epd-selfupdate/"+Version) // без него GitHub отказывает
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	src := io.LimitReader(resp.Body, limit)
	if progress != nil && resp.StatusCode == http.StatusOK {
		total := resp.ContentLength
		progress(0, total)
		src = &counter{r: src, total: total, report: progress}
	}
	body, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) == limit {
		return nil, fmt.Errorf("ответ %s больше %d байт", u, limit)
	}
	return &response{code: resp.StatusCode, header: resp.Header, body: body}, nil
}

// counter сообщает report о каждом чтении r.
type counter struct {
	r      io.Reader
	done   int64
	total  int64
	report Progress
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.done += int64(n)
		c.report(c.done, c.total)
	}
	return n, err
}

// compareVer сравнивает версии «X.Y.Z[-pre][+build]» («v» в начале
// необязательна): -1, 0 или +1; 0, если любая нечитаема — Newer тогда ложно.
// Предрелиз раньше своего релиза; метки предрелизов сравниваются как строки.
func compareVer(a, b string) int {
	an, ap, aok := parseVer(a)
	bn, bp, bok := parseVer(b)
	if !aok || !bok {
		return 0
	}
	for i := range an {
		if an[i] != bn[i] {
			if an[i] > bn[i] {
				return 1
			}
			return -1
		}
	}
	switch {
	case ap == bp:
		return 0
	case ap == "":
		return 1
	case bp == "":
		return -1
	case ap > bp:
		return 1
	default:
		return -1
	}
}

// parseVer разбирает «vX.Y.Z-pre+build» на числа и метку предрелиза; ok
// ложно для всего, кроме трёх неотрицательных десятичных полей.
func parseVer(v string) (nums [3]int, pre string, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		pre, v = v[i+1:], v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nums, "", false
	}
	for i, p := range parts {
		if p == "" || strings.TrimLeft(p, "0123456789") != "" {
			return nums, "", false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nums, "", false
		}
		nums[i] = n
	}
	return nums, pre, true
}
