<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink, useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Message from 'primevue/message'
import EpdAdviceNote from '../components/EpdAdviceNote.vue'
import IndustryNote from '../components/IndustryNote.vue'
import ItsContracts from '../components/ItsContracts.vue'
import PageHeader from '../components/PageHeader.vue'
import { clientKey as cardKey } from '../domain'
import { requisites, shortName, when } from '../format'
import {
  downloadRequestFile,
  requestFileName,
  requestState,
  requestStateLabels,
} from '../requestFile'
import { autoMainReg, autoRegCandidates, extraRegOptions, regOptions as regOptionList } from '../regChoice'
import {
  api,
  ApiError,
  type ItsSender,
  type ItsClientCard,
  type ItsClientProgram,
  type EpdAdvice,
  type ItsIssue,
  type ItsRequest,
  type ItsRow,
  type ItsSavedRequest,
  type ItsSubscriber,
  type ItsSuggestion,
  type ItsTariff,
} from '../api/client'

const tariffs = ref<ItsTariff[]>([])
const issues = ref<ItsIssue[]>([])
const notes = ref<ItsIssue[]>([])
const suggestions = ref<ItsSuggestion[]>([])
const blocking = ref(false)
const checking = ref(false)
/** Проверка ещё не проходила: пустую форму нельзя называть «заполненной верно». */
const validated = ref(false)
const sending = ref(false)
const error = ref('')
const notice = ref('')
/** Почта не настроена: к ошибке добавляем ссылку на экран настроек. */
const notConfigured = ref(false)

const title = ref('')
const savedId = ref(0)
const revision = ref(0)
const saving = ref(false)
/** Строка списка заявок для открытой заявки: статус и отметки выгрузки и отправки. */
const savedInfo = ref<ItsSavedRequest | null>(null)

/** «Дата начала» по умолчанию — первое число текущего месяца, ДД.ММ.ГГ. */
function firstOfMonth(now = new Date()): string {
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const year = String(now.getFullYear() % 100).padStart(2, '0')
  return `01.${month}.${year}`
}

function emptyRow(): ItsRow {
  return {
    tariffCode: '', regNumber: '', companyName: '', inn: '', kpp: '',
    workplaces: 1, activityType: '', director: '', responsible: '',
    postalCode: '', city: '', street: '', house: '', building: '', flat: '',
    phoneCode: '', phone: '', fax: '', email: '', startDate: firstOfMonth(),
    deliveryType: '0', distributorCode: '', ownerCode: '', login: '',
    operation: '0', refusalDate: '', refusalReason: '',
    extraRegNumbers: ['', '', '', ''],
  }
}

/** Черновики, сохранённые до появления новых полей, дополняем пустыми значениями. */
function fillRow(saved: ItsRow): ItsRow {
  const row = { ...emptyRow(), ...saved }
  row.extraRegNumbers = [0, 1, 2, 3].map((index) => row.extraRegNumbers?.[index] ?? '')
  return row
}

const form = ref<ItsRequest>({
  partnerCode: '',
  responsible: '',
  email: '',
  password: '',
  newPassword: '',
  rows: [emptyRow()],
})

const deliveryOptions = [
  { label: 'Самовывоз', value: '0' },
  { label: 'Через дистрибьютора', value: '1' },
]

const operationOptions = [
  { label: 'Новый договор или продление', value: '0' },
  { label: 'Отказ', value: '1' },
]

/** Причины отказа: коды справочника шаблона. */
const refusalReasons = [
  { label: '1 — перерыв по финансовым причинам клиента', value: '1' },
  { label: '2 — партнёр не планирует работать с клиентом', value: '2' },
  { label: '3 — клиент отказался от сопровождения', value: '3' },
  { label: '4 — фирма ликвидирована', value: '4' },
  { label: '5 — договор зарегистрирован ошибочно', value: '5' },
]

const row = computed(() => form.value.rows[0] as ItsRow)

/** Отправитель заявки из настроек; null — не загрузился, форма работает как раньше. */
const sender = ref<ItsSender | null>(null)

async function loadSender() {
  try {
    sender.value = await api.itsSender()
  } catch {
    sender.value = null
  }
}

/**
 * Шапка новой заявки — из настроек. Пустое значение настроек не стирает
 * код партнёра по умолчанию. «Новый пароль» — разовое действие, всегда пуст.
 */
function applySender() {
  const s = sender.value
  form.value.newPassword = ''
  if (!s) return
  form.value.partnerCode = s.partnerCode || form.value.partnerCode
  form.value.responsible = s.responsible
  form.value.email = s.email
  form.value.password = s.password
}

/** Следующий порядковый номер заявки: наибольший сохранённый плюс один. */
const nextNumber = ref(1)

async function loadNextNumber() {
  try {
    const list = (await api.itsRequests()).requests ?? []
    nextNumber.value = list.reduce((max, item) => Math.max(max, item.number ?? 0), 0) + 1
  } catch {
    // Без списка нумерация начинается с единицы: название всё равно можно поправить.
  }
}

/** Имя по умолчанию: порядковый номер и сегодняшняя дата — придумывать его не нужно. */
const defaultTitle = computed(
  () => `Заявка ${nextNumber.value} от ${new Date().toLocaleDateString('ru-RU')}`,
)

/**
 * Замечания по конкретному полю: показываются прямо под ним. У шапки и строки
 * есть одноимённые поля (responsible, email) — их разводит номер строки:
 * 0 — шапка, 1 — строка таблицы. Без номера — замечания с любой строки.
 */
function issuesFor(field: string, row?: number): ItsIssue[] {
  return issues.value.filter(
    (issue) => issue.field === field && (row === undefined || issue.row === row),
  )
}

/**
 * Цвет замечания: блокирующее — красное, предупреждение — жёлтое, «не
 * проверено» (1С не ответила) — приглушённое.
 */
function issueClass(issue: ItsIssue): string {
  if (issue.unchecked) return 'unchecked'
  return issue.blocking ? 'bad' : 'warn'
}

/** Справки из 1С к полю: продукт по регномеру, действующий договор. */
function notesFor(field: string): ItsIssue[] {
  return notes.value.filter((note) => note.field === field)
}

const suggestionLabels: Record<string, string> = {
  companyName: 'наименование', inn: 'ИНН', kpp: 'КПП', ownerCode: 'код владельца',
  regNumber: 'регномер', extraRegNumbers: 'регномера 1–4', startDate: 'дата начала',
}

/** Подсказки, которые ещё можно подставить: поле по-прежнему пустое. */
const pendingSuggestions = computed(() =>
  suggestions.value.filter((item) => {
    const target = form.value.rows[item.row - 1]
    if (!target) return false
    if (item.field === 'extraRegNumbers') return target.extraRegNumbers.every((value) => !value.trim())
    const value = target[item.field as keyof ItsRow]
    return typeof value === 'string' && !value.trim()
  }),
)

