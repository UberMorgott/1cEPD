/**
 * Поводы «что сделать» из /api/dashboard в одном виде для Сводки и Клиентов:
 * один и тот же счёт видят оба экрана, поэтому и числа у них одинаковые.
 * Пункт относится к клиентам (ключи карточек); пункт уровня абонента без
 * организаций в базе — «без карточки», его считаем отдельно, чтобы не потерять.
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

/** Вкладка карточки клиента, где подробности повода. */
export type CardTab = 'edo' | 'its' | 'anomalies'

export interface TopicItem {
  id: string
  clients: ClientRef[]
  /** Кто это, если карточки нет: название организации или абонента. */
  name: string
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
  tab: CardTab
  items: TopicItem[]
  /** Клиенты, к которым относится повод. */
  keys: Set<string>
  /** Пункты без карточки клиента. */
  unmatched: TopicItem[]
  /** Клиентов плюс пунктов без карточки: то, что покажет фильтр Клиентов. */
  count: number
}

function group(
  key: Topic,
  label: string,
  short: string,
  tone: TopicGroup['tone'],
  tab: CardTab,
  items: TopicItem[],
): TopicGroup {
  const keys = new Set(items.flatMap((item) => item.clients.map((client) => client.key)))
  const unmatched = items.filter((item) => !item.clients.length)
  return { key, label, short, tone, tab, items, keys, unmatched, count: keys.size + unmatched.length }
}

/** Все поводы сводки; порядок — как на экране. */
export function topicGroups(d: Dashboard): Record<Topic, TopicGroup> {
  const subscriberName = (item: { subscriberCode: string; clients: { clientName: string }[] }, org = '') =>
    org || item.clients[0]?.clientName || item.subscriberCode
  return {
    invoice: group(
      'invoice',
      'Выставить счёт',
      'счёт',
      'danger',
      'edo',
      d.billing.overLimit.map(({ clients, item }, i) => ({
        id: `invoice/${i}`,
        clients,
        name: item.clientName,
        text: `сверх лимита ${item.billable}, счёт ${money(item.amount)}`,
        late: true,
      })),
    ),
    forecast: group(
      'forecast',
      d.forecast.period ? `Выйдут за лимит в ${monthLabel(d.forecast.period)}` : 'Выйдут за лимит',
      'прогноз',
      'danger',
      'edo',
      d.forecast.items.map(({ clients, item }, i) => ({
        id: `forecast/${i}`,
        clients,
        name: item.clientName,
        text: forecastText(item),
        late: item.exceeded,
      })),
    ),
    low: group(
      'low',
      'Скоро кончится лимит',
      'лимит',
      'warn',
      'edo',
      d.billing.lowRemainder.map(({ clients, item }, i) => ({
        id: `low/${i}`,
        clients,
        name: item.clientName,
        text: `израсходовано ${item.used} из ${item.limit ?? '—'}`,
        late: false,
      })),
    ),
    renewal: group(
      'renewal',
      'Продление 1С:ИТС',
      'продление',
      'warn',
      'its',
      d.renewals.items.map(({ clients, item }, i) => ({
        id: `renewal/${i}`,
        clients,
        name: subscriberName(item),
        text: `${contractName(item.contract)} до ${moscowDate(item.contract.end)}, ${daysText(item.daysLeft)}`,
        late: item.daysLeft < 0,
      })),
    ),
    licenses: group('licenses', 'Лицензии сервисов', 'лицензии', 'warn', 'its', [
      ...d.licenses.expiring.map(({ clients, item }, i) => ({
        id: `licenses/e/${i}`,
        clients,
        name: subscriberName(item, item.tariff.orgName),
        text: `${item.tariff.name} до ${moscowDate(item.tariff.end)}, ${daysText(item.daysLeft)}`,
        late: item.daysLeft < 0,
      })),
      ...d.licenses.low.map(({ clients, item }, i) => ({
        id: `licenses/l/${i}`,
        clients,
        name: subscriberName(item, item.orgName),
        text: `${serviceName(item.option.type)}, ${item.tariffName}: ${optionText(item.option)}`,
        late: item.over,
      })),
    ]),
    anomalies: group(
      'anomalies',
      'Находки ЭДО',
      'находки',
      'danger',
      'anomalies',
      // Срочные — первыми: короткий список Сводки показывает их.
      [...d.anomalies]
        .sort((a, b) => Number(b.item.confidence === 'high') - Number(a.item.confidence === 'high'))
        .map(({ clients, item }) => ({
          id: `anomalies/${item.id}`,
          clients,
          name: item.clientName || item.edoId,
          text: `${anomalyKindLabels[item.kind] ?? item.kind}: ${item.details}`,
          late: item.confidence === 'high',
        })),
    ),
    tariff: group(
      'tariff',
      'Выгоднее другой тариф ЭПД',
      'тариф',
      'success',
      'edo',
      d.advice.map(({ clients, item }, i) => ({
        id: `tariff/${i}`,
        clients,
        name: item.clientName,
        text: `${epdBestText(item)}: экономия ${money(item.savings)} в год`,
        late: false,
      })),
    ),
    industry: group(
      'industry',
      'Нужен ИТС Отраслевой',
      'отраслевой',
      'info',
      'its',
      d.industry.map(({ clients, item }, i) => ({
        id: `industry/${i}`,
        clients,
        name: subscriberName(item),
        text: `нужен для «${item.programs.join('», «')}»`,
        late: false,
      })),
    ),
    gap: group(
      'gap',
      'ЭДО без биллинга',
      'ЭДО без биллинга',
      'warn',
      'edo',
      d.gaps.edoWithoutBilling.map(({ clients, item }, i) => ({
        id: `gap/${i}`,
        clients,
        name: item.name || item.code,
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
      'edo',
      d.gaps.notInBase.map(({ clients, item }, i) => ({
        id: `notinbase/${i}`,
        clients,
        name: item.clientName || item.edoIds[0] || '—',
        text: `в биллинге, владелец ${item.ownerCodes.join(', ') || 'не указан'}; в базе абонентов 1С не найден`,
        late: false,
      })),
    ),
  }
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
