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
import AnomalyAckDialog from '../components/AnomalyAckDialog.vue'
import EpdAdviceNote from '../components/EpdAdviceNote.vue'
import ForecastNote from '../components/ForecastNote.vue'
import IndustryNote from '../components/IndustryNote.vue'
import ItsContracts from '../components/ItsContracts.vue'
import LicensesList from '../components/LicensesList.vue'
import MonthBars, { type MonthBar } from '../components/MonthBars.vue'
import PageHeader from '../components/PageHeader.vue'
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

const route = useRoute()
const router = useRouter()

const key = computed(() => String(route.params.key ?? ''))
const card = ref<ClientCard | null>(null)

const { loading, refreshing, error, refreshFailed, dataAsOf, load } = useRefreshable(
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

// Переход к соседней организации — тот же экран с другим ключом.
watch(key, (next) => {
  if (next && card.value?.key !== next) {
    card.value = null
    void load()
  }
})

type TabKey = 'overview' | 'edo' | 'its' | 'anomalies' | 'requests' | 'other'
const tabKeys: TabKey[] = ['overview', 'edo', 'its', 'anomalies', 'requests', 'other']

/** Вкладка — в адресе: ссылку на «Находки клиента» можно переслать. */
const tab = computed<TabKey>({
  get: () => {
    const value = route.query.tab
    return tabKeys.includes(value as TabKey) ? (value as TabKey) : 'overview'
  },
  set: (value) => {
    void router.replace({ query: { ...route.query, tab: value === 'overview' ? undefined : value } })
  },
})

function onTab(value: string | number) {
  tab.value = value as TabKey
}

const activeAnomalies = computed(() =>
  (card.value?.anomalies ?? []).filter((item) => !item.acknowledged && !item.suppressed),
)

const billingRows = computed(() => card.value?.billing.clients ?? [])
const billingDue = computed(() =>
  billingRows.value.filter((row) => row.overLimit).map((row) => row.amount),
)

/** Самый поздний конец договора 1С:ИТС у абонентов клиента. */
const itsEnd = computed(() =>
  (card.value?.its ?? [])
    .flatMap((sub) => sub.contracts.map((contract) => contract.end))
    .filter(Boolean)
    .sort()
    .at(-1),
)

interface Alert {
  tone: 'error' | 'warn' | 'info'
  text: string
  tab: TabKey
}

/** Что сделать по клиенту: то же, что Сводка показывает по всем. */
const alerts = computed<Alert[]>(() => {
  const c = card.value
  if (!c) return []
  const list: Alert[] = []
  for (const row of billingRows.value) {
    if (row.overLimit) {
      list.push({
        tone: 'error',
        text: `Выставить счёт: сверх лимита ${row.billable} пакетов, ${money(row.amount)} (${monthLabel(c.billing.period)})`,
        tab: 'edo',
      })
    } else if (row.lowRemainder) {
      list.push({ tone: 'warn', text: `Лимит кончается: ${row.used} из ${row.limit ?? '—'}`, tab: 'edo' })
    }
  }
  for (const item of c.forecast?.clients ?? []) {
    if (item.warn) list.push({ tone: 'warn', text: `Прогноз: ${forecastText(item)}`, tab: 'edo' })
  }
  if (c.advice && !c.advice.optimal) {
    list.push({
      tone: 'info',
      text: `Выгоднее ${epdBestText(c.advice)}: экономия ${money(c.advice.savings)} в год`,
      tab: 'edo',
    })
  }
  for (const item of c.itsExpiring) {
    list.push({
      tone: item.daysLeft < 0 ? 'error' : 'warn',
      text: `${contractName(item.contract)} до ${moscowDate(item.contract.end)}, ${daysText(item.daysLeft)}`,
      tab: 'its',
    })
  }
  for (const sub of c.its) {
    if (sub.industry?.missing.length) {
      list.push({ tone: 'warn', text: `ИТС Отраслевой: ${industryText(sub.industry)}`, tab: 'its' })
    }
  }
  for (const item of c.licensesLow) {
    list.push({
      tone: item.over ? 'error' : 'warn',
      text: `${serviceName(item.option.type)}, ${item.tariffName}: ${optionText(item.option)}`,
      tab: 'its',
    })
  }
  if (activeAnomalies.value.length) {
    const urgent = activeAnomalies.value.filter((item) => item.confidence === 'high').length
    list.push({
      tone: urgent ? 'error' : 'warn',
      text: `Находки: ${activeAnomalies.value.length}${urgent ? `, срочных ${urgent}` : ''}`,
      tab: 'anomalies',
    })
  }
  return list
})

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

/** Абоненты с соседями или регномерами — на вкладке «Прочее». */
const otherSubscribers = computed(() =>
  (card.value?.subscribers ?? []).filter((item) => neighbours(item.organizations).length || item.regNumbers.length),
)

const anomalyQuery = computed(() => {
  const c = card.value
  if (!c) return ''
  return validInn(c.inn) ? c.inn : (c.identifiers[0]?.edoId ?? '')
})

function anomalyState(item: Anomaly): string {
  if (item.suppressed) return `погашена топологией${item.topologyPurpose ? `: ${item.topologyPurpose}` : ''}`
  if (item.acknowledged) return `это нормально${item.ackReason ? `: ${item.ackReason}` : ''}`
  return ''
}

/** «Это нормально» и «Вернуть» — прямо в карточке, тем же диалогом, что в «Находках». */
const ackTarget = ref<Anomaly | null>(null)
const restoreBusy = ref(0)
const anomalyError = ref('')

async function acked() {
  ackTarget.value = null
  await load(true)
}

async function restore(item: Anomaly) {
  restoreBusy.value = item.id
  anomalyError.value = ''
  try {
    if (item.acknowledged) await api.unacknowledge(item.edoId, item.fingerprint)
    if (item.suppressed) await api.removeTopology({ inn: item.inn, kpp: item.kpp, edoId: item.edoId })
    await load(true)
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

const title = computed(() => {
  const c = card.value
  if (!c) return ''
  return shortName(c.clientName) || requisites(c)
})
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
        <div
          v-if="card"
          class="head"
        >
          <strong :title="card.clientName">{{ title }}</strong>
          <small>{{ requisites(card) }}</small>
        </div>
        <span
          v-else-if="loading"
          class="meta"
        >Загружаем карточку…</span>
      </template>
      <template #tools>
        <template v-if="card">
          <Button
            v-if="canRequest"
            label="Заявка"
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
        </template>
      </template>
    </PageHeader>

    <template v-if="card">
      <div class="badges">
        <Tag
          v-for="sub in card.subscribers"
          :key="sub.code"
          :severity="sub.inBase ? (sub.gone ? 'warn' : 'info') : card.ours ? 'info' : 'secondary'"
          :value="
            sub.inBase
              ? `${sub.code}${sub.gone ? ' · пропал из 1С' : ''}`
              : card.ours
                ? `${sub.code} · 1С-ЭДО`
                : `${sub.code} · нет в базе`
          "
          :title="
            [
              sub.name,
              sub.inBase ? '' : 'Код владельца из нашего 1С-ЭДО, база абонентов 1С его не отдаёт',
            ]
              .filter(Boolean)
              .join(' — ')
          "
        />
        <Tag
          v-if="!card.subscribers.length"
          severity="warn"
          value="Абонент не известен"
        />
        <Tag
          v-if="!card.inBase && card.subscribers.length && card.subscribers.every((sub) => sub.inBase)"
          severity="warn"
          value="нет в базе абонентов"
        />
        <Tag
          v-if="!card.identifiers.length"
          severity="secondary"
          value="ЭДО в биллинге нет"
        />
        <span
          v-for="login in card.logins"
          :key="login"
          class="login"
        >{{ login }}</span>
      </div>

      <Tabs
        :value="tab"
        scrollable
        @update:value="onTab"
      >
        <TabList>
          <Tab value="overview">
            Обзор
          </Tab>
          <Tab value="edo">
            ЭДО и биллинг · {{ card.identifiers.length }}
          </Tab>
          <Tab value="its">
            1С:ИТС и программы
          </Tab>
          <Tab value="anomalies">
            Находки · {{ activeAnomalies.length }}
          </Tab>
          <Tab value="requests">
            Заявки · {{ card.requests.length }}
          </Tab>
          <Tab value="other">
            Прочее
          </Tab>
        </TabList>
        <TabPanels>
          <TabPanel value="overview">
            <ul
              v-if="alerts.length"
              class="alerts"
            >
              <li
                v-for="(alert, index) in alerts"
                :key="index"
                :class="alert.tone"
              >
                <span>{{ alert.text }}</span>
                <Button
                  label="Подробнее"
                  size="small"
                  text
                  @click="tab = alert.tab"
                />
              </li>
            </ul>
            <p
              v-else
              class="calm"
            >
              Срочного по клиенту нет.
            </p>

            <dl class="facts">
              <dt>Абонент</dt>
              <dd>
                <template v-if="card.subscribers.length">
                  <span
                    v-for="sub in card.subscribers"
                    :key="sub.code"
                    class="line"
                  >{{ sub.code }}<template v-if="sub.name"> — {{ sub.name }}</template>
                    <small v-if="neighbours(sub.organizations).length">
                      · ещё {{ neighbours(sub.organizations).length }} орг.
                    </small>
                  </span>
                </template>
                <template v-else>
                  не известен
                </template>
              </dd>
              <dt>Идентификаторы ЭДО</dt>
              <dd>
                <template v-if="card.identifiers.length">
                  <code
                    v-for="item in card.identifiers"
                    :key="item.edoId"
                    class="line"
                  >{{ item.edoId }}</code>
                </template>
                <template v-else>
                  нет
                </template>
              </dd>
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
              <dt>Программы</dt>
              <dd>
                {{ card.programs.length ? `${card.programs.length}, проверено ${when(card.programsCheckedAt)}` : 'не проверялись' }}
              </dd>
              <dt>Заявки</dt>
              <dd>{{ card.requests.length || 'нет' }}</dd>
            </dl>
          </TabPanel>

          <TabPanel value="edo">
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
          </TabPanel>

          <TabPanel value="its">
            <article
              v-if="card.itsExpiring.length"
              class="block"
            >
              <header class="block-head">
                <strong>Продление</strong>
              </header>
              <ul class="plain">
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
            </article>

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
                <small class="dim found">Найдено записей: {{ card.programs.length }}</small>
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
                <small class="dim">Проверено {{ when(card.programsCheckedAt) }}</small>
              </template>
              <small
                v-else
                class="dim"
              >Ещё не проверялись.</small>
            </article>

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
                  <RouterLink :to="{ name: 'client', params: { key: org.key } }">
                    {{ shortName(org.clientName) || '—' }}
                  </RouterLink>
                  <small>{{ requisites(org) }}</small>
                </li>
              </ul>
            </article>
          </TabPanel>

          <TabPanel value="anomalies">
            <div class="panel-tools">
              <RouterLink
                v-if="anomalyQuery"
                :to="{ name: 'clients', query: { show: 'anomalies', q: anomalyQuery } }"
              >
                Все находки списком
              </RouterLink>
            </div>
            <Message
              v-if="anomalyError"
              severity="error"
              :closable="false"
            >
              {{ anomalyError }}
            </Message>
            <p
              v-if="!card.anomalies.length"
              class="calm"
            >
              Находок по клиенту нет.
            </p>
            <ul class="plain rows">
              <li
                v-for="item in card.anomalies"
                :key="item.id"
                :class="{ muted: item.acknowledged || item.suppressed }"
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
                  <small v-if="anomalyState(item)">{{ anomalyState(item) }}</small>
                </div>
                <span class="row-tools">
                  <Button
                    v-if="!item.acknowledged && !item.suppressed"
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
          </TabPanel>

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
          <TabPanel value="other">
            <p
              v-if="!otherSubscribers.length"
              class="calm"
            >
              Других сведений нет.
            </p>
            <article
              v-for="sub in otherSubscribers"
              :key="`orgs/${sub.code}`"
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
                  <RouterLink :to="{ name: 'client', params: { key: org.key } }">
                    {{ shortName(org.clientName) || '—' }}
                  </RouterLink>
                  <small>{{ requisites(org) }}</small>
                </li>
              </ul>
            </article>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </template>
    <p
      v-else-if="!loading && !error"
      class="calm"
    >
      Клиент не найден.
    </p>
    <p
      v-if="card && !canRequest"
      class="dim page-note"
    >
      ИНН клиента в 1С не заполнен (ключ {{ clientKey(card.inn, card.kpp) || card.key }}): заявку на него не завести.
    </p>
    <AnomalyAckDialog
      :target="ackTarget"
      @close="ackTarget = null"
      @done="acked"
    />
  </section>
</template>

<style scoped>
section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  color: var(--ui-text);
  font-size: 14px;
  line-height: 20px;
}

.head {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.head strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ui-text-highlighted);
}

.head small,
.dim,
.login {
  color: var(--ui-text-muted);
}

/* Пояснение под вкладками стоит прямо на странице — своя плотная подложка,
   чтобы узор фона не ложился под текст. */
.page-note {
  padding: 8px 12px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg);
}

