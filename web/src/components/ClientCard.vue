<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import TabPanel from 'primevue/tabpanel'
import TabPanels from 'primevue/tabpanels'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import AnomalyAckDialog from './AnomalyAckDialog.vue'
import EpdAdviceNote from './EpdAdviceNote.vue'
import ForecastNote from './ForecastNote.vue'
import IndustryNote from './IndustryNote.vue'
import ItsContracts from './ItsContracts.vue'
import LicensesList from './LicensesList.vue'
import MonthBars, { type MonthBar } from './MonthBars.vue'
import { useRefreshable } from '../composables/useRefreshable'
import {
  api,
  type Anomaly,
  type ClientCard,
  type ClientRef,
  type Identifier,
  type ItsExpiring,
  type ItsSavedRequest,
} from '../api/client'
import {
  anomalyImportance,
  anomalyKindLabels,
  clientKey,
  contractName,
  epdBestText,
  forecastText,
  industryText,
  optionText,
  serviceName,
  validInn,
} from '../domain'
import { daysText, formatDate, money, monthLabel, moscowDate, reportDate, requisites, shortName, when } from '../format'
import { requestState, requestStateLabels, type RequestState } from '../requestFile'

/**
 * Карточка клиента — панель справа поверх списка Клиентов (маршрут client,
 * вложенный в clients). Три вкладки: «Данные» — всё, что о клиенте известно,
 * и откуда; «Проблемы» — поводы, продление и находки ЭДО с пометкой «это
 * нормально»; «Заявки» — его заявки и новая. Вкладка — в адресе (?section=).
 */
const emit = defineEmits<{ changed: []; title: [text: string] }>()

const route = useRoute()
const router = useRouter()

const key = computed(() => String(route.params.key ?? ''))
const card = ref<ClientCard | null>(null)

const { loading, refreshing, error, load } = useRefreshable(
  async () => {
    const found = await api.client(key.value)
    card.value = found
    // /clients/<ИНН> открывает единственный КПП: в адресе — полный ключ.
    if (found.key !== key.value) {
      void router.replace({ name: 'client', params: { key: found.key }, query: route.query })
    }
  },
  { failText: 'Не удалось загрузить карточку клиента.' },
)

// Переход к соседней организации — та же панель с другим ключом.
watch(key, (next) => {
  if (next && card.value?.key !== next) {
    card.value = null
    void load()
  }
})

const title = computed(() => {
  const c = card.value
  if (!c) return ''
  return shortName(c.clientName) || requisites(c)
})
watch(title, (text) => emit('title', text), { immediate: true })

type Section = 'data' | 'problems' | 'requests'
const sections: Section[] = ['data', 'problems', 'requests']

const section = computed<Section>({
  get: () => {
    const value = route.query.section
    return sections.includes(value as Section) ? (value as Section) : 'data'
  },
  set: (value) => {
    void router.replace({ query: { ...route.query, section: value === 'data' ? undefined : value } })
  },
})

function onSection(value: string | number) {
  section.value = value as Section
}

/** Сосед по ИНН или абоненту — та же панель, фильтры списка под ней сохраняются. */
function cardLink(org: ClientRef) {
  return { name: 'client', params: { key: org.key }, query: { ...route.query, section: undefined } }
}

const billingRows = computed(() => card.value?.billing.clients ?? [])
const billingDue = computed(() => billingRows.value.filter((row) => row.overLimit).map((row) => row.amount))

/** Самый поздний конец договора 1С:ИТС у абонентов клиента. */
const itsEnd = computed(() =>
  (card.value?.its ?? [])
    .flatMap((sub) => sub.contracts.map((contract) => contract.end))
    .filter(Boolean)
    .sort()
    .at(-1),
)

const sourceLabels: Record<string, string> = {
  registry: 'реестр ЭДО (биллинг партнёра)',
  traffic: 'отчёты трафика ЭДО',
  import: 'выгрузка биллинга ЭПД (CSV)',
  subscriber: 'база абонентов 1С',
  request: 'заявки',
}

interface Alert {
  tone: 'error' | 'warn' | 'info'
  text: string
}