/** Подставляет значения из 1С только в пустые поля: введённое вручную не трогаем. */
function applySuggestions(): string[] {
  const applied: string[] = []
  for (const item of pendingSuggestions.value) {
    const target = form.value.rows[item.row - 1]
    if (!target) continue
    if (item.field === 'extraRegNumbers') {
      const values = item.value.split(',')
      target.extraRegNumbers = [0, 1, 2, 3].map((index) => values[index] ?? '')
    } else {
      ;(target as unknown as Record<string, string>)[item.field] = item.value
    }
    applied.push(suggestionLabels[item.field] ?? item.field)
  }
  return applied
}

/** Клиент строки: по нему подсказки 1С подставляются сами — один раз. */
function suggestionKey(): string {
  const current = row.value
  return [current.inn, current.ownerCode, current.regNumber].map((value) => value.trim()).join('|')
}
let autoFilledFor = ''

function fillFrom1C() {
  const applied = applySuggestions()
  if (applied.length) notice.value = `Из 1С подставлено: ${applied.join(', ')}.`
}

async function check() {
  checking.value = true
  error.value = ''
  try {
    const result = await api.itsValidate(form.value)
    issues.value = result.issues ?? []
    blocking.value = result.blocking
    notes.value = result.notes ?? []
    suggestions.value = result.suggestions ?? []
    validated.value = true
    // Сами подставляем один раз на клиента: если человек потом стёр поле,
    // значит, так и задумано, — дальше только по кнопке.
    const key = suggestionKey()
    if (pendingSuggestions.value.length && key !== autoFilledFor) {
      autoFilledFor = key
      fillFrom1C()
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось проверить заявку.'
  } finally {
    checking.value = false
  }
}

/**
 * Тело для скачивания и отправки: форма плюс номер сохранённой заявки. Без
 * номера сервер не знает, какую заявку отметить выгруженной или отправленной.
 */
function outgoing(): ItsRequest {
  return { ...form.value, id: savedId.value }
}

/** Отметки выгрузки и отправки открытой заявки — из списка заявок. */
async function loadSavedInfo() {
  if (!savedId.value) {
    savedInfo.value = null
    return
  }
  try {
    const list = (await api.itsRequests()).requests ?? []
    savedInfo.value = list.find((item) => item.id === savedId.value) ?? null
  } catch {
    // Без отметок форма работает как раньше.
  }
}

/**
 * Скачивание только собирает файл: письмо не уходит, поэтому и сообщение своё —
 * иначе на экране остаётся прошлое «Заявка отправлена».
 */
async function download() {
  notice.value = ''
  await check()
  if (blocking.value) return
  const request = outgoing()
  const failed = await downloadRequestFile(request)
  if (failed) {
    error.value = failed
    return
  }
  notice.value = `Файл ${requestFileName(request)} собран и скачан. Письмо не отправлялось.`
  await loadSavedInfo()
}

/**
 * Отправка — отдельный шаг: сначала проверка и сводка «кому и что уйдёт»,
 * письмо — только по кнопке в этом окне.
 */
const confirmOpen = ref(false)
/** Адресаты из настроек почты: письмо уходит им, а не на жёстко заданный адрес. */
const recipients = ref<string[]>([])

async function askSend() {
  await check()
  if (blocking.value) return
  try {
    recipients.value = (await api.settings()).mailTo ?? []
  } catch {
    recipients.value = []
  }
  confirmOpen.value = true
}

const tariffName = computed(
  () => tariffs.value.find((item) => item.code === row.value.tariffCode)?.name ?? (row.value.tariffCode || '—'),
)

/** Отправка письмом: то же тело, что у скачивания, только уходит на сервер почты. */
async function send() {
  sending.value = true
  error.value = ''
  notice.value = ''
  notConfigured.value = false
  try {
    const result = await api.itsSend(outgoing())
    notice.value = `Заявка отправлена ${when(result.sentAt)}.`
    confirmOpen.value = false
    await loadSavedInfo()
  } catch (err) {
    confirmOpen.value = false
    if (err instanceof ApiError && err.code === 'not_configured') {
      notConfigured.value = true
      error.value = err.message
    } else {
      error.value = err instanceof Error ? err.message : 'Не удалось отправить заявку.'
    }
  } finally {
    sending.value = false
  }
}

/**
 * Несохранённые правки: форма и название сравниваются со снимком, сделанным
 * после открытия или сохранения. Пароли не сохраняются вовсе — их не считаем.
 */
const baseline = ref('')
function formSnapshot(): string {
  return JSON.stringify({ title: title.value, form: { ...form.value, password: '', newPassword: '' } })
}
function markClean() {
  baseline.value = formSnapshot()
}
const dirty = computed(() => baseline.value !== '' && formSnapshot() !== baseline.value)

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('В заявке есть несохранённые изменения. Уйти без сохранения?')
})

function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) event.preventDefault()
}
window.addEventListener('beforeunload', beforeUnload)
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))

/**
 * Сохранение. id и revision запоминаем: второй раз заявка обновится,
 * а не создастся заново. Новая заявка получает свой адрес /requests/:id.
 */
async function save() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await api.itsSaveRequest({
      id: savedId.value,
      title: title.value.trim() || defaultTitle.value,
      revision: revision.value,
      request: form.value,
    })
    const created = savedId.value === 0
    savedId.value = result.id
    revision.value = result.revision
    markClean()
    notice.value = 'Черновик сохранён. Письмо не отправлялось.'
    if (created) await router.replace({ name: 'request', params: { id: result.id } })
    await Promise.all([loadSavedInfo(), loadClients(), loadNextNumber()])
  } catch (err) {
    // 409 stale: заявку успели изменить в другом окне. Форму не трогаем —
    // правки должны остаться у пользователя, а не пропасть молча.
    if (err instanceof ApiError && err.code === 'stale') {
      error.value =
        'Заявку изменили в другом окне — сохранение отменено. Ваши правки остались в форме: ' +
        'откройте заявку заново из списка заявок и перенесите их.'
    } else {
      error.value = err instanceof Error ? err.message : 'Не удалось сохранить заявку.'
    }
  } finally {
    saving.value = false
  }
}

