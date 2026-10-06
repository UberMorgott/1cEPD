package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
)

// Сроки жизни ответов 1С для проверки заявки. Форма проверяет заявку на каждое
// изменение, а перед выпуском файла — ещё раз: без кэша одна заявка стоила бы
// десятков одинаковых запросов, а отчёт трафика — ещё и часового лимита задач.
const (
	// subscribersTTL — база абонентов меняется редко, а выгружается целиком.
	subscribersTTL = 6 * time.Hour
	// lookupTTL — точечные справки: регномер, логин, договоры, трафик по ИНН.
	lookupTTL = 30 * time.Minute
	// subscribersRetry — пауза перед новой выгрузкой базы после отказа 1С.
	subscribersRetry = time.Minute
)

// DirectorySource — чтение из партнёрского API, нужное проверке заявки;
// *partner.Client ему удовлетворяет.
type DirectorySource interface {
	Subscribers(ctx context.Context) ([]partner.Subscriber, error)
	NomenclatureByRegNumbers(ctx context.Context, regNumbers []string) ([]partner.Nomenclature, error)
	CheckITSByRegNumbers(ctx context.Context, regNumbers []string) ([]partner.ITSCheck, error)
	ProgramsByLogin(ctx context.Context, login string) ([]partner.ProgramAccess, error)
	UserExists(ctx context.Context, login, email string) (bool, error)
	RegNumberRegistered(ctx context.Context, regNumber string) (bool, error)
	EDOTrafficLogins(ctx context.Context, inn string, from, to time.Time) ([]string, error)
}

// RequestDirectory отдаёт проверке заявки данные 1С с кэшем в памяти.
// Удовлетворяет itsreq.Directory и itsreq.Portal. Ошибки не кэшируются.
type RequestDirectory struct {
	source DirectorySource
	now    func() time.Time

	// subscribersMu держится на всю выгрузку: параллельные проверки ждут одну.
	subscribersMu sync.Mutex
	subscribers   cached[[]partner.Subscriber]
	// subscribersErr — последний отказ выгрузки и его время.
	subscribersErr      error
	subscribersFailedAt time.Time

	mu        sync.Mutex
	products  map[string]cached[string]
	contracts map[string]cached[[]itsreq.Contract]
	logins    map[string]cached[[]string]
	users     map[string]cached[bool]
	regs      map[string]cached[bool]
	traffic   map[string]cached[[]string]
}

type cached[T any] struct {
	value T
	// known — для products: регномер есть в номенклатуре.
	known bool
	at    time.Time
}

// NewRequestDirectory создаёт кэширующий справочник поверх API.
func NewRequestDirectory(source DirectorySource) *RequestDirectory {
	return &RequestDirectory{
		source: source, now: time.Now,
		products: map[string]cached[string]{}, contracts: map[string]cached[[]itsreq.Contract]{},
		logins: map[string]cached[[]string]{}, users: map[string]cached[bool]{},
		regs: map[string]cached[bool]{}, traffic: map[string]cached[[]string]{},
	}
}

func (d *RequestDirectory) fresh(at time.Time, ttl time.Duration) bool {
	return !at.IsZero() && d.now().Sub(at) < ttl
}

// AllSubscribers — вся база абонентов партнёра как её отдаёт 1С. Этот же кэш
// читает суточная выгрузка базы в хранилище (SubscriberRefresher), так что
// проверка заявки и справочник абонентов не выгружают базу дважды.
func (d *RequestDirectory) AllSubscribers(ctx context.Context) ([]partner.Subscriber, error) {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()
	if d.fresh(d.subscribers.at, subscribersTTL) {
		return d.subscribers.value, nil
	}
	// Форма проверяет заявку на каждое изменение: при отказе 1С (например,
	// 401) не повторяем выгрузку чаще раза в subscribersRetry.
	if d.fresh(d.subscribersFailedAt, subscribersRetry) {
		return nil, d.subscribersErr
	}
	found, err := d.source.Subscribers(ctx)
	if err != nil {
		d.subscribersErr, d.subscribersFailedAt = err, d.now()
		return nil, err
	}
	d.subscribers = cached[[]partner.Subscriber]{value: found, at: d.now()}
	return found, nil
}

// Subscribers — вся база абонентов партнёра в полях проверки заявки.
func (d *RequestDirectory) Subscribers(ctx context.Context) ([]itsreq.Subscriber, error) {
	found, err := d.AllSubscribers(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]itsreq.Subscriber, 0, len(found))
	for _, s := range found {
		item := itsreq.Subscriber{Code: s.Code, Name: s.Name, RegNumbers: s.RegNumbers}
		for _, org := range s.Organizations {
			item.Organizations = append(item.Organizations, itsreq.Organization{Name: org.Name, INN: org.INN, KPP: org.KPP})
		}
		list = append(list, item)
	}
	return list, nil
}

