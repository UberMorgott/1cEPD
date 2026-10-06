/** Ошибка, пришедшая от нашего бэкенда в едином формате. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string }
}

/**
 * request выполняет запрос к нашему API.
 *
 * credentials: 'same-origin' обязателен: сессия живёт в cookie, без этого
 * браузер её не пошлёт и каждый запрос будет получать 401.
 */
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  // Файл (FormData) уходит multipart: границу в Content-Type ставит браузер.
  const json = !(init.body instanceof FormData)
  const response = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers: {
      ...(json ? { 'Content-Type': 'application/json' } : {}),
      ...(init.headers ?? {}),
    },
  })

  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()
  const payload = text ? (JSON.parse(text) as unknown) : null

  if (!response.ok) {
    // Сессия протухла: помечаем это в сторе, чтобы охранник маршрутов увёл на
    // форму входа. Ответ самой формы разбирается на месте и сюда не попадает.
    if (response.status === 401 && path !== '/api/login') {
      const { useSessionStore } = await import('../stores/session')
      useSessionStore().expire()
    }
    const body = payload as ErrorBody | null
    throw new ApiError(
      response.status,
      body?.error?.code ?? 'unknown',
      body?.error?.message ?? `Запрос завершился ошибкой ${response.status}`,
    )
  }

  return payload as T
}