/** Заявка из базы в форму. copy — «Дублировать»: данные те же, а заявка новая. */
async function openRequest(id: number, copy = false) {
  error.value = ''
  notice.value = ''
  try {
    const draft = await api.itsRequest(id)
    draft.request.rows = draft.request.rows.length ? draft.request.rows.map(fillRow) : [emptyRow()]
    form.value = { ...draft.request, id: undefined }
    // Копия — новая заявка: шапка из настроек. В сохранённой заявке пароля нет
    // (черновики его не хранят) — подставляем сохранённый в настройках.
    if (copy) applySender()
    else if (!form.value.password && sender.value) form.value.password = sender.value.password
    title.value = copy ? defaultTitle.value : draft.title
    savedId.value = copy ? 0 : draft.id
    revision.value = copy ? 0 : draft.revision
    innMatches.value = []
    checked.value = null
    syncClient()
    if (copy) notice.value = `Копия заявки «${draft.title}». Проверьте поля и сохраните.`
  } catch {
    error.value = 'Не удалось открыть заявку.'
  }
}

const passwordInput = ref<{ $el: HTMLInputElement } | null>(null)

/** Карточки клиентов: реестр ЭДО и прошлые заявки. Подбор идёт по ним локально. */
const clients = ref<ItsClientCard[]>([])
const innPattern = /^\d{10}$|^\d{12}$/

/** Выбранный клиент: к нему относятся его регномера и проверка программ в 1С. */
const currentClient = ref<ItsClientCard | null>(null)
const pickedKey = ref<string | null>(null)
/** Полный ИНН совпал у нескольких клиентов (разные КПП): список выбора сужен до них. */
const innMatches = ref<ItsClientCard[]>([])
const clientPicker = ref<{ show: (isFocus?: boolean) => void } | null>(null)

function clientKey(card: ItsClientCard): string {
  return `${card.row.inn}/${card.row.kpp}`
}

/** Карточка клиента строки: по ИНН и КПП из формы, пока ИНН настоящий. */
const rowCardKey = computed(() => cardKey(row.value.inn, row.value.kpp))

async function loadClients() {
  try {
    clients.value = (await api.itsClients()).clients ?? []
  } catch {
    // Без подбора форма работает как раньше: заполняется вручную.
    clients.value = []
  }
  syncClient()
}

/**
 * Регномера клиента с программами: основной из прошлой заявки, затем из
 * проверки в 1С (с выполненными условиями — первыми), затем прочие из заявки.
 */
function clientRegNumbers(
  row: Partial<ItsRow>,
  programs: ItsClientProgram[] = [],
): { value: string; label: string }[] {
  return regOptionList(programs, row.regNumber, row.extraRegNumbers)
}

function clientLabel(card: ItsClientCard): string {
  const kpp = card.row.kpp ? ` / ${card.row.kpp}` : ''
  return `${shortName(card.row.companyName) || 'Без названия'} — ИНН ${card.row.inn}${kpp}`
}

/** Строка выбора: поиск идёт по названию, ИНН/КПП, логину, регномерам и идентификаторам ЭДО. */
const clientOptions = computed(() =>
  (innMatches.value.length ? innMatches.value : clients.value).map((card) => ({
    key: clientKey(card),
    card,
    label: clientLabel(card),
    login: card.row.login || card.programsLogin || '',
    regNumbers: clientRegNumbers(card.row, card.programs).map((reg) => reg.value),
    edoIds: card.edoIds ?? [],
    search: [
      clientLabel(card),
      card.row.companyName,
      card.row.login,
      card.programsLogin,
      ...clientRegNumbers(card.row, card.programs).map((reg) => reg.value),
      ...(card.edoIds ?? []),
    ]
      .filter(Boolean)
      .join(' '),
  })),
)

/**
 * Подставляет в форму всё, что известно о клиенте.
 *
 * Правило: пустые поля и поля со значением по умолчанию заполняются молча.
 * Если данные клиента расходятся с тем, что уже введено, — спрашиваем:
 * «ОК» заменяет и их, «Отмена» заполняет только пустые.
 */
/** Поля строки, относящиеся к заявке, а не к клиенту: при смене клиента они остаются. */
const requestOnlyFields = new Set<keyof ItsRow>([
  'tariffCode', 'startDate', 'operation', 'refusalDate', 'refusalReason',
])

/**
 * Что форма берёт из карточки клиента сразу: регномера прошлой заявки.
 * Основной по программам клиента подставляет проверка в 1С после выбора
 * (autoFillReg); «Другие программы» только предлагаются, не заполняются.
 */
function clientValues(card: ItsClientCard): { main: string; extras: string[]; regs: string[] } {
  const regs = clientRegNumbers(card.row, card.programs).map((reg) => reg.value)
  const main = card.row.regNumber ?? ''
  const extras = card.row.extraRegNumbers ?? []
  return { main, extras: extras.slice(0, 4), regs }
}

/** Клиент, чьи данные последними подставлены в форму: при смене клиента их убираем. */
const formClient = ref<ItsClientCard | null>(null)

function applyClient(card: ItsClientCard) {
  const target = row.value
  const blank = emptyRow()
  const previous =
    formClient.value && clientKey(formClient.value) !== clientKey(card) ? formClient.value : null
  // Любой регномер прошлого клиента — его, даже выбранный из нескольких вручную.
  const previousRegs = previous ? clientValues(previous).regs : []
  const previousReg = (value: string) => (previousRegs.includes(value) ? value : undefined)
  const changes: { edited: boolean; set: () => void }[] = []
  const empty = (value: unknown) => value === undefined || value === null || value === '' || value === 0
  // fromPrevious — значение, которое в форму положил прошлый клиент. Оно не
  // «введено вручную»: при смене клиента заменяется и стирается без вопроса.
  const offer = (
    current: unknown,
    initial: unknown,
    next: unknown,
    fromPrevious: unknown,
    set: (value: unknown) => void,
  ) => {
    // У нового клиента поля нет: оставляем введённое, но не чужое.
    const value = empty(next) ? (previous ? initial : undefined) : next
    if (value === undefined || value === current) return
    const edited = current !== initial && !(previous && current === fromPrevious)
    changes.push({ edited, set: () => set(value) })
  }

  const { main, extras } = clientValues(card)

  for (const key of Object.keys(blank) as (keyof ItsRow)[]) {
    if (key === 'extraRegNumbers') continue
    if (requestOnlyFields.has(key) && empty(card.row[key])) continue
    const next = key === 'regNumber' ? main : card.row[key]
    const old = key === 'regNumber' ? previousReg(target.regNumber) : previous?.row[key]
    offer(target[key], blank[key], next, old, (value) => {
      ;(target as unknown as Record<string, unknown>)[key] = value
    })
  }
  blank.extraRegNumbers.forEach((_, index) => {
    const current = target.extraRegNumbers[index] ?? ''
    offer(current, '', extras[index], previousReg(current), (value) => {
      target.extraRegNumbers[index] = String(value)
    })
  })
  // Ответственный и почта в шапке — контакт партнёра, а не клиента: их задают
  // настройки «Отправитель заявки ИТС». Шапка прошлой заявки заполняет только
  // пустые поля — когда настройки не заполнены.
  if (empty(form.value.responsible) && !empty(card.responsible)) {
    changes.push({ edited: false, set: () => { form.value.responsible = String(card.responsible) } })
  }
  if (empty(form.value.email) && !empty(card.email)) {
    changes.push({ edited: false, set: () => { form.value.email = String(card.email) } })
  }

  const name = card.row.companyName || card.row.inn
  const edited = changes.filter((change) => change.edited).length
  const overwrite =
    edited === 0 ||
    window.confirm(
      `У клиента «${name}» другие данные в уже заполненных полях (${edited}). Заменить их?\n` +
        '«Отмена» заполнит только пустые поля.',
    )
  const chosen = overwrite ? changes : changes.filter((change) => !change.edited)
  chosen.forEach((change) => change.set())

  currentClient.value = card
  formClient.value = card
  pickedKey.value = clientKey(card)
  innMatches.value = []
  checked.value = null

  error.value = ''
  notice.value = chosen.length
    ? `Подставлены данные клиента «${name}», полей: ${chosen.length}.`
    : `Данные клиента «${name}» уже в форме.`
  void autoFillReg(card)
}

