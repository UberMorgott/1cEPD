package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// memoryEPDImport — хранилище выгрузки биллинга ЭПД в памяти.
type memoryEPDImport struct {
	rows []partner.EPDBillingRow
	info *store.EPDImportInfo
}

func (m *memoryEPDImport) Rows(context.Context) ([]partner.EPDBillingRow, error) { return m.rows, nil }

func (m *memoryEPDImport) Replace(_ context.Context, name string, rows []partner.EPDBillingRow, at time.Time) error {
	m.rows = rows
	m.info = &store.EPDImportInfo{ImportedAt: at, FileName: name, Total: len(rows)}
	return nil
}

func (m *memoryEPDImport) Last(context.Context) (store.EPDImportInfo, bool, error) {
	if m.info == nil {
		return store.EPDImportInfo{}, false, nil
	}
	return *m.info, true, nil
}

// Клиент только из выгрузки ЭПД попадает в подбор; клиенту из API выгрузка
// дописывает лишь пустые поля; ФИО владельца — ответственный, почта — нет.
func TestClientsMergeEPDImport(t *testing.T) {
	registry := registryClients{
		{EDOID: "2AE-A1", INN: "7700000201", KPP: "770001001", ClientName: "ООО Из API", Login: "api-login"},
	}
	imported := &memoryEPDImport{rows: []partner.EPDBillingRow{
		{EDOID: "2AE-A1", INN: "7700000201", KPP: "770001001", ClientName: "ООО Из выгрузки", Login: "csv-login",
			Owner: "CL-900010 - Тестова Мария Ивановна", OwnerCode: "CL-900010", OwnerContact: "Тестова Мария Ивановна",
			ITSTariffs: "ЭПД-100"},
		{EDOID: "2AE-I1", INN: "770000000202", ClientName: "ИП Пробный", Login: "1c-fresh_probe@example.test",
			Owner: "FR-FR-900011 - probe@example.test", OwnerCode: "FR-FR-900011", OwnerContact: "probe@example.test",
			ITSTariffs: "ИТСааС тест"},
		{EDOID: "2AE-I2", INN: "7700000203", KPP: "770001001", ClientName: "ООО Вторая проба",
			Owner: "FR-FR-900011 - probe@example.test", OwnerCode: "FR-FR-900011", OwnerContact: "probe@example.test"},
	}}
	cards := fetchClients(t, NewRequests(draftsStore{}, nil).WithClients(registry).WithEPDImport(imported))

	api := cards["7700000201"]
	if api.Row.CompanyName != "ООО Из API" || api.Row.Login != "api-login" {
		t.Errorf("выгрузка затёрла данные API: %+v", api.Row)
	}
	if api.Row.OwnerCode != "CL-900010" || api.Row.Responsible != "Тестова Мария Ивановна" {
		t.Errorf("пустые поля не дополнены выгрузкой: %+v", api.Row)
	}
	ip, ok := cards["770000000202"]
	if !ok {
		t.Fatalf("клиента из выгрузки нет в подборе: %+v", cards)
	}
	if ip.Row.CompanyName != "ИП Пробный" || ip.Row.KPP != "" || ip.Row.OwnerCode != "FR-FR-900011" ||
		ip.Row.Login != "1c-fresh_probe@example.test" || !slices.Equal(ip.EDOIDs, []string{"2AE-I1"}) {
		t.Errorf("ИП из выгрузки: %+v", ip)
	}
	if ip.Row.Responsible != "" {
		t.Errorf("почта владельца стала ответственным: %q", ip.Row.Responsible)
	}
	if _, ok := cards["7700000203"]; !ok {
		t.Error("вторая организация владельца из выгрузки потеряна")
	}
}

