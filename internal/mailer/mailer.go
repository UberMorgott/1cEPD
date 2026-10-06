// Package mailer отправляет письма через SMTP. Только стандартная библиотека:
// одно письмо с одним вложением не стоит зависимости.
package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const (
	// implicitTLSPort — порт, на котором TLS начинается сразу, без STARTTLS.
	implicitTLSPort = 465
	// dialTimeout не даёт запросу висеть на молчащем сервере.
	dialTimeout = 10 * time.Second
	// base64Line — длина строки вложения по RFC 2045.
	base64Line = 76
)

// Config — всё, что нужно для отправки.
type Config struct {
	Host     string
	Port     int
	Login    string
	Password string
	From     string
	To       []string
}

// Send отправляет письмо. Пустое attachName означает письмо без вложения.
func Send(ctx context.Context, cfg Config, subject, body, attachName string, attach []byte) error {
	if len(cfg.To) == 0 {
		return errors.New("не указан ни один получатель")
	}

	msg, err := message(cfg, subject, body, attachName, attach)
	if err != nil {
		return err
	}

	client, stop, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer stop()
	defer func() { _ = client.Close() }()

	if err := client.Auth(smtp.PlainAuth("", cfg.Login, cfg.Password, cfg.Host)); err != nil {
		return hint(err)
	}
	if err := client.Mail(cfg.From); err != nil {
		return hint(err)
	}
	for _, to := range cfg.To {
		if err := client.Rcpt(to); err != nil {
			return hint(err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return hint(err)
	}
	if _, err := w.Write(msg); err != nil {
		return hint(err)
	}
	if err := w.Close(); err != nil {
		return hint(err)
	}
	return hint(client.Quit())
}

// dial поднимает соединение и возвращает функцию, снимающую слежение за
// контекстом: отмена запроса должна рвать соединение, но только пока идёт отправка.
func dial(ctx context.Context, cfg Config) (*smtp.Client, func(), error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	netDialer := &net.Dialer{Timeout: dialTimeout}

	var conn net.Conn
	var err error
	if cfg.Port == implicitTLSPort {
		dialer := &tls.Dialer{
			NetDialer: netDialer,
			Config:    &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12},
		}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = netDialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("не подключиться к SMTP-серверу %s: %w", addr, err)
	}

	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		stop()
		_ = conn.Close()
		return nil, nil, fmt.Errorf("SMTP-сервер %s не отвечает приветствием: %w", addr, err)
	}

	if cfg.Port != implicitTLSPort {
		tlsConf := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsConf); err != nil {
			stop()
			_ = client.Close()
			return nil, nil, fmt.Errorf("не включить STARTTLS на %s: %w", addr, err)
		}
	}
	return client, func() { stop() }, nil
}

// message собирает письмо: текст плюс необязательное вложение в base64.
func message(cfg Config, subject, body, attachName string, attach []byte) ([]byte, error) {
	var parts bytes.Buffer
	mp := multipart.NewWriter(&parts)

	text, err := mp.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, fmt.Errorf("не собрать текст письма: %w", err)
	}
	if _, err := text.Write([]byte(body)); err != nil {
		return nil, fmt.Errorf("не собрать текст письма: %w", err)
	}

	if attachName != "" {
		// Имя файла кириллическое, поэтому уезжает в заголовок закодированным по RFC 2047.
		// Чисто латинское имя Encode вернёт как есть, поэтому перевод строки снимаем сами.
		encoded := mime.BEncoding.Encode("utf-8", headerSafe(attachName))
		file, err := mp.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {`application/octet-stream; name="` + encoded + `"`},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {`attachment; filename="` + encoded + `"`},
		})
		if err != nil {
			return nil, fmt.Errorf("не собрать вложение письма: %w", err)
		}
		if _, err := file.Write(wrapBase64(attach)); err != nil {
			return nil, fmt.Errorf("не собрать вложение письма: %w", err)
		}
	}

	if err := mp.Close(); err != nil {
		return nil, fmt.Errorf("не закрыть письмо: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("From: " + headerSafe(cfg.From) + "\r\n")
	out.WriteString("To: " + headerSafe(strings.Join(cfg.To, ", ")) + "\r\n")
	out.WriteString("Subject: " + mime.BEncoding.Encode("utf-8", headerSafe(subject)) + "\r\n")
	out.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	out.WriteString("MIME-Version: 1.0\r\n")
	out.WriteString(`Content-Type: multipart/mixed; boundary="` + mp.Boundary() + "\"\r\n\r\n")
	out.Write(parts.Bytes())
	return out.Bytes(), nil
}

// headerSafe убирает из значения заголовка перевод строки. Без этого адрес или
// тема с "\r\n" дописали бы в письмо свой заголовок или чужого получателя.
// Вызывающая сторона свои значения проверяет, но заголовки собираются здесь,
// поэтому и защита живёт здесь.
var headerBreaks = strings.NewReplacer("\r", " ", "\n", " ")

func headerSafe(value string) string {
	return headerBreaks.Replace(value)
}

// wrapBase64 режет base64 на строки: почтовые серверы не обязаны принимать
// одну строку длиной в мегабайт.
func wrapBase64(data []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(data)
	var out bytes.Buffer
	for len(encoded) > base64Line {
		out.WriteString(encoded[:base64Line])
		out.WriteString("\r\n")
		encoded = encoded[base64Line:]
	}
	out.WriteString(encoded)
	return out.Bytes()
}

// hint дополняет ответ сервера подсказкой: у Яндекса два отказа из трёх — это
// не включённый доступ или чужой адрес отправителя, а текст ответа об этом молчит.
func hint(err error) error {
	if err == nil {
		return nil
	}

	var proto *textproto.Error
	text := err.Error()
	switch {
	case (errors.As(err, &proto) && proto.Code == 535) || strings.Contains(text, "535"):
		return fmt.Errorf("SMTP-сервер не принял логин и пароль: %w. "+
			"Проверьте, что пароль приложения уже активен — Яндекс включает его через 2-3 часа, "+
			"а в настройках ящика разрешён доступ почтовым клиентам и включён IMAP", err)
	case strings.Contains(text, "not owned by auth user"):
		return fmt.Errorf("SMTP-сервер отклонил адрес отправителя: %w. "+
			"Адрес «От кого» должен совпадать с почтовым ящиком или его алиасом", err)
	}
	return fmt.Errorf("SMTP-сервер вернул ошибку: %w", err)
}
