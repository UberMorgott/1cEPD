<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import Drawer from 'primevue/drawer'
import InputNumber from 'primevue/inputnumber'
import Menu from 'primevue/menu'
import Message from 'primevue/message'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import Tag from 'primevue/tag'
import Tooltip from 'primevue/tooltip'
import ClientSummary from '../components/ClientSummary.vue'
import FindingsPanel from '../components/FindingsPanel.vue'
import InfiniteTable from '../components/InfiniteTable.vue'
import MonthBars, { type MonthBar } from '../components/MonthBars.vue'
import PageHeader from '../components/PageHeader.vue'
import { useInfiniteRows } from '../composables/useInfiniteRows'
import { useRefreshable } from '../composables/useRefreshable'
import { api, type BillingHistory, type ClientList, type ClientListItem, type Dashboard } from '../api/client'
import {
  problemsByClient,
  saveExpiryDays,
  savedExpiryDays,
  severityOrder,
  topicGroups,
  topicKeys,
  type ClientProblem,
  type Topic,
  type TopicGroup,
} from '../dashboard'
import { validInn } from '../domain'
import { amount, formatDate, money, monthLabel, plural, when } from '../format'
import { useLiveStore } from '../stores/live'

/**
 * Клиенты — единственный рабочий экран: одна строка — одна организация (ИНН+КПП).
 *
 * Сверху вниз: шапка (поиск, «Наши / Прочие / Все», группировка, «Загрузить
 * CSV», редкие действия в «⋯», свежесть данных в подсказке), строка счётчиков
 * (клик фильтрует таблицу, повторный — снимает фильтр), таблица от срочного к
 * спокойному. Строка открывает справа карточку клиента (вложенный маршрут
 * client) — список с фильтрами остаётся под ней.
 *
 * Адрес: ?q= поиск, ?tab=other|all охват, ?group=sub группировка, ?sub= абонент,
 * ?filter= счётчик (action — требуют действий, findings — находки ЭДО списком,
 * остальное — ключ повода из dashboard.ts). Старый ?show= переводится в ?filter=.
 */
const route = useRoute()
const router = useRouter()
const live = useLiveStore()
const vTooltip = Tooltip

type Row = ClientListItem & {
  /** Абонент для группировки: первый код, без абонента — пусто. */
  group: string
  /** Поводы клиента от срочного к подсказкам. */
  problems: ClientProblem[]
  /** Порядок по срочности: меньше — нужнее. */
  rank: number
}

const list = ref<ClientList | null>(null)
const summary = ref<Dashboard | null>(null)
/** Окно напоминаний о продлении, дней: договоры и лицензии, кончающиеся за этот срок. */
const expiryDays = ref(savedExpiryDays())

const { loading, refreshing, error, refreshFailed, dataAsOf, load } = useRefreshable(
  async () => {
    const [clients, dashboard] = await Promise.all([api.clients(), api.dashboard(expiryDays.value)])
    list.value = clients
    summary.value = dashboard
  },
  { failText: 'Не удалось загрузить клиентов.' },
)

watch(expiryDays, (days) => {
  if (!Number.isInteger(days) || days < 1 || days > 366) return
  saveExpiryDays(days)
  void load(true)
})

const groups = computed(() => (summary.value ? topicGroups(summary.value) : null))
const problems = computed(() => (groups.value ? problemsByClient(groups.value) : new Map<string, ClientProblem[]>()))

const rows = computed<Row[]>(() =>
  (list.value?.clients ?? []).map((client) => {
    const own = problems.value.get(client.key) ?? []
    const first = own[0] ? severityOrder.indexOf(own[0].topic) : severityOrder.length
    return {
      ...client,
      group: client.subscriberCodes[0] ?? '',
      problems: own,
      rank: first * 100 - Math.min(own.length, 99),
    }
  }),
)

// ---- Адрес ----

function queryText(name: string): string {
  const value = route.query[name]
  return typeof value === 'string' ? value : ''
}

function setQuery(name: string, value: string) {
  const query = { ...route.query }
  if (value) query[name] = value
  else delete query[name]
  void router.replace({ query })
}

/** Поиск живёт в адресе (?q=): ссылку на отфильтрованный список можно передать. */
const search = ref(queryText('q'))
watch(search, (value) => setQuery('q', value.trim() ? value : ''))
watch(
  () => route.query.q,
  (value) => {
    const text = typeof value === 'string' ? value : ''
    if (text !== search.value) search.value = text
  },
)

type Scope = 'ours' | 'other' | 'all'
const scope = computed<Scope>(() => {
  const value = queryText('tab')
  return value === 'other' || value === 'all' ? value : 'ours'
})
const oursCount = computed(() => rows.value.filter((row) => row.ours).length)
const scopeOptions = computed(() => [
  { label: `Наши · ${oursCount.value}`, value: 'ours', title: 'Клиенты 1С-ЭДО с нашими идентификаторами' },
  { label: `Прочие · ${rows.value.length - oursCount.value}`, value: 'other', title: 'Остальная база партнёра' },
  { label: `Все · ${rows.value.length}`, value: 'all', title: 'Все организации' },
])
const scopeRows = computed(() =>
  scope.value === 'all' ? rows.value : rows.value.filter((row) => row.ours === (scope.value === 'ours')),
)

