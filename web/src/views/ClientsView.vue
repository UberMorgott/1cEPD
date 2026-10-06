<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import Message from 'primevue/message'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import ClientSummary from '../components/ClientSummary.vue'
import InfiniteTable from '../components/InfiniteTable.vue'
import PageHeader from '../components/PageHeader.vue'
import { useInfiniteRows } from '../composables/useInfiniteRows'
import { useRefreshable } from '../composables/useRefreshable'
import { api, type ClientList, type ClientListItem, type Dashboard } from '../api/client'
import { savedExpiryDays, topicGroups, topicKeys, type Topic, type TopicGroup } from '../dashboard'
import { money, monthLabel, moscowDate, plural, shortName, when } from '../format'

/**
 * Клиенты: одна строка — одна организация (ИНН+КПП), бывшие «Биллинг» и
 * «Абоненты» в одной таблице. Счётчики — те же поводы, что на Сводке: клик
 * оставляет в таблице их клиентов (?show=), абонент — фильтр (?sub=) или
 * группировка (?group=sub). Строка открывает карточку клиента.
 *
 * Вкладки: «Наши» — клиенты 1С-ЭДО с нашими идентификаторами (последний биллинг
 * и отчёты трафика партнёрского API, плюс загруженная руками выгрузка «Детализация
 * биллинга» ЭПД — её в API нет), «Прочие» (?tab=other) — остальная база партнёра.
 */
const route = useRoute()
const router = useRouter()

type Row = ClientListItem & {
  /** Абонент для группировки: первый код, без абонента — пусто. */
  group: string
  topics: Topic[]
}

const list = ref<ClientList | null>(null)
const summary = ref<Dashboard | null>(null)

const { loading, refreshing, error, refreshFailed, dataAsOf, load } = useRefreshable(
  async () => {
    const [clients, dashboard] = await Promise.all([api.clients(), api.dashboard(savedExpiryDays())])
    list.value = clients
    summary.value = dashboard
  },
  { failText: 'Не удалось загрузить клиентов.' },
)

const groups = computed(() => (summary.value ? topicGroups(summary.value) : null))

const rows = computed<Row[]>(() =>
  (list.value?.clients ?? []).map((client) => ({
    ...client,
    group: client.subscriberCodes[0] ?? '',
    topics: groups.value ? topicKeys.filter((key) => groups.value![key].keys.has(client.key)) : [],
  })),
)

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

const active = computed<Topic | null>(() => {
  const value = queryText('show')
  return topicKeys.includes(value as Topic) ? (value as Topic) : null
})
const activeGroup = computed<TopicGroup | null>(() => (active.value && groups.value ? groups.value[active.value] : null))
const subscriber = computed(() => queryText('sub'))

type TabKey = 'ours' | 'other'
const tab = computed<TabKey>(() => (queryText('tab') === 'other' ? 'other' : 'ours'))
const oursCount = computed(() => rows.value.filter((row) => row.ours).length)
const tabRows = computed(() => rows.value.filter((row) => row.ours === (tab.value === 'ours')))

function onTab(value: string | number) {
  setQuery('tab', value === 'other' ? 'other' : '')
}
const grouped = computed(() => queryText('group') === 'sub')

const chips = computed(() =>
  groups.value ? topicKeys.map((key) => groups.value![key]).filter((g) => g.count > 0 || g.key === active.value) : [],
)

function toggleChip(key: Topic) {
  setQuery('show', active.value === key ? '' : key)
}

const subscriberNames = computed(() => {
  const map = new Map<string, string>()
  for (const row of rows.value) {
    for (const code of row.subscriberCodes) if (row.subscriberName && !map.has(code)) map.set(code, row.subscriberName)
  }
  return map
})

const visible = computed<Row[]>(() => {
  const query = search.value.trim().toLowerCase()
  const topic = activeGroup.value
  const filtered = tabRows.value.filter((row) => {
    if (topic && !topic.keys.has(row.key)) return false
    if (subscriber.value && !row.subscriberCodes.includes(subscriber.value)) return false
    if (!query) return true
    return [row.clientName, row.inn, row.kpp, row.subscriberName, ...row.logins, ...row.edoIds, ...row.subscriberCodes]
      .join(' ')
      .toLowerCase()
      .includes(query)
  })
  // Группы идут подряд: без абонента — в конце, внутри группы — по названию.
  if (grouped.value) {
    return [...filtered].sort(
      (a, b) => Number(!a.group) - Number(!b.group) || a.group.localeCompare(b.group) || 0,
    )
  }
  return filtered
})

/** Лента вместо страниц: порции дорисовываются при прокрутке, фильтры начинают заново. */
const feed = useInfiniteRows(() => visible.value, { resetOn: [search, active, subscriber, grouped, tab] })

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

function openCard({ data }: { data: Row }) {
  void router.push({ name: 'client', params: { key: data.key } })
}

