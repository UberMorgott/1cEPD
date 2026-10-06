<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import ProgressBar from 'primevue/progressbar'
import { api, type UpdateStatus } from '../api/client'
import { downloadPercent, megabytes, restartedWithNewVersion, versionLabel } from '../update'

/**
 * Версия сборки рядом с названием программы. Щелчок открывает окно и сразу
 * проверяет GitHub (спрашивает сервер, страница в GitHub не ходит). Есть
 * новая — кнопка «Обновить до …»: загрузка с ходом в процентах, перезапуск,
 * а страница ждёт новую версию по /api/health и перезагружается сама.
 */

const status = ref<UpdateStatus | null>(null)
const open = ref(false)
const checking = ref(false)
const applying = ref(false)
const waiting = ref(false)
const failure = ref('')

const current = computed(() => status.value?.current ?? 'dev')
const label = computed(() => versionLabel(current.value))
const percent = computed(() => downloadPercent(status.value?.downloaded, status.value?.size))
const progressText = computed(() => {
  const done = megabytes(status.value?.downloaded)
  return percent.value === null
    ? `Загружено ${done} МБ`
    : `Загружено ${done} из ${megabytes(status.value?.size)} МБ (${percent.value}%)`
})
const failed = computed(() => !!failure.value || (!!status.value?.failed && !checking.value))
const canInstall = computed(
  () => !!status.value?.available && !failed.value && !checking.value && !applying.value && !status.value?.busy,
)
const message = computed(() => {
  if (failure.value) return failure.value
  if (waiting.value) return 'Новая версия запускается, страница обновится сама…'
  return status.value?.text ?? ''
})

let poll: ReturnType<typeof setInterval> | null = null
function stopPoll() {
  if (poll !== null) clearInterval(poll)
  poll = null
}
onUnmounted(stopPoll)

onMounted(async () => {
  try {
    status.value = await api.updateStatus()
  } catch {
    // Без версии шапка покажет «dev»: из-за этого ломать страницу незачем.
  }
})

async function show() {
  open.value = true
  failure.value = ''
  if (status.value?.busy) return
  checking.value = true
  try {
    status.value = await api.checkUpdate()
  } catch (err) {
    failure.value = err instanceof Error ? err.message : 'Проверка не удалась.'
  } finally {
    checking.value = false
  }
}

/** Пока идёт загрузка, ход берём опросом: ответ apply придёт только в конце. */
function pollProgress() {
  stopPoll()
  poll = setInterval(async () => {
    try {
      const s = await api.updateStatus()
      if (applying.value) status.value = s
    } catch {
      // Сервер уже перезапускается — дальше ждёт waitForRestart.
    }
  }, 500)
}

/** Ждём, пока поднимется новая версия, и перезагружаем страницу. */
function waitForRestart(before: string) {
  waiting.value = true
  stopPoll()
  const deadline = Date.now() + 120_000
  poll = setInterval(async () => {
    if (Date.now() > deadline) {
      stopPoll()
      waiting.value = false
      failure.value = 'Новая версия не ответила за две минуты: проверьте значок в трее.'
      return
    }
    try {
      if (restartedWithNewVersion(before, await api.health())) {
        stopPoll()
        window.location.reload()
      }
    } catch {
      // Сервер ещё не поднялся.
    }
  }, 1000)
}

async function install() {
  failure.value = ''
  applying.value = true
  const before = current.value
  pollProgress()
  try {
    const s = await api.applyUpdate()
    status.value = s
    if (s.restarting) waitForRestart(before)
    else stopPoll()
  } catch (err) {
    stopPoll()
    failure.value = err instanceof Error ? err.message : 'Обновление не установлено.'
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <button
    type="button"
    class="version"
    :title="`Версия ${label}: проверить обновления`"
    :aria-label="`Версия ${label}: проверить обновления`"
    @click="show"
  >
    {{ label }}
    <span
      v-if="status?.available"
      class="dot"
    />
  </button>

  <Dialog
    v-model:visible="open"
    modal
    :header="`Пульт ЭПД ${label}`"
    :style="{ width: '26rem' }"
  >
    <div class="update">
      <ProgressBar
        v-if="checking || waiting || (status?.restarting && !failed)"
        mode="indeterminate"
        class="bar-thin"
      />
      <template v-else-if="status?.installing">
        <ProgressBar
          :value="percent ?? 0"
          :mode="percent === null ? 'indeterminate' : 'determinate'"
          :show-value="false"
          class="bar-thin"
        />
        <span class="muted">{{ progressText }}</span>
      </template>
      <p
        v-if="checking"
        class="muted"
      >
        Проверяем обновления…
      </p>
      <p
        v-else-if="message"
        :class="failed ? 'bad' : 'muted'"
      >
        {{ message }}
      </p>
    </div>
    <template
      v-if="canInstall || failed"
      #footer
    >
      <Button
        v-if="canInstall"
        :label="`Обновить до v${status?.latest ?? ''}`"
        icon="pi pi-download"
        @click="install"
      />
      <Button
        v-else
        label="Проверить ещё раз"
        text
        @click="show"
      />
    </template>
  </Dialog>
</template>

<style scoped>
.version {
  position: relative;
  padding: 2px 6px;
  border: 0;
  border-radius: var(--ui-radius);
  background: transparent;
  font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 16px;
  font-weight: 400;
  color: var(--ui-text-muted);
  cursor: pointer;
}

.version:hover {
  background: var(--ui-bg-elevated);
  color: var(--ui-text-highlighted);
}

.dot {
  position: absolute;
  top: -2px;
  right: -2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ui-primary);
}

.update {
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-size: 14px;
  line-height: 20px;
}

.update p {
  margin: 0;
}

.muted {
  color: var(--ui-text-muted);
}

.bad {
  color: var(--ui-error);
}

.bar-thin {
  height: 4px;
}
</style>