function onScope(value: Scope | null) {
  if (value) setQuery('tab', value === 'ours' ? '' : value)
}

const grouped = computed(() => queryText('group') === 'sub')
const groupOptions = [
  { label: 'Без группировки', value: 'none' },
  { label: 'По абонентам', value: 'sub' },
]

const subscriber = computed(() => queryText('sub'))

// ---- Счётчики и фильтр ----

/** Фильтр счётчика: action — любой повод, findings — находки ЭДО списком, иначе — повод. */
type Filter = 'action' | 'findings' | Exclude<Topic, 'anomalies'>
const extraTopics = topicKeys.filter(
  (key): key is Exclude<Topic, 'anomalies'> => !['invoice', 'renewal', 'anomalies'].includes(key),
)

const legacyShow: Record<string, Filter> = { problems: 'action', anomalies: 'findings' }

function asFilter(value: string): Filter | null {
  if (value === 'action' || value === 'findings') return value
  if (legacyShow[value]) return legacyShow[value]
  return topicKeys.includes(value as Topic) && value !== 'anomalies' ? (value as Filter) : null
}

const filter = computed<Filter | null>(() => asFilter(queryText('filter')))

// Старые ссылки (?show=problems, ?show=anomalies, ?show=invoice) — в новый ключ.
onMounted(() => {
  const show = queryText('show')
  if (!show) return
  const query = { ...route.query }
  delete query.show
  const next = asFilter(show)
  if (next) query.filter = next
  void router.replace({ query })
})

function toggleFilter(key: Filter) {
  setQuery('filter', filter.value === key ? '' : key)
}

function topicCount(topic: Topic): number {
  return scopeRows.value.filter((row) => row.problems.some((p) => p.topic === topic)).length
}

const actionCount = computed(() => scopeRows.value.filter((row) => row.problems.length).length)
const findingsCount = computed(() => summary.value?.anomalies.length ?? 0)
const urgentFindings = computed(
  () => (summary.value?.anomalies ?? []).filter((entry) => entry.item.confidence === 'high').length,
)

interface Counter {
  key: Filter
  label: string
  count: number
  tone: TopicGroup['tone'] | 'neutral'
  hint: string
}

const counters = computed<Counter[]>(() => {
  const g = groups.value
  if (!g || !summary.value) return []
  const s = summary.value
  return [
    {
      key: 'action',
      label: 'Требуют действий',
      count: actionCount.value,
      tone: 'neutral',
      hint: 'Клиенты с любым поводом: счёт, лимит, продление, находки, подсказки',
    },
    {
      key: 'invoice',
      label: 'Выставить счёт',
      count: topicCount('invoice'),
      tone: 'danger',
      hint: g.invoice.items.length
        ? `К выставлению ${money(s.billing.totalDue)} за ${monthLabel(s.billing.period)}`
        : `Сверх лимита за ${monthLabel(s.billing.period)} никого`,
    },
    {
      key: 'renewal',
      label: 'Продление ИТС',
      count: topicCount('renewal'),
      tone: 'warn',
      hint: `Договоры 1С:ИТС, кончающиеся за ${expiryDays.value} дн.`,
    },
    {
      key: 'findings',
      label: 'Находки ЭДО',
      count: findingsCount.value,
      tone: 'danger',
      hint: `Активных находок ${findingsCount.value}, срочных ${urgentFindings.value}` +
        (s.counts.reviewDue ? `; скрытых пора пересмотреть: ${s.counts.reviewDue}` : ''),
    },
  ]
})

/** Остальные поводы — в меню «Ещё фильтры»: их много и они реже. */
const moreMenu = ref<InstanceType<typeof Menu> | null>(null)
const moreItems = computed(() =>
  groups.value
    ? extraTopics.map((key) => ({
      label: `${groups.value![key].label} · ${topicCount(key)}`,
      icon: filter.value === key ? 'pi pi-check' : undefined,
      disabled: topicCount(key) === 0 && filter.value !== key,
      command: () => toggleFilter(key),
    }))
    : [],
)
const extraActive = computed(() =>
  filter.value && (extraTopics as string[]).includes(filter.value) ? groups.value?.[filter.value as Topic] : undefined,
)

/** Неотправленные заявки: черновики и выгруженные файлом — ссылка в Заявки. */
const requestsCount = computed(() => (summary.value ? summary.value.counts.drafts + summary.value.counts.exported : 0))

// ---- Таблица ----

