package itsreq

// Значения, которые правила ЭПД задают жёстко.
const (
	// IssuesCount — «Количество выпусков» для тарифов ЭПД всегда 12 месяцев.
	IssuesCount = "12"
	// PaymentPrepaid — «Способ оплаты»: только предоплата за весь срок.
	PaymentPrepaid = "1"
	// OperationNew — «Операция»: новый договор или продление.
	OperationNew = "0"
	// OperationRefusal — «Операция»: отказ от помесячного либо льготного договора.
	// Для тарифов ЭПД правилами не допускается (docs/REQUESTS.md §6).
	OperationRefusal = "1"
	// maxExtraRegNumbers — «Регистрационные номера 1–4», колонки 29–32.
	maxExtraRegNumbers = 4
)

// Row — строка таблицы регистрации.
type Row struct {
	TariffCode      string `json:"tariffCode"`
	RegNumber       string `json:"regNumber"`
	CompanyName     string `json:"companyName"`
	INN             string `json:"inn"`
	KPP             string `json:"kpp"`
	Workplaces      int    `json:"workplaces"`
	ActivityType    string `json:"activityType"`
	Director        string `json:"director"`
	Responsible     string `json:"responsible"`
	PostalCode      string `json:"postalCode"`
	City            string `json:"city"`
	Street          string `json:"street"`
	House           string `json:"house"`
	Building        string `json:"building"`
	Flat            string `json:"flat"`
	PhoneCode       string `json:"phoneCode"`
	Phone           string `json:"phone"`
	Fax             string `json:"fax"`
	Email           string `json:"email"`
	StartDate       string `json:"startDate"`
	DeliveryType    string `json:"deliveryType"`
	DistributorCode string `json:"distributorCode"`
	// Operation — «Операция»: OperationNew или OperationRefusal.
	// Пустое значение читается как OperationNew: старые черновики его не знают.
	Operation string `json:"operation"`
	// RefusalDate — «Дата отказа», формат ММ.ГГ. Только при отказе.
	RefusalDate string `json:"refusalDate"`
	// RefusalReason — «Причина отказа», код 1–5. Только при отказе.
	RefusalReason string `json:"refusalReason"`
	// ExtraRegNumbers — другие программы пользователя, покрываемые договором.
	// В файл попадают первые maxExtraRegNumbers непустых.
	ExtraRegNumbers []string `json:"extraRegNumbers"`
	// OwnerCode — код абонента-владельца идентификатора ЭДО.
	// В файл не попадает, но решает, можно ли вообще подавать заявку.
	OwnerCode string `json:"ownerCode"`
	// Login — логин Личного кабинета клиента на Портале ИТС. В файл не попадает:
	// нужен проверке, что с этим логином связан идентификатор ЭДО.
	Login string `json:"login"`
}

// Request — заявка целиком.
type Request struct {
	ID          int64  `json:"id"`
	PartnerCode string `json:"partnerCode"`
	Responsible string `json:"responsible"`
	Email       string `json:"email"`
	// Password — пароль подтверждения подлинности заявки: не пароль Портала и не
	// пароль API. Секрет, в лог не пишется.
	Password string `json:"password"`
	// NewPassword — назначение пароля впервые (Password пуст) либо его смена.
	NewPassword string `json:"newPassword"`
	Rows        []Row  `json:"rows"`
}

// operation возвращает код операции строки, подставляя «новый договор»
// вместо пустого значения.
func (r Row) operation() string {
	if r.Operation == "" {
		return OperationNew
	}
	return r.Operation
}

// Issue — замечание валидатора.
type Issue struct {
	// Row — номер строки таблицы с единицы; ноль означает шапку.
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
	// Blocking означает, что заявку выпускать нельзя.
	Blocking bool `json:"blocking"`
	// Unchecked означает, что проверку выполнить не удалось — обычно 1С не
	// ответила. Это не приговор заявке: такое замечание никогда не блокирует.
	Unchecked bool `json:"unchecked,omitempty"`
}
