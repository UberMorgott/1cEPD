package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/store"
)

func TestTariffsEndpointListsThirteen(t *testing.T) {
	h := NewRequests(nil, nil)

	rec := httptest.NewRecorder()
	h.Tariffs(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/tariffs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d", rec.Code)
	}
	var body struct {
		Tariffs []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"tariffs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не разбирается: %v", err)
	}
	if len(body.Tariffs) != 13 {
		t.Errorf("тарифов %d, ожидали 13", len(body.Tariffs))
	}
}

func TestValidateEndpointReportsFreshOwner(t *testing.T) {
	h := NewRequests(nil, nil)

	payload := `{"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
		{"tariffCode":"2083","regNumber":"1","companyName":"ООО","inn":"7811000310",
		 "kpp":"780001001","workplaces":1,"responsible":"Иван","phoneCode":"812",
		 "phone":"1","startDate":"01.10.26","deliveryType":"0","ownerCode":"FR-FR-1"}]}`

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/validate", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, проверка должна отвечать 200 со списком замечаний", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Фреш") {
		t.Errorf("в ответе нет замечания про Фреш: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"blocking":true`) {
		t.Error("замечание должно быть блокирующим")
	}
}

// stubDirectory — справочник 1С с одним абонентом; реквизиты вымышлены.
type stubDirectory struct{}

func (stubDirectory) Subscribers(context.Context) ([]itsreq.Subscriber, error) {
	return []itsreq.Subscriber{{Code: "CL-1000001", RegNumbers: []string{"800000001"},
		Organizations: []itsreq.Organization{{Name: "ООО Альфа", INN: "7700000001", KPP: "770001001"}}}}, nil
}

func (stubDirectory) Products(context.Context, []string) (map[string]string, error) {
	return map[string]string{"800000001": "Бухгалтерия"}, nil
}

func (stubDirectory) Contracts(context.Context, []string) (map[string][]itsreq.Contract, error) {
	return map[string][]itsreq.Contract{}, nil
}

func (stubDirectory) LoginRegNumbers(context.Context, string) ([]string, error) { return nil, nil }

func (stubDirectory) UserExists(context.Context, string, string) (bool, error) { return true, nil }

func TestValidateEndpointReturnsSuggestionsAndBlocksKPP(t *testing.T) {
	h := NewRequests(nil, nil).WithDirectory(stubDirectory{})

	payload := `{"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
		{"tariffCode":"2083","regNumber":"800000001","companyName":"","inn":"7700000001",
		 "kpp":"770001002","workplaces":1,"responsible":"Иван","phoneCode":"812",
		 "phone":"1","startDate":"01.10.26","deliveryType":"0"}]}`
	rec := httptest.NewRecorder()
	h.Validate(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/validate",
		strings.NewReader(payload)))

	var body struct {
		Blocking    bool                `json:"blocking"`
		Issues      []itsreq.Issue      `json:"issues"`
		Notes       []itsreq.Issue      `json:"notes"`
		Suggestions []itsreq.Suggestion `json:"suggestions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не разбирается: %v", err)
	}
	if !body.Blocking {
		t.Errorf("КПП не как в 1С должен блокировать: %+v", body.Issues)
	}
	fields := map[string]string{}
	for _, s := range body.Suggestions {
		fields[s.Field] = s.Value
	}
	if fields["companyName"] != "ООО Альфа" || fields["ownerCode"] != "CL-1000001" {
		t.Errorf("подсказки: %+v", body.Suggestions)
	}
	if len(body.Notes) == 0 {
		t.Error("нет справки о продукте")
	}
}

// База абонентов 1С отдаёт не всех владельцев наших идентификаторов: у
// «Центра переработки» (CL-7000582, ИНН 4200000320) идентификатор в биллинге,
// а в /rest/public/subscriber абонента нет. Сверка не должна звать его чужим.
func TestValidateKnowsOurClientsMissingFromSubscriberBase(t *testing.T) {
	registry := registryClients{
		{EDOID: "2AE-4729181C", INN: "4200000320", KPP: "420001001", ClientName: `ООО "Центр переработки"`,
			OwnerCode: "CL-7000582", LastPeriod: "2026-08"},
		{EDOID: "2AE-9D04CF45", INN: "4200000320", KPP: "420001001", LastPeriod: "2026-08"},
		{EDOID: "2AE-ORPHAN", INN: "7700000055", KPP: "770001001", LastPeriod: "2026-08"},
	}
	h := NewRequests(nil, nil).WithDirectory(stubDirectory{}).WithClients(registry)

	for _, row := range []string{
		`"inn":"4200000320","kpp":"420001001","ownerCode":"CL-7000582"`,
		`"inn":"7700000055","kpp":"770001001"`,
	} {
		payload := `{"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
			{"tariffCode":"2083","regNumber":"800000001","companyName":"ООО","workplaces":1,"responsible":"Иван",
			 "phoneCode":"812","phone":"1","startDate":"01.10.26","deliveryType":"0",` + row + `}]}`
		rec := httptest.NewRecorder()
		h.Validate(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/validate",
			strings.NewReader(payload)))
		var body struct {
			Issues []itsreq.Issue `json:"issues"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("ответ не разбирается: %v", err)
		}
		for _, issue := range body.Issues {
			if issue.Field == "ownerCode" || issue.Field == "inn" {
				t.Errorf("строка %s: ложное замечание %+v", row, issue)
			}
		}
	}
}

