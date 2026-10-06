package itsreq

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"partnerops/internal/xlsfill"
)

// readBuilt читает первый лист готового файла.
func readBuilt(t *testing.T, path string) map[[2]int]xlsfill.Value {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // тест читает файл, который сам же создал в t.TempDir()
	if err != nil {
		t.Fatalf("файл не прочитать: %v", err)
	}
	cells, err := xlsfill.ReadCells(raw, 0)
	if err != nil {
		t.Fatalf("файл не читается как .xls: %v", err)
	}
	return cells
}

func TestBuildRefusesInvalidRequest(t *testing.T) {
	builder := NewBuilder()
	request := validRequest()
	request.Rows[0].OwnerCode = "FR-FR-1"

	_, err := builder.Build(context.Background(), request, t.TempDir())

	if err == nil {
		t.Fatal("заявка с блокирующим замечанием не должна собираться")
	}
	if !strings.Contains(err.Error(), "Фреш") {
		t.Errorf("ошибка должна называть причину: %v", err)
	}
}

func TestBuildFillsTemplate(t *testing.T) {
	builder := NewBuilder()
	outDir := t.TempDir()
	request := validRequest()
	request.Password = "Secret2026"
	request.NewPassword = "Secret2027"

	out, err := builder.Build(context.Background(), request, outDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Файл лежит там, где обещает Build, и назван по коду партнёра.
	if want := filepath.Join(outDir, "ip00000.xls"); out != want {
		t.Errorf("путь к файлу = %q, ожидали %q", out, want)
	}

	// Шапка — E2:E4 и I2:I3, таблица — со строки 11 (индекс 10).
	cells := readBuilt(t, out)
	row := request.Rows[0]
	for _, want := range []struct {
		name     string
		row, col int
		value    string
	}{
		{"код партнёра", 1, 4, request.PartnerCode},
		{"пароль", 1, 8, request.Password},
		{"ответственный", 2, 4, request.Responsible},
		{"новый пароль", 2, 8, request.NewPassword},
		{"почта", 3, 4, request.Email},
		{"код партнёра в строке", firstDataRow, 1, request.PartnerCode},
		{"тариф", firstDataRow, 4, row.TariffCode},
		{"рег. номер", firstDataRow, 5, row.RegNumber},
		{"организация", firstDataRow, 6, row.CompanyName},
		{"ИНН", firstDataRow, 7, row.INN},
		{"КПП", firstDataRow, 8, row.KPP},
		{"рабочие места", firstDataRow, 9, "5"},
		{"операция", firstDataRow, 23, row.operation()},
	} {
		got := cells[[2]int{want.row, want.col}]
		if got.IsNumber || got.Text != want.value {
			t.Errorf("%s: ячейка (%d,%d) = %+v, ожидали текст %q", want.name, want.row, want.col, got, want.value)
		}
	}
	if number := cells[[2]int{firstDataRow, 0}]; !number.IsNumber || number.Number != 1 {
		t.Errorf("№ п/п = %+v, ожидали число 1", number)
	}

	// Пароли заявки на диск не попадают: кроме готового файла в outDir ничего нет.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("в outDir лишние файлы: %v", entries)
	}
}

func TestBuildTwoRowsGetSeparateLines(t *testing.T) {
	builder := NewBuilder()
	request := validRequest()
	second := validRow()
	second.RegNumber = "18117999"
	request.Rows = append(request.Rows, second)

	out, err := builder.Build(context.Background(), request, t.TempDir())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cells := readBuilt(t, out)
	if got := cells[[2]int{firstDataRow + 1, 5}].Text; got != "18117999" {
		t.Errorf("вторая строка ушла в ячейку (%d,5) = %q", firstDataRow+1, got)
	}
	if got := cells[[2]int{firstDataRow + 1, 0}]; !got.IsNumber || got.Number != 2 {
		t.Errorf("№ п/п второй строки = %+v", got)
	}
}

// Шаблон вшит в бинарь: сборка не зависит от рабочей папки, а вшитые байты
// между заявками не портятся.
func TestBuildIgnoresWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	original := bytes.Clone(formTemplate)
	builder := NewBuilder()

	for range 2 {
		if _, err := builder.Build(context.Background(), validRequest(), t.TempDir()); err != nil {
			t.Fatalf("Build вне корня репозитория: %v", err)
		}
	}
	if !bytes.Equal(formTemplate, original) {
		t.Error("Build изменил вшитый шаблон")
	}
}

func TestBuildFailsOnBrokenTemplate(t *testing.T) {
	outDir := t.TempDir()
	builder := &Builder{template: []byte("не настоящий BIFF8")}

	_, err := builder.Build(context.Background(), validRequest(), outDir)
	if err == nil || !strings.Contains(err.Error(), "не заполнить шаблон") {
		t.Fatalf("битый шаблон обязан вернуть ошибку заполнения: %v", err)
	}
	if entries, _ := os.ReadDir(outDir); len(entries) != 0 {
		t.Errorf("при ошибке файл не должен появиться: %v", entries)
	}
}

func TestFileNameUsesPartnerCode(t *testing.T) {
	if got := FileName("00000"); got != "ip00000.xls" {
		t.Errorf("имя файла = %q, ожидали ip00000.xls", got)
	}
}