/** Поводы клиента, кроме продления и находок: у тех свои блоки ниже. */
const alerts = computed<Alert[]>(() => {
  const c = card.value
  if (!c) return []
  const list: Alert[] = []
  for (const row of billingRows.value) {
    if (row.overLimit) {
      list.push({
        tone: 'error',
        text: `Выставить счёт: сверх лимита ${row.billable} пакетов, ${money(row.amount)} (${monthLabel(c.billing.period)})`,
      })
    } else if (row.lowRemainder) {
      list.push({ tone: 'warn', text: `Лимит кончается: ${row.used} из ${row.limit ?? '—'}` })
    }
  }
  for (const item of c.forecast?.clients ?? []) {
    if (item.warn) list.push({ tone: 'warn', text: `Прогноз: ${forecastText(item)}` })
  }
  for (const sub of c.its) {
    if (sub.industry?.missing.length) {
      list.push({ tone: 'warn', text: `ИТС Отраслевой: ${industryText(sub.industry)}` })
    }
  }
  for (const item of c.licensesLow) {
    list.push({
      tone: item.over ? 'error' : 'warn',
      text: `${serviceName(item.option.type)}, ${item.tariffName}: ${optionText(item.option)}`,
    })
  }
  if (c.advice && !c.advice.optimal) {
    list.push({ tone: 'info', text: `Выгоднее ${epdBestText(c.advice)}: экономия ${money(c.advice.savings)} в год` })
  }
  return list
})

function hidden(item: Anomaly) {
  return item.acknowledged || item.suppressed === true
}

const activeAnomalies = computed(() => (card.value?.anomalies ?? []).filter((item) => !hidden(item)))
const hiddenAnomalies = computed(() => (card.value?.anomalies ?? []).filter(hidden))
const reviewDue = computed(() => hiddenAnomalies.value.filter((item) => item.reviewDue).length)
const showHidden = ref(false)
const shownAnomalies = computed(() => (showHidden.value ? hiddenAnomalies.value : activeAnomalies.value))

const problemCount = computed(
  () => alerts.value.length + (card.value?.itsExpiring.length ?? 0) + activeAnomalies.value.length,
)

/** Заявку можно завести только на настоящий ИНН: нулевой робот 1С не примет. */
const canRequest = computed(() => validInn(card.value?.inn ?? ''))

function newRequest() {
  const c = card.value
  if (!c) return
  void router.push({ name: 'request-new', query: { inn: c.inn, kpp: c.kpp } })
}

function renew(renewal: { startDate: string; tariffCode: string } | null | undefined) {
  const c = card.value
  if (!c || !renewal) return
  void router.push({
    name: 'request-new',
    query: {
      inn: c.inn,
      kpp: c.kpp,
      start: renewal.startDate,
      ...(renewal.tariffCode ? { tariff: renewal.tariffCode } : {}),
    },
  })
}

function renewContract(item: ItsExpiring) {
  renew(item.renewal)
}

/** «Проверить в 1С» договоры ИТС: сервер сам не повторит проверку моложе 10 минут. */
const itsRefreshing = ref(false)
const itsError = ref('')

async function refreshIts() {
  itsRefreshing.value = true
  itsError.value = ''
  try {
    await api.refreshItsContracts()
    await load(true)
    emit('changed')
  } catch (err) {
    itsError.value = err instanceof Error ? err.message : 'Не удалось проверить договоры в 1С.'
  } finally {
    itsRefreshing.value = false
  }
}

/** Строка биллинга и прогноз идентификатора — рядом с ним, а не отдельной таблицей. */
function billingOf(item: Identifier) {
  return billingRows.value.find((row) => row.edoId === item.edoId)
}

function forecastOf(item: Identifier) {
  return card.value?.forecast?.clients.find((row) => row.edoId === item.edoId)
}

const monthStatusText: Record<string, string> = {
  pending: 'ещё не загружен',
  none: 'в 1С биллинга нет',
  failed: '1С не отдала отчёт',
}

function barsOf(item: Identifier): MonthBar[] {
  const history = card.value?.history
  const own = history?.clients.find((row) => row.edoId === item.edoId)
  if (!history || !own || own.packets.every((value) => value === null)) return []
  return history.months.map((month, index) => {
    const value = own.packets[index] ?? null
    const billable = own.billable[index] ?? 0
    const limit = own.limits[index]
    return {
      key: month.period,
      label: monthLabel(month.period),
      value,
      over: billable > 0,
      note:
        value === null
          ? (monthStatusText[month.status] ?? 'не было в отчёте')
          : [limit === null || limit === undefined ? 'без лимита' : `из ${limit}`, billable > 0 ? `сверх лимита ${billable}` : '']
              .filter(Boolean)
              .join(', '),
    }
  })
}