const visible = computed<Row[]>(() => {
  const query = search.value.trim().toLowerCase()
  const f = filter.value
  const filtered = scopeRows.value.filter((row) => {
    if (f === 'action' && !row.problems.length) return false
    if (f && f !== 'action' && f !== 'findings' && !row.problems.some((p) => p.topic === f)) return false
    if (subscriber.value && !row.subscriberCodes.includes(subscriber.value)) return false
    if (!query) return true
    return [row.clientName, row.inn, row.kpp, row.subscriberName, ...row.logins, ...row.edoIds, ...row.subscriberCodes]
      .join(' ')
      .toLowerCase()
      .includes(query)
  })
  // Группы идут подряд: без абонента — в конце, внутри группы — по срочности.
  if (grouped.value) {
    return [...filtered].sort(
      (a, b) => Number(!a.group) - Number(!b.group) || a.group.localeCompare(b.group) || a.rank - b.rank,
    )
  }
  // По умолчанию — сначала те, кому нужнее, дальше по названию.
  return [...filtered].sort((a, b) => a.rank - b.rank || a.clientName.localeCompare(b.clientName, 'ru'))
})

/** Лента вместо страниц: порции дорисовываются при прокрутке, фильтры начинают заново. */
const feed = useInfiniteRows(() => visible.value, {
  resetOn: [search, filter, subscriber, grouped, scope],
})

// Группировка держит свой порядок строк: сортировка из шапки его бы разорвала.
watch(grouped, (on) => {
  if (on) {
    feed.setSortField(undefined)
    feed.setSortOrder(undefined)
  }
})

function groupSize(code: string): number {
  return visible.value.filter((row) => row.group === code).length
}

const subscriberNames = computed(() => {
  const map = new Map<string, string>()
  for (const row of rows.value) {
    for (const code of row.subscriberCodes) if (row.subscriberName && !map.has(code)) map.set(code, row.subscriberName)
  }
  return map
})

const tagSeverity: Record<TopicGroup['tone'], string> = {
  danger: 'danger',
  warn: 'warn',
  info: 'info',
  success: 'success',
}

function problemLabel(row: Row, problem: ClientProblem): string {
  if (problem.topic === 'anomalies') return `находки ${row.anomalies || ''}`.trim()
  return groups.value?.[problem.topic].short ?? problem.topic
}

/** Подсказка статуса: все поводы клиента построчно. */
function problemsHint(row: Row): string {
  return row.problems.map((p) => `${groups.value?.[p.topic].label}: ${p.text}`).join('\n')
}

function usageClass(row: Row) {
  return { over: row.overLimit, low: row.lowRemainder && !row.overLimit }
}

const emptyText = computed(() => {
  if (search.value.trim()) return `По запросу «${search.value.trim()}» никого не нашлось.`
  if (subscriber.value) return `У абонента ${subscriber.value} организаций в этом списке нет.`
  switch (filter.value) {
    case 'action':
      return 'Никому ничего делать не нужно.'
    case 'invoice':
      return 'Счетов к выставлению нет.'
    case 'renewal':
      return `Договоров 1С:ИТС, кончающихся за ${expiryDays.value} дн., нет.`
    case null:
    case 'findings':
      return 'Клиентов нет.'
    default:
      return `«${groups.value?.[filter.value].label}»: таких клиентов нет.`
  }
})

function resetView() {
  search.value = ''
  const query = { ...route.query }
  for (const name of ['q', 'filter', 'sub']) delete query[name]
  void router.replace({ query })
}

function newRequest(row: Row) {
  void router.push({ name: 'request-new', query: { inn: row.inn, kpp: row.kpp } })
}

// ---- Карточка клиента справа ----

const openKey = computed(() => (route.name === 'client' ? String(route.params.key ?? '') : ''))
const cardTitle = ref('')

function openCard(key: string) {
  void router.push({ name: 'client', params: { key }, query: { ...route.query, section: undefined } })
}

function closeCard() {
  void router.push({ name: 'clients', query: { ...route.query, section: undefined } })
}

function onRowClick({ data }: { data: Row }) {
  openCard(data.key)
}

function rowClass(data: Row) {
  return data.key === openKey.value ? 'current' : ''
}

// ---- Шапка: свежесть, загрузка CSV, меню «⋯» ----

/** Свежесть каждого источника: из чего собран экран и насколько это старо. */
const sources = computed(() => {
  const s = summary.value?.sources
  if (!s) return []
  const period = summary.value?.billing.period
  const dayAgo = Date.now() - 36 * 3600 * 1000
  return [
    { label: period ? `биллинг ${monthLabel(period)}` : 'биллинг', at: s.billing, daily: false },
    { label: 'база абонентов', at: s.subscribers, daily: true },
    { label: 'договоры 1С:ИТС', at: s.its, daily: true },
    { label: 'трафик месяца', at: s.traffic, daily: true },
    { label: 'расход ЭПД', at: s.epdUsage, daily: true },
    { label: 'лицензии', at: s.licenses, daily: true },
  ].map((item) => ({ ...item, stale: !item.at || (item.daily && Date.parse(item.at) < dayAgo) }))
})

