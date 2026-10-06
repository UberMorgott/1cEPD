package mailer

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func TestMessageBuildsMultipartWithCyrillicAttachment(t *testing.T) {
	cfg := Config{From: "robot@example.com", To: []string{"a@example.com", "b@example.com"}}
	attach := []byte(strings.Repeat("данные заявки ", 40))

	raw, err := message(cfg, "Заявка ЭПД", "Во вложении заявка.", "ip00000 заявка.xls", attach)
	if err != nil {
		t.Fatalf("message: %v", err)
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("письмо не разбирается: %v", err)
	}

	decoder := new(mime.WordDecoder)
	subject, err := decoder.DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subject != "Заявка ЭПД" {
		t.Fatalf("тема письма = %q, %v; хотим «Заявка ЭПД»", subject, err)
	}
	if got := msg.Header.Get("To"); got != "a@example.com, b@example.com" {
		t.Fatalf("получатели = %q", got)
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("тип письма = %q, %v; хотим multipart/mixed", mediaType, err)
	}
	if params["boundary"] == "" {
		t.Fatal("в письме нет boundary")
	}

	reader := multipart.NewReader(msg.Body, params["boundary"])

	text, err := reader.NextPart()
	if err != nil {
		t.Fatalf("первая часть не читается: %v", err)
	}
	if got := text.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("тип текстовой части = %q", got)
	}
	body, err := io.ReadAll(text)
	if err != nil || string(body) != "Во вложении заявка." {
		t.Fatalf("текст письма = %q, %v", body, err)
	}

	// multipart.Reader сам заголовки не декодирует, поэтому имя проверяем сырым:
	// оно должно уехать закодированным по RFC 2047, а не кириллицей в заголовке.
	file, err := reader.NextPart()
	if err != nil {
		t.Fatalf("вложение не читается: %v", err)
	}
	disposition := file.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, "=?utf-8?b?") && !strings.Contains(disposition, "=?UTF-8?B?") {
		t.Fatalf("имя файла не закодировано по RFC 2047: %q", disposition)
	}
	_, dispParams, err := mime.ParseMediaType(disposition)
	if err != nil {
		t.Fatalf("Content-Disposition не разбирается: %v", err)
	}
	name, err := decoder.DecodeHeader(dispParams["filename"])
	if err != nil || name != "ip00000 заявка.xls" {
		t.Fatalf("имя файла = %q, %v", name, err)
	}
	if got := file.Header.Get("Content-Transfer-Encoding"); got != "base64" {
		t.Fatalf("кодировка вложения = %q", got)
	}

	encoded, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("тело вложения не читается: %v", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(encoded)), "\r\n") {
		if len(line) > base64Line {
			t.Fatalf("строка base64 длиной %d символов", len(line))
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(encoded), "\r\n", ""))
	if err != nil || string(decoded) != string(attach) {
		t.Fatalf("вложение не восстанавливается: %v", err)
	}

	if _, err := reader.NextPart(); err != io.EOF {
		t.Fatalf("после вложения ждём конец письма, получили %v", err)
	}
}

func TestMessageWithoutAttachmentHasOnlyText(t *testing.T) {
	cfg := Config{From: "robot@example.com", To: []string{"a@example.com"}}
	raw, err := message(cfg, "Проверка настроек SMTP", "Тест.", "", nil)
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	if strings.Contains(string(raw), "Content-Disposition: attachment") {
		t.Fatal("в письме без вложения появилось вложение")
	}
}

// Перевод строки в теме, адресе или имени файла не должен добавлять заголовки.
func TestMessageRejectsHeaderInjection(t *testing.T) {
	cfg := Config{
		From: "robot@example.com\r\nBcc: evil@example.com",
		To:   []string{"a@example.com"},
	}

	raw, err := message(cfg, "Заявка\r\nX-Injected: yes", "текст", "ip00000.xls\r\nX-Also: yes", []byte("x"))
	if err != nil {
		t.Fatalf("message: %v", err)
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("письмо не разбирается: %v", err)
	}
	for _, name := range []string{"Bcc", "X-Injected", "X-Also"} {
		if got := msg.Header.Get(name); got != "" {
			t.Errorf("в письме появился заголовок %s = %q", name, got)
		}
	}
}