/** Выгрузка базы абонентов из 1С: свежую (моложе 10 минут) сервер не повторяет. */
const exporting = ref(false)
const notice = ref('')

async function refreshSubscribers() {
  exporting.value = true
  notice.value = ''
  try {
    const result = await api.refreshSubscribers()
    if (!result.fetched) notice.value = 'База выгружалась меньше 10 минут назад — показаны эти данные.'
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

function usageClass(row: Row) {
  return { over: row.overLimit, low: row.lowRemainder && !row.overLimit }
}

const tagSeverity: Record<TopicGroup['tone'], string> = {
  danger: 'danger',
  warn: 'warn',
  info: 'info',
  success: 'success',
}
</script>

<template>
  <section>
    <PageHeader
      v-model:search="search"
      placeholder="Клиент, ИНН, логин, абонент, ID"
      :refreshing="refreshing"
      :data-as-of="dataAsOf"
      :refresh-failed="refreshFailed"
      :error="error"
      @refresh="load(true)"
    >
      <template #meta>
        <small
          v-if="list"
          class="meta"
        >
          {{ visible.length }} из {{ tabRows.length }} {{ plural(tabRows.length, 'клиента', 'клиентов', 'клиентов') }} ·
          <span :title="list.takenAt ? `Снимок биллинга от ${when(list.takenAt)}` : ''">биллинг {{ monthLabel(list.period) }}</span>
          <template v-if="list.epdImport">
            · <span :title="`${list.epdImport.fileName}, строк ${list.epdImport.rows}`">выгрузка ЭПД {{ when(list.epdImport.importedAt) }}</span>
          </template>
        </small>
      </template>
      <template #tools>
        <Button
          label="Выгрузить базу абонентов"
          class="export"
          icon="pi pi-download"
          size="small"
          outlined
          :loading="exporting"
          :title="list?.subscribersFetchedAt ? `База абонентов выгружена ${when(list.subscribersFetchedAt)}` : 'Базу абонентов ещё не выгружали'"
          @click="refreshSubscribers"
        />
        <Button
          label="Загрузить выгрузку биллинга (CSV)"
          class="export"
          icon="pi pi-upload"
          size="small"
          outlined
          :loading="importing"
          :title="list?.epdImport ? `Выгрузка биллинга ЭПД загружена ${when(list.epdImport.importedAt)}: ${list.epdImport.fileName}, строк ${list.epdImport.rows}` : 'Выгрузку «Детализация биллинга» ЭПД ещё не загружали'"
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
      v-if="list && !list.subscribersFetchedAt"
      severity="info"
      :closable="false"
    >
      Базу абонентов ещё не выгружали: она обновляется раз в сутки, или нажмите «Выгрузить базу абонентов».
    </Message>

    <template v-if="!error">
      <div class="tabs-row">
        <Tabs
          :value="tab"
          @update:value="onTab"
        >
          <TabList>
            <Tab value="ours">
              Наши (1С-ЭДО) <span class="tab-count">{{ oursCount }}</span>
            </Tab>
            <Tab value="other">
              Прочие <span class="tab-count">{{ rows.length - oursCount }}</span>
            </Tab>
          </TabList>
        </Tabs>
      </div>

      <div
        class="chips"
        role="group"
        aria-label="Кому что сделать"
      >
        <button
          v-for="chip in chips"
          :key="chip.key"
          type="button"
          class="counter"
          :class="[chip.tone, { active: active === chip.key }]"
          :aria-pressed="active === chip.key"
          @click="toggleChip(chip.key)"
        >
          <span>{{ chip.label }}</span>
          <strong>{{ chip.count }}</strong>
        </button>
        <span
          v-if="groups && !chips.length"
          class="quiet"
        >Счетов, лимитов, продлений и подсказок сейчас нет.</span>
      </div>

      <div class="filters">
        <Button
          :label="grouped ? 'Без группировки' : 'По абонентам'"
          icon="pi pi-sitemap"
          size="small"
          outlined
          @click="setQuery('group', grouped ? '' : 'sub')"
        />
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
        <template v-if="activeGroup">
          <span class="hint">{{ activeGroup.label }}: {{ activeGroup.count }}
            {{ plural(activeGroup.count, 'клиент', 'клиента', 'клиентов') }}.</span>
          <Button
            label="Показать всех"
            size="small"
            text
            @click="toggleChip(activeGroup.key)"
          />
        </template>
      </div>

      <div
        v-if="activeGroup?.unmatched.length"
        class="unmatched"
      >
        <small>Без карточки клиента — у абонента нет организаций в базе и идентификаторов ЭДО:</small>
        <ul>
          <li
            v-for="item in activeGroup.unmatched"
            :key="item.id"
          >
            <strong :title="item.name">{{ shortName(item.name) }}</strong> —
            <span :class="{ late: item.late }">{{ item.text }}</span>
          </li>
        </ul>
      </div>

      <InfiniteTable
        :feed="feed"
        :loading="loading"
        data-key="key"
        :row-group-mode="grouped ? 'subheader' : undefined"
        group-rows-by="group"
        class="clients"
        @row-click="openCard"
      >
        <template #empty>
          Клиентов по условию нет.
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
        >
          <template #body="{ data }">
            <ClientSummary
              :name="data.clientName"
              :inn="data.inn"
              :kpp="data.kpp"
              :to="data.key"
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
          header="ID ЭДО"
          class="num wide-only"
          style="width: 6rem"
        >
          <template #body="{ data }">
            <span :title="data.edoIds.join('\n')">{{ data.edoIds.length || '—' }}</span>
          </template>
        </Column>

        <Column
          field="used"
          header="Пакеты / лимит"
          :sortable="!grouped"
          class="num"
          style="width: 10rem"
        >
          <template #body="{ data }">
            <span v-if="!data.inBilling">—</span>
            <span
              v-else
              class="usage"
              :class="usageClass(data)"
            >
              {{ data.used }}
              <small>{{ data.limit === null ? 'без лимита' : `из ${data.limit}` }}</small>
            </span>
          </template>
        </Column>

        <Column
          field="billable"
          header="Счёт"
          :sortable="!grouped"
          class="num wide-only"
          style="width: 9rem"
        >
          <template #body="{ data }">
            {{ data.billable > 0 ? money(data.amount) : '—' }}
          </template>
        </Column>

        <Column
          header="Что с клиентом"
          class="wide-only"
        >
          <template #body="{ data }">
            <div class="tags">
              <Tag
                v-for="key in data.topics"
                :key="key"
                :severity="tagSeverity[groups![key as Topic].tone]"
                :value="key === 'anomalies' ? `находки ${data.anomalies}` : groups![key as Topic].short"
                :title="groups![key as Topic].label"
              />
              <Tag
                v-if="!data.inBase && data.sources.includes('registry') && !data.topics.includes('notinbase')"
                severity="secondary"
                value="нет в базе абонентов"
              />
              <Tag
                v-if="data.itsEnd"
                severity="secondary"
                :value="`ИТС до ${moscowDate(data.itsEnd)}`"
              />
              <Tag
                v-if="data.requests"
                severity="secondary"
                :value="`заявки ${data.requests}`"
              />
            </div>
          </template>
        </Column>
      </InfiniteTable>
    </template>
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

.tabs-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 16px;
  min-width: 0;
}

.tabs-row :deep(.p-tablist-tab-list) {
  background: transparent;
}

.tabs-row :deep(.p-tab) {
  padding: 8px 12px;
  white-space: nowrap;
}

.tab-count {
  margin-left: 4px;
  font-variant-numeric: tabular-nums;
  color: var(--ui-text-muted);
}

.chips,
.filters,
.tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.tags {
  gap: 4px;
}

.counter {
  --tone: var(--ui-info);

  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  min-height: 32px;
  padding: 6px 12px;
  box-sizing: border-box;
  border: 1px solid color-mix(in oklab, var(--tone) 35%, transparent);
  border-radius: var(--ui-radius-lg);
  background: color-mix(in oklab, var(--tone) 8%, var(--ui-bg));
  color: var(--ui-text);
  font: inherit;
  cursor: pointer;
}

.counter:hover {
  background: color-mix(in oklab, var(--tone) 16%, var(--ui-bg));
}

.counter strong {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--tone);
}