/** Выбор в списке клиентов; очистка списка отвязывает форму от клиента. */
function onClientPicked(key: string | null) {
  const card = clientOptions.value.find((option) => option.key === key)?.card
  if (card) {
    applyClient(card)
    return
  }
  currentClient.value = null
  innMatches.value = []
  checked.value = null
  regHint.value = ''
}

/**
 * Ввод ИНН: полный ИНН одного известного клиента заполняет форму сразу, а при
 * нескольких (разные КПП) открывается список выбора только с ними.
 */
function lookupByInn(value: string | undefined) {
  const inn = (value ?? '').trim()
  if (currentClient.value && currentClient.value.row.inn !== inn) {
    currentClient.value = null
    pickedKey.value = null
    checked.value = null
  }
  innMatches.value = []
  if (!innPattern.test(inn)) return
  const found = clients.value.filter((card) => card.row.inn === inn)
  if (found.length === 1) {
    applyClient(found[0] as ItsClientCard)
  } else if (found.length > 1) {
    innMatches.value = found
    notice.value = `С ИНН ${inn} известно клиентов: ${found.length} (разные КПП) — выберите нужного в списке.`
    clientPicker.value?.show(true)
  }
}

/** Открытая заявка или обновлённый список: находим карточку клиента из формы, ничего не подставляя. */
function syncClient() {
  const inn = row.value.inn.trim()
  const kpp = row.value.kpp.trim()
  const card = inn ? clients.value.find((c) => c.row.inn === inn && c.row.kpp === kpp) : undefined
  currentClient.value = card ?? null
  formClient.value = card ?? null
  pickedKey.value = card ? clientKey(card) : null
}

/** Регномера выбранного клиента: из них выбирается основной. */
const regOptions = computed(() =>
  currentClient.value || checked.value
    ? clientRegNumbers(currentClient.value?.row ?? {}, programs.value)
    : [],
)

/** «Другие программы»: предлагаются остальные регномера клиента, кроме основного. */
const extraOptions = computed(() => extraRegOptions(regOptions.value, row.value.regNumber))

/**
 * Проверки программ за сессию по логину клиента: повторный выбор того же
 * клиента не ходит в 1С снова. Неудачная проверка из кэша убирается.
 */
const programsCache = new Map<string, Promise<{ programs: ItsClientProgram[]; checkedAt: string }>>()

function programsCacheKey(inn: string, login: string): string {
  return `${inn.trim()}|${login.trim().toLowerCase()}`
}

/** Клиент, для которого сейчас идёт проверка программ после выбора. */
const autoCheckingKey = ref<string | null>(null)
const autoChecking = computed(() => autoCheckingKey.value !== null && autoCheckingKey.value === pickedKey.value)
/** Тихая подсказка под регномером, если проверка не удалась. */
const regHint = ref('')

/**
 * После выбора клиента: проверка программ в 1С по его логину (без логина или
 * при сбое — сохранённый результат) и регномер из его программ. Введённое
 * человеком не трогается; несколько регномеров — выбор в списке, первый уже стоит.
 */
async function autoFillReg(card: ItsClientCard) {
  const key = clientKey(card)
  const login = row.value.login.trim() || (card.programsLogin ?? '').trim()
  regHint.value = ''
  if (login) {
    const cacheKey = programsCacheKey(card.row.inn, login)
    let pending = programsCache.get(cacheKey)
    if (!pending) {
      pending = api.itsCheckPrograms({ inn: card.row.inn, kpp: card.row.kpp, login, regNumber: '' })
      programsCache.set(cacheKey, pending)
      pending.catch(() => programsCache.delete(cacheKey))
    }
    autoCheckingKey.value = key
    try {
      const result = await pending
      // Результат сохранён сервером в карточке клиента; держим форму в согласии с ним.
      card.programs = result.programs ?? []
      card.programsLogin = login
      card.programsCheckedAt = result.checkedAt
    } catch {
      if (pickedKey.value === key) {
        regHint.value = card.programs?.length
          ? 'Проверить программы в 1С не удалось — регномера по прошлой проверке.'
          : 'Проверить программы в 1С не удалось — регномер можно ввести вручную.'
      }
    } finally {
      if (autoCheckingKey.value === key) autoCheckingKey.value = null
    }
  }
  // Пока шла проверка, выбрали другого клиента — его регномер не наш.
  if (pickedKey.value !== key) return
  checked.value = null
  const next = autoMainReg(row.value.regNumber, autoRegCandidates(card.programs ?? []))
  if (next !== row.value.regNumber) row.value.regNumber = next
}

/** Результат проверки, не попавший в карточку: клиента ещё нет в базе. */
const checked = ref<{ programs: ItsClientProgram[]; checkedAt: string } | null>(null)
const checkingPrograms = ref(false)

const programs = computed(() => checked.value?.programs ?? currentClient.value?.programs ?? [])
const programsCheckedAt = computed(
  () => checked.value?.checkedAt ?? currentClient.value?.programsCheckedAt,
)

function conditionText(program: ItsClientProgram): string {
  if (program.hasAccess) return 'Выполнены'
  return program.missing?.length ? `Не выполнены: ${program.missing.join('; ')}` : 'Не выполнены'
}

/**
 * «Проверить в 1С» — то же, что страница портала «Проверка условий
 * сопровождения», но через партнёрский API: по логину, без него — по регномеру.
 */
