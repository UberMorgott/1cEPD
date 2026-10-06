package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"partnerops/internal/events"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

type fakeReports struct {
	raw  []byte
	err  error
	took int
}

func (f *fakeReports) EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error) {
	f.took++
	return f.raw, f.err
}

type fakeStore struct {
	period    string
	rows      []partner.EDOBillingRow
	saved     int
	duplicate bool
}

func (f *fakeStore) Save(
	ctx context.Context, period string, takenAt time.Time, raw []byte, rows []partner.EDOBillingRow,
) (store.SaveResult, error) {
	f.period = period
	f.rows = rows
	f.saved++
	return store.SaveResult{
		ID:         int64(f.saved),
		IsBaseline: f.saved == 1,
		Duplicate:  f.duplicate,
	}, nil
}

const sampleCSV = "\ufeffКод партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
	"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
	"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
	"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
	"Сумма для партнера;Дополнительная информация\r\n" +
	"00000;Партнёр;CL-1;login;2AE-A;Клиент;7700000001;770001001;тариф;100;2;0;2;2;2;0;0,00;0,00;0,00;\r\n"

const sampleCSVWithOrphan = "\ufeffКод партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
	"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
	"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
	"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
	"Сумма для партнера;Дополнительная информация\r\n" +
	"00000;Партнёр;;login;2AE-ORPHAN;Клиент;7700000001;770001001;;;0;0;56;;;;;;;\r\n"

type fakeRegistry struct {
	upserted []store.IdentifierRecord
	events   []store.AnomalyEvent
	owners   map[string]string
	known    []store.IdentifierRecord
}

func (f *fakeRegistry) Upsert(
	ctx context.Context, records []store.IdentifierRecord, period string, seenAt time.Time,
) error {
	f.upserted = append(f.upserted, records...)
	return nil
}

func (f *fakeRegistry) RecordEvent(ctx context.Context, event store.AnomalyEvent, at time.Time) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeRegistry) OwnersByID(ctx context.Context) (map[string]string, error) {
	return f.owners, nil
}

func (f *fakeRegistry) All(ctx context.Context) ([]store.IdentifierRecord, error) {
	return f.known, nil
}

// fetchLogStore — хранилище, которое помнит время последнего отчёта.
type fetchLogStore struct {
	fakeStore
	last time.Time
}

func (f *fetchLogStore) LastFetched(ctx context.Context, period string) (time.Time, error) {
	return f.last, nil
}

func TestTakeIfStaleReusesFreshReport(t *testing.T) {
	date := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)

	reports := &fakeReports{raw: []byte(sampleCSV)}
	fresh := &fetchLogStore{last: time.Now().Add(-time.Hour)}
	took, err := NewSnapshot(reports, fresh, events.NewBus()).TakeIfStale(t.Context(), date, SnapshotRefreshInterval)
	if err != nil || took || reports.took != 0 {
		t.Fatalf("свежий отчёт: took=%v, отчётов %d, err=%v; ожидали без отчёта", took, reports.took, err)
	}

	stale := &fetchLogStore{last: time.Now().Add(-SnapshotRefreshInterval - time.Minute)}
	took, err = NewSnapshot(reports, stale, events.NewBus()).TakeIfStale(t.Context(), date, SnapshotRefreshInterval)
	if err != nil || !took || reports.took != 1 || stale.saved != 1 {
		t.Fatalf("старый отчёт: took=%v, отчётов %d, сохранений %d, err=%v", took, reports.took, stale.saved, err)
	}

	never := &fetchLogStore{}
	if took, err := NewSnapshot(reports, never, events.NewBus()).TakeIfStale(t.Context(), date, SnapshotRefreshInterval); err != nil || !took {
		t.Fatalf("отчёта не было: took=%v, err=%v", took, err)
	}
}