const staleSources = computed(() => sources.value.filter((item) => item.stale).length)

const freshnessHint = computed(() =>
  [
    'Данные из 1С:',
    ...sources.value.map((item) => `${item.stale ? '⚠ ' : ''}${item.label} — ${item.at ? when(item.at) : 'ещё не было'}`),
  ].join('\n'),
)

/** «Проверить в 1С» договоры ИТС: сервер сам не повторит проверку моложе 10 минут. */
const itsRefreshing = ref(false)
const itsError = ref('')

async function refreshIts() {
  itsRefreshing.value = true
  itsError.value = ''
  try {
    await api.refreshItsContracts()
    await load(true)
  } catch (err) {
    itsError.value = err instanceof Error ? err.message : 'Не удалось проверить договоры в 1С.'
  } finally {
    itsRefreshing.value = false
  }
}

/** Выгрузка базы абонентов из 1С: свежую (моложе 10 минут) сервер не повторяет. */
const exporting = ref(false)
const notice = ref('')

async function refreshSubscribers() {
  exporting.value = true
  notice.value = ''
  try {
    const result = await api.refreshSubscribers()
    notice.value = result.fetched
      ? 'База абонентов выгружена из 1С.'
      : 'База выгружалась меньше 10 минут назад — показаны эти данные.'
    await load(true)
  } catch (err) {
    notice.value = err instanceof Error ? err.message : 'Не удалось выгрузить базу абонентов.'
  } finally {
    exporting.value = false
  }
}

/**
 * Выгрузка «Детализация биллинга» ЭПД с портала 1С-ЭДО: в партнёрском API её
 * нет, без неё клиенты только с ЭПД в «Наших» не видны. Новая заменяет прежнюю.
 */
const importInput = ref<HTMLInputElement | null>(null)
const importing = ref(false)
const importResult = ref('')
const importFailed = ref(false)

async function importEpdBilling(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  importing.value = true
  importResult.value = ''
  importFailed.value = false
  try {
    const result = await api.importEpdBilling(file)
    importResult.value =
      `Выгрузка биллинга ЭПД загружена: строк ${result.epdImport.rows}, ` +
      `новых клиентов ${result.added}, обновлено ${result.updated}.`
    await load(true)
  } catch (err) {
    importFailed.value = true
    importResult.value = err instanceof Error ? err.message : 'Не удалось загрузить выгрузку биллинга.'
  } finally {
    importing.value = false
  }
}

const importHint = computed(() =>
  list.value?.epdImport
    ? `Выгрузка «Детализация биллинга» ЭПД с портала 1С-ЭДО. Последняя: ${when(list.value.epdImport.importedAt)}, ` +
      `${list.value.epdImport.fileName}, строк ${list.value.epdImport.rows}`
    : 'Выгрузка «Детализация биллинга» ЭПД с портала 1С-ЭДО: её ещё не загружали',
)

/** История биллинга за 12 закрытых месяцев: прошлые месяцы догружаются по одному в час. */
const history = ref<BillingHistory | null>(null)
const historyOpen = ref(false)

async function loadHistory() {
  try {
    history.value = await api.billingHistory()
  } catch {
    // Без графика экран работает.
    history.value = null
  }
}

const monthStatusText: Record<string, string> = {
  pending: 'ещё не загружен',
  none: 'в 1С биллинга нет',
  failed: '1С не отдала отчёт',
}

const historyLoaded = computed(() => (history.value?.months ?? []).filter((m) => m.status === 'ok'))

const totalBars = computed<MonthBar[]>(() =>
  (history.value?.months ?? []).map((month) => ({
    key: month.period,
    label: monthLabel(month.period),
    value: month.status === 'ok' ? month.packets : null,
    over: month.billable > 0,
    note:
      month.status === 'ok'
        ? `сверх лимита ${month.billable}, к выставлению ${money(month.totalDue)}`
        : monthStatusText[month.status],
  })),
)

const historyDue = computed(() => {
  const sum = historyLoaded.value.reduce((total, month) => total + (amount(month.totalDue) || 0), 0)
  return money(sum.toFixed(2).replace('.', ','))
})

function openHistory() {
  historyOpen.value = true
  void loadHistory()
}

const actionsMenu = ref<InstanceType<typeof Menu> | null>(null)
const actionItems = computed(() => [
  {
    label: 'Расход по месяцам',
    icon: 'pi pi-chart-bar',
    command: openHistory,
  },
  {
    label: exporting.value ? 'Выгружаем базу абонентов…' : 'Выгрузить базу абонентов из 1С',
    icon: 'pi pi-download',
    disabled: exporting.value,
    command: refreshSubscribers,
  },
  {
    label: itsRefreshing.value ? 'Проверяем договоры…' : 'Проверить договоры 1С:ИТС в 1С',
    icon: 'pi pi-refresh',
    disabled: itsRefreshing.value,
    command: refreshIts,
  },
])