async function checkPrograms() {
  checkingPrograms.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await api.itsCheckPrograms({
      inn: row.value.inn.trim(),
      kpp: row.value.kpp.trim(),
      login: row.value.login.trim(),
      regNumber: row.value.regNumber.trim(),
    })
    checked.value = { programs: result.programs ?? [], checkedAt: result.checkedAt }
    if (row.value.login.trim()) {
      programsCache.set(programsCacheKey(row.value.inn, row.value.login), Promise.resolve(result))
    }
    await loadClients()
    // Карточка нашлась — показываем её, в ней те же программы.
    if (currentClient.value?.programs?.length) checked.value = null
    const regs = [...new Set(programs.value.map((program) => program.regNumber).filter(Boolean))]
    if (!row.value.regNumber && regs.length === 1) {
      row.value.regNumber = regs[0] ?? ''
    }
    notice.value = programs.value.length
      ? `Программ в Личном кабинете: ${programs.value.length}.` +
        (!row.value.regNumber && regs.length > 1 ? ' Выберите основной регномер.' : '')
      : 'В Личном кабинете клиента программ не найдено.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось проверить программы в 1С.'
  } finally {
    checkingPrograms.value = false
  }
}

/** Договоры 1С:ИТС по коду абонента: показываются у выбранного клиента. */
const itsSubscribers = ref<ItsSubscriber[]>([])

async function loadIts() {
  try {
    itsSubscribers.value = (await api.itsContracts(30)).subscribers ?? []
  } catch {
    // Без договоров форма работает как раньше.
    itsSubscribers.value = []
  }
}

const clientIts = computed(() => {
  const code = (currentClient.value?.row.ownerCode ?? row.value.ownerCode).trim()
  return code ? itsSubscribers.value.find((item) => item.code === code) : undefined
})

/** Подсказки тарифа ЭПД по годовому расходу: показываются под выбором тарифа. */
const epdAdvice = ref<EpdAdvice[]>([])

async function loadEpdAdvice() {
  try {
    epdAdvice.value = (await api.epdAdvice()).clients ?? []
  } catch {
    epdAdvice.value = []
  }
}

const formAdvice = computed(() => {
  const inn = row.value.inn.trim()
  const kpp = row.value.kpp.trim()
  return inn ? epdAdvice.value.find((item) => item.inn === inn && item.kpp === kpp) : undefined
})

const route = useRoute()
const router = useRouter()

/**
 * «Заявка на продление» с экрана биллинга приходит ссылкой ?inn&kpp&start&tariff:
 * клиент подставляется тем же подбором, что и вручную, дата начала и тариф —
 * поверх. Параметры потом убираем, чтобы обновление страницы не заполняло снова.
 */
function applyRenewalQuery() {
  const text = (value: unknown) => (typeof value === 'string' ? value.trim() : '')
  const inn = text(route.query.inn)
  if (!inn) return
  const kpp = text(route.query.kpp)
  const card = clients.value.find((c) => c.row.inn === inn && c.row.kpp === kpp)
  if (card) {
    applyClient(card)
  } else {
    row.value.inn = inn
    row.value.kpp = kpp
  }
  row.value.operation = '0'
  const start = text(route.query.start)
  if (start) row.value.startDate = start
  const tariff = text(route.query.tariff)
  if (tariff && tariffs.value.some((item) => item.code === tariff)) row.value.tariffCode = tariff
  // Из карточки клиента «Заявка» приходит без даты начала: это не продление.
  notice.value =
    `${start ? 'Черновик продления' : 'Новая заявка'}: ${card ? `клиент «${card.row.companyName || inn}»` : `ИНН ${inn}`}` +
    (start ? `, начало ${start}` : '') +
    (row.value.tariffCode ? '' : '. Выберите тариф') +
    '. Проверьте поля и сохраните.'
  void router.replace({ query: {} })
}

/**
 * Значения новой заявки: настоящие значения полей, а не подсказки — проверка
 * их видит, а выбор клиента не трогает.
 */
function applyNewDefaults() {
  applySender()
  title.value = defaultTitle.value
}

/** Пустая форма: переход с открытой заявки на «Новую заявку» в том же экране. */
function resetForm() {
  form.value = { ...form.value, password: '', newPassword: '', rows: [emptyRow()] }
  title.value = ''
  savedId.value = 0
  revision.value = 0
  savedInfo.value = null
  currentClient.value = null
  formClient.value = null
  pickedKey.value = null
  innMatches.value = []
  checked.value = null
  error.value = ''
  notice.value = ''
}

/**
 * Адрес решает, что в форме: /requests/:id — сохранённая заявка,
 * /requests/new — новая, в том числе копия (?from=id) или продление (?inn&kpp…).
 * ?download=1 — пришли из списка скачать файл заявки, которой нужен пароль.
 */
async function loadRoute() {
  const id = Number(route.params.id)
  if (route.name === 'request' && id) {
    if (id !== savedId.value) {
      await openRequest(id)
      markClean()
    }
    if (route.query.download) {
      if (!form.value.password) {
        notice.value =
          'Пароли в черновиках не хранятся: введите пароль заявки и нажмите «Скачать файл».'
        await nextTick()
        passwordInput.value?.$el.focus()
      }
      void router.replace({ query: {} })
    }
  } else if (route.name === 'request-new') {
    // Снимок «без правок» — пустая форма: продление и копия сразу считаются правкой.
    if (savedId.value !== 0 || !baseline.value) {
      if (savedId.value !== 0) resetForm()
      applyNewDefaults()
      markClean()
    }
    const from = Number(route.query.from)
    if (from) {
      await openRequest(from, true)
      void router.replace({ query: {} })
    } else {
      applyRenewalQuery()
    }
  }
  await loadSavedInfo()
}

onMounted(async () => {
  try {
    tariffs.value = (await api.itsTariffs()).tariffs ?? []
  } catch {
    error.value = 'Не удалось загрузить справочник тарифов.'
  }
  await Promise.all([loadClients(), loadIts(), loadEpdAdvice(), loadSender(), loadNextNumber()])
  await loadRoute()
  // Экран тот же для /requests/new и /requests/:id: смену адреса отрабатываем сами.
  watch(() => [route.name, route.params.id], () => void loadRoute())
})

// Проверяем на лету, но не на каждое нажатие клавиши.
let timer: number | undefined
watch(
  form,
  () => {
    window.clearTimeout(timer)
    timer = window.setTimeout(check, 600)
  },
  { deep: true },
)
</script>

