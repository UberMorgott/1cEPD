<script setup lang="ts">
import { computed, ref } from 'vue'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, type Anomaly } from '../api/client'
import { anomalyImportance, anomalyKindLabels, clientKey } from '../domain'
import { formatDate } from '../format'
import AnomalyAckDialog from './AnomalyAckDialog.vue'
import ClientSummary from './ClientSummary.vue'
import InfiniteTable from './InfiniteTable.vue'
import { useInfiniteRows } from '../composables/useInfiniteRows'
import { useRefreshable } from '../composables/useRefreshable'

/**
 * Находки ЭДО списком — то, что показывает счётчик «Находки ЭДО» в Клиентах
 * (?filter=findings): связи идентификаторов, которые стоит проверить, пометка
 * «это нормально» и скрытые находки с возвратом. Поиск — общий с Клиентами,
 * приходит свойством; строка открывает карточку клиента (событие open).
 */
const props = defineProps<{ search: string }>()
const emit = defineEmits<{ 'clear-search': []; changed: []; open: [key: string] }>()

const items = ref<Anomaly[]>([])
/** Режим: активные находки или скрытые — помеченные законными и погашенные топологией. */
const mode = ref<'active' | 'hidden'>('active')
const search = computed(() => props.search)

const ackTarget = ref<Anomaly | null>(null)
const restoreBusy = ref(0)

const kindLabels = anomalyKindLabels
const importance = anomalyImportance

function hidden(item: Anomaly) {
  return item.acknowledged || item.suppressed === true
}

const { loading, error, load } = useRefreshable(
  async () => {
    // Всегда со скрытыми: панель считает, сколько из них пора пересмотреть.
    items.value = (await api.anomalies(true)).anomalies ?? []
  },
  { failText: 'Не удалось загрузить находки.' },
)

const active = computed(() => items.value.filter((item) => !hidden(item)))
const hiddenItems = computed(() => items.value.filter(hidden))
const reviewDue = computed(() => hiddenItems.value.filter((item) => item.reviewDue).length)

const visible = computed(() => {
  const query = search.value.trim().toLowerCase()
  const source = mode.value === 'active' ? active.value : hiddenItems.value
  if (!query) return source
  return source.filter((item) =>
    [item.clientName, item.inn, item.login, item.edoId]
      .join(' ')
      .toLowerCase()
      .includes(query),
  )
})

/** Лента вместо страниц: порции дорисовываются при прокрутке, поиск и режим начинают заново. */
const feed = useInfiniteRows(() => visible.value, { resetOn: [search, mode] })

// Считаем от полного списка: иначе при поиске панель врёт, что срочного нет.
const urgent = computed(() => active.value.filter((item) => item.confidence === 'high'))

const searching = computed(() => search.value.trim() !== '')

async function acked() {
  ackTarget.value = null
  await load(true)
  emit('changed')
}
/** Вернуть скрытую находку в активные: снять пометку или убрать связь из топологии. */
async function restore(item: Anomaly) {
  restoreBusy.value = item.id
  try {
    if (item.acknowledged) await api.unacknowledge(item.edoId, item.fingerprint)
    if (item.suppressed) await api.removeTopology({ inn: item.inn, kpp: item.kpp, edoId: item.edoId })
    await load(true)
    emit('changed')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось вернуть находку.'
  } finally {
    restoreBusy.value = 0
  }
}

function openClient({ data }: { data: Anomaly }) {
  const key = clientKey(data.inn, data.kpp, data.edoId)
  if (key) emit('open', key)
}

defineExpose({ reload: () => load(true) })
</script>