function limitOf(item: Identifier): number | null {
  const own = card.value?.history.clients.find((row) => row.edoId === item.edoId)
  return own?.limits.filter((value) => value !== null).at(-1) ?? null
}

function trafficText(item: Identifier): string {
  const t = item.traffic
  if (!t) return ''
  return `исходящие: СФ ${t.invoicesOut}, прочие ${t.nonInvoicesOut}, ЭПД ${t.epdOut}; ` +
    `входящие: СФ ${t.invoicesIn}, прочие ${t.nonInvoicesIn}, ЭПД ${t.epdIn}`
}

function trafficPeriod(item: Identifier): string {
  const t = item.traffic
  if (!t?.periodFrom) return 'за 12 мес.'
  if (t.periodFrom === t.periodTo) return `за ${monthLabel(t.periodFrom)}`
  return `за ${monthLabel(t.periodFrom)} — ${monthLabel(t.periodTo)}`
}

/** Проверка программ в 1С — как форма портала: по регномеру или по логину, только по нажатию. */
const programsLogin = ref('')
const programsRegNumber = ref('')
const checking = ref<'' | 'login' | 'regNumber'>('')
const checkError = ref('')

watch(
  () => card.value?.key,
  () => {
    const c = card.value
    programsLogin.value = c?.programsLogin || c?.logins[0] || ''
    programsRegNumber.value = c?.programs[0]?.regNumber ?? ''
    showHidden.value = false
  },
  { immediate: true },
)

async function checkPrograms(by: 'login' | 'regNumber') {
  const c = card.value
  if (!c) return
  checking.value = by
  checkError.value = ''
  try {
    await api.itsCheckPrograms({
      inn: c.inn,
      kpp: c.kpp,
      login: by === 'login' ? programsLogin.value.trim() : '',
      regNumber: by === 'regNumber' ? programsRegNumber.value.trim() : '',
    })
    await load(true)
  } catch (err) {
    checkError.value = err instanceof Error ? err.message : 'Не удалось проверить программы в 1С.'
  } finally {
    checking.value = ''
  }
}

/** Соседи по абоненту — кроме самой карточки. */
function neighbours(orgs: ClientRef[]): ClientRef[] {
  return orgs.filter((org) => org.key !== card.value?.key)
}

function anomalyState(item: Anomaly): string {
  if (item.suppressed) return `погашена топологией${item.topologyPurpose ? `: ${item.topologyPurpose}` : ''}`
  if (item.acknowledged) {
    const review = item.reviewAt ? `, пересмотр ${formatDate(item.reviewAt)}` : ''
    return `это нормально${item.ackReason ? `: ${item.ackReason}` : ''}${review}`
  }
  return ''
}

/** «Это нормально» и «Вернуть» — тем же диалогом, что в списке находок. */
const ackTarget = ref<Anomaly | null>(null)
const restoreBusy = ref(0)
const anomalyError = ref('')

async function acked() {
  ackTarget.value = null
  await load(true)
  emit('changed')
}

async function restore(item: Anomaly) {
  restoreBusy.value = item.id
  anomalyError.value = ''
  try {
    if (item.acknowledged) await api.unacknowledge(item.edoId, item.fingerprint)
    if (item.suppressed) await api.removeTopology({ inn: item.inn, kpp: item.kpp, edoId: item.edoId })
    await load(true)
    emit('changed')
  } catch (err) {
    anomalyError.value = err instanceof Error ? err.message : 'Не удалось вернуть находку.'
  } finally {
    restoreBusy.value = 0
  }
}

const stateSeverity: Record<RequestState, string> = {
  draft: 'secondary',
  exported: 'info',
  sent: 'success',
}

function openRequest(item: ItsSavedRequest) {
  void router.push({ name: 'request', params: { id: item.id } })
}

function duplicateRequest(item: ItsSavedRequest) {
  void router.push({ name: 'request-new', query: { from: item.id } })
}
</script>