.counter.active {
  border-color: var(--tone);
  background: color-mix(in oklab, var(--tone) 22%, var(--ui-bg));
  color: var(--ui-text-highlighted);
}

.counter.danger {
  --tone: var(--ui-error);
}

.counter.warn {
  --tone: var(--ui-warning);
}

.counter.success {
  --tone: var(--ui-success);
}

.quiet,
.hint,
.dim {
  color: var(--ui-text-muted);
}

.hint {
  font-size: 13px;
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

.unmatched {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 16px;
  border-radius: var(--ui-radius-lg);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-panel);
}

.unmatched small {
  color: var(--ui-text-muted);
}

.unmatched ul {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.unmatched strong {
  font-weight: 500;
  color: var(--ui-text-highlighted);
}

.late {
  color: var(--ui-error);
}

.stack {
  display: flex;
  flex-direction: column;
  min-width: 0;
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

/* Узкий экран: клиент и пакеты — остальное в карточке, абонент — в группировке. */
@media (width <= 900px) {
  :deep(.wide-only) {
    display: none;
  }

  /* Шапка липкая: кнопка выгрузки — одним значком, чтобы не занимать строку. */
  :deep(.export .p-button-label) {
    display: none;
  }

  :deep(th.num) {
    white-space: nowrap;
  }
}
</style>