<template>
  <section class="findings">
    <header class="findings-head">
      <Button
        :label="`Активные · ${active.length}`"
        size="small"
        :outlined="mode !== 'active'"
        @click="mode = 'active'"
      />
      <Button
        :label="`Скрытые · ${hiddenItems.length}`"
        size="small"
        :outlined="mode !== 'hidden'"
        @click="mode = 'hidden'"
      />
    </header>

    <Message
      v-if="error"
      severity="error"
      :closable="false"
    >
      {{ error }}
    </Message>

    <template v-else>
      <Message
        v-if="!loading && reviewDue > 0 && mode === 'active'"
        severity="warn"
        :closable="false"
      >
        <span class="stale">
          Скрытых находок пора пересмотреть: {{ reviewDue }}. Пометки «это законно» живут полгода.
          <Button
            label="Открыть скрытые"
            size="small"
            text
            @click="mode = 'hidden'"
          />
        </span>
      </Message>

      <p
        v-if="mode === 'hidden'"
        class="hint"
      >
        Здесь находки, помеченные законными, и погашенные реестром ожидаемой топологии.
        «Вернуть» снимает пометку или убирает связь из реестра — находка снова станет активной.
      </p>

      <Message
        v-if="!loading && mode === 'active' && urgent.length === 0"
        severity="success"
        :closable="false"
      >
        Срочных находок нет.
      </Message>

      <InfiniteTable
        :feed="feed"
        :loading="loading"
        data-key="id"
        class="table"
        @row-click="openClient"
      >
        <template #empty>
          <div
            v-if="searching"
            class="empty"
          >
            <span>По запросу ничего не найдено.</span>
            <Button
              label="Сбросить поиск"
              size="small"
              text
              @click="emit('clear-search')"
            />
          </div>
          <span v-else>Находок нет.</span>
        </template>
        <!-- Важность и суть в одной колонке, логин с идентификатором — в другой:
             семь узких колонок не влезали в 1400px и прятали «Действие» за прокруткой. -->
        <Column
          field="confidence"
          header="Находка"
          sortable
          style="width: 11rem"
        >
          <template #body="{ data }">
            <div class="finding">
              <Tag
                :severity="data.confidence === 'high' ? 'danger' : 'secondary'"
                :value="importance(data)"
              />
              <strong class="kind">{{ kindLabels[data.kind as Anomaly['kind']] ?? data.kind }}</strong>
            </div>
          </template>
        </Column>

        <Column
          field="clientName"
          header="Клиент"
          sortable
        >
          <template #body="{ data }">
            <ClientSummary
              :name="data.clientName"
              :inn="data.inn"
            />
          </template>
        </Column>

        <Column
          field="edoId"
          header="Логин и идентификатор"
          class="wide-only"
          style="width: 19.5rem"
        >
          <template #body="{ data }">
            <div class="client">
              <span class="login">{{ data.login || '—' }}</span>
              <code>{{ data.edoId }}</code>
            </div>
          </template>
        </Column>

        <Column
          field="details"
          header="Подробности"
          style="min-width: 16rem"
        >
          <template #body="{ data }">
            <div class="client">
              <span>{{ data.details }}</span>
              <small v-if="data.suppressed">Связь в реестре топологии: {{ data.topologyPurpose }}</small>
              <small v-if="data.acknowledged">
                Законно: {{ data.ackReason || 'без причины' }} · {{ data.ackAuthor }}
                {{ formatDate(data.ackedAt, '') }}<template v-if="data.reviewAt">, пересмотр {{ formatDate(data.reviewAt) }}</template>
              </small>
            </div>
          </template>
        </Column>

        <Column
          header="Действие"
          style="width: 12rem"
        >
          <template #body="{ data }">
            <Button
              v-if="!hidden(data)"
              label="Это нормально"
              size="small"
              severity="secondary"
              outlined
              @click.stop="ackTarget = data"
            />
            <div
              v-else
              class="client"
            >
              <Tag
                v-if="data.reviewDue"
                value="пора пересмотреть"
                severity="warn"
              />
              <Button
                label="Вернуть"
                size="small"
                severity="secondary"
                outlined
                :loading="restoreBusy === data.id"
                @click.stop="restore(data)"
              />
            </div>
          </template>
        </Column>
      </InfiniteTable>
    </template>

    <AnomalyAckDialog
      :target="ackTarget"
      @close="ackTarget = null"
      @done="acked"
    />
  </section>
</template>

<style scoped>
.findings-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  color: var(--ui-text-highlighted);
}

section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  color: var(--ui-text);
  font-size: 14px;
  line-height: 20px;
}

.empty {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ui-text-muted);
}

.stale {
  display: flex;
  align-items: center;
  gap: 8px;
}

.client {
  display: flex;
  flex-direction: column;
}

.kind {
  font-weight: 500;
  color: var(--ui-text-highlighted);
}

