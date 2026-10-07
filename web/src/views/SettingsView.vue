<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import Message from 'primevue/message'
import { api, type ItsSender, type Settings } from '../api/client'
import { versionLabel } from '../update'

const form = ref<Settings>({
  smtpHost: '',
  smtpPort: 465,
  smtpLogin: '',
  smtpPasswordSet: false,
  smtpFrom: '',
  mailTo: ['itsrobot@1c.ru'],
})

// Список получателей правится текстом: по адресу в строке.
const mailToText = ref('itsrobot@1c.ru')
// Пароль только пишется: поле всегда пустое, сервер его не отдаёт.
const password = ref('')
const clearPassword = ref(false)

/** Отправитель заявки ИТС: подставляется в каждую новую заявку. */
const sender = ref<ItsSender>({ partnerCode: '', responsible: '', email: '', password: '' })

const saving = ref(false)
const testing = ref(false)
const error = ref('')
const notice = ref('')

const portOptions = [
  { label: '465 — SSL/TLS', value: 465 },
  { label: '587 — STARTTLS', value: 587 },
]

function apply(next: Settings) {
  form.value = next
  mailToText.value = (next.mailTo ?? []).join('\n')
  password.value = ''
  clearPassword.value = false
}

/** Самообновление: сохраняется сразу по щелчку, кнопка «Сохранить» ему не нужна. */
const updates = ref({ autoCheck: true, autoInstall: true, current: '' })

async function saveUpdates() {
  error.value = ''
  notice.value = ''
  try {
    const s = await api.saveUpdateSettings({
      autoCheck: updates.value.autoCheck,
      autoInstall: updates.value.autoInstall,
    })
    updates.value = { autoCheck: s.autoCheck, autoInstall: s.autoInstall, current: s.current }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось сохранить настройки обновления.'
  }
}

async function load() {
  try {
    const [next, nextSender, upd] = await Promise.all([api.settings(), api.itsSender(), api.updateStatus()])
    apply(next)
    sender.value = nextSender
    updates.value = { autoCheck: upd.autoCheck, autoInstall: upd.autoInstall, current: upd.current }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось загрузить настройки.'
  }
}

async function save() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    sender.value = await api.saveItsSender(sender.value)
    apply(
      await api.saveSettings({
        smtpHost: form.value.smtpHost,
        smtpPort: form.value.smtpPort,
        smtpLogin: form.value.smtpLogin,
        smtpFrom: form.value.smtpFrom,
        mailTo: mailToText.value.split('\n').map((line) => line.trim()).filter(Boolean),
        // Ключ отсутствует — пароль остаётся прежним; пустую строку сервер отвергает.
        ...(password.value ? { smtpPassword: password.value }
          : clearPassword.value ? { smtpPassword: null } : {}),
      }),
    )
    notice.value = 'Настройки сохранены.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось сохранить настройки.'
  } finally {
    saving.value = false
  }
}