<template>
  <section>
    <PageHeader
      :error="error"
      :refreshable="false"
    >
      <template #meta>
        <div class="meta heading">
          <RouterLink
            to="/requests"
            class="back"
          >
            <i class="pi pi-arrow-left" />
            Все заявки
          </RouterLink>
          <span>
            <template v-if="savedId">
              {{ title || defaultTitle }} · {{ requestStateLabels[requestState(savedInfo ?? {})] }}
              <template v-if="savedInfo?.sentAt">{{ when(savedInfo.sentAt) }}</template>
              <template v-else-if="savedInfo?.exportedAt">{{ when(savedInfo.exportedAt) }}</template>
            </template>
            <template v-else>Новая заявка на регистрацию тарифа ЭПД</template>
            <template v-if="dirty"> · есть несохранённые изменения</template>
          </span>
        </div>
      </template>
      <template #actions>
        <Button
          label="Сохранить"
          icon="pi pi-save"
          :disabled="saving"
          @click="save"
        />
        <Button
          label="Скачать файл"
          icon="pi pi-download"
          :disabled="!validated || blocking || checking"
          @click="download"
        />
        <Button
          label="Отправить…"
          icon="pi pi-send"
          :loading="sending"
          :disabled="!validated || blocking || checking || sending"
          @click="askSend"
        />
      </template>
      <template #error>
        <RouterLink
          v-if="notConfigured"
          to="/settings"
        >
          Открыть настройки
        </RouterLink>
      </template>
    </PageHeader>

    <Message
      v-if="notice"
      severity="success"
      :closable="false"
    >
      {{ notice }}
    </Message>

    <Message
      v-if="blocking"
      severity="error"
      :closable="false"
    >
      Заявку выпускать нельзя: исправьте отмеченные поля.
    </Message>
    <Message
      v-else-if="issues.some((issue) => !issue.unchecked)"
      severity="warn"
      :closable="false"
    >
      Файл выпустить можно, но сверка с 1С нашла расхождения — посмотрите жёлтые замечания,
      иначе робот 1С может заявку не принять.
    </Message>
    <Message
      v-else-if="validated && issues.length === 0 && !checking"
      severity="success"
      :closable="false"
    >
      Заявка заполнена верно. Кнопка «Отправить…» покажет сводку и пошлёт файл на itsrobot@1c.ru, протокол
      придёт с autoits@1c.ru на указанный e-mail.
    </Message>

    <!-- Данные 1С для пустых полей, которые человек очистил после автоподстановки. -->
    <div
      v-if="pendingSuggestions.length"
      class="suggest"
    >
      <span>
        В 1С есть: {{ pendingSuggestions.map((item) => suggestionLabels[item.field] ?? item.field).join(', ') }}.
      </span>
      <Button
        label="Подставить из 1С"
        icon="pi pi-download"
        size="small"
        @click="fillFrom1C"
      />
    </div>

    <!-- Замечания обо всей заявке: ни одно поле формы их не показывает. -->
    <p
      v-for="issue in issuesFor('rows')"
      :key="issue.message"
      :class="issueClass(issue)"
      class="banner"
    >
      {{ issue.message }}
    </p>

    <div class="form">
      <fieldset class="picker">
        <legend>Клиент из базы</legend>
        <!-- Не <label>: щелчок по label переотправляется в Select и тут же закрывает список. -->
        <div class="field">
          <span id="client-picker-label">Работаем со старым клиентом — выберите его, и все известные данные попадут в заявку</span>
          <Select
            ref="clientPicker"
            v-model="pickedKey"
            aria-labelledby="client-picker-label"
            :options="clientOptions"
            option-label="label"
            option-value="key"
            filter
            auto-filter-focus
            :filter-fields="['search']"
            filter-placeholder="Название, ИНН, регномер, логин или идентификатор ЭДО"
            show-clear
            :placeholder="clients.length ? 'Выберите клиента' : 'Сохранённых клиентов пока нет'"
            empty-filter-message="Клиент не найден"
            @update:model-value="onClientPicked"
          >
            <template #option="{ option }">
              <div class="client-option">
                <span>{{ option.label }}</span>
                <small v-if="option.login || option.regNumbers.length">
                  <template v-if="option.login">Логин {{ option.login }}</template>
                  <template v-if="option.login && option.regNumbers.length"> · </template>
                  <template v-if="option.regNumbers.length">
                    Рег. номер {{ option.regNumbers.join(', ') }}
                  </template>
                </small>
                <small v-if="option.edoIds.length">
                  Идентификатор ЭДО {{ option.edoIds.join(', ') }}
                </small>
              </div>
            </template>
          </Select>
        </div>
        <p
          v-if="innMatches.length"
          class="fixed"
        >
          В списке только клиенты с ИНН {{ innMatches[0]?.row.inn }}: выберите нужный КПП.
        </p>
        <RouterLink
          v-if="rowCardKey"
          class="card-link"
          :to="{ name: 'client', params: { key: rowCardKey } }"
        >
          <i class="pi pi-id-card" /> Карточка клиента
        </RouterLink>
      </fieldset>

      <fieldset>
        <legend>Отправитель</legend>
        <label>Название заявки
          <InputText
            v-model="title"
            :placeholder="defaultTitle"
          />
        </label>
        <label>Код партнёра
          <InputText v-model="form.partnerCode" />
          <small
            v-for="issue in issuesFor('partnerCode')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>Ответственный
          <InputText v-model="form.responsible" />
          <small
            v-for="issue in issuesFor('responsible', 0)"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>E-mail для протокола
          <InputText v-model="form.email" />
          <small
            v-for="issue in issuesFor('email', 0)"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>Пароль заявки
          <InputText
            ref="passwordInput"
            v-model="form.password"
            type="password"
            autocomplete="off"
          />
          <small
            v-for="issue in issuesFor('password')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>Новый пароль
          <InputText
            v-model="form.newPassword"
            type="password"
            autocomplete="off"
          />
          <small
            v-for="issue in issuesFor('newPassword')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <p class="fixed">
          Пароль подтверждает подлинность заявки — это не пароль Портала и не пароль API.
          По умолчанию его нет: чтобы назначить, заполните только «Новый пароль».
        </p>
      </fieldset>

      <fieldset>
        <legend>Тариф</legend>
        <!-- Не <label>: в нём Select закрывает свой список тем же щелчком. -->
        <div class="field">
          <span id="tariff-label">Вид 1С:ИТС</span>
          <Select
            v-model="row.tariffCode"
            aria-labelledby="tariff-label"
            :options="tariffs"
            option-label="name"
            option-value="code"
            placeholder="Выберите тариф"
          />
          <small
            v-for="issue in issuesFor('tariffCode')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
          <EpdAdviceNote
            v-if="formAdvice"
            :advice="formAdvice"
            class="advice"
          >
            <Button
              v-if="formAdvice.best && !formAdvice.fresh && row.tariffCode !== formAdvice.best.code"
              :label="`Выбрать ${formAdvice.best.name.replace('1С-ЭДО. ', '')}`"
              size="small"
              outlined
              @click="row.tariffCode = formAdvice.best.code"
            />
          </EpdAdviceNote>
        </div>
        <label>Код абонента-владельца
          <InputText
            v-model="row.ownerCode"
            placeholder="CL-1000530"
          />
          <small
            v-for="issue in issuesFor('ownerCode')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>Дата начала
          <InputText
            v-model="row.startDate"
            placeholder="01.10.26"
          />
          <small
            v-for="issue in issuesFor('startDate')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
          <small
            v-for="note in notesFor('startDate')"
            :key="note.message"
            class="note"
          >
            {{ note.message }}
          </small>
        </label>
        <p class="fixed">
          Количество выпусков 12 и предоплата за весь срок подставляются автоматически:
          для тарифов ЭПД других значений не бывает.
        </p>
      </fieldset>

      <fieldset>
        <legend>Клиент</legend>
        <label>Наименование фирмы
          <InputText v-model="row.companyName" />
          <small
            v-for="issue in issuesFor('companyName')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>ИНН
          <InputText
            v-model="row.inn"
            @update:model-value="lookupByInn"
          />
          <small
            v-for="issue in issuesFor('inn')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>КПП
          <InputText
            v-model="row.kpp"
            :placeholder="row.inn.trim().length === 12 ? 'у ИП КПП нет' : ''"
          />
          <small
            v-for="issue in issuesFor('kpp')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
          <small
            v-if="row.inn.trim().length === 12 && !row.kpp.trim()"
            class="note"
          >
            У ИП КПП нет — поле остаётся пустым.
          </small>
        </label>
        <!-- Не <label>: в нём Select закрывает свой список тем же щелчком. -->
        <div class="field">
          <span id="reg-number-label">Регистрационный номер</span>
          <!-- У клиента несколько программ: основной выбирается из его регномеров,
               но вписать другой по-прежнему можно. -->
          <Select
            v-if="regOptions.length > 1"
            v-model="row.regNumber"
            aria-labelledby="reg-number-label"
            :options="regOptions"
            option-label="label"
            option-value="value"
            editable
            placeholder="Выберите основной регномер"
          />
          <InputText
            v-else
            v-model="row.regNumber"
            aria-labelledby="reg-number-label"
          />
          <small
            v-if="autoChecking"
            class="note"
          >
            <i class="pi pi-spin pi-spinner" /> Проверяем программы клиента в 1С…
          </small>
          <small
            v-else-if="regHint"
            class="note"
          >
            {{ regHint }}
          </small>
          <small
            v-for="issue in issuesFor('regNumber')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
          <small
            v-for="note in notesFor('regNumber')"
            :key="note.message"
            class="note"
          >
            {{ note.message }}
          </small>
        </div>
        <label>Логин Личного кабинета
          <InputText
            v-model="row.login"
            placeholder="client@example.ru"
          />
          <small
            v-for="issue in issuesFor('login')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
          <small
            v-if="currentClient?.edoIds?.length"
            class="note"
          >
            Идентификатор ЭДО: {{ currentClient.edoIds.join(', ') }} — должен быть привязан к этому логину.
          </small>
        </label>
        <!-- То же, что «Проверка условий сопровождения» в Личном кабинете 1С. -->
        <div class="programs">
          <div class="programs-head">
            <span>Программы в Личном кабинете</span>
            <Button
              label="Проверить в 1С"
              icon="pi pi-refresh"
              size="small"
              :loading="checkingPrograms"
              :disabled="checkingPrograms || !row.inn.trim() || !(row.login.trim() || row.regNumber.trim())"
              @click="checkPrograms"
            />
          </div>
          <table v-if="programs.length">
            <thead>
              <tr>
                <th>Рег. номер</th>
                <th>Программа</th>
                <th>Условия сопровождения</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(program, index) in programs"
                :key="index"
              >
                <td>{{ program.regNumber }}</td>
                <td>{{ program.program }}</td>
                <td :class="program.hasAccess ? 'ok' : 'bad'">
                  {{ conditionText(program) }}
                </td>
              </tr>
            </tbody>
          </table>
          <p class="fixed">
            <template v-if="programsCheckedAt">
              Проверено {{ when(programsCheckedAt) }}.
            </template>
            Ищется по логину Личного кабинета, без него — по регистрационному номеру.
          </p>
        </div>
        <div
          v-if="clientIts"
          class="programs"
        >
          <div class="programs-head">
            <span>Договоры 1С:ИТС абонента {{ clientIts.code }}</span>
          </div>
          <ItsContracts
            :subscriber="clientIts"
            class="its"
            checked
          />
          <IndustryNote
            :industry="clientIts.industry"
            label="ИТС Отраслевой"
          />
        </div>
        <label>Рабочих мест
          <InputNumber
            v-model="row.workplaces"
            :min="1"
          />
        </label>
        <label>Ответственный у клиента
          <InputText v-model="row.responsible" />
          <small
            v-for="issue in issuesFor('responsible', 1)"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>E-mail клиента
          <InputText v-model="row.email" />
          <small
            v-for="issue in issuesFor('email', 1)"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
        <label>Код города
          <InputText v-model="row.phoneCode" />
        </label>
        <label>Телефон
          <InputText v-model="row.phone" />
          <small
            v-for="issue in issuesFor('phone')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
      </fieldset>

      <fieldset>
        <legend>Получение</legend>
        <div class="field">
          <span id="delivery-label">Способ получения</span>
          <Select
            v-model="row.deliveryType"
            aria-labelledby="delivery-label"
            :options="deliveryOptions"
            option-label="label"
            option-value="value"
          />
        </div>
        <label v-if="row.deliveryType === '1'">Код дистрибьютора
          <InputText v-model="row.distributorCode" />
          <small
            v-for="issue in issuesFor('distributorCode')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </label>
      </fieldset>

      <fieldset>
        <legend>Операция</legend>
        <div class="field">
          <span id="operation-label">Операция</span>
          <Select
            v-model="row.operation"
            aria-labelledby="operation-label"
            :options="operationOptions"
            option-label="label"
            option-value="value"
          />
          <small
            v-for="issue in issuesFor('operation')"
            :key="issue.message"
            :class="issueClass(issue)"
          >
            {{ issue.message }}
          </small>
        </div>
        <template v-if="row.operation === '1'">
          <label>Дата отказа
            <InputText
              v-model="row.refusalDate"
              placeholder="11.26"
            />
            <small
              v-for="issue in issuesFor('refusalDate')"
              :key="issue.message"
              :class="issueClass(issue)"
            >
              {{ issue.message }}
            </small>
          </label>
          <div class="field">
            <span id="refusal-reason-label">Причина отказа</span>
            <Select
              v-model="row.refusalReason"
              aria-labelledby="refusal-reason-label"
              :options="refusalReasons"
              option-label="label"
              option-value="value"
              placeholder="Выберите причину"
            />
            <small
              v-for="issue in issuesFor('refusalReason')"
              :key="issue.message"
              :class="issueClass(issue)"
            >
              {{ issue.message }}
            </small>
          </div>
        </template>
        <p class="fixed">
          Отказ регистрируется только со следующего месяца. Для тарифов ЭПД он не
          оформляется вовсе: правила допускают только новый договор или продление.
        </p>
      </fieldset>

      <fieldset>
        <legend>Другие программы по договору</legend>
        <label
          v-for="(_, index) in row.extraRegNumbers"
          :key="index"
        >Регистрационный номер {{ index + 1 }}
          <!-- Остальные регномера клиента только предлагаются: заполняет человек. -->
          <Select
            v-if="extraOptions.length"
            v-model="row.extraRegNumbers[index]"
            :options="extraOptions"
            option-label="label"
            option-value="value"
            editable
            show-clear
          />
          <InputText
            v-else
            v-model="row.extraRegNumbers[index]"
          />
        </label>
        <p class="fixed">
          Необязательно: регистрационные номера других программ пользователя,
          покрываемых этим договором.
        </p>
      </fieldset>
    </div>

    <Dialog
      v-model:visible="confirmOpen"
      modal
      header="Проверьте перед отправкой"
      :style="{ width: '32rem', maxWidth: 'calc(100vw - 32px)' }"
    >
      <dl class="summary">
        <dt>Кому</dt>
        <dd>{{ recipients.length ? recipients.join(', ') : 'адресат не задан в настройках почты' }}</dd>
        <dt>Клиент</dt>
        <dd>
          {{ shortName(row.companyName) || '—' }}
          <small>{{ requisites(row) }}</small>
        </dd>
        <dt>Строк в заявке</dt>
        <dd>{{ form.rows.length }}</dd>
        <dt>Тариф</dt>
        <dd>{{ tariffName }}</dd>
        <dt>Дата начала</dt>
        <dd>{{ row.startDate || '—' }}</dd>
        <dt>Операция</dt>
        <dd>{{ operationOptions.find((item) => item.value === row.operation)?.label ?? row.operation }}</dd>
        <dt>Файл</dt>
        <dd>{{ requestFileName(form) }}</dd>
        <dt>Протокол придёт</dt>
        <dd>с autoits@1c.ru на {{ form.email || '—' }}</dd>
      </dl>
      <p class="fixed">
        Письмо с файлом уйдёт сразу, отозвать его нельзя.
        <template v-if="dirty">
          Несохранённые правки уйдут в письме, но в заявке останутся несохранёнными.
        </template>
      </p>
      <template #footer>
        <Button
          label="Отмена"
          text
          @click="confirmOpen = false"
        />
        <Button
          label="Отправить письмо"
          icon="pi pi-send"
          :loading="sending"
          @click="send"
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

