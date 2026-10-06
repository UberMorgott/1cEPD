<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import ClientSummary from '../components/ClientSummary.vue'
import InfiniteTable from '../components/InfiniteTable.vue'
import PageHeader from '../components/PageHeader.vue'
import { useInfiniteRows } from '../composables/useInfiniteRows'
import { useRefreshable } from '../composables/useRefreshable'
import { api, type ItsSavedRequest } from '../api/client'
import { clientKey } from '../domain'
import { when } from '../format'
import {
  downloadRequestFile,
  requestFileName,
  requestState,
  requestStateLabels,
  type RequestState,
} from '../requestFile'

const router = useRouter()

const requests = ref<ItsSavedRequest[]>([])
const search = ref('')
type Filter = 'all' | RequestState
const filter = ref<Filter>('all')

const { loading, refreshing, error, refreshFailed, dataAsOf, load } = useRefreshable(
  async () => {
    requests.value = (await api.itsRequests()).requests ?? []
  },
  { failText: 'Не удалось загрузить заявки.' },
)

const counts = computed(() => {
  const result: Record<Filter, number> = { all: requests.value.length, draft: 0, exported: 0, sent: 0 }
  for (const item of requests.value) result[requestState(item)] += 1
  return result
})

const filters = computed<{ key: Filter; label: string }[]>(() => [
  { key: 'all', label: `Все · ${counts.value.all}` },
  { key: 'draft', label: `Черновики · ${counts.value.draft}` },
  { key: 'exported', label: `Выгружены · ${counts.value.exported}` },
  { key: 'sent', label: `Отправлены · ${counts.value.sent}` },
])

const visible = computed(() => {
  const query = search.value.trim().toLowerCase()
  return requests.value.filter((item) => {
    if (filter.value !== 'all' && requestState(item) !== filter.value) return false
    if (!query) return true
    return [item.title, item.companyName, item.inn, item.kpp].join(' ').toLowerCase().includes(query)
  })
})

/** Лента вместо страниц: заявок становится только больше. */
const feed = useInfiniteRows(() => visible.value, { resetOn: [search, filter] })

const stateSeverity: Record<RequestState, string> = {
  draft: 'secondary',
  exported: 'info',
  sent: 'success',
}

function open(item: ItsSavedRequest) {
  void router.push({ name: 'request', params: { id: item.id } })
}

/** Строка кликабельна целиком, как в биллинге; кнопки в ней работают сами. */
function onRowClick({ data }: { data: ItsSavedRequest }) {
  open(data)
}

function duplicate(item: ItsSavedRequest) {
  void router.push({ name: 'request-new', query: { from: item.id } })
}

/**
 * Скачивание прямо из списка. Пароли в черновик не сохраняются, а робот 1С
 * отвергнет файл без пароля у партнёра, которому пароль назначен. Поэтому
 * пароль спрашиваем здесь же: пусто — пароля нет, файл собирается как есть.
 */
const downloadTarget = ref<ItsSavedRequest | null>(null)
const downloadPassword = ref('')
const downloadError = ref('')
const downloading = ref(false)
const notice = ref('')

function askDownload(item: ItsSavedRequest) {
  downloadTarget.value = item
  downloadPassword.value = ''
  downloadError.value = ''
}

async function confirmDownload() {
  const target = downloadTarget.value
  if (!target) return
  downloading.value = true
  downloadError.value = ''
  try {
    const draft = await api.itsRequest(target.id)
    const request = { ...draft.request, id: draft.id, password: downloadPassword.value, newPassword: '' }
    const failed = await downloadRequestFile(request)
    if (failed) {
      downloadError.value = failed
      return
    }
    downloadTarget.value = null
    notice.value = `Файл ${requestFileName(request)} по заявке «${target.title}» собран и скачан. Письмо не отправлялось.`
    await load(true)
  } catch (err) {
    downloadError.value = err instanceof Error ? err.message : 'Не удалось собрать файл.'
  } finally {
    downloading.value = false
  }
}

/** Пароль нужен новый или пароль сменить — это делается в форме, а не в списке. */
function openForPassword() {
  const target = downloadTarget.value
  downloadTarget.value = null
  if (target) void router.push({ name: 'request', params: { id: target.id }, query: { download: '1' } })
}
</script>