const findingsPanel = ref<{ reload: () => Promise<void> } | null>(null)

/** Кнопка обновления: клиенты, поводы и находки разом. */
function refreshAll() {
  void load(true)
  void findingsPanel.value?.reload()
  if (historyOpen.value) void loadHistory()
}

/** Карточка что-то поменяла (находка, проверка ИТС) — счётчики и таблица тоже. */
function cardChanged() {
  void load(true)
  void findingsPanel.value?.reload()
}

watch(
  () => live.lastEvent,
  () => {
    if (historyOpen.value) void loadHistory()
  },
)
</script>

<template>
  <section>
    <PageHeader
      v-model:search="search"
      placeholder="Клиент, ИНН, логин, абонент, ID"
      :refreshing="refreshing"
      :data-as-of="dataAsOf"
      stamp=""
      :refresh-failed="refreshFailed"
      :error="error"
      @refresh="refreshAll"
    >
      <template #meta>
        <div class="view-controls">
          <SelectButton
            :model-value="scope"
            :options="scopeOptions"
            option-label="label"
            option-value="value"
            :allow-empty="false"
            aria-label="Чьи клиенты"
            class="scope"
            @update:model-value="onScope"
          >
            <template #option="{ option }">
              <span :title="option.title">{{ option.label }}</span>
            </template>
          </SelectButton>
          <Select
            :model-value="grouped ? 'sub' : 'none'"
            :options="groupOptions"
            option-label="label"
            option-value="value"
            aria-label="Группировка"
            class="grouping"
            @update:model-value="(value: string) => setQuery('group', value === 'sub' ? 'sub' : '')"
          />
        </div>
      </template>
      <template #tools>
        <span
          v-if="dataAsOf || sources.length"
          v-tooltip.bottom="freshnessHint"
          class="freshness"
          :class="{ stale: staleSources }"
          tabindex="0"
          :aria-label="freshnessHint"
        >
          <span class="fresh-word">{{ refreshing ? 'Обновляем…' : 'Обновлено в' }}</span>
          <template v-if="!refreshing">{{ dataAsOf }}</template>
          <i :class="staleSources ? 'pi pi-exclamation-circle' : 'pi pi-info-circle'" />
        </span>
        <Button
          label="Загрузить CSV"
          icon="pi pi-upload"
          size="small"
          class="import"
          :loading="importing"
          :title="importHint"
          @click="importInput?.click()"
        />
        <input
          ref="importInput"
          type="file"
          accept=".csv,text/csv"
          hidden
          data-testid="epd-import-file"
          @change="importEpdBilling"
        >
        <Button
          icon="pi pi-ellipsis-h"
          size="small"
          outlined
          aria-label="Ещё действия"
          aria-haspopup="true"
          aria-controls="clients-actions"
          @click="actionsMenu?.toggle($event)"
        />
        <Menu
          id="clients-actions"
          ref="actionsMenu"
          :model="actionItems"
          popup
        />
      </template>
    </PageHeader>

    <Message
      v-if="notice"
      severity="info"
      :closable="false"
    >
      {{ notice }}
    </Message>
    <Message
      v-if="importResult"
      :severity="importFailed ? 'error' : 'success'"
      :closable="false"
    >
      {{ importResult }}
    </Message>
    <Message
      v-if="itsError"
      severity="error"
      :closable="false"
    >
      {{ itsError }}
    </Message>
    <Message
      v-if="list && !list.subscribersFetchedAt"
      severity="info"
      :closable="false"
    >
      Базу абонентов ещё не выгружали: она обновляется раз в сутки, или «⋯» → «Выгрузить базу абонентов из 1С».
    </Message>

    <div
      v-if="!error"
      class="surface"
    >
      <p
        v-if="list"
        class="meta-line"
      >
        {{ visible.length }} из {{ scopeRows.length }} {{ plural(scopeRows.length, 'клиента', 'клиентов', 'клиентов') }}
        · <span :title="list.takenAt ? `Снимок биллинга от ${when(list.takenAt)}` : ''">биллинг {{ monthLabel(list.period) }}</span>
        ·
        <span
          v-if="list.epdImport"
          :title="`${list.epdImport.fileName}, строк ${list.epdImport.rows}`"
        >CSV ЭПД загружен {{ when(list.epdImport.importedAt) }}</span>
        <span v-else>CSV ЭПД не загружали</span>
      </p>

      <!-- Счётчики: клик оставляет в таблице их клиентов, повторный — снимает. -->
      <div
        class="counters"
        role="group"
        aria-label="Кому что сделать"
      >
        <button
          v-for="item in counters"
          :key="item.key"
          type="button"
          class="counter"
          :class="[item.tone, { active: filter === item.key, zero: !item.count }]"
          :aria-pressed="filter === item.key"
          :title="item.hint"
          @click="toggleFilter(item.key)"
        >
          <strong>{{ item.count }}</strong>
          <span>{{ item.label }}</span>
          <i
            v-if="filter === item.key"
            class="pi pi-times"
            aria-hidden="true"
          />
        </button>
        <Button
          :label="extraActive ? `${extraActive.label} · ${topicCount(extraActive.key)}` : 'Ещё фильтры'"
          :icon="extraActive ? 'pi pi-filter-fill' : 'pi pi-filter'"
          icon-pos="left"
          size="small"
          :outlined="!extraActive"
          class="more"
          aria-haspopup="true"
          aria-controls="clients-more"
          @click="moreMenu?.toggle($event)"
        />
        <Menu
          id="clients-more"
          ref="moreMenu"
          :model="moreItems"
          popup
        />
        <RouterLink
          v-if="requestsCount"
          :to="{ name: 'requests' }"
          class="counter info link"
          :title="`черновиков ${summary?.counts.drafts ?? 0} · выгружено файлом ${summary?.counts.exported ?? 0}`"
        >
          <strong>{{ requestsCount }}</strong>
          <span>Неотправленные заявки</span>
          <i
            class="pi pi-arrow-right"
            aria-hidden="true"
          />
        </RouterLink>
      </div>

      <!-- Состояние фильтра: что показано и чем его настроить. -->
      <div
        v-if="filter === 'renewal'"
        class="filter-bar"
      >
        <label class="window">
          Договоры и лицензии, кончающиеся за
          <InputNumber
            v-model="expiryDays"
            :min="1"
            :max="366"
            :use-grouping="false"
            input-class="days"
            aria-label="Окно напоминаний, дней"
          />
          дн.
        </label>
        <Button
          label="Проверить в 1С"
          icon="pi pi-refresh"
          size="small"
          outlined
          :loading="itsRefreshing"
          @click="refreshIts"
        />
        <small class="dim">{{ summary?.sources.its ? `проверено ${when(summary.sources.its)}` : 'договоры ещё не проверялись' }}</small>
      </div>
      <div
        v-if="subscriber || extraActive"
        class="filter-bar"
      >
        <span
          v-if="extraActive"
          class="filter-tag"
        >
          {{ extraActive.label }}
          <Button
            icon="pi pi-times"
            size="small"
            text
            :aria-label="`Снять фильтр «${extraActive.label}»`"
            @click="toggleFilter(extraActive.key as Filter)"
          />
        </span>
        <span
          v-if="subscriber"
          class="filter-tag"
        >
          Абонент <code>{{ subscriber }}</code><template v-if="subscriberNames.get(subscriber)">
            — {{ subscriberNames.get(subscriber) }}</template>
          <Button
            icon="pi pi-times"
            size="small"
            text
            aria-label="Снять фильтр по абоненту"
            @click="setQuery('sub', '')"
          />
        </span>
      </div>

      <!-- Находки ЭДО — по одной строке на находку: у каждой своё действие. -->
      <FindingsPanel
        v-if="filter === 'findings'"
        ref="findingsPanel"
        :search="search"
        @clear-search="search = ''"
        @changed="load(true)"
        @open="openCard"
      />

      <InfiniteTable
        v-else
        :feed="feed"
        :loading="loading"
        data-key="key"
        :row-group-mode="grouped ? 'subheader' : undefined"
        group-rows-by="group"
        :row-class="rowClass"
        class="clients"
        @row-click="onRowClick"
      >
        <template #empty>
          <div class="empty">
            <span>{{ emptyText }}</span>
            <Button
              v-if="search.trim() || filter || subscriber"
              label="Показать всех"
              size="small"
              text
              @click="resetView"
            />
          </div>
        </template>

        <template #groupheader="{ data }">
          <div class="group-head">
            <template v-if="data.group">
              <button
                type="button"
                class="linkish"
                :title="`Только абонент ${data.group}`"
                @click.stop="setQuery('sub', data.group)"
              >
                <code>{{ data.group }}</code>
              </button>
              <span>{{ subscriberNames.get(data.group) || 'нет в базе абонентов' }}</span>
            </template>
            <span v-else>Абонент не известен</span>
            <small>{{ groupSize(data.group) }} {{ plural(groupSize(data.group), 'организация', 'организации', 'организаций') }}</small>
          </div>
        </template>

        <Column
          field="clientName"
          header="Клиент"
          :sortable="!grouped"
          style="min-width: 15rem"
        >
          <template #body="{ data }">
            <ClientSummary
              :name="data.clientName"
              :inn="data.inn"
              :kpp="data.kpp"
            />
          </template>
        </Column>

        <Column
          field="group"
          header="Абонент"
          :sortable="!grouped"
          class="wide-only"
        >
          <template #body="{ data }">
            <div
              v-if="data.subscriberCodes.length"
              class="stack"
            >
              <span class="codes">
                <button
                  v-for="code in data.subscriberCodes"
                  :key="code"
                  type="button"
                  class="linkish"
                  :title="`Все организации абонента ${code}`"
                  @click.stop="setQuery('sub', code)"
                >
                  <code>{{ code }}</code>
                </button>
              </span>
              <small
                v-if="data.subscriberName"
                class="ellipsis"
                :title="data.subscriberName"
              >{{ data.subscriberName }}</small>
            </div>
            <span
              v-else
              class="dim"
            >—</span>
          </template>
        </Column>

        <Column
          field="used"
          header="Пакеты / лимит"
          :sortable="!grouped"
          class="num"
          style="width: 10.5rem"
        >
          <template #body="{ data }">
            <div class="stack end">
              <span
                v-if="data.inBilling"
                class="usage"
                :class="usageClass(data)"
              >
                {{ data.used }}
                <small>{{ data.limit === null ? 'без лимита' : `из ${data.limit}` }}</small>
              </span>
              <span
                v-else
                class="dim"
              >—</span>
              <small
                v-if="data.tariff"
                class="ellipsis tariff"
                :title="`Тариф: ${data.tariff}`"
              >{{ data.tariff }}</small>
            </div>
          </template>
        </Column>

        <Column
          field="billable"
          header="Счёт"
          :sortable="!grouped"
          class="num wide-only"
          style="width: 8rem"
        >
          <template #body="{ data }">
            <span :class="{ dim: data.billable <= 0 }">{{ data.billable > 0 ? money(data.amount) : '—' }}</span>
          </template>
        </Column>

        <Column
          field="rank"
          header="Статус"
          :sortable="!grouped"
          style="width: 10rem"
        >
          <template #body="{ data }">
            <span
              v-if="data.problems.length"
              v-tooltip.left="problemsHint(data)"
              class="status"
            >
              <Tag
                :severity="tagSeverity[groups![data.problems[0].topic as Topic].tone]"
                :value="problemLabel(data, data.problems[0])"
              />
              <small
                v-if="data.problems.length > 1"
                class="more-count"
              >+{{ data.problems.length - 1 }}</small>
            </span>
            <span
              v-else
              class="dim"
            >—</span>
          </template>
        </Column>

        <Column
          field="lastRequestAt"
          header="Последняя заявка"
          :sortable="!grouped"
          class="wide-only nowrap"
          style="width: 10rem"
        >
          <template #body="{ data }">
            <span
              v-if="data.requests"
              :title="`Заявок на клиента: ${data.requests}`"
            >{{ data.requests }} · {{ formatDate(data.lastRequestAt) }}</span>
            <span
              v-else
              class="dim"
            >—</span>
          </template>
        </Column>

        <Column
          header=""
          class="row-action"
          style="width: 6.5rem"
        >
          <template #body="{ data }">
            <Button
              label="Заявка"
              icon="pi pi-plus"
              size="small"
              text
              :disabled="!validInn(data.inn)"
              :title="validInn(data.inn) ? `Новая заявка на ${data.clientName}` : 'ИНН в 1С не заполнен: заявку не завести'"
              @click.stop="newRequest(data)"
            />
          </template>
        </Column>
      </InfiniteTable>
    </div>

    <Drawer
      :visible="!!openKey"
      position="right"
      :header="cardTitle || 'Карточка клиента'"
      class="client-drawer"
      :style="{ width: 'min(760px, 100vw)' }"
      @update:visible="(value: boolean) => { if (!value) closeCard() }"
    >
      <RouterView
        v-if="openKey"
        v-slot="{ Component }"
      >
        <component
          :is="Component"
          @changed="cardChanged"
          @title="(text: string) => (cardTitle = text)"
        />
      </RouterView>
    </Drawer>

    <Dialog
      v-model:visible="historyOpen"
      modal
      header="Расход по месяцам, пакетов, все клиенты"
      :style="{ width: 'min(960px, calc(100vw - 32px))' }"
    >
      <p
        v-if="!history"
        class="dim"
      >
        Загружаем историю биллинга…
      </p>
      <template v-else>
        <p class="dim">
          Загружено {{ historyLoaded.length }} из {{ history.months.length }} мес.
          <template v-if="historyLoaded.length">
            · выставлено за загруженные месяцы {{ historyDue }}
          </template>
        </p>
        <MonthBars
          :bars="totalBars"
          unit="пакетов"
          title="Пакеты документов ЭДО по месяцам, все клиенты"
        />
      </template>
    </Dialog>
  </section>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
  color: var(--ui-text);
  font-size: 14px;
  line-height: 20px;
}