<template>
  <div class="card">
    <Message
      v-if="error"
      severity="error"
      :closable="false"
    >
      {{ error }}
    </Message>
    <p
      v-else-if="loading && !card"
      class="calm"
    >
      Загружаем карточку…
    </p>

    <template v-if="card">
      <header class="head">
        <div class="head-text">
          <small>{{ requisites(card) }}<template v-if="card.clientName !== title"> · {{ card.clientName }}</template></small>
        </div>
        <div class="head-tools">
          <Button
            v-if="canRequest"
            label="Новая заявка"
            icon="pi pi-plus"
            size="small"
            @click="newRequest"
          />
          <Button
            v-if="canRequest && card.renewal"
            label="Продлить"
            icon="pi pi-replay"
            size="small"
            outlined
            :title="`Заявка на продление с ${card.renewal.startDate}`"
            @click="renew(card.renewal)"
          />
          <Button
            icon="pi pi-refresh"
            size="small"
            text
            aria-label="Обновить карточку"
            :loading="refreshing"
            @click="load(true)"
          />
        </div>
      </header>

      <Tabs
        :value="section"
        @update:value="onSection"
      >
        <TabList>
          <Tab value="data">
            Данные
          </Tab>
          <Tab value="problems">
            Проблемы <span
              v-if="problemCount"
              class="count"
              :class="{ alarm: alerts.some((a) => a.tone === 'error') || activeAnomalies.some((a) => a.confidence === 'high') }"
            >{{ problemCount }}</span>
          </Tab>
          <Tab value="requests">
            Заявки <span
              v-if="card.requests.length"
              class="count"
            >{{ card.requests.length }}</span>
          </Tab>
        </TabList>
        <TabPanels>
          <!-- Данные: всё о клиенте и откуда это известно. -->
          <TabPanel value="data">
            <dl class="facts">
              <dt>Абонент</dt>
              <dd>
                <template v-if="card.subscribers.length">
                  <span
                    v-for="sub in card.subscribers"
                    :key="sub.code"
                    class="line"
                  ><code>{{ sub.code }}</code><template v-if="sub.name"> — {{ sub.name }}</template>
                    <Tag
                      v-if="!sub.inBase"
                      severity="secondary"
                      :value="card.ours ? 'только 1С-ЭДО' : 'нет в базе'"
                    />
                    <Tag
                      v-if="sub.gone"
                      severity="warn"
                      value="пропал из 1С"
                    />
                  </span>
                </template>
                <template v-else>
                  не известен
                </template>
              </dd>
              <template v-if="card.logins.length">
                <dt>Логины</dt>
                <dd>
                  <span
                    v-for="login in card.logins"
                    :key="login"
                    class="line"
                  >{{ login }}</span>
                </dd>
              </template>
              <dt>Биллинг за {{ monthLabel(card.billing.period) }}</dt>
              <dd>
                <template v-if="billingRows.length">
                  {{ billingRows.reduce((sum, row) => sum + row.used, 0) }} пакетов
                  <template v-if="billingDue.length">
                    · к выставлению {{ billingDue.map(money).join(' + ') }}
                  </template>
                </template>
                <template v-else>
                  строк нет
                </template>
              </dd>
              <dt>Договор 1С:ИТС</dt>
              <dd>{{ itsEnd ? `до ${moscowDate(itsEnd)}` : 'не проверялся' }}</dd>
              <dt>Источники</dt>
              <dd>{{ card.sources.map((s) => sourceLabels[s] ?? s).join(', ') || '—' }}</dd>
            </dl>

            <h3>Идентификаторы ЭДО · {{ card.identifiers.length }}</h3>
            <p
              v-if="!card.identifiers.length"
              class="calm"
            >
              У клиента нет идентификаторов ЭДО в биллинге партнёра.
            </p>
            <article
              v-for="item in card.identifiers"
              :key="item.edoId"
              class="block"
            >
              <header class="block-head">
                <code>{{ item.edoId }}</code>
                <Tag
                  v-if="billingOf(item)?.overLimit"
                  severity="danger"
                  value="сверх лимита"
                />
                <Tag
                  v-else-if="billingOf(item)?.lowRemainder"
                  severity="warn"
                  value="остаток мал"
                />
                <Tag
                  v-else-if="!billingOf(item)"
                  severity="secondary"
                  value="нет в снимке"
                />
              </header>
              <dl class="facts">
                <template v-if="item.login">
                  <dt>Логин</dt>
                  <dd>{{ item.login }}</dd>
                </template>
                <dt>Владелец</dt>
                <dd>{{ item.owner || (item.packets > 0 ? 'не назначен — трафик без тарифа' : 'не назначен') }}</dd>
                <template v-if="item.tariffs">
                  <dt>Тарифы</dt>
                  <dd>{{ item.tariffs }}</dd>
                </template>
                <template v-if="billingOf(item)">
                  <dt>{{ monthLabel(card.billing.period) }}</dt>
                  <dd>
                    {{ billingOf(item)!.used }} {{ billingOf(item)!.limit === null ? '(лимита нет)' : `из ${billingOf(item)!.limit}` }}
                    <template v-if="billingOf(item)!.billable">
                      · сверх лимита {{ billingOf(item)!.billable }}, счёт {{ money(billingOf(item)!.amount) }}
                    </template>
                    · доход партнёра {{ money(billingOf(item)!.partnerAmount) }}
                  </dd>
                </template>
                <template v-if="forecastOf(item)">
                  <dt>Прогноз на {{ monthLabel(card.forecast?.period ?? '') }}</dt>
                  <dd><ForecastNote :item="forecastOf(item)!" /></dd>
                </template>
                <template v-if="item.traffic">
                  <dt v-if="item.traffic.operator">
                    Оператор
                  </dt>
                  <dd v-if="item.traffic.operator">
                    {{ item.traffic.operator }}
                  </dd>
                  <dt v-if="item.traffic.idRegistered || item.traffic.linkCreated">
                    Выдан / связь
                  </dt>
                  <dd v-if="item.traffic.idRegistered || item.traffic.linkCreated">
                    {{ reportDate(item.traffic.idRegistered) || '…' }} / {{ reportDate(item.traffic.linkCreated) || '…' }}
                  </dd>
                  <dt>Документы {{ trafficPeriod(item) }}</dt>
                  <dd>{{ trafficText(item) }}</dd>
                </template>
                <dt>В реестре</dt>
                <dd>{{ formatDate(item.firstSeen) }} — {{ formatDate(item.lastSeen) }}</dd>
              </dl>
              <div
                v-if="barsOf(item).length"
                class="trend"
              >
                <small>Пакеты по месяцам</small>
                <MonthBars
                  :bars="barsOf(item)"
                  :limit="limitOf(item)"
                  unit="пакетов"
                  :title="`Пакеты по месяцам: ${item.edoId}`"
                />
              </div>
            </article>

            <article
              v-if="card.advice"
              class="block"
            >
              <header class="block-head">
                <strong>Тариф ЭПД</strong>
              </header>
              <EpdAdviceNote :advice="card.advice" />
            </article>

            <h3>1С:ИТС и программы</h3>
            <article
              v-for="sub in card.its"
              :key="sub.code"
              class="block"
            >
              <header class="block-head">
                <strong>Договоры 1С:ИТС · {{ sub.code }}</strong>
              </header>
              <ItsContracts
                :subscriber="sub"
                checked
              />
              <IndustryNote
                :industry="sub.industry"
                label="ИТС Отраслевой"
              />
            </article>
            <p
              v-if="!card.its.length"
              class="calm"
            >
              Договоры 1С:ИТС по абонентам клиента не проверялись.
            </p>

            <article
              v-if="card.licenses.length"
              class="block"
            >
              <header class="block-head">
                <strong>Лицензии сервисов</strong>
              </header>
              <LicensesList :tariffs="card.licenses" />
            </article>

            <article class="block">
              <header class="block-head">
                <strong>Программы в Личном кабинете</strong>
              </header>
              <form
                class="check-form"
                @submit.prevent
              >
                <label for="programs-reg">По рег. номеру</label>
                <InputText
                  id="programs-reg"
                  v-model="programsRegNumber"
                  size="small"
                  inputmode="numeric"
                  autocomplete="off"
                />
                <Button
                  label="Проверить"
                  size="small"
                  :loading="checking === 'regNumber'"
                  :disabled="!canRequest || !!checking || !programsRegNumber.trim()"
                  @click="checkPrograms('regNumber')"
                />
                <label for="programs-login">По логину</label>
                <InputText
                  id="programs-login"
                  v-model="programsLogin"
                  size="small"
                  autocomplete="off"
                />
                <Button
                  label="Проверить"
                  size="small"
                  :loading="checking === 'login'"
                  :disabled="!canRequest || !!checking || !programsLogin.trim()"
                  @click="checkPrograms('login')"
                />
              </form>
              <Message
                v-if="checkError"
                severity="error"
                :closable="false"
              >
                {{ checkError }}
              </Message>
              <template v-if="card.programsCheckedAt">
                <small class="dim">Найдено записей: {{ card.programs.length }} · проверено {{ when(card.programsCheckedAt) }}</small>
                <div
                  v-if="card.programs.length"
                  class="programs-scroll"
                >
                  <table class="programs">
                    <thead>
                      <tr>
                        <th>Рег. номер</th>
                        <th>Название программы</th>
                        <th>Условия сопровождения</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="program in card.programs"
                        :key="`${program.regNumber}/${program.program}`"
                      >
                        <td>{{ program.regNumber }}</td>
                        <td>{{ program.program }}</td>
                        <td
                          v-if="program.hasAccess"
                          class="ok-cell"
                        >
                          Выполнены
                        </td>
                        <td
                          v-else
                          class="bad-cell"
                        >
                          Не выполнены{{ program.missing?.length ? `: ${program.missing.join(', ')}` : '' }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </template>
              <small
                v-else
                class="dim"
              >Ещё не проверялись.</small>
            </article>

            <template v-if="card.sameInn.length || card.subscribers.some((s) => neighbours(s.organizations).length || s.regNumbers.length)">
              <h3>Связанные организации</h3>
              <article
                v-if="card.sameInn.length"
                class="block"
              >
                <header class="block-head">
                  <strong>Тот же ИНН, другой КПП</strong>
                </header>
                <ul class="orgs">
                  <li
                    v-for="org in card.sameInn"
                    :key="org.key"
                  >
                    <RouterLink :to="cardLink(org)">
                      {{ shortName(org.clientName) || '—' }}
                    </RouterLink>
                    <small>{{ requisites(org) }}</small>
                  </li>
                </ul>
              </article>
              <template
                v-for="sub in card.subscribers"
                :key="`orgs/${sub.code}`"
              >
                <article
                  v-if="neighbours(sub.organizations).length || sub.regNumbers.length"
                  class="block"
                >
                  <header class="block-head">
                    <strong>Абонент {{ sub.code }}<template v-if="sub.name"> — {{ sub.name }}</template></strong>
                  </header>
                  <p
                    v-if="sub.regNumbers.length"
                    class="dim"
                  >
                    Регномера: {{ sub.regNumbers.join(', ') }}
                  </p>
                  <ul
                    v-if="neighbours(sub.organizations).length"
                    class="orgs"
                  >
                    <li
                      v-for="org in neighbours(sub.organizations)"
                      :key="org.key"
                    >
                      <RouterLink :to="cardLink(org)">
                        {{ shortName(org.clientName) || '—' }}
                      </RouterLink>
                      <small>{{ requisites(org) }}</small>
                    </li>
                  </ul>
                </article>
              </template>
            </template>
          </TabPanel>

          <!-- Проблемы: поводы, продление, находки ЭДО. -->
          <TabPanel value="problems">
            <ul
              v-if="alerts.length"
              class="alerts"
            >
              <li
                v-for="(alert, index) in alerts"
                :key="index"
                :class="alert.tone"
              >
                {{ alert.text }}
              </li>
            </ul>

            <section class="group">
              <header class="block-head">
                <h3>Продление 1С:ИТС</h3>
                <Button
                  label="Проверить в 1С"
                  icon="pi pi-refresh"
                  size="small"
                  outlined
                  :loading="itsRefreshing"
                  title="Проверить договоры 1С:ИТС в 1С заново (не чаще раза в 10 минут)"
                  @click="refreshIts"
                />
              </header>
              <small
                v-if="itsError"
                class="bad"
              >{{ itsError }}</small>
              <ul
                v-if="card.itsExpiring.length"
                class="plain rows"
              >
                <li
                  v-for="item in card.itsExpiring"
                  :key="`${item.subscriberCode}/${item.contract.name}/${item.contract.end}`"
                >
                  <span>
                    {{ contractName(item.contract) }} до {{ moscowDate(item.contract.end) }},
                    <span :class="item.daysLeft < 0 ? 'bad' : 'low'">{{ daysText(item.daysLeft) }}</span>
                  </span>
                  <Button
                    v-if="canRequest"
                    label="Продлить"
                    icon="pi pi-file-edit"
                    size="small"
                    @click="renewContract(item)"
                  />
                </li>
              </ul>
              <p
                v-else
                class="calm"
              >
                {{ itsEnd ? `Договор до ${moscowDate(itsEnd)}: продлевать пока рано.` : 'Договоры 1С:ИТС не проверялись.' }}
              </p>
            </section>

            <section class="group">
              <header class="block-head">
                <h3>Находки ЭДО</h3>
                <span class="seg">
                  <Button
                    :label="`Активные · ${activeAnomalies.length}`"
                    size="small"
                    :outlined="showHidden"
                    @click="showHidden = false"
                  />
                  <Button
                    :label="`Скрытые · ${hiddenAnomalies.length}`"
                    size="small"
                    :outlined="!showHidden"
                    @click="showHidden = true"
                  />
                </span>
              </header>
              <Message
                v-if="reviewDue && !showHidden"
                severity="warn"
                :closable="false"
              >
                Скрытых находок пора пересмотреть: {{ reviewDue }}. Пометки «это нормально» живут полгода.
              </Message>
              <Message
                v-if="anomalyError"
                severity="error"
                :closable="false"
              >
                {{ anomalyError }}
              </Message>
              <p
                v-if="!shownAnomalies.length"
                class="calm"
              >
                {{ showHidden ? 'Скрытых находок нет.' : 'Активных находок нет.' }}
              </p>
              <ul class="plain rows">
                <li
                  v-for="item in shownAnomalies"
                  :key="item.id"
                >
                  <div class="stack">
                    <span>
                      <Tag
                        :severity="item.confidence === 'high' ? 'danger' : 'warn'"
                        :value="anomalyImportance(item)"
                      />
                      <strong>{{ anomalyKindLabels[item.kind] ?? item.kind }}</strong>
                    </span>
                    <small>{{ item.details }}</small>
                    <small><code>{{ item.edoId }}</code> · найдена {{ formatDate(item.detectedAt) }}</small>
                    <small v-if="anomalyState(item)">{{ anomalyState(item) }}
                      <Tag
                        v-if="item.reviewDue"
                        severity="warn"
                        value="пора пересмотреть"
                      /></small>
                  </div>
                  <span class="row-tools">
                    <Button
                      v-if="!hidden(item)"
                      label="Это нормально"
                      size="small"
                      outlined
                      @click="ackTarget = item"
                    />
                    <Button
                      v-else
                      label="Вернуть"
                      size="small"
                      outlined
                      :loading="restoreBusy === item.id"
                      @click="restore(item)"
                    />
                  </span>
                </li>
              </ul>
            </section>
          </TabPanel>

          <!-- Заявки клиента и новая. -->
          <TabPanel value="requests">
            <div class="panel-tools">
              <Button
                v-if="canRequest"
                label="Новая заявка"
                icon="pi pi-plus"
                size="small"
                @click="newRequest"
              />
            </div>
            <p
              v-if="!card.requests.length"
              class="calm"
            >
              Заявок на клиента нет.
            </p>
            <ul class="plain rows">
              <li
                v-for="item in card.requests"
                :key="item.id"
              >
                <div class="stack">
                  <span>{{ item.title || `Заявка №${item.id}` }}</span>
                  <small>
                    <Tag
                      :severity="stateSeverity[requestState(item)]"
                      :value="requestStateLabels[requestState(item)]"
                    />
                    изменена {{ when(item.updatedAt) }}
                  </small>
                </div>
                <span class="row-tools">
                  <Button
                    label="Открыть"
                    size="small"
                    @click="openRequest(item)"
                  />
                  <Button
                    icon="pi pi-copy"
                    size="small"
                    outlined
                    aria-label="Дублировать"
                    title="Дублировать: новая заявка с теми же данными"
                    @click="duplicateRequest(item)"
                  />
                </span>
              </li>
            </ul>
          </TabPanel>
        </TabPanels>
      </Tabs>
      <p
        v-if="!canRequest"
        class="dim note"
      >
        ИНН клиента в 1С не заполнен (ключ {{ clientKey(card.inn, card.kpp) || card.key }}): заявку на него не завести.
      </p>
    </template>
    <p
      v-else-if="!loading && !error"
      class="calm"
    >
      Клиент не найден.
    </p>
    <AnomalyAckDialog
      :target="ackTarget"
      @close="ackTarget = null"
      @done="acked"
    />
  </div>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
.card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  color: var(--ui-text);
  font-size: 14px;
  line-height: 20px;
}

