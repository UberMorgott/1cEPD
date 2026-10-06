package itsreq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"partnerops/internal/partner"
)

// trafficCSV — усечённый отчёт трафика ЭДО: проверке нужны только две колонки.
const trafficCSV = "\ufeffИдентификатор ЭДО;Логин\r\n" +
	"2AE@1CL@a1b2;client@example.ru\r\n"

// portalStub поднимает httptest вместо партнёрского API 1С и возвращает
// настоящего клиента, нацеленного на него. Реальная сеть не задействована.
func portalStub(t *testing.T, regNumberFound bool, traffic string) Portal {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/client-program-access/search/reg-number":
			if regNumberFound {
				_, _ = w.Write([]byte(`[{"regNumber":18117482,"hasAccess":true}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case "/edo/reports/client-traffic":
			_, _ = w.Write([]byte(`{"taskUeid":"task-1"}`))
		case "/edo/reports/client-traffic/task-1":
			// Отчёт отдаём сразу готовым, чтобы тест не ждал опроса.
			_, _ = w.Write([]byte(traffic))
		default:
			t.Errorf("неожиданный путь %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return partner.New(srv.URL, "login", "secret")
}

func requestWithLogin() Request {
	request := validRequest()
	request.Rows[0].Login = "client@example.ru"
	return request
}

func TestPreflightPasses(t *testing.T) {
	portal := portalStub(t, true, trafficCSV)

	issues := ValidateWithPortal(context.Background(), portal, requestWithLogin())
	if len(issues) != 0 {
		t.Errorf("ожидали отсутствие замечаний, получили: %+v", issues)
	}
}

func TestPreflightRejectsUnknownRegNumber(t *testing.T) {
	portal := portalStub(t, false, trafficCSV)

	issues := ValidateWithPortal(context.Background(), portal, requestWithLogin())
	issue := findIssue(t, issues, "regNumber")
	if !issue.Blocking || issue.Unchecked {
		t.Errorf("замечание о регномере = %+v, ожидали блокирующее и проверенное", issue)
	}
}

func TestPreflightRejectsLoginWithoutEDOLink(t *testing.T) {
	// В отчёте связь есть, но у другого логина — это приговор, а не отсутствие данных.
	portal := portalStub(t, true, trafficCSV)

	request := requestWithLogin()
	request.Rows[0].Login = "someone-else@example.ru"

	issues := ValidateWithPortal(context.Background(), portal, request)
	issue := findIssue(t, issues, "login")
	if !issue.Blocking || issue.Unchecked {
		t.Errorf("замечание о логине = %+v, ожидали блокирующее и проверенное", issue)
	}
}

// Пустой отчёт означает «трафика не было», а не «связи нет»: блокировать нельзя.
func TestPreflightDegradesOnEmptyTrafficReport(t *testing.T) {
	portal := portalStub(t, true, "\ufeffИдентификатор ЭДО;Логин\r\n")

	issues := ValidateWithPortal(context.Background(), portal, requestWithLogin())
	issue := findIssue(t, issues, "login")
	if issue.Blocking || !issue.Unchecked {
		t.Errorf("замечание о логине = %+v, ожидали непроверенное и не блокирующее", issue)
	}
}

// Отказ авторизации — беда инфраструктуры, а не заявки. Иначе при отвалившемся
// доступе к 1С встанет вся работа.
func TestPreflightDegradesOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":401,"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	portal := partner.New(srv.URL, "login", "wrong")

	issues := ValidateWithPortal(context.Background(), portal, requestWithLogin())
	if Blocking(issues) {
		t.Fatalf("401 не должен блокировать заявку, получили: %+v", issues)
	}
	if !Unchecked(issues) {
		t.Fatalf("ожидали отметку «не проверено», получили: %+v", issues)
	}
	for _, field := range []string{"regNumber", "login"} {
		if issue := findIssue(t, issues, field); !issue.Unchecked {
			t.Errorf("поле %s: %+v, ожидали непроверенное", field, issue)
		}
	}
}

// Молчащая 1С деградирует так же, как 401: заявка остаётся выпускаемой.
func TestPreflightDegradesOnTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer func() {
		close(release)
		srv.Close()
	}()

	portal := partner.New(srv.URL, "login", "secret")

	// Таймаут проверки измеряется десятками секунд, поэтому обрываем контекстом:
	// путь деградации от этого не меняется — обе проверки упираются в ctx.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	issues := ValidateWithPortal(ctx, portal, requestWithLogin())
	if Blocking(issues) {
		t.Fatalf("таймаут не должен блокировать заявку, получили: %+v", issues)
	}
	if !Unchecked(issues) {
		t.Fatalf("ожидали отметку «не проверено», получили: %+v", issues)
	}
}

// Без настроенного API проверки не выполняются, но заявка остаётся выпускаемой.
func TestPreflightWithoutPortal(t *testing.T) {
	issues := ValidateWithPortal(context.Background(), nil, requestWithLogin())
	if Blocking(issues) || !Unchecked(issues) {
		t.Errorf("без портала ожидали только отметку «не проверено», получили: %+v", issues)
	}
}

// Форматная проверка идёт первой: при плохом формате 1С не дёргаем.
func TestPreflightSkippedWhenFormatIsBad(t *testing.T) {
	portal := portalStub(t, true, trafficCSV)

	request := requestWithLogin()
	request.Rows[0].INN = "нет"

	issues := ValidateWithPortal(context.Background(), portal, request)
	for _, issue := range issues {
		if issue.Unchecked || issue.Field == "login" {
			t.Errorf("при плохом формате проверок в 1С быть не должно: %+v", issue)
		}
	}
}

func findIssue(t *testing.T, issues []Issue, field string) Issue {
	t.Helper()
	for _, issue := range issues {
		if issue.Field == field {
			return issue
		}
	}
	t.Fatalf("замечание по полю %s не найдено среди %+v", field, issues)
	return Issue{}
}

func TestParseEDOTrafficLoginsSkipsRowsWithoutIdentifier(t *testing.T) {
	raw := trafficCSV + ";orphan@example.ru\r\n2AE@1CL@a1b2;client@example.ru\r\n"

	logins, err := partner.ParseEDOTrafficLogins([]byte(raw))
	if err != nil {
		t.Fatalf("разбор отчёта: %v", err)
	}
	if strings.Join(logins, ",") != "client@example.ru" {
		t.Errorf("логины = %v, ожидали только client@example.ru", logins)
	}
}
