package itsreq

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"partnerops/internal/xlsfill"
)

// formTemplate — шаблон формы заявки вер. 3.09. Вшит в бинарь: путь от рабочей
// папки ломался, стоило запустить программу ярлыком из другого места.
//
//go:embed ip00000_ru.xls
var formTemplate []byte

// Builder собирает файл-заявку из шаблона .xls (BIFF8): робот принимает только
// этот формат, запись в него делает пакет xlsfill прямо в процессе.
type Builder struct {
	template []byte
}

// NewBuilder создаёт сборщик на вшитом шаблоне.
func NewBuilder() *Builder {
	return &Builder{template: formTemplate}
}

// FileName возвращает имя файла по правилам «1С»: ip плюс код партнёра.
func FileName(partnerCode string) string {
	return "ip" + partnerCode + ".xls"
}

// Build проверяет заявку, раскладывает её по ячейкам и заполняет шаблон.
// Возвращает путь к готовому файлу внутри outDir.
func (b *Builder) Build(ctx context.Context, request Request, outDir string) (string, error) {
	issues := Validate(request)
	if Blocking(issues) {
		return "", fmt.Errorf("itsreq: заявка не прошла проверку: %s", FirstBlocking(issues))
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Значения, включая пароли заявки, идут в файл из памяти: промежуточных
	// файлов с ними на диске нет.
	data, err := xlsfill.Fill(b.template, Cells(request))
	if err != nil {
		return "", fmt.Errorf("itsreq: не заполнить шаблон: %w", err)
	}

	outPath := filepath.Join(outDir, FileName(request.PartnerCode))
	// 0o600: в файле пароли заявки. Имя — из кода партнёра, а Validate выше
	// пропускает только пять цифр, так что за outDir путь не выйдет.
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return "", fmt.Errorf("itsreq: файл не создан: %w", err)
	}
	return outPath, nil
}

// FirstBlocking возвращает текст первого блокирующего замечания.
func FirstBlocking(issues []Issue) string {
	for _, issue := range issues {
		if issue.Blocking {
			if issue.Row > 0 {
				return fmt.Sprintf("строка %d: %s", issue.Row, issue.Message)
			}
			return issue.Message
		}
	}
	return "неизвестная причина"
}