.finding {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

/* Логин — почта без пробелов: без разрешённого переноса он распирает колонку. */
.login {
  overflow-wrap: anywhere;
}

.client small {
  color: var(--ui-text-muted);
  font-size: 12px;
  line-height: 16px;
}

.hint {
  margin-top: 0;
  font-size: 13px;
  line-height: 18px;
  color: var(--ui-text-muted);
}

code {
  font-size: 12px;
  color: var(--ui-text-muted);

  /* Идентификатор — единый токен: перенос по дефисам ломает его на три строки. */
  white-space: nowrap;
}

/* Таблица без обёртки: приглушённая шапка, разделители строк. */
:deep(.p-datatable-thead > tr > th) {
  padding: 10px 12px;
  background: var(--ui-bg-panel-hover);
  border-width: 0 0 1px;
  border-color: var(--ui-border);
  color: var(--ui-text-highlighted);
  font-size: 14px;
  font-weight: 600;
  text-transform: none;
}

:deep(.p-datatable-tbody > tr) {
  background: var(--ui-bg);
  color: var(--ui-text);
}

:deep(.p-datatable-tbody > tr > td) {
  padding: 10px 12px;
  border-width: 0 0 1px;
  border-color: var(--ui-border);
  font-size: 14px;
}

:deep(.p-datatable-tbody > tr:last-child > td) {
  border-bottom-width: 0;
}

:deep(.p-datatable-tbody > tr:hover) {
  background: var(--ui-bg-panel-hover);
}

/* Строка открывает карточку клиента. */
:deep(.p-datatable-tbody > tr) {
  cursor: pointer;
}

/* Статусы — тонированные пилюли вместо сплошной заливки. */
:deep(.p-tag) {
  --tone: var(--ui-info);

  padding: 2px 8px;
  border-radius: var(--ui-radius-md);
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  background: color-mix(in oklab, var(--tone) 12%, var(--ui-bg));
  color: var(--tone);
}

:deep(.p-tag-danger) {
  --tone: var(--ui-error);
}

:deep(.p-tag-warn) {
  --tone: var(--ui-warning);
}

:deep(.p-tag-success) {
  --tone: var(--ui-success);
}

:deep(.p-tag-secondary) {
  --tone: var(--ui-text-muted);
}

:deep(.p-button) {
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--ui-primary);
  border-radius: var(--ui-radius-md);
  background: var(--ui-primary);
  color: var(--ui-bg);
  font-size: 14px;
  font-weight: 500;
}

:deep(.p-button:not(:disabled):hover) {
  background: var(--ui-primary-hover);
  border-color: var(--ui-primary-hover);
  color: var(--ui-bg);
}

:deep(.p-button.p-button-icon-only) {
  width: 32px;
  padding: 0;
}

:deep(.p-button.p-button-sm) {
  height: 32px;
  font-size: 14px;
}

:deep(.p-button.p-button-text),
:deep(.p-button.p-button-outlined),
:deep(.p-button.p-button-secondary) {
  background: transparent;
  border-color: transparent;
  color: var(--ui-text-muted);
}

:deep(.p-button.p-button-outlined),
:deep(.p-button.p-button-secondary) {
  border-color: var(--ui-border);
  background: var(--ui-bg);
  color: var(--ui-text);
}

:deep(.p-button.p-button-text:not(:disabled):hover),
:deep(.p-button.p-button-outlined:not(:disabled):hover),
:deep(.p-button.p-button-secondary:not(:disabled):hover) {
  background: var(--ui-bg-panel-hover);
  color: var(--ui-text);
}

:deep(.p-inputtext) {
  height: 32px;
  padding: 0 10px;
  background: var(--ui-bg);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  font-size: 14px;
  color: var(--ui-text);
}

:deep(.p-inputtext::placeholder) {
  color: var(--ui-text-dimmed);
}

:deep(.p-inputtext:enabled:focus) {
  border-color: var(--ui-primary);
  box-shadow: none;
  outline: none;
}

:deep(.p-message) {
  --tone: var(--ui-info);

  margin: 0;
  border: 1px solid color-mix(in oklab, var(--tone) 35%, transparent);
  border-radius: var(--ui-radius-lg);
  background: color-mix(in oklab, var(--tone) 12%, var(--ui-bg));
  color: var(--tone);
  font-size: 14px;
}

:deep(.p-message-error) {
  --tone: var(--ui-error);
}

:deep(.p-message-warn) {
  --tone: var(--ui-warning);
}

:deep(.p-message-success) {
  --tone: var(--ui-success);
}

:deep(.p-message-content) {
  padding: 10px 12px;
}

/* Узкий экран: логин и идентификатор — в карточке клиента, таблица и так
   прокручивается вбок из-за подробностей. */
@media (width <= 900px) {
  :deep(.wide-only) {
    display: none;
  }
}

/* Стили диалога переехали в глобальный блок App.vue: Dialog телепортируется
   в body и в scoped-стили не попадает. */
</style>