// exportStore записывает отметки выгрузки и отправки; остальное хранилищу в тесте не нужно.
type exportStore struct {
	RequestStore
	exported []int64
	sent     []int64
}

func (s *exportStore) MarkExported(_ context.Context, id int64, _ time.Time) error {
	s.exported = append(s.exported, id)
	return nil
}

func (s *exportStore) MarkSent(_ context.Context, id int64, _ time.Time) error {
	s.sent = append(s.sent, id)
	return nil
}

// fileBuilder кладёт в outDir пустой файл вместо настоящей заявки.
type fileBuilder struct{}

func (fileBuilder) Build(_ context.Context, request itsreq.Request, outDir string) (string, error) {
	path := filepath.Join(outDir, itsreq.FileName(request.PartnerCode))
	return path, os.WriteFile(path, []byte("xls"), 0o600)
}

func TestDownloadMarksSavedRequestExported(t *testing.T) {
	const good = `"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
		{"tariffCode":"2083","regNumber":"1","companyName":"ООО","inn":"7811000310",
		 "kpp":"780001001","workplaces":1,"responsible":"Иван","phoneCode":"812",
		 "phone":"1","startDate":"01.10.26","deliveryType":"0","ownerCode":"CL-1"}]}`

	for _, tc := range []struct {
		name string
		body string
		want []int64
	}{
		{"сохранённая", `{"id":7,` + good, []int64{7}},
		{"несохранённая", `{` + good, nil},
	} {
		st := &exportStore{}
		h := NewRequests(st, fileBuilder{})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/download", strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		h.Download(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: код %d: %s", tc.name, rec.Code, rec.Body.String())
		}
		if !slices.Equal(st.exported, tc.want) {
			t.Errorf("%s: отметки выгрузки %v, ожидали %v", tc.name, st.exported, tc.want)
		}
		// Скачивание письмо не шлёт: заявка не должна стать отправленной.
		if len(st.sent) != 0 {
			t.Errorf("%s: скачивание отметило отправку %v", tc.name, st.sent)
		}
	}
}

func TestValidateEndpointAcceptsGoodRequest(t *testing.T) {
	h := NewRequests(nil, nil)

	payload := `{"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
		{"tariffCode":"2083","regNumber":"1","companyName":"ООО","inn":"7811000310",
		 "kpp":"780001001","workplaces":1,"responsible":"Иван","phoneCode":"812",
		 "phone":"1","startDate":"01.10.26","deliveryType":"0","ownerCode":"CL-1"}]}`

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/validate", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)

	var body struct {
		Issues   []any `json:"issues"`
		Blocking bool  `json:"blocking"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не разбирается: %v", err)
	}
	if body.Blocking {
		t.Errorf("корректная заявка не должна блокироваться: %s", rec.Body.String())
	}
}

// listStore отдаёт заранее заданный список черновиков.
type listStore struct {
	RequestStore
	drafts []store.RequestDraft
}

func (s listStore) List(context.Context) ([]store.RequestDraft, error) { return s.drafts, nil }

func TestListNamesClientOfFirstRow(t *testing.T) {
	h := NewRequests(listStore{drafts: []store.RequestDraft{
		{ID: 2, Title: "Продление", Status: "draft", Revision: 3, UpdatedAt: time.Now(),
			PayloadJSON: `{"rows":[{"companyName":"ООО Ромашка","inn":"7811000310","kpp":"780001001"},{"inn":"1"}]}`},
		{ID: 1, Title: "Битая", Status: "draft", UpdatedAt: time.Now(), PayloadJSON: "{"},
	}}, nil)

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/requests", nil))
	var body struct {
		Requests []struct {
			ID          int64  `json:"id"`
			CompanyName string `json:"companyName"`
			INN         string `json:"inn"`
			KPP         string `json:"kpp"`
			Rows        int    `json:"rows"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Requests) != 2 {
		t.Fatalf("ответ %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	first, broken := body.Requests[0], body.Requests[1]
	if first.CompanyName != "ООО Ромашка" || first.INN != "7811000310" || first.KPP != "780001001" || first.Rows != 2 {
		t.Errorf("клиент заявки: %+v", first)
	}
	if broken.ID != 1 || broken.CompanyName != "" || broken.Rows != 0 {
		t.Errorf("повреждённый черновик: %+v", broken)
	}
}