// missing — ключи, которых нет в кэше или они устарели.
func missing[T any](d *RequestDirectory, cache map[string]cached[T], keys []string) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var list []string
	for _, key := range keys {
		if !d.fresh(cache[key].at, lookupTTL) {
			list = append(list, key)
		}
	}
	return list
}

// Products — наименование продукта по регномеру.
func (d *RequestDirectory) Products(ctx context.Context, regNumbers []string) (map[string]string, error) {
	if ask := missing(d, d.products, regNumbers); len(ask) > 0 {
		found, err := d.source.NomenclatureByRegNumbers(ctx, ask)
		if err != nil {
			return nil, err
		}
		at := d.now()
		d.mu.Lock()
		for _, key := range ask {
			d.products[key] = cached[string]{at: at}
		}
		for _, item := range found {
			d.products[item.RegNumber] = cached[string]{value: item.Name, known: true, at: at}
		}
		d.mu.Unlock()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	result := map[string]string{}
	for _, key := range regNumbers {
		if entry := d.products[key]; entry.known {
			result[key] = entry.value
		}
	}
	return result, nil
}

// Contracts — договоры 1С:ИТС и сервисов по регномеру.
func (d *RequestDirectory) Contracts(ctx context.Context, regNumbers []string) (map[string][]itsreq.Contract, error) {
	if ask := missing(d, d.contracts, regNumbers); len(ask) > 0 {
		checks, err := d.source.CheckITSByRegNumbers(ctx, ask)
		if err != nil {
			return nil, err
		}
		at := d.now()
		d.mu.Lock()
		for _, key := range ask {
			d.contracts[key] = cached[[]itsreq.Contract]{at: at}
		}
		for _, check := range checks {
			var list []itsreq.Contract
			for _, c := range check.Contracts {
				list = append(list, itsreq.Contract{Name: firstNonBlank(c.TypeNameForUser, c.TypeName),
					TypeNumber: c.TypeNumber, Start: c.Start, End: c.End})
			}
			d.contracts[check.RegNumber] = cached[[]itsreq.Contract]{value: list, at: at}
		}
		d.mu.Unlock()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	result := map[string][]itsreq.Contract{}
	for _, key := range regNumbers {
		result[key] = d.contracts[key].value
	}
	return result, nil
}

// lookupOne — кэш одного значения по ключу.
func lookupOne[T any](ctx context.Context, d *RequestDirectory, cache map[string]cached[T], key string,
	load func(context.Context) (T, error),
) (T, error) {
	d.mu.Lock()
	entry := cache[key]
	d.mu.Unlock()
	if d.fresh(entry.at, lookupTTL) {
		return entry.value, nil
	}
	value, err := load(ctx)
	if err != nil {
		return value, err
	}
	d.mu.Lock()
	cache[key] = cached[T]{value: value, at: d.now()}
	d.mu.Unlock()
	return value, nil
}

// LoginRegNumbers — регномера программ Личного кабинета с этим логином.
func (d *RequestDirectory) LoginRegNumbers(ctx context.Context, login string) ([]string, error) {
	return lookupOne(ctx, d, d.logins, strings.ToLower(strings.TrimSpace(login)),
		func(ctx context.Context) ([]string, error) {
			programs, err := d.source.ProgramsByLogin(ctx, login)
			if err != nil {
				if partner.IsLoginNotFound(err) {
					return nil, itsreq.ErrLoginNotFound
				}
				return nil, err
			}
			list := []string{}
			for _, p := range programs {
				if p.RegNumber != "" {
					list = append(list, p.RegNumber)
				}
			}
			return list, nil
		})
}

// UserExists — есть ли на Портале пользователь с этими логином и e-mail.
func (d *RequestDirectory) UserExists(ctx context.Context, login, email string) (bool, error) {
	key := strings.ToLower(strings.TrimSpace(login) + "\x00" + strings.TrimSpace(email))
	return lookupOne(ctx, d, d.users, key, func(ctx context.Context) (bool, error) {
		return d.source.UserExists(ctx, login, email)
	})
}

// RegNumberRegistered — itsreq.Portal с кэшем.
func (d *RequestDirectory) RegNumberRegistered(ctx context.Context, regNumber string) (bool, error) {
	return lookupOne(ctx, d, d.regs, strings.TrimSpace(regNumber), func(ctx context.Context) (bool, error) {
		return d.source.RegNumberRegistered(ctx, regNumber)
	})
}

// EDOTrafficLogins — itsreq.Portal с кэшем по ИНН: каждый отчёт трафика
// расходует часовой лимит задач 1С, а окно отчёта — год, так что полчаса
// разницы в периоде ответа не меняют.
func (d *RequestDirectory) EDOTrafficLogins(ctx context.Context, inn string, from, to time.Time) ([]string, error) {
	return lookupOne(ctx, d, d.traffic, strings.TrimSpace(inn), func(ctx context.Context) ([]string, error) {
		return d.source.EDOTrafficLogins(ctx, inn, from, to)
	})
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