// ФИО из базы абонентов 1С и последняя заявка важнее ФИО из выгрузки.
func TestClientsEPDImportResponsiblePriority(t *testing.T) {
	registry := registryClients{}
	imported := &memoryEPDImport{rows: []partner.EPDBillingRow{
		{EDOID: "2AE-P1", INN: "7700000301", KPP: "770001001", ClientName: "ООО Первая",
			OwnerCode: "CL-900020", OwnerContact: "Выгрузкин Пётр Петрович"},
		{EDOID: "2AE-P2", INN: "7700000302", KPP: "770001001", ClientName: "ООО Вторая",
			OwnerCode: "CL-900021", OwnerContact: "Выгрузкин Иван Петрович"},
	}}
	subs := subscriberBase{{Code: "CL-900020", Name: "Базова Анна Сергеевна"}}
	drafts := draftsStore{payloads: []string{`{"rows":[{"inn":"7700000302","kpp":"770001001","responsible":"Заявкин"}]}`}}
	cards := fetchClients(t, NewRequests(drafts, nil).WithClients(registry).WithSubscribers(subs).WithEPDImport(imported))
	if got := cards["7700000301"].Row.Responsible; got != "Базова Анна Сергеевна" {
		t.Errorf("база абонентов важнее выгрузки: %q", got)
	}
	if got := cards["7700000302"].Row.Responsible; got != "Заявкин" {
		t.Errorf("последняя заявка важнее выгрузки: %q", got)
	}
}

func postEPDImport(t *testing.T, h *Registry, csv string) map[string]any {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "billing.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(csv))
	_ = form.Close()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/clients/epd-import", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ImportEPDBilling(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestImportEPDBillingAddsClients(t *testing.T) {
	imported := &memoryEPDImport{}
	h := fixtureRegistryAPI(t).WithEPDImport(imported)
	header := "Код партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;Наименование клиента;ИНН клиента;" +
		"КПП клиента;Тарифы ИТС;Лимит;Кол-во документов ЭПД;Сумма документов ЭПД по владельцу;Льгота;" +
		"Количество документов ЭПД к оплате;Тариф для клиента;Сумма для клиента;Сумма для партнера;Исх. ЭТРН;Исх. ЭПЛ;" +
		"Дополнительная информация"
	csv := header + "\r\n" +
		"1;П;FR-FR-900030 - new@example.test;1c-fresh_new;2AE-N1;ИП Новенький;770000000401;;;;0;;;;;;;;;\r\n" +
		";;;1c-fresh_new;2AE-N2;ООО Новое;7700000402;770001001;;;3;;;;;;;;;\r\n" +
		// Клиент, уже известный реестру: обновлён, а не добавлен.
		"1;П;CL-100 - Переработка;;2AE-R1;Центр переработки;" + processorINN + ";" + processorKPP + ";;;0;;;;;;;;;\r\n"

	out := postEPDImport(t, h, csv)
	if out["added"] != float64(2) || out["updated"] != float64(1) {
		t.Errorf("счётчики: %v", out)
	}

	var list struct {
		Clients []struct {
			Key             string   `json:"key"`
			Ours            bool     `json:"ours"`
			Sources         []string `json:"sources"`
			SubscriberCodes []string `json:"subscriberCodes"`
		} `json:"clients"`
		EPDImport map[string]any `json:"epdImport"`
	}
	if code := getJSON(t, h.ClientList, "GET /api/clients", "/api/clients", &list); code != http.StatusOK {
		t.Fatalf("код %d", code)
	}
	if list.EPDImport["rows"] != float64(3) || list.EPDImport["fileName"] != "billing.csv" {
		t.Errorf("сведения о загрузке: %v", list.EPDImport)
	}
	found := 0
	for _, c := range list.Clients {
		if c.Key == "770000000401" || c.Key == "7700000402-770001001" {
			found++
			if !c.Ours || !slices.Contains(c.Sources, sourceImport) || !slices.Contains(c.SubscriberCodes, "FR-FR-900030") {
				t.Errorf("клиент из выгрузки: %+v", c)
			}
		}
	}
	if found != 2 {
		t.Errorf("клиентов из выгрузки в реестре %d, ожидали 2", found)
	}

	// Повторная загрузка заменяет прежнюю: оба клиента теперь известны.
	out = postEPDImport(t, h, csv)
	if out["added"] != float64(0) || out["updated"] != float64(3) {
		t.Errorf("повторная загрузка: %v", out)
	}
}

func TestImportEPDBillingRejectsOtherFiles(t *testing.T) {
	h := fixtureRegistryAPI(t).WithEPDImport(&memoryEPDImport{})
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "x.csv")
	_, _ = part.Write([]byte(strings.Repeat("a;b\r\n", 3)))
	_ = form.Close()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/clients/epd-import", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ImportEPDBilling(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("чужой файл: код %d", rec.Code)
	}
}