.badges {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.login {
  font-size: 12px;
  overflow-wrap: anywhere;
}

.calm {
  margin: 0;
  color: var(--ui-text-muted);
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
  margin-bottom: 16px;
}

.alerts li {
  --tone: var(--ui-info);

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
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

.facts {
  display: grid;
  grid-template-columns: minmax(9rem, max-content) 1fr;
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
  display: block;
}

.block {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
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

.rows li.muted {
  opacity: 0.6;
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
  grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
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

.panel-tools {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.row-tools {
  display: flex;
  gap: 8px;
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

.found {
  align-self: flex-end;
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

@media (width <= 480px) {
  .check-form {
    grid-template-columns: minmax(0, 1fr) max-content;
  }

  .check-form label {
    grid-column: 1 / -1;
  }
}

/* Вкладки — сплошная область цвета страницы: узор фона не ложится под текст
   вкладок и их содержимого. */
:deep(.p-tabs) {
  padding: 0 16px 16px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg);
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

:deep(.p-message-content) {
  padding: 10px 12px;
}

@media (width <= 900px) {
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

  /* «Подробнее» остаётся в строке повода, справа; длинный текст переносится сам. */
  .alerts li {
    flex-wrap: wrap;
  }

  .alerts li :deep(.p-button) {
    margin-left: auto;
  }
}
</style>