.form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(20rem, 100%), 1fr));
  gap: 16px;
  align-items: start;
}

fieldset {
  display: flex;
  flex-direction: column;
  gap: 16px;

  /* Иначе fieldset растягивается по самой длинной строке — выбранному клиенту
     в списке — и на телефоне форма уезжает за край экрана. */
  min-width: 0;
  margin: 0;
  padding: 16px;
  background: var(--ui-bg-panel);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
}

/* Подпись сидит на рамке: верхняя половина — над страницей, нижняя — над
   панелью. Сплошной фон по половинам прячет узор за текстом, не меняя вида. */
legend {
  padding: 0 6px;
  background: linear-gradient(var(--ui-bg) 50%, var(--ui-bg-panel) 50%);
  font-size: 14px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

label,
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
  color: var(--ui-error);
}

.ok {
  color: var(--ui-success);
}

/* Выбор клиента — во всю ширину формы: с него начинается заявка. */
.picker {
  grid-column: 1 / -1;
}

.picker .card-link {
  align-self: flex-start;
  font-size: 13px;
  color: var(--ui-primary);
  text-decoration: none;
}

.picker .card-link:hover {
  text-decoration: underline;
}

.client-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.client-option small {
  font-size: 12px;
  color: var(--ui-text-dimmed);
}

