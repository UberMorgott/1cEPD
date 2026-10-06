/** Общие правила вывода текста: одинаковые на всех экранах. */

/** Форма слова для числа: 1 организация, 2 организации, 5 организаций. */
export function plural(n: number, one: string, few: string, many: string) {
  const tens = n % 100
  const units = n % 10
  if (tens >= 11 && tens <= 14) return many
  if (units === 1) return one
  if (units >= 2 && units <= 4) return few
  return many
}

const legalForms: [RegExp, string][] = [
  [/^общество с ограниченной ответственностью/i, 'ООО'],
  [/^публичное акционерное общество/i, 'ПАО'],
  [/^непубличное акционерное общество/i, 'АО'],
  [/^акционерное общество/i, 'АО'],
  [/^индивидуальный предприниматель/i, 'ИП'],
]

/** Название с сокращённой правовой формой: полное остаётся в подсказке. */
export function shortName(name: string) {
  const found = legalForms.find(([pattern]) => pattern.test(name))
  return found ? name.replace(found[0], found[1]) : name
}

/** Реквизиты одной строкой: «ИНН 7701… / 7701…», КПП — если есть. */
export function requisites(item: { inn?: string; kpp?: string }) {
  return `ИНН ${item.inn || '—'}${item.kpp ? ` / ${item.kpp}` : ''}`
}

const rubles = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})

/** Сумма бэкенда («12345,67») числом; нечисло — NaN. */
export function amount(text: string | undefined): number {
  return Number((text ?? '').replace(/\s/g, '').replace(',', '.'))
}

/**
 * money приводит сумму бэкенда («12345,67») к виду «12 345,67 ₽».
 * Знак рубля отделяем неразрывным пробелом, чтобы он не отрывался переносом.
 */
export function money(text: string | undefined): string {
  if (!text) return '—'
  const value = amount(text)
  return `${Number.isFinite(value) ? rubles.format(value) : text}\u00A0₽`
}

/** Дата без времени по часовому поясу браузера; пусто — прочерк. */
export function formatDate(value: string | undefined, empty = '—'): string {
  return value ? new Date(value).toLocaleDateString('ru-RU') : empty
}

/** Даты договоров 1С отдаёт в UTC, а кончаются они в московскую полночь. */
export function moscowDate(value: string | undefined): string {
  return value ? new Date(value).toLocaleDateString('ru-RU', { timeZone: 'Europe/Moscow' }) : '—'
}

/** Дата и время коротко: «12.09.2026, 10:15». */
export function when(value: string | undefined): string {
  return value
    ? new Date(value).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' })
    : '—'
}


/** Часы и минуты: отметка «Обновлено в 10:15». */
export function clockTime(value: Date | null | undefined): string {
  return value ? value.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : ''
}

/** Сколько осталось до даты: «через 5 дн.», «кончается сегодня», «истёк 3 дн. назад». */
export function daysText(days: number): string {
  if (days < 0) return `истёк ${-days} дн. назад`
  if (days === 0) return 'кончается сегодня'
  return `через ${days} дн.`
}

/** «2026-08» → «авг 26»: год подписываем, чтобы январь не путался с прошлым. */
export function monthLabel(period: string): string {
  const [year, month] = period.split('-').map(Number)
  const name = new Date(Date.UTC(year ?? 0, (month ?? 1) - 1, 1))
    .toLocaleDateString('ru-RU', { month: 'short', timeZone: 'UTC' })
    .replace('.', '')
  return `${name} ${String(year).slice(2)}`
}

/** Дата отчёта трафика «2017-02-15» → «15.02.2017». */
export function reportDate(value: string): string {
  return value ? value.split('-').reverse().join('.') : ''
}