/** Проверка: текст ошибки от Яндекса показываем как есть, ради него всё и затевалось. */
async function test() {
  testing.value = true
  error.value = ''
  notice.value = ''
  try {
    await api.testSmtp()
    notice.value = 'Тестовое письмо отправлено.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Проверка не удалась.'
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>

<template>
  <section>
    <header class="bar">
      <small class="meta">Почта для отправки заявок</small>
      <div class="tools">
        <Button
          label="Проверить"
          icon="pi pi-envelope"
          :loading="testing"
          :disabled="testing || saving"
          @click="test"
        />
        <Button
          label="Сохранить"
          icon="pi pi-save"
          :loading="saving"
          :disabled="saving || testing"
          @click="save"
        />
      </div>
    </header>

    <Message
      v-if="error"
      severity="error"
      :closable="false"
    >
      {{ error }}
    </Message>
    <Message
      v-if="notice"
      severity="success"
      :closable="false"
    >
      {{ notice }}
    </Message>

    <div class="form">
      <fieldset>
        <legend>SMTP</legend>
        <label>SMTP-хост
          <InputText
            v-model="form.smtpHost"
            placeholder="smtp.example.ru"
          />
        </label>
        <!-- Не <label>: щелчок по label переотправляется в Select и тут же закрывает список. -->
        <div class="field">
          <span id="smtp-port-label">Порт</span>
          <Select
            v-model="form.smtpPort"
            aria-labelledby="smtp-port-label"
            :options="portOptions"
            option-label="label"
            option-value="value"
          />
        </div>
        <label>Логин
          <InputText
            v-model="form.smtpLogin"
            placeholder="robot@example.ru"
          />
        </label>
        <label>Пароль приложения
          <Password
            v-model="password"
            :feedback="false"
            toggle-mask
            placeholder="Оставьте пустым, чтобы не менять"
          />
          <small
            v-if="form.smtpPasswordSet && !clearPassword"
            class="hint"
          >
            Пароль сохранён.
            <a
              href="#"
              @click.prevent="clearPassword = true; password = ''"
            >Удалить</a>
          </small>
          <small
            v-else-if="clearPassword"
            class="bad"
          >
            Пароль будет удалён при сохранении.
            <a
              href="#"
              @click.prevent="clearPassword = false"
            >Отменить</a>
          </small>
        </label>
        <label>Адрес отправителя (From)
          <InputText
            v-model="form.smtpFrom"
            placeholder="robot@example.ru"
          />
        </label>
        <label>Получатели
          <Textarea
            v-model="mailToText"
            rows="3"
          />
          <small class="hint">По одному адресу в строке.</small>
        </label>
      </fieldset>

      <fieldset>
        <legend>Отправитель заявки ИТС</legend>
        <label>Код партнёра
          <InputText
            v-model="sender.partnerCode"
            placeholder="00000"
          />
        </label>
        <label>Ответственный
          <InputText v-model="sender.responsible" />
        </label>
        <label>E-mail для протокола
          <InputText
            v-model="sender.email"
            placeholder="its@example.ru"
          />
        </label>
        <label>Пароль заявки
          <Password
            v-model="sender.password"
            :feedback="false"
            toggle-mask
            :input-props="{ autocomplete: 'new-password' }"
          />
        </label>
        <p class="fixed">
          Подставляются в каждую новую заявку; в самой заявке их можно поправить.
          Пароль подтверждает подлинность заявки — это не пароль Портала и не пароль API.
        </p>
      </fieldset>

      <fieldset>
        <legend>Обновления</legend>
        <label class="check">
          <input
            v-model="updates.autoCheck"
            type="checkbox"
            @change="saveUpdates"
          >
          Проверять новые версии раз в 6 часов
        </label>
        <label class="check">
          <input
            v-model="updates.autoInstall"
            type="checkbox"
            :disabled="!updates.autoCheck"
            @change="saveUpdates"
          >
          Устанавливать найденную версию автоматически
        </label>
        <p class="fixed">
          Версия {{ versionLabel(updates.current) }}. Проверить сейчас — щелчок по версии рядом с названием
          программы или пункт «Проверить обновления» в меню значка в трее. Обновления берутся
          с GitHub, файл сверяется по SHA-256; после установки программа перезапускается сама.
        </p>
      </fieldset>

      <fieldset>
        <legend>Яндекс: что включить</legend>
        <p class="fixed">
          Нужен пароль приложения, обычный пароль от ящика не подойдёт. Он начинает
          работать через 2–3 часа после выпуска.
        </p>
        <p class="fixed">
          Выпустить:
          Яндекс ID → «Пароли приложений», тип «Почта». Как это устроено —
          в справке Яндекса, раздел «Пароли приложений».
        </p>
        <p class="fixed">
          В настройках ящика включить «Разрешить доступ к почтовому ящику с помощью
          почтовых клиентов», IMAP и «Пароли приложений и OAuth-токены». Для Яндекс 360
          админ дополнительно включает «Использовать протоколы → IMAP».
        </p>
        <p class="fixed">
          Адрес отправителя обязан совпадать с логином или быть его алиасом.
        </p>
      </fieldset>
    </div>
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

.hint {
  color: var(--ui-text-dimmed);
}

label.check {
  flex-direction: row;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 400;
  color: var(--ui-text);
}

label.check input {
  accent-color: var(--ui-primary);
}

.hint a,
.fixed a,
.bad a {
  color: var(--ui-primary);
}

.bad {
  color: var(--ui-error);
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

:deep(.p-textarea) {
  height: auto;
  padding: 6px 10px;
  line-height: 20px;
}

:deep(.p-password) {
  width: 100%;
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
  padding: 0 10px;
  font-size: 14px;
  color: var(--ui-text);
}

:deep(.p-select:not(.p-disabled).p-focus) {
  border-color: var(--ui-primary);
  box-shadow: none;
  outline: none;
}

:deep(.p-select-dropdown) {
  color: var(--ui-text-muted);
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

:deep(.p-message-success) {
  --tone: var(--ui-success);
}

:deep(.p-message-content) {
  padding: 10px 12px;
}
</style>