/* Всё под шапкой — одна плотная панель: узор фона страницы виден по краям,
   но не ложится под счётчики и строки. */
.surface {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding: 12px 16px 16px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg);
}

.view-controls {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px;
}

.view-controls :deep(.p-selectbutton .p-togglebutton) {
  height: 32px;
  padding: 0 10px;
  font-size: 13px;
}

.view-controls :deep(.p-select) {
  height: 32px;
  align-items: center;
  font-size: 13px;
}

.view-controls :deep(.p-select-label) {
  padding: 0 10px;
  font-size: 13px;
}

.freshness {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
  white-space: nowrap;
  cursor: help;
}

.freshness .pi {
  font-size: 12px;
}

.freshness.stale .pi {
  color: var(--ui-warning);
}

.meta-line {
  margin: 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.counters,
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.counter {
  --tone: var(--ui-info);

  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  padding: 6px 10px;
  box-sizing: border-box;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg-panel);
  color: var(--ui-text);
  font: inherit;
  text-decoration: none;
  cursor: pointer;
}

.counter strong {
  min-width: 1.5ch;
  font-size: 16px;
  line-height: 24px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--tone);
}

.counter .pi {
  font-size: 11px;
  color: var(--ui-text-muted);
}

.counter:hover {
  background: var(--ui-bg-panel-hover);
}

