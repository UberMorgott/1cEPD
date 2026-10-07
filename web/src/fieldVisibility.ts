/**
 * Когда форма заявки показывает замечание под полем. Сервер проверяет заявку
 * целиком и сразу, но новой заявке незачем встречать человека красным: пустое
 * поле, которого он ещё не касался, молчит до попытки скачать или отправить.
 */

export interface VisibilityState {
  /** Была попытка скачать или отправить, либо открыт сохранённый черновик: видно всё. */
  revealed: boolean
  /** Поля, которые человек уже покидал (blur). */
  touched: ReadonlySet<string>
  /** Поле под курсором: пока человек вводит, замечание не мелькает. */
  focused: string | null
  /** Значение поля по ключу: заполненное, но неверное (из клиента или черновика) видно сразу. */
  value: (key: string) => unknown
}

export function filled(value: unknown): boolean {
  if (value === undefined || value === null) return false
  if (typeof value === 'number') return value > 0
  return String(value).trim() !== ''
}

/**
 * Замечание у поля видно, если была попытка выпуска, поле уже покидали
 * или в нём есть значение, которое человек сейчас не вводит. key — ключ поля
 * как в data-field формы.
 */
export function issueVisible(key: string, state: VisibilityState): boolean {
  if (state.revealed || state.touched.has(key)) return true
  return state.focused !== key && filled(state.value(key))
}

/** Минимум полей строки заявки, нужный для подсчёта обязательных. */
export interface RequiredSource {
  partnerCode: string
  responsible: string
  email: string
  row: {
    companyName: string
    inn: string
    kpp: string
    regNumber: string
    responsible: string
    phone: string
    tariffCode: string
    workplaces: number | null
    startDate: string
    deliveryType: string
    distributorCode: string
  }
}

/**
 * Обязательные поля заявки — те же, что требует сервер (internal/itsreq/validate.go).
 * У ИП (ИНН из 12 цифр) КПП не нужен, код дистрибьютора — только при доставке через него.
 */
export function requiredFields(source: RequiredSource): { key: string; filled: boolean }[] {
  const { row } = source
  const fields: [string, unknown][] = [
    ['partnerCode', source.partnerCode],
    ['responsible:0', source.responsible],
    ['email:0', source.email],
    ['tariffCode', row.tariffCode],
    ['startDate', row.startDate],
    ['companyName', row.companyName],
    ['inn', row.inn],
    ['responsible:1', row.responsible],
    ['phone', row.phone],
    ['regNumber', row.regNumber],
    ['workplaces', row.workplaces],
  ]
  if (row.inn.trim().length !== 12) fields.push(['kpp', row.kpp])
  if (row.deliveryType === '1') fields.push(['distributorCode', row.distributorCode])
  return fields.map(([key, value]) => ({ key, filled: filled(value) }))
}

/**
 * Замечание обо всей заявке (без своего поля): до попытки выпуска — только когда
 * у строки уже есть, по чему сверяться с 1С, иначе пустая форма показывает сбои
 * проверок, которым ещё нечего проверять.
 */
export function orphanVisible(revealed: boolean, clientInputs: unknown[]): boolean {
  return revealed || clientInputs.some(filled)
}