.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.head-text {
  min-width: 0;
  color: var(--ui-text-muted);
}

.head-tools,
.row-tools,
.seg {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.seg {
  gap: 4px;
}

h3 {
  margin: 16px 0 8px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

.block-head h3 {
  margin: 0;
}

.count {
  margin-left: 4px;
  padding: 0 6px;
  border-radius: 999px;
  background: var(--ui-bg-accented);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--ui-text);
}

.count.alarm {
  background: color-mix(in oklab, var(--ui-error) 25%, var(--ui-bg));
  color: var(--ui-error);
}

.calm,
.dim {
  margin: 0;
  color: var(--ui-text-muted);
}

.note {
  font-size: 12px;
}

.alerts,
.plain,
.orgs {
  margin: 0;
  padding: 0;
  list-style: none;
}

.alerts {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}

.alerts li {
  --tone: var(--ui-info);

  padding: 6px 12px;
  border: 1px solid color-mix(in oklab, var(--tone) 35%, transparent);
  border-radius: var(--ui-radius-lg);
  background: color-mix(in oklab, var(--tone) 12%, var(--ui-bg));
  color: var(--tone);
}

.alerts li.error {
  --tone: var(--ui-error);
}

.alerts li.warn {
  --tone: var(--ui-warning);
}

.group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 16px;
}