<template>
  <section>
    <PageHeader
      v-model:search="search"
      placeholder="Название, клиент, ИНН"
      :refreshing="refreshing"
      :data-as-of="dataAsOf"
      :refresh-failed="refreshFailed"
      :error="error"
      @refresh="load(true)"
    >
      <template #tools>
        <Button
          label="Новая заявка"
          icon="pi pi-plus"
          size="small"
          @click="router.push({ name: 'request-new' })"
        />
      </template>
    </PageHeader>

    <Message
      v-if="notice"
      severity="success"
      :closable="false"
    >
      {{ notice }}
    </Message>

    <div class="chips">
      <Button
        v-for="item in filters"
        :key="item.key"
        :label="item.label"
        size="small"
        :outlined="filter !== item.key"
        @click="filter = item.key"
      />
    </div>

    <InfiniteTable
      :feed="feed"
      :loading="loading"
      data-key="id"
      class="requests"
      @row-click="onRowClick"
    >
      <template #empty>
        <span v-if="requests.length">По условию заявок нет.</span>
        <span v-else>Сохранённых заявок пока нет — начните с «Новой заявки».</span>
      </template>

      <Column
        field="number"
        header="№"
        sortable
        class="number-col"
      >
        <template #body="{ data }">
          {{ data.number || '—' }}
        </template>
      </Column>
      <Column
        field="createdAt"
        header="Дата"
        sortable
      >
        <template #body="{ data }">
          {{ data.createdAt ? new Date(data.createdAt).toLocaleDateString('ru-RU') : '—' }}
        </template>
      </Column>
      <Column
        field="title"
        header="Заявка"
        sortable
      >
        <template #body="{ data }">
          <div class="title">
            <span>{{ data.title }}</span>
            <small v-if="data.rows && data.rows > 1">строк: {{ data.rows }}</small>
          </div>
        </template>
      </Column>
      <Column
        field="companyName"
        header="Клиент"
        sortable
      >
        <template #body="{ data }">
          <ClientSummary
            v-if="data.companyName || data.inn"
            :name="data.companyName ?? ''"
            :inn="data.inn"
            :kpp="data.kpp"
            :to="clientKey(data.inn, data.kpp)"
          />
          <span v-else>—</span>
        </template>
      </Column>
      <Column
        header="Статус"
        style="width: 9rem"
      >
        <template #body="{ data }">
          <div class="state">
            <Tag
              :severity="stateSeverity[requestState(data)]"
              :value="requestStateLabels[requestState(data)]"
            />
            <small v-if="data.sentAt || data.exportedAt">{{ when(data.sentAt ?? data.exportedAt) }}</small>
          </div>
        </template>
      </Column>
      <Column
        field="updatedAt"
        header="Изменена"
        sortable
        class="wide-only"
        style="width: 9rem"
      >
        <template #body="{ data }">
          {{ when(data.updatedAt) }}
        </template>
      </Column>
      <Column class="tools-col">
        <template #body="{ data }">
          <div class="row-tools">
            <Button
              label="Открыть"
              size="small"
              class="open"
              @click="open(data)"
            />
            <Button
              icon="pi pi-copy"
              size="small"
              outlined
              aria-label="Дублировать"
              title="Дублировать: новая заявка с теми же данными"
              @click="duplicate(data)"
            />
            <Button
              icon="pi pi-download"
              size="small"
              outlined
              aria-label="Скачать файл"
              title="Скачать файл"
              @click="askDownload(data)"
            />
          </div>
        </template>
      </Column>
    </InfiniteTable>

    <Dialog
      :visible="downloadTarget !== null"
      modal
      header="Скачать файл заявки"
      :style="{ width: '28rem', maxWidth: 'calc(100vw - 32px)' }"
      @update:visible="downloadTarget = null"
    >
      <p class="hint">
        «{{ downloadTarget?.title }}». Пароли в черновиках не хранятся: если у партнёра
        назначен пароль заявки, введите его — без него робот 1С файл не примет.
        Пароля нет — оставьте поле пустым.
      </p>
      <label class="field">
        Пароль заявки
        <InputText
          v-model="downloadPassword"
          type="password"
          autocomplete="off"
          @keyup.enter="confirmDownload"
        />
      </label>
      <p
        v-if="downloadError"
        class="bad"
      >
        {{ downloadError }}
      </p>
      <template #footer>
        <Button
          label="Открыть в форме"
          text
          @click="openForPassword"
        />
        <Button
          label="Скачать"
          icon="pi pi-download"
          :loading="downloading"
          @click="confirmDownload"
        />
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
section {
  display: flex;
  flex-direction: column;
  gap: 24px;
  color: var(--ui-text);
  font-size: 14px;
  line-height: 20px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.title,
.state {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.title small,
.state small {
  color: var(--ui-text-muted);
}

.row-tools {
  display: flex;
  gap: 8px;
}

.hint {
  margin-top: 0;
  font-size: 13px;
  line-height: 18px;
  color: var(--ui-text-muted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  color: var(--ui-text-muted);
}

.bad {
  margin: 12px 0 0;
  color: var(--ui-error);
}

.requests :deep(.p-datatable-tbody > tr) {
  cursor: pointer;
}

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

:deep(.p-tag) {
  --tone: var(--ui-info);

  padding: 2px 8px;
  border-radius: var(--ui-radius-md);
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  white-space: nowrap;
  background: color-mix(in oklab, var(--tone) 12%, var(--ui-bg));
  color: var(--tone);
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

:deep(.p-button.p-button-text),
:deep(.p-button.p-button-outlined) {
  background: transparent;
  border-color: transparent;
  color: var(--ui-text-muted);
}

:deep(.p-button.p-button-outlined) {
  border-color: var(--ui-border);
  background: var(--ui-bg);
  color: var(--ui-text);
}

:deep(.p-button.p-button-text:not(:disabled):hover),
:deep(.p-button.p-button-outlined:not(:disabled):hover) {
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

:deep(.tools-col) {
  width: 11rem;
}

/* Узкий экран: даты прячем — статус уже говорит, что с заявкой. */
@media (width <= 900px) {
  :deep(.wide-only) {
    display: none;
  }

  /* Строка и так открывается нажатием: «Открыть» лишняя, копия — одной иконкой. */
  .row-tools {
    flex-direction: column;
  }

  .row-tools .open {
    display: none;
  }

  :deep(.p-datatable-thead > tr > th),
  :deep(.p-datatable-tbody > tr > td) {
    padding: 8px;
  }

  :deep(.tools-col) {
    width: auto;
  }
}
</style>
