<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import Button from 'primevue/button'
import InputNumber from 'primevue/inputnumber'
import Tag from 'primevue/tag'
import MonthBars, { type MonthBar } from '../components/MonthBars.vue'
import PageHeader from '../components/PageHeader.vue'
import { useRefreshable } from '../composables/useRefreshable'
import { api, type BillingHistory, type ClientRef, type Dashboard } from '../api/client'
import {
  saveExpiryDays,
  savedExpiryDays,
  topicGroups,
  topicKeys,
  type CardTab,
  type TopicGroup,
} from '../dashboard'
import { amount, money, monthLabel, plural, shortName, when } from '../format'
import { requestState, requestStateLabels, type RequestState } from '../requestFile'
import { useLiveStore } from '../stores/live'

const live = useLiveStore()

/** Сколько пунктов повода показывать на плитке; остальные — в Клиентах. */
const shortList = 5

const data = ref<Dashboard | null>(null)
const expiryDays = ref(savedExpiryDays())

const { loading, refreshing, error, refreshFailed, dataAsOf, load } = useRefreshable(
  async () => {
    data.value = await api.dashboard(expiryDays.value)
  },
  { failText: 'Не удалось загрузить сводку.' },
)

watch(expiryDays, (days) => {
  if (!Number.isInteger(days) || days < 1 || days > 366) return
  saveExpiryDays(days)
  void load(true)
})

const groups = computed(() => (data.value ? topicGroups(data.value) : null))
const tiles = computed<TopicGroup[]>(() => (groups.value ? topicKeys.map((key) => groups.value![key]) : []))
/** Плитки с поводами; «Продление» — всегда: в ней окно напоминаний и проверка в 1С. */
const busyTiles = computed(() => tiles.value.filter((group) => group.items.length || group.key === 'renewal'))
/** Поводов нет — одной строкой внизу, а не пустыми плитками. */
const calmTiles = computed(() => [
  ...tiles.value.filter((group) => !busyTiles.value.includes(group)).map((group) => group.label),
  ...(data.value && !requestsCount.value ? ['Заявки не отправлены'] : []),
])

/**
 * Число плитки — поводы (счета, договоры, находки — как в «Находках»); сколько
 * за ними клиентов — строкой ниже, столько же покажет фильтр Клиентов.
 */
function tileCount(group: TopicGroup): number {
  return group.items.length
}

function clientsText(n: number): string {
  return `у ${n} ${plural(n, 'клиента', 'клиентов', 'клиентов')}`
}

function allLink(group: TopicGroup): RouteLocationRaw {
  if (group.key === 'anomalies') return { name: 'anomalies' }
  return { name: 'clients', query: { show: group.key } }
}

function cardLink(client: ClientRef, tab: CardTab): RouteLocationRaw {
  return { name: 'client', params: { key: client.key }, query: { tab } }
}

function othersTitle(clients: ClientRef[]): string {
  return clients.slice(1).map((client) => shortName(client.clientName)).join(', ')
}

const urgent = computed(() => (data.value?.anomalies ?? []).filter((entry) => entry.item.confidence === 'high').length)

const stateSeverity: Record<RequestState, string> = { draft: 'secondary', exported: 'info', sent: 'success' }
const requestsCount = computed(() => (data.value ? data.value.counts.drafts + data.value.counts.exported : 0))

/** Свежесть каждого источника: из чего собрана сводка и насколько это старо. */
const sources = computed(() => {
  const s = data.value?.sources
  if (!s) return []
  const period = data.value?.billing.period
  return [
    { label: period ? `биллинг ${monthLabel(period)}` : 'биллинг', at: s.billing },
    { label: 'база абонентов', at: s.subscribers },
    { label: 'договоры 1С:ИТС', at: s.its },
    { label: 'трафик месяца', at: s.traffic },
    { label: 'расход ЭПД', at: s.epdUsage },
    { label: 'лицензии', at: s.licenses },
  ]
})

/** «Проверить в 1С»: сервер сам не повторит проверку моложе 10 минут. */
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

/** История биллинга за 12 закрытых месяцев: прошлые месяцы догружаются по одному в час. */
const history = ref<BillingHistory | null>(null)