func TestTakeSavesAndPublishes(t *testing.T) {
	reports := &fakeReports{raw: []byte(sampleCSV)}
	st := &fakeStore{}
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	svc := NewSnapshot(reports, st, bus)
	err := svc.Take(context.Background(), time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Take: %v", err)
	}

	if st.saved != 1 {
		t.Errorf("сохранений %d, ожидали 1", st.saved)
	}
	if st.period != "2026-08" {
		t.Errorf("период %q, ожидали 2026-08", st.period)
	}
	if len(st.rows) != 1 {
		t.Errorf("строк %d, ожидали 1", len(st.rows))
	}

	select {
	case event := <-ch:
		if event.Kind != "snapshot.completed" {
			t.Errorf("событие %q, ожидали snapshot.completed", event.Kind)
		}
	case <-time.After(time.Second):
		t.Error("событие не опубликовано")
	}
}

func TestTakeFillsRegistryAndRecordsAnomalies(t *testing.T) {
	reports := &fakeReports{raw: []byte(sampleCSVWithOrphan)}
	st := &fakeStore{}
	reg := &fakeRegistry{}
	bus := events.NewBus()

	svc := NewSnapshot(reports, st, bus)
	svc.WithRegistry(reg)

	if err := svc.Take(context.Background(), time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Take: %v", err)
	}

	if len(reg.upserted) != 1 {
		t.Errorf("в реестр записано %d идентификаторов, ожидали 1", len(reg.upserted))
	}
	if len(reg.events) != 1 {
		t.Fatalf("находок записано %d, ожидали 1", len(reg.events))
	}
	if reg.events[0].Kind != KindOrphanWithTraffic {
		t.Errorf("kind = %q, ожидали %q", reg.events[0].Kind, KindOrphanWithTraffic)
	}
}

func TestTakeDoesNotSaveOnReportError(t *testing.T) {
	reports := &fakeReports{err: errors.New("1С недоступна")}
	st := &fakeStore{}

	svc := NewSnapshot(reports, st, events.NewBus())
	if err := svc.Take(context.Background(), time.Now()); err == nil {
		t.Fatal("ожидали ошибку")
	}
	if st.saved != 0 {
		t.Error("при ошибке отчёта ничего сохранять нельзя")
	}
}

func TestTakeDoesNotSaveEmptyReport(t *testing.T) {
	// Отчёт с заголовком, но без строк, — признак сбоя на стороне 1С,
	// а не признак того, что клиентов не осталось. Затирать историю нельзя.
	header := sampleCSV[:len(sampleCSV)-len(
		"00000;Партнёр;CL-1;login;2AE-A;Клиент;7700000001;770001001;тариф;100;2;0;2;2;2;0;0,00;0,00;0,00;\r\n")]
	reports := &fakeReports{raw: []byte(header)}
	st := &fakeStore{}

	svc := NewSnapshot(reports, st, events.NewBus())
	if err := svc.Take(context.Background(), time.Now()); err == nil {
		t.Fatal("ожидали ошибку на пустом отчёте")
	}
	if st.saved != 0 {
		t.Error("пустой отчёт сохранять нельзя")
	}
}

func TestTakeStaysSilentOnDuplicate(t *testing.T) {
	// 1С отдаёт одни и те же данные при повторных запусках за день.
	// Событие в таком случае публиковать нельзя: интерфейс мигнёт без причины.
	reports := &fakeReports{raw: []byte(sampleCSV)}
	st := &fakeStore{duplicate: true}
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	svc := NewSnapshot(reports, st, bus)
	if err := svc.Take(context.Background(), time.Now()); err != nil {
		t.Fatalf("Take: %v", err)
	}

	select {
	case event := <-ch:
		t.Errorf("на дубликате опубликовано событие %q", event.Kind)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTakeMarksBaseline(t *testing.T) {
	// Первый снимок сравнивать не с чем, событие должно это сообщать,
	// иначе последующий разбор сочтёт весь реестр «появившимся».
	reports := &fakeReports{raw: []byte(sampleCSV)}
	st := &fakeStore{}
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	svc := NewSnapshot(reports, st, bus)
	if err := svc.Take(context.Background(), time.Now()); err != nil {
		t.Fatalf("Take: %v", err)
	}

	select {
	case event := <-ch:
		if event.Payload["baseline"] != true {
			t.Errorf("payload = %v, ожидали baseline=true", event.Payload)
		}
	case <-time.After(time.Second):
		t.Error("событие не опубликовано")
	}
}
