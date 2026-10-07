<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Message from 'primevue/message'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const router = useRouter()
const login = ref('')
const password = ref('')

async function submit() {
  if (await session.login(login.value, password.value)) {
    await router.push({ name: 'clients' })
  }
}
</script>

<template>
  <div class="login">
    <form
      class="card"
      @submit.prevent="submit"
    >
      <div class="brand">
        <span
          class="mark"
          aria-hidden="true"
        />
        <h1>Пульт ЭПД</h1>
      </div>

      <div class="field">
        <label for="login">Логин</label>
        <InputText
          id="login"
          v-model="login"
          autocomplete="username"
          autofocus
        />
      </div>

      <div class="field">
        <label for="password">Пароль</label>
        <Password
          id="password"
          v-model="password"
          :feedback="false"
          toggle-mask
          input-id="password"
          autocomplete="current-password"
        />
      </div>

      <Message
        v-if="session.error"
        severity="error"
        :closable="false"
      >
        {{ session.error }}
      </Message>

      <Button
        type="submit"
        label="Войти"
        :loading="session.busy"
      />
    </form>
  </div>
</template>

<style scoped>
.login {
  display: grid;
  place-items: center;
  min-height: 100vh;
  padding: 1rem;
  color: var(--ui-text);
}

.card {
  display: flex;
  flex-direction: column;

  /* Между парами «подпись + поле» 16px, внутри пары — 6px. */
  gap: 16px;
  box-sizing: border-box;
  width: min(22rem, calc(100vw - 2rem));
  padding: 24px;
  background: var(--ui-bg-panel);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}

.mark {
  width: 24px;
  height: 24px;
  flex: 0 0 auto;
  border-radius: var(--ui-radius-md);
  background: var(--ui-primary);
}

h1 {
  margin: 0;
  font-size: 15px;
  line-height: 20px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

label {
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  color: var(--ui-text-muted);
}

/* Поля и кнопка PrimeVue: геометрия и цвета берутся из токенов оболочки. */
:deep(.p-inputtext) {
  width: 100%;
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

:deep(.p-password) {
  width: 100%;
}

:deep(.p-password-toggle-mask-icon) {
  color: var(--ui-text-muted);
}

:deep(.p-button) {
  width: 100%;
  height: 32px;
  justify-content: center;
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

/* Ошибка входа — просто красная строка, без плашки. */
:deep(.p-message) {
  margin: 0;
  border: 0;
  background: transparent;
  color: var(--ui-error);
  font-size: 13px;
  line-height: 18px;
}

:deep(.p-message-content) {
  padding: 0;
  gap: 6px;
}
</style>