async function loadHistory() {
  try {
    history.value = await api.billingHistory()
  } catch {
    // Без графика сводка работает.
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

onMounted(loadHistory)
watch(() => live.lastEvent, loadHistory)
</script>

<template>
  <section>
    <PageHeader
      :refreshing="refreshing"
      :data-as-of="dataAsOf"
      :refresh-failed="refreshFailed"
      :error="error"
      @refresh="load(true)"
    >
      <template #meta>
        <span
          v-if="loading && !data"
          class="meta"
        >Собираем сводку…</span>
      </template>
    </PageHeader>

    <!-- Свежесть источников — под шапкой, а не в ней: липкая шапка из шести
         строк на телефоне закрывала бы треть экрана. -->
    <p
      v-if="sources.length"
      class="sources"
    >
      <span>Данные:</span>
      <span
        v-for="item in sources"
        :key="item.label"
        :class="{ never: !item.at }"
      >{{ item.label }} — {{ item.at ? when(item.at) : 'ещё не было' }}</span>
    </p>

    <div
      v-if="data"
      class="tiles"
    >
      <article
        v-for="group in busyTiles"
        :key="group.key"
        class="tile"
        :class="[group.tone, { quiet: tileCount(group) === 0 }]"
      >
        <header class="tile-head">
          <RouterLink
            :to="allLink(group)"
            class="tile-title"
          >
            {{ group.label }}
          </RouterLink>
          <strong class="count">{{ tileCount(group) }}</strong>
        </header>
        <small
          v-if="group.key === 'invoice' && group.items.length"
          class="sub"
        >к выставлению {{ money(data.billing.totalDue) }} за {{ monthLabel(data.billing.period) }}</small>
        <small
          v-else-if="group.key === 'anomalies' && group.items.length"
          class="sub"
        >срочных {{ urgent }} · {{ clientsText(group.count) }}<template
          v-if="data.counts.reviewDue"
        > · пора пересмотреть скрытых {{ data.counts.reviewDue }}</template></small>
        <small
          v-else-if="group.key !== 'anomalies' && group.items.length !== group.count"
          class="sub"
        >{{ clientsText(group.count) }}</small>

        <div
          v-if="group.key === 'renewal'"
          class="tile-tools"
        >
          <label class="window">
            договоры и лицензии, кончающиеся за
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
            :title="data.sources.its ? `Проверено ${when(data.sources.its)}` : 'Договоры ещё не проверялись'"
            @click="refreshIts"
          />
          <small
            v-if="itsError"
            class="late"
          >{{ itsError }}</small>
        </div>

        <ul
          v-if="group.items.length"
          class="items"
        >
          <li
            v-for="item in group.items.slice(0, shortList)"
            :key="item.id"
          >
            <span class="who">
              <RouterLink
                v-if="item.clients[0]"
                :to="cardLink(item.clients[0], group.tab)"
                :title="`Карточка клиента: ${item.clients[0].clientName}`"
              >{{ shortName(item.clients[0].clientName) || item.clients[0].key }}</RouterLink>
              <span
                v-else
                :title="item.name"
              >{{ shortName(item.name) }}</span>
              <small
                v-if="item.clients.length > 1"
                :title="othersTitle(item.clients)"
              > и ещё {{ item.clients.length - 1 }}</small>
            </span>
            <small :class="{ late: item.late }">{{ item.text }}</small>
          </li>
        </ul>
        <p
          v-else
          class="calm"
        >
          Нет.
        </p>
        <RouterLink
          v-if="group.items.length"
          :to="allLink(group)"
          class="more"
        >
          {{ group.key === 'anomalies' ? `Все находки: ${group.items.length}` : `Клиенты: ${group.count}` }} →
        </RouterLink>
      </article>

      <article
        v-if="requestsCount"
        class="tile info"
      >
        <header class="tile-head">
          <RouterLink
            :to="{ name: 'requests' }"
            class="tile-title"
          >
            Заявки не отправлены
          </RouterLink>
          <strong class="count">{{ requestsCount }}</strong>
        </header>
        <small
          v-if="requestsCount"
          class="sub"
        >черновиков {{ data.counts.drafts }} · выгружено файлом {{ data.counts.exported }}</small>
        <ul
          v-if="data.requests.length"
          class="items"
        >
          <li
            v-for="entry in data.requests.slice(0, shortList)"
            :key="entry.item.id"
          >
            <span class="who">
              <RouterLink :to="{ name: 'request', params: { id: entry.item.id } }">
                {{ entry.item.title || `Заявка №${entry.item.id}` }}
              </RouterLink>
              <Tag
                :severity="stateSeverity[requestState(entry.item)]"
                :value="requestStateLabels[requestState(entry.item)]"
              />
            </span>
            <small>
              <RouterLink
                v-if="entry.clients[0]"
                :to="{ name: 'client', params: { key: entry.clients[0].key }, query: { tab: 'requests' } }"
              >{{ shortName(entry.clients[0].clientName) }}</RouterLink>
              <template v-else>{{ shortName(entry.item.companyName ?? '') || 'клиент не указан' }}</template>
              · изменена {{ when(entry.item.updatedAt) }}
            </small>
          </li>
        </ul>
        <p
          v-else
          class="calm"
        >
          Нет.
        </p>
        <RouterLink
          v-if="data.requests.length > shortList"
          :to="{ name: 'requests' }"
          class="more"
        >
          Все заявки →
        </RouterLink>
      </article>
    </div>

    <p
      v-if="data && calmTiles.length"
      class="calm-line"
    >
      Поводов нет: {{ calmTiles.join(' · ') }}.
    </p>

    <details
      v-if="history"
      class="trend"
    >
      <summary>
        Расход по месяцам, пакетов, все клиенты
        <small>загружено {{ historyLoaded.length }} из {{ history.months.length }} мес.</small>
      </summary>
      <p
        v-if="historyLoaded.length"
        class="trend-totals"
      >
        выставлено за загруженные месяцы {{ historyDue }}
      </p>
      <MonthBars
        :bars="totalBars"
        unit="пакетов"
        title="Пакеты документов ЭДО по месяцам, все клиенты"
      />
    </details>
  </section>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
section {
  display: flex;
  flex-direction: column;
  gap: 24px;
  color: var(--ui-text);
  font-size: 14px;
  line-height: 20px;
}

.sources,
.calm-line {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 16px;
  margin: 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

/* Строка свежести данных стоит прямо на странице — даём ей свою плотную
   подложку, чтобы узор фона не ложился под текст. */
.sources {
  padding: 8px 12px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg);
}

.sources .never {
  color: var(--ui-warning);
}

.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 20rem), 1fr));
  gap: 16px;
  align-items: start;
}

