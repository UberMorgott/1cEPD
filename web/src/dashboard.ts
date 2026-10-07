/**
 * Поводы «что сделать» из /api/dashboard для Клиентов: счётчики, метка
 * статуса строки и её подсказка. Пункт относится к клиентам (ключи карточек);
 * пункт уровня абонента без организаций в базе в таблицу не попадает.
 */
import type { ClientRef, Dashboard } from './api/client'
import {
  anomalyKindLabels,
  contractName,
  epdBestText,
  forecastText,
  optionText,
  serviceName,
} from './domain'
import { daysText, money, monthLabel, moscowDate } from './format'

export type Topic =
  | 'invoice'
  | 'low'
  | 'forecast'
  | 'renewal'
  | 'licenses'
  | 'anomalies'
  | 'tariff'
  | 'industry'
  | 'gap'
  | 'notinbase'

export const topicKeys: Topic[] = [
  'invoice',
  'forecast',
  'low',
  'renewal',
  'licenses',
  'anomalies',
  'tariff',
  'industry',
  'gap',
  'notinbase',
]

export interface TopicItem {
  clients: ClientRef[]
  text: string
  /** Уже поздно: сверх лимита, договор истёк, срочная находка. */
  late: boolean
}

export interface TopicGroup {
  key: Topic
  label: string
  /** Короткая метка строки в Клиентах. */
  short: string
  /** Цвет повода: красный — деньги или авария, янтарный — скоро, остальное — подсказки. */
  tone: 'danger' | 'warn' | 'info' | 'success'
  items: TopicItem[]
}

function group(
  key: Topic,
  label: string,
  short: string,
  tone: TopicGroup['tone'],
  items: TopicItem[],
): TopicGroup {
  return { key, label, short, tone, items }
}

/** Все поводы сводки; порядок — как на экране. */
export function topicGroups(d: Dashboard): Record<Topic, TopicGroup> {
  return {
    invoice: group(
      'invoice',
      'Выставить счёт',
      'счёт',
      'danger',
      d.billing.overLimit.map(({ clients, item }) => ({
        clients,
        text: `сверх лимита ${item.billable}, счёт ${money(item.amount)}`,
        late: true,
      })),
    ),
    forecast: group(
      'forecast',
      d.forecast.period ? `Выйдут за лимит в ${monthLabel(d.forecast.period)}` : 'Выйдут за лимит',
      'прогноз',
      'danger',
      d.forecast.items.map(({ clients, item }) => ({
        clients,
        text: forecastText(item),
        late: item.exceeded,
      })),
    ),
    low: group(
      'low',
      'Скоро кончится лимит',
      'лимит',
      'warn',
      d.billing.lowRemainder.map(({ clients, item }) => ({
        clients,
        text: `израсходовано ${item.used} из ${item.limit ?? '—'}`,
        late: false,
      })),
    ),
    renewal: group(
      'renewal',
      'Продление 1С:ИТС',
      'продление',
      'warn',
      d.renewals.items.map(({ clients, item }) => ({
        clients,
        text: `${contractName(item.contract)} до ${moscowDate(item.contract.end)}, ${daysText(item.daysLeft)}`,
        late: item.daysLeft < 0,
      })),
    ),
    licenses: group('licenses', 'Лицензии сервисов', 'лицензии', 'warn', [
      ...d.licenses.expiring.map(({ clients, item }) => ({
        clients,
        text: `${item.tariff.name} до ${moscowDate(item.tariff.end)}, ${daysText(item.daysLeft)}`,
        late: item.daysLeft < 0,
      })),
      ...d.licenses.low.map(({ clients, item }) => ({
        clients,
        text: `${serviceName(item.option.type)}, ${item.tariffName}: ${optionText(item.option)}`,
        late: item.over,
      })),
    ]),
    anomalies: group(
      'anomalies',
      'Находки ЭДО',
      'находки',
      'danger',
      // Срочные — первыми: короткий список Сводки показывает их.
      [...d.anomalies]
        .sort((a, b) => Number(b.item.confidence === 'high') - Number(a.item.confidence === 'high'))
        .map(({ clients, item }) => ({
          clients,
          text: `${anomalyKindLabels[item.kind] ?? item.kind}: ${item.details}`,
          late: item.confidence === 'high',
        })),
    ),
    tariff: group(
      'tariff',
      'Выгоднее другой тариф ЭПД',
      'тариф',
      'success',
      d.advice.map(({ clients, item }) => ({
        clients,
        text: `${epdBestText(item)}: экономия ${money(item.savings)} в год`,
        late: false,
      })),
    ),
    industry: group(
      'industry',
      'Нужен ИТС Отраслевой',
      'отраслевой',
      'info',
      d.industry.map(({ clients, item }) => ({
        clients,
        text: `нужен для «${item.programs.join('», «')}»`,
        late: false,
      })),
    ),
    gap: group(
      'gap',
      'ЭДО без биллинга',
      'ЭДО без биллинга',
      'warn',
      d.gaps.edoWithoutBilling.map(({ clients, item }) => ({
        clients,
        text: `абонент ${item.code}: ${item.inTraffic ? 'трафик есть' : 'направление ЭДО в 1С'}, ` +
          `в биллинге${d.gaps.period ? ` за ${monthLabel(d.gaps.period)}` : ''} нет`,
        late: false,
      })),
    ),
    notinbase: group(
      'notinbase',
      'Нет в базе абонентов',
      'нет в базе',
      'info',
      d.gaps.notInBase.map(({ clients, item }) => ({
        clients,
        text: `в биллинге, владелец ${item.ownerCodes.join(', ') || 'не указан'}; в базе абонентов 1С не найден`,
        late: false,
      })),
    ),
  }
}

/**
 * Поводы от срочного к подсказкам: первый повод клиента — его главная метка в
 * таблице, порядок же сортирует таблицу «сначала те, кому нужнее».
 */
export const severityOrder: Topic[] = [
  'invoice',
  'forecast',
  'anomalies',
  'renewal',
  'low',
  'licenses',
  'gap',
  'notinbase',
  'industry',
  'tariff',
]

export interface ClientProblem {
  topic: Topic
  /** Что именно: текст пункта повода. */
  text: string
  late: boolean
}

/** Поводы каждого клиента, от срочного к подсказкам: ключ карточки → список. */
export function problemsByClient(groups: Record<Topic, TopicGroup>): Map<string, ClientProblem[]> {
  const map = new Map<string, ClientProblem[]>()
  for (const topic of severityOrder) {
    for (const item of groups[topic].items) {
      for (const client of item.clients) {
        const list = map.get(client.key) ?? []
        list.push({ topic, text: item.text, late: item.late })
        map.set(client.key, list)
      }
    }
  }
  return map
}

/** Окно напоминаний о продлении, дней: общее для Сводки и Клиентов, запоминается в браузере. */
const expiryKey = 'billing.expiryDays'

export function savedExpiryDays(): number {
  try {
    const value = Number(localStorage.getItem(expiryKey))
    return Number.isInteger(value) && value >= 1 && value <= 366 ? value : 30
  } catch {
    return 30
  }
}

export function saveExpiryDays(days: number) {
  try {
    localStorage.setItem(expiryKey, String(days))
  } catch {
    // Без хранилища окно просто не запомнится.
  }
}