.programs {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.advice {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.programs-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-weight: 500;
}

.programs table {
  width: 100%;
  border-collapse: collapse;
}

.programs th,
.programs td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--ui-border);
  text-align: left;
  vertical-align: top;
}

.programs th {
  color: var(--ui-text-highlighted);
  font-weight: 600;
}

.programs td:not(.ok, .bad) {
  color: var(--ui-text);
}

/* Замечание обо всей заявке: тот же приглушённый тон, что у полей. */
.banner {
  margin: 0;
  font-size: 12px;
  line-height: 16px;
}

/* Проверку выполнить не удалось — это не «неверно», поэтому не красный. */
.unchecked {
  color: var(--ui-text-dimmed);
}

/* Сверка с 1С нашла расхождение, но выпуск не запрещён. */
.warn {
  color: var(--ui-warning);
}

/* Справка из 1С к полю: не замечание, а сведения. */
.note {
  color: var(--ui-text-muted);
}

.suggest {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--ui-text);
  text-decoration: none;
}

.back:hover {
  color: var(--ui-text-highlighted);
}

.back .pi {
  font-size: 12px;
}

/* Сводка перед отправкой: подпись слева, значение справа. */
.summary {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 16px;
  margin: 0 0 12px;
  font-size: 14px;
  line-height: 20px;
}

.summary dt {
  color: var(--ui-text-muted);
}

.summary dd {
  margin: 0;
  color: var(--ui-text);
}

.summary small {
  display: block;
  color: var(--ui-text-muted);
}

.fixed {
  margin: 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-dimmed);
}

:deep(.p-inputtext),
:deep(.p-select) {
  width: 100%;
  height: 32px;
  background: var(--ui-bg);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  font-size: 14px;
  font-weight: 400;
  color: var(--ui-text);
}

:deep(.p-inputtext) {
  padding: 0 10px;
}

:deep(.p-inputtext::placeholder) {
  color: var(--ui-text-dimmed);
}

:deep(.p-inputtext:enabled:focus) {
  border-color: var(--ui-primary);
  box-shadow: none;
  outline: none;
}

:deep(.p-select) {
  display: flex;
  align-items: center;
}

:deep(.p-select-label) {
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 10px;
  font-size: 14px;
  color: var(--ui-text);
}

:deep(.p-select-label.p-placeholder) {
  color: var(--ui-text-dimmed);
}

:deep(.p-select:not(.p-disabled).p-focus) {
  border-color: var(--ui-primary);
  box-shadow: none;
  outline: none;
}

:deep(.p-select-dropdown) {
  color: var(--ui-text-muted);
}

:deep(.p-inputnumber),
:deep(.p-inputnumber-input) {
  width: 100%;
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

:deep(.p-button:disabled) {
  opacity: 0.5;
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

/* Таблица истории: та же приглушённая шапка и разделители, что на других экранах. */
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
</style>
