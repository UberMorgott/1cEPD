/**
 * Тексты о клиенте, одинаковые на всех экранах: договоры 1С:ИТС, ИТС Отраслевой,
 * тариф ЭПД, прогноз лимита, лицензии сервисов. Вёрстка — в components/*Note,
 * здесь только слова, чтобы одно и то же не писалось по-разному.
 */
import type {
  Anomaly,
  EpdAdvice,
  ForecastClient,
  ItsContract,
  ItsIndustry,
  LicenseOption,
  LicenseTariff,
} from './api/client'
import { moscowDate } from './format'

/** Настоящий ИНН: 10 цифр у организации, 12 у ИП, не одни нули. */
export function validInn(inn: string): boolean {
  const value = inn.trim()
  return /^(\d{10}|\d{12})$/.test(value) && !/^0+$/.test(value)
}

/**
 * Ключ карточки клиента — то же правило, что internal/httpapi/clientindex.go
 * clientKey: «ИНН-КПП», без КПП (или КПП из нулей) — «ИНН». Без настоящего ИНН
 * — «edo-<ID>» по идентификатору ЭДО, а без него ссылки нет: пустая строка.
 */
export function clientKey(inn: string | undefined, kpp: string | undefined, edoId = ''): string {
  const i = (inn ?? '').trim()
  const k = (kpp ?? '').trim()
  if (validInn(i)) return k && !/^0+$/.test(k) ? `${i}-${k}` : i
  return edoId ? `edo-${edoId}` : ''
}

export const anomalyKindLabels: Record<Anomaly['kind'], string> = {
  orphan_with_traffic: 'Трафик мимо тарифа',
  owner_lost: 'Владелец пропал',
  orphan_idle: 'Без владельца, без трафика',
  disappeared: 'Идентификатор пропал',
  replaced: 'Замена идентификатора',
}

export function anomalyImportance(item: Anomaly): string {
  if (item.confidence === 'high') return 'срочно'
  return item.kind === 'disappeared' ? 'подозрение' : 'наблюдение'
}


export function contractTerm(contract: { start: string; end: string }): string {
  return `${moscowDate(contract.start)} — ${moscowDate(contract.end)}`
}

export function contractName(contract: ItsContract): string {
  return contract.nameForUser || contract.name
}

/** ИТС Отраслевой одной строкой; пусто — 1С ответила пустым статусом, показывать нечего. */
export function industryText(industry: ItsIndustry | undefined): string {
  if (!industry) return ''
  if (industry.missing.length) return `нужен для «${industry.missing.join('», «')}», не оформлен`
  const subs = industry.programs.flatMap((program) =>
    program.subscriptions.map((sub) => `${program.name}: ${sub.name} до ${moscowDate(sub.end)}`),
  )
  return subs.length ? subs.join('; ') : industry.description
}

/** Как клиент платит за ЭПД сейчас: «Тариф ×2 + Тариф» или «поштучно». */
export function epdCurrentText(item: EpdAdvice): string {
  if (!item.current.length) return 'поштучно'
  return item.current.map((t) => (t.count && t.count > 1 ? `${t.name} ×${t.count}` : t.name)).join(' + ')
}

/** Самый выгодный способ оплаты ЭПД; во Фреше — с кодом Менеджера сервиса. */
export function epdBestText(item: EpdAdvice): string {
  if (!item.best) return 'поштучно'
  return item.fresh ? `${item.best.name} (Менеджер сервиса, код ${item.best.freshCode})` : item.best.name
}

export function forecastText(item: ForecastClient): string {
  if (item.exceeded) return `уже ${item.used} из ${item.limit}, к концу месяца ~${item.projected}`
  return `${item.used} из ${item.limit} сейчас, к концу месяца ~${item.projected}`
}

/** Названия видов отчёта по опциям (docs/API.md §8.3). */
const serviceNames: Record<string, string> = {
  REPORTING: '1С-Отчетность',
  SIGN: '1С:Подпись',
  CLOUD_BACKUP: '1С:Облачный архив',
  COUNTERAGENT: '1С:Контрагент',
  SPARK_RISKS: '1СПАРК Риски',
  LINK: '1С:Линк',
  NOMENCLATURE: '1С:Номенклатура',
  ESS: '1С:Кабинет сотрудника',
  DOCUMENT_RECOGNITION: '1С:Распознавание документов',
  CHECK_SCAN: '1С:Сканер чеков',
  MAG1C: '1С:Маг1С',
}

export function serviceName(type: string): string {
  return serviceNames[type] ?? type
}

export function tariffServices(tariff: LicenseTariff): string {
  return tariff.services.map(serviceName).join(', ')
}

export function optionText(option: LicenseOption): string {
  if (!option.quantitative || option.max === null) return option.name
  const used = option.used ?? 0
  return `${option.name}: ${used} из ${option.max}` +
    (option.remaining !== null && option.remaining >= 0 ? `, остаток ${option.remaining}` : `, сверх объёма ${used - option.max}`)
}