.facts {
  display: grid;
  grid-template-columns: minmax(8rem, max-content) 1fr;
  gap: 6px 16px;
  margin: 0;
}

.facts dt {
  color: var(--ui-text-muted);
}

.facts dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.block {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
  padding: 12px 16px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg-panel);
}

.block-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.block-head code {
  overflow-wrap: anywhere;
}

.trend small {
  color: var(--ui-text-muted);
}

.plain li,
.rows li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 0;
}

.rows li {
  border-bottom: 1px solid var(--ui-border);
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.stack small {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  color: var(--ui-text-muted);
  overflow-wrap: anywhere;
}

.stack strong {
  margin-left: 6px;
}

.orgs {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr));
  gap: 6px 16px;
}

.orgs li {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.orgs small {
  color: var(--ui-text-muted);
}

a {
  color: var(--ui-primary);
  text-decoration: none;
}

a:hover {
  text-decoration: underline;
}

code {
  font-size: 12px;
  color: var(--ui-text-muted);
}

.panel-tools {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.low {
  color: var(--ui-warning);
}

.bad {
  color: var(--ui-error);
}

/* Проверка условий сопровождения — как форма портала 1С. */
.check-form {
  display: grid;
  grid-template-columns: max-content minmax(0, 16rem) max-content;
  align-items: center;
  gap: 8px 12px;
  margin: 0;
}

.check-form :deep(.p-inputtext) {
  width: 100%;
  min-width: 0;
}

.programs-scroll {
  overflow-x: auto;
}

.programs {
  width: 100%;
  border-collapse: collapse;
}

.programs th,
.programs td {
  padding: 6px 10px;
  border: 1px solid var(--ui-border);
  text-align: left;
  vertical-align: top;
}

.programs th {
  color: var(--ui-text-muted);
  font-weight: 500;
  white-space: nowrap;
}

.programs td:first-child {
  white-space: nowrap;
}

.ok-cell {
  background: color-mix(in oklab, var(--ui-success) 18%, var(--ui-bg));
  color: var(--ui-success);
}

.bad-cell {
  background: color-mix(in oklab, var(--ui-error) 14%, var(--ui-bg));
  color: var(--ui-error);
}

:deep(.p-tablist-tab-list) {
  background: transparent;
  border-color: var(--ui-border);
}

:deep(.p-tab) {
  padding: 10px 14px;
  font-size: 14px;
  font-weight: 500;
  color: var(--ui-text-muted);
  white-space: nowrap;
}

:deep(.p-tab-active) {
  color: var(--ui-text-highlighted);
}

:deep(.p-tabpanels) {
  padding: 16px 0 0;
  background: transparent;
}

@media (width <= 600px) {
  .facts {
    grid-template-columns: 1fr;
    gap: 2px;
  }

  .facts dd {
    margin-bottom: 6px;
  }

  .rows li {
    flex-direction: column;
    align-items: flex-start;
  }

  .check-form {
    grid-template-columns: minmax(0, 1fr) max-content;
  }

  .check-form label {
    grid-column: 1 / -1;
  }
}
</style>