.counter.active {
  border-color: var(--tone);
  background: color-mix(in oklab, var(--tone) 18%, var(--ui-bg));
  color: var(--ui-text-highlighted);
}

.counter.zero strong {
  color: var(--ui-text-dimmed);
}

.counter.neutral {
  --tone: var(--ui-text-highlighted);
}

.counter.danger {
  --tone: var(--ui-error);
}

.counter.warn {
  --tone: var(--ui-warning);
}

.counter.link {
  margin-left: auto;
}

.more {
  margin-left: 0;
}

.filter-bar {
  padding: 8px 12px;
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg-panel);
}

.window {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ui-text-muted);
}

.window :deep(.days) {
  width: 4.5rem;
  text-align: right;
}

.filter-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding-left: 10px;
  border: 1px solid var(--ui-border-accented);
  border-radius: var(--ui-radius-lg);
  font-size: 13px;
}

.dim {
  color: var(--ui-text-muted);
}

.empty {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ui-text-muted);
}

.stack {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.stack.end {
  align-items: flex-end;
}

.stack small {
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.ellipsis {
  max-width: 16rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tariff {
  max-width: 10rem;
  font-variant-numeric: normal;
}

.status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.more-count {
  padding: 0 6px;
  border-radius: 999px;
  background: var(--ui-bg-accented);
  font-size: 12px;
  line-height: 18px;
  color: var(--ui-text);
}

.codes {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.linkish {
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  cursor: pointer;
}

.linkish code {
  font-size: 12px;
  color: var(--ui-text);
  text-decoration: underline;
  text-decoration-color: var(--ui-border-accented);
  text-underline-offset: 3px;
}

.linkish:hover code {
  color: var(--ui-primary);
}

.group-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 12px;
  font-weight: 500;
  color: var(--ui-text-highlighted);
}

.group-head small {
  font-weight: 400;
  color: var(--ui-text-muted);
}

.usage small {
  font-size: 12px;
  color: var(--ui-text-muted);
  white-space: nowrap;
}

.usage.low small {
  color: var(--ui-warning);
  font-weight: 500;
}

.usage.over small {
  color: var(--ui-error);
  font-weight: 500;
}

code {
  font-size: 12px;
  color: var(--ui-text-muted);
  white-space: nowrap;
}

:deep(.p-datatable-row-group-header > td) {
  padding: 8px 12px;
  background: var(--ui-bg-panel-hover);
}

/* Открытая в панели карточка — её строка подсвечена. */
:deep(.p-datatable-tbody > tr.current) {
  background: color-mix(in oklab, var(--ui-primary) 12%, var(--ui-bg));
}

/* Действие строки видно при наведении, в покое не шумит. */
:deep(td.row-action) {
  text-align: right;
}

:deep(td.row-action .p-button) {
  opacity: 0.55;
}

:deep(tr:hover td.row-action .p-button),
:deep(td.row-action .p-button:focus-visible) {
  opacity: 1;
}

/* Заголовок «Последняя заявка» — одной строкой. */
:deep(th.nowrap .p-datatable-column-title) {
  white-space: nowrap;
}

/* Ноутбук: действие строки — одним значком, колонка клиента шире, шапка
   в одну строку — поиск уже, от отметки свежести остаётся время. */
@media (900px < width <= 1500px) {
  :deep(.bar .filter) {
    width: 13rem;
  }

  .fresh-word {
    display: none;
  }
}

@media (width <= 1440px) {
  :deep(td.row-action .p-button-label) {
    display: none;
  }

  .ellipsis {
    max-width: 12rem;
  }
}

@media (width <= 1280px) {
  .counter.link {
    margin-left: 0;
  }
}

/* Узкий экран: клиент, пакеты и статус — остальное в карточке. */
@media (width <= 900px) {
  .surface {
    padding: 8px;
  }

  .view-controls {
    flex-wrap: wrap;
  }

  :deep(.wide-only) {
    display: none;
  }

  :deep(.import .p-button-label) {
    display: none;
  }
}
</style>