.tile {
  --tone: var(--ui-info);

  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  padding: 12px 16px;
  border: 1px solid var(--ui-border);
  border-top: 3px solid var(--tone);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg-panel);
}

.tile.danger {
  --tone: var(--ui-error);
}

.tile.warn {
  --tone: var(--ui-warning);
}

.tile.success {
  --tone: var(--ui-success);
}

/* Повода нет — плитка не спорит за внимание. */
.tile.quiet {
  --tone: var(--ui-border-accented);
}

.tile-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.tile-title {
  font-weight: 600;
  color: var(--ui-text-highlighted);
  text-decoration: none;
}

.tile-title:hover {
  color: var(--ui-primary);
}

.count {
  font-size: 24px;
  line-height: 32px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--tone);
}

.quiet .count {
  color: var(--ui-text-dimmed);
}

.sub,
.calm {
  margin: 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.tile-tools {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.window {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.window :deep(.days) {
  width: 4.5rem;
  text-align: right;
}

.items {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.items li {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.who {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 8px;
  min-width: 0;
  overflow-wrap: anywhere;
}

.items small {
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
  overflow-wrap: anywhere;
}

.items a,
.more {
  color: var(--ui-text);
  text-decoration: underline;
  text-decoration-color: var(--ui-border-accented);
  text-underline-offset: 3px;
}

.items a:hover,
.more:hover {
  color: var(--ui-primary);
}

.more {
  align-self: flex-start;
  font-size: 13px;
}

.late,
.items small.late {
  color: var(--ui-error);
}

.trend {
  padding: 12px 16px;
  border-radius: var(--ui-radius-lg);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-panel);
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-dimmed);
}

.trend summary {
  cursor: pointer;
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  color: var(--ui-text);
}

.trend summary small {
  margin-left: 8px;
  font-weight: 400;
  color: var(--ui-text-muted);
}

.trend-totals {
  margin: 8px 0;
  color: var(--ui-text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