export const api = {
  login: (login: string, password: string) =>
    request<{ ok: boolean }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ login, password }),
    }),

  logout: () => request<{ ok: boolean }>('/api/logout', { method: 'POST' }),

  anomalies: (includeAcknowledged = false) =>
    request<{ anomalies: Anomaly[] }>(
      `/api/anomalies${includeAcknowledged ? '?all=1' : ''}`,
    ),

  acknowledge: (edoId: string, fingerprint: string, reason: string) =>
    request<{ ok: boolean }>('/api/anomalies/ack', {
      method: 'POST',
      body: JSON.stringify({ edoId, fingerprint, reason }),
    }),

  /** Снять пометку «это законно»: находка вернётся в активные. */
  unacknowledge: (edoId: string, fingerprint: string) =>
    request<{ ok: boolean }>('/api/anomalies/unack', {
      method: 'POST',
      body: JSON.stringify({ edoId, fingerprint }),
    }),

  /** Реестр ожидаемой топологии: законные связи «организация → идентификатор». */
  putTopology: (entry: Omit<TopologyEntry, 'author' | 'updatedAt'>) =>
    request<{ ok: boolean }>('/api/topology', { method: 'POST', body: JSON.stringify(entry) }),

  removeTopology: (entry: { inn: string; kpp: string; edoId: string }) =>
    request<{ ok: boolean }>('/api/topology/remove', {
      method: 'POST',
      body: JSON.stringify(entry),
    }),

  billingHistory: () => request<BillingHistory>('/api/billing/history'),

  itsTariffs: () => request<{ tariffs: ItsTariff[] }>('/api/its/tariffs'),

  itsValidate: (payload: ItsRequest) =>
    request<{
      issues: ItsIssue[]
      blocking: boolean
      notes?: ItsIssue[]
      suggestions?: ItsSuggestion[]
    }>('/api/its/validate', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  itsRequests: () => request<{ requests: ItsSavedRequest[] }>('/api/its/requests'),

  /** Отправка письмом: тело то же, что у /api/its/download. */
  itsSend: (payload: ItsRequest) =>
    request<{ ok: boolean; sentAt: string }>('/api/its/send', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  settings: () => request<Settings>('/api/settings'),

  /** smtpPassword: ключа нет — оставить, строка — заменить, null — удалить. */
  saveSettings: (payload: SettingsInput) =>
    request<Settings>('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),

  testSmtp: () => request<{ ok: boolean }>('/api/settings/smtp/test', { method: 'POST' }),

  /** Версия сборки без сессии: по ней страница ждёт новую версию после обновления. */
  health: () => request<{ status: string; version?: string }>('/api/health'),

  updateStatus: () => request<UpdateStatus>('/api/update'),

  /** Проверить GitHub сейчас. */
  checkUpdate: () => request<UpdateStatus>('/api/update/check', { method: 'POST' }),

  /** Поставить новую версию; ответ приходит, когда она на месте и программа перезапускается. */
  applyUpdate: () => request<UpdateStatus>('/api/update/apply', { method: 'POST' }),

  saveUpdateSettings: (prefs: { autoCheck: boolean; autoInstall: boolean }) =>
    request<UpdateStatus>('/api/update/settings', { method: 'PUT', body: JSON.stringify(prefs) }),

  itsSender: () => request<ItsSender>('/api/settings/its-sender'),

  saveItsSender: (payload: ItsSender) =>
    request<ItsSender>('/api/settings/its-sender', {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),

  itsRequest: (id: number) => request<ItsSavedDraft>(`/api/its/requests/${id}`),

  /** Карточки клиентов для автозаполнения заявки: реестр ЭДО и прошлые заявки. */
  itsClients: () => request<{ clients: ItsClientCard[] }>('/api/its/clients'),

  /** Проверка условий сопровождения в 1С по логину (или регномеру); ответ запоминается в карточке. */
  itsCheckPrograms: (client: { inn: string; kpp: string; login: string; regNumber: string }) =>
    request<{ programs: ItsClientProgram[]; checkedAt: string }>('/api/its/clients/programs', {
      method: 'POST',
      body: JSON.stringify(client),
    }),

  /** Договоры 1С:ИТС клиентов и кончающиеся в ближайшие days суток. */
  itsContracts: (days: number) =>
    request<ItsContractsReport>(`/api/its/contracts?days=${days}`),

  /** Проверка договоров в 1С сейчас; свежую (моложе 10 минут) сервер не повторяет. */
  refreshItsContracts: () =>
    request<{ ok: boolean; checked: boolean }>('/api/its/contracts/refresh', { method: 'POST' }),

  /** Подсказки тарифа ЭПД по расходу за 12 закрытых месяцев. */
  epdAdvice: () => request<EpdAdviceReport>('/api/its/epd-advice'),

  /** Выгрузка базы из 1С сейчас; свежую (моложе 10 минут) сервер не повторяет. */
  refreshSubscribers: () =>
    request<{ ok: boolean; fetched: boolean }>('/api/subscribers/refresh', { method: 'POST' }),

  /** Реестр клиентов: одна строка — одна организация (ИНН+КПП). */
  clients: () => request<ClientList>('/api/clients'),

  /** Загрузка выгрузки «Детализация биллинга» ЭПД (CSV с портала): заменяет прежнюю. */
  importEpdBilling: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<{ added: number; updated: number; epdImport: EpdImportInfo }>('/api/clients/epd-import', {
      method: 'POST',
      body: form,
    })
  },

  /** Карточка клиента по ключу (domain.ts clientKey): только сохранённые данные, без 1С. */
  client: (key: string) => request<ClientCard>(`/api/clients/${encodeURIComponent(key)}`),

  /** Сводка «что сделать» по всем клиентам. */
  dashboard: (days?: number) => request<Dashboard>(`/api/dashboard${days ? `?days=${days}` : ''}`),

  /** Сохранение с оптимистичной блокировкой: revision возвращаем ту, что дали нам. */
  itsSaveRequest: (draft: { id: number; title: string; revision: number; request: ItsRequest }) =>
    request<{ id: number; revision: number; number: number }>('/api/its/requests', {
      method: 'POST',
      body: JSON.stringify(draft),
    }),
}

export interface ItsTariff {
  code: string
  name: string
  volume: number
  retail: string
  partner: string
  freshCode: string
}

export interface ItsIssue {
  row: number
  field: string
  message: string
  blocking: boolean
  /** Проверку выполнить не удалось: замечание не блокирующее, а предупреждающее. */
  unchecked?: boolean
}

/** Значение из 1С для пустого поля строки; у extraRegNumbers — регномера через запятую. */
export interface ItsSuggestion {
  row: number
  field: string
  value: string
}

export interface ItsRow {
  tariffCode: string
  regNumber: string
  companyName: string
  inn: string
  kpp: string
  workplaces: number
  activityType: string
  director: string
  responsible: string
  postalCode: string
  city: string
  street: string
  house: string
  building: string
  flat: string
  phoneCode: string
  phone: string
  fax: string
  email: string
  startDate: string
  deliveryType: string
  distributorCode: string
  ownerCode: string
  login: string
  /** Операция: «0» — новый договор или продление, «1» — отказ. */
  operation: string
  /** Дата отказа, ММ.ГГ. */
  refusalDate: string
  /** Причина отказа, код 1–5. */
  refusalReason: string
  /** Регистрационные номера 1–4: другие программы по тому же договору. */
  extraRegNumbers: string[]
}

export interface ItsRequest {
  /** Номер сохранённой заявки: по нему сервер отмечает выгрузку и отправку. 0 — не сохранена. */
  id?: number
  partnerCode: string
  responsible: string
  email: string
  /** Пароль подтверждения подлинности заявки, не пароль Портала. */
  password: string
  /** Новый пароль: назначение впервые либо смена. */
  newPassword: string
  rows: ItsRow[]
}

/** Строка истории заявок: сервер отдаёт её уже отсортированной. */
export interface ItsSavedRequest {
  id: number
  /** Порядковый номер: «Заявка N от …». */
  number: number
  /** Дата создания, RFC 3339. */
  createdAt: string
  title: string
  status: string
  revision: number
  updatedAt: string
  exportedAt?: string
  sentAt?: string
  /** Клиент первой строки заявки и число строк. */
  companyName?: string
  inn?: string
  kpp?: string
  rows?: number
}

/** Настройки почты. Пароль сервер не отдаёт — только признак, что он задан. */
export interface Settings {
  smtpHost: string
  smtpPort: number
  smtpLogin: string
  smtpPasswordSet: boolean
  smtpFrom: string
  mailTo: string[]
}

/**
 * Отправитель заявки ИТС из настроек: шапка формы заявки. Пароль заявки
 * сервер отдаёт — форма подставляет его сама.
 */
export interface ItsSender {
  partnerCode: string
  responsible: string
  email: string
  password: string
}

/** Самообновление с GitHub: версия, найденная новая и ход установки. */
export interface UpdateStatus {
  current: string
  latest?: string
  available: boolean
  busy: boolean
  restarting?: boolean
  /** Сборка с версией: «dev» не обновляется. */
  enabled: boolean
  text?: string
  failed?: boolean
  installing?: boolean
  downloaded?: number
  size?: number
  autoCheck: boolean
  autoInstall: boolean
}

export type SettingsInput =Omit<Settings, 'smtpPasswordSet'> & {
  smtpPassword?: string | null
}

/**
 * Что известно о клиенте. Пустое поле row — «неизвестно», форма его не трогает.
 * responsible и email — шапка последней заявки на этого клиента.
 */
export interface ItsClientCard {
  row: Partial<ItsRow> & { inn: string; kpp: string; companyName: string }
  responsible?: string
  email?: string
  source: string
  /** «Тарифы ИТС» из реестра ЭДО или выгрузки биллинга ЭПД. */
  itsTariffs?: string
  /** Идентификаторы участника ЭДО организации по реестру и отчётам трафика. */
  edoIds?: string[]
  /** Программы Личного кабинета по последней проверке в 1С. */
  programs?: ItsClientProgram[]
  programsLogin?: string
  programsCheckedAt?: string
}

/** Строка «Проверки условий сопровождения»: регномер, программа, выполнены ли условия. */
export interface ItsClientProgram {
  regNumber: string
  program: string
  hasAccess: boolean
  /** Недостающие условия сопровождения; пусто, если выполнены. */
  missing?: string[]
}

export interface ItsSavedDraft {
  id: number
  number: number
  title: string
  status: string
  revision: number
  request: ItsRequest
}

export interface Anomaly {
  id: number
  edoId: string
  kind:
    | 'orphan_with_traffic'
    | 'owner_lost'
    | 'orphan_idle'
    | 'disappeared'
    | 'replaced'
  confidence: 'high' | 'low'
  fingerprint: string
  inn: string
  kpp: string
  clientName: string
  login: string
  details: string
  detectedAt: string
  acknowledged: boolean
  ackReason?: string
  ackAuthor?: string
  ackedAt?: string
  /** Когда пометку пора пересмотреть (раз в полгода); reviewDue — уже пора. */
  reviewAt?: string
  reviewDue?: boolean
  /** Погашена реестром ожидаемой топологии; topologyPurpose — назначение связи. */
  suppressed?: boolean
  topologyPurpose?: string
}

export interface TopologyEntry {
  inn: string
  kpp: string
  edoId: string
  purpose: string
  author?: string
  updatedAt?: string
}

export interface Identifier {
  edoId: string
  clientName: string
  inn: string
  kpp: string
  login: string
  owner: string
  ownerCode: string
  tariffs: string
  limit: number | null
  packets: number
  firstSeen: string
  lastSeen: string
  period: string
  /** Из отчёта трафика: оператор, даты, документооборот; null — трафика не было. */
  traffic: IdentifierTraffic | null
}

/** Даты — ГГГГ-ММ-ДД, пусто — 1С не заполнила. Счётчики — документы за periodFrom…periodTo. */
export interface IdentifierTraffic {
  periodFrom: string
  periodTo: string
  operator: string
  linkCreated: string
  idRegistered: string
  supportFrom: string
  supportTo: string
  invoicesIn: number
  nonInvoicesIn: number
  epdIn: number
  invoicesOut: number
  nonInvoicesOut: number
  epdOut: number
}

export interface BillingClient {
  edoId: string
  clientName: string
  inn: string
  kpp: string
  login: string
  owner: string
  tariffs: string
  limit: number | null
  used: number
  remaining: number | null
  billable: number
  amount: string
  /** Доход партнёра: отдельная колонка отчёта, не равна сумме клиенту. */
  partnerAmount: string
  overLimit: boolean
  lowRemainder: boolean
}

/** Итоги месяца в истории биллинга. status: pending — ещё догружается. */
export interface BillingHistoryMonth {
  period: string
  status: 'ok' | 'pending' | 'none' | 'failed'
  clients: number
  packets: number
  billable: number
  totalDue: string
  partnerTotal: string
}

/** Помесячный расход идентификатора; позиции совпадают с months, null — строки нет. */
export interface BillingHistoryClient {
  edoId: string
  packets: (number | null)[]
  limits: (number | null)[]
  billable: number[]
}

export interface BillingHistory {
  months: BillingHistoryMonth[]
  clients: BillingHistoryClient[]
}

/** Прогноз расхода лимита к концу текущего месяца по трафику с первого числа. */
export interface ForecastClient {
  edoId: string
  clientName: string
  inn: string
  kpp: string
  login: string
  limit: number
  /** Исходящие СФ с начала месяца — оценка пакетов биллинга. */
  used: number
  projected: number
  overBy: number
  exceeded: boolean
  /** Прошло достаточно дней, чтобы экстраполировать. */
  reliable: boolean
  warn: boolean
}

export interface BillingForecast {
  period?: string
  asOf?: string
  daysElapsed?: number
  daysInMonth?: number
  clients: ForecastClient[]
  empty?: boolean
}

/** Опция тарифа сервиса. У качественной (quantitative false) объёмов нет. */
export interface LicenseOption {
  /** Вид отчёта 1С: REPORTING, SIGN, CLOUD_BACKUP… */
  type: string
  name: string
  quantitative: boolean
  max: number | null
  used: number | null
  remaining: number | null
}

/** Тариф сервиса из отчёта по опциям. Даты — ISO в UTC. */
export interface LicenseTariff {
  name: string
  typeNumber: number | null
  orgName: string
  orgInn: string
  orgKpp: string
  start: string
  end: string
  /** Виды отчёта, в которых пришёл тариф. */
  services: string[]
  options: LicenseOption[]
}

export interface LicenseExpiring {
  subscriberCode: string
  tariff: LicenseTariff
  /** Полных суток до конца; отрицательное — уже истёк. */
  daysLeft: number
  clients: ItsContractClient[]
}

export interface LicenseLow {
  subscriberCode: string
  tariffName: string
  orgName: string
  end: string
  option: LicenseOption
  /** Израсходовано больше объёма. */
  over: boolean
  clients: ItsContractClient[]
}

/** Договор 1С:ИТС из проверки по коду абонента. Даты — ISO в UTC. */
export interface ItsContract {
  name: string
  nameForUser: string
  typeNumber: number | null
  description: string
  start: string
  end: string
}

/** Организация из реестра ЭДО, принадлежащая абоненту. */
export interface ItsContractClient {
  inn: string
  kpp: string
  clientName: string
  tariffs: string
}

export interface ItsSubscriber {
  code: string
  /** Числовой код статуса: 1 — договор оформлен, 109 — проверка не предусмотрена (Фреш). */
  statusCode: number
  status: string
  description: string
  contracts: ItsContract[]
  clients: ItsContractClient[]
  checkedAt: string
  /** Проверка ИТС Отраслевого; нет — ещё не проверялся. */
  industry?: ItsIndustry
}

/** ИТС Отраслевой: statusCode 107 — есть конфигурации, которым он нужен. */
export interface ItsIndustry {
  statusCode: number
  description: string
  programs: { name: string; subscriptions: { name: string; begin: string; end: string }[] }[]
  /** Конфигурации, которым нужен ИТС Отраслевой, а он не оформлен. */
  missing: string[]
}

export interface ItsExpiring {
  subscriberCode: string
  contract: ItsContract
  /** Полных суток до конца; отрицательное — уже истёк. */
  daysLeft: number
  clients: ItsContractClient[]
  /** Предзаполнение заявки на продление: дата начала ДД.ММ.ГГ и код тарифа ЭПД, если известен. */
  renewal: { startDate: string; tariffCode: string }
}

export interface ItsContractsReport {
  days: number
  checkedAt?: string
  subscribers: ItsSubscriber[]
  expiring: ItsExpiring[]
  /** Абоненты, которым нужен ИТС Отраслевой, а он не оформлен. */
  industryMissing: { subscriberCode: string; programs: string[]; clients: ItsContractClient[] }[]
}
/**
 * Организация из биллинга, которой нет в базе абонентов ни по коду, ни по ИНН.
 * Её идентификаторы ЭДО (ИНН+КПП) собраны в одну запись.
 */
export interface BillingNotInBase {
  clientName: string
  inn: string
  kpp: string
  logins: string[]
  /** Из биллинга, а если там пусто — из отчёта трафика по ИНН. */
  ownerCodes: string[]
  edoIds: string[]
}

export interface EpdTariffRef {
  code: string
  name: string
  volume: number
  freshCode: string
  /** Число подписок на тариф у клиента (только в current). */
  count?: number
}

/** Подсказка тарифа ЭПД для организации. Суммы — розница за год, «1400,00». */
export interface EpdAdvice {
  inn: string
  kpp: string
  clientName: string
  subscriber: string
  /** Владелец в облаке Фреш: тариф оформляется в Менеджере сервиса, код freshCode. */
  fresh: boolean
  /** Исходящие ЭПД за 12 месяцев: как в биллинге, платит отправитель. */
  epdOut: number
  epdIn: number
  current: EpdTariffRef[]
  currentCost: string
  perPieceCost: string
  /** null — выгоднее поштучно. */
  best: EpdTariffRef | null
  bestCost: string
  savings: string
  /** От 200 000 в год — индивидуальный тариф по запросу на epd@1c.ru. */
  individual: boolean
  /** Текущий способ оплаты уже самый дешёвый. */
  optimal: boolean
}

export interface EpdAdviceReport {
  periodFrom?: string
  periodTo?: string
  fetchedAt?: string
  clients: EpdAdvice[]
  empty?: boolean
}
/** Клиент в ссылке: ключ карточки и реквизиты. */
export interface ClientRef {
  key: string
  clientName: string
  inn: string
  kpp: string
}

/** Строка реестра клиентов; итоги биллинга — за последний закрытый месяц. */
export interface ClientListItem extends ClientRef {
  subscriberCodes: string[]
  /** Пусто — абонента в базе нет. */
  subscriberName: string
  logins: string[]
  edoIds: string[]
  /** registry — реестр ЭДО, traffic — отчёты трафика ЭДО, import — выгрузка биллинга ЭПД, subscriber — база абонентов, request — заявки. */
  sources: string[]
  inBase: boolean
  /** Наш клиент 1С-ЭДО: его идентификатор есть в последнем биллинге или в отчёте трафика, сопровождение не кончилось. */
  ours: boolean
  inBilling: boolean
  limit: number | null
  used: number
  billable: number
  amount: string
  overLimit: boolean
  lowRemainder: boolean
  itsEnd?: string
  anomalies: number
  requests: number
}

export interface ClientList {
  period: string
  takenAt?: string
  subscribersFetchedAt?: string
  /** Последняя загруженная выгрузка биллинга ЭПД; нет — не загружали. */
  epdImport?: EpdImportInfo
  clients: ClientListItem[]
}

/** Загруженная руками выгрузка «Детализация биллинга» ЭПД. */
export interface EpdImportInfo {
  importedAt: string
  fileName: string
  rows: number
}

/** Абонент клиента; inBase false — код известен только из биллинга. */
export interface ClientSubscriber {
  code: string
  name: string
  subjects: string[]
  regNumbers: string[]
  inBase: boolean
  gone: boolean
  /** Все организации абонента, включая эту. */
  organizations: ClientRef[]
}

/** Всё о клиенте из сохранённых данных. */
export interface ClientCard extends ClientRef {
  sources: string[]
  inBase: boolean
  /** Наш клиент 1С-ЭДО: абонент-владелец известен по нашему ЭДО, даже без базы абонентов 1С. */
  ours: boolean
  logins: string[]
  subscribers: ClientSubscriber[]
  /** Другие КПП того же ИНН. */
  sameInn: ClientRef[]
  identifiers: Identifier[]
  billing: { period: string; takenAt?: string; clients: BillingClient[] }
  history: BillingHistory
  forecast: BillingForecast | null
  advice: EpdAdvice | null
  its: ItsSubscriber[]
  itsExpiring: ItsExpiring[]
  /** Предзаполнение продления по самому позднему договору 1С:ИТС. */
  renewal: { startDate: string; tariffCode: string } | null
  licenses: LicenseTariff[]
  licensesLow: LicenseLow[]
  programs: ItsClientProgram[]
  programsLogin?: string
  programsCheckedAt?: string
  anomalies: Anomaly[]
  requests: ItsSavedRequest[]
}

/** Пункт сводки и клиенты, к которым он относится. */
export interface DashboardEntry<T> {
  clients: ClientRef[]
  item: T
}

export interface Dashboard {
  counts: {
    invoice: number
    lowRemainder: number
    forecast: number
    advice: number
    renewals: number
    industry: number
    licenses: number
    anomalies: number
    reviewDue: number
    edoWithoutBilling: number
    notInBase: number
    drafts: number
    exported: number
  }
  billing: {
    period: string
    takenAt?: string
    totalDue: string
    overLimit: DashboardEntry<BillingClient>[]
    lowRemainder: DashboardEntry<BillingClient>[]
  }
  forecast: { period?: string; asOf?: string; items: DashboardEntry<ForecastClient>[] }
  advice: DashboardEntry<EpdAdvice>[]
  renewals: { days: number; checkedAt?: string; items: DashboardEntry<ItsExpiring>[] }
  industry: DashboardEntry<{ subscriberCode: string; programs: string[]; clients: ItsContractClient[] }>[]
  licenses: { expiring: DashboardEntry<LicenseExpiring>[]; low: DashboardEntry<LicenseLow>[] }
  anomalies: DashboardEntry<Anomaly>[]
  /** Сверка базы абонентов с биллингом за period. */
  gaps: {
    period: string
    edoWithoutBilling: DashboardEntry<{ code: string; name: string; inTraffic: boolean; edoIds: string[] }>[]
    notInBase: DashboardEntry<BillingNotInBase>[]
  }
  /** Неотправленные заявки, свежие сверху. */
  requests: DashboardEntry<ItsSavedRequest>[]
  /** Когда обновлялся каждый источник; нет — ещё ни разу. */
  sources: {
    billing?: string
    subscribers?: string
    its?: string
    traffic?: string
    epdUsage?: string
    licenses?: string
  }
}
