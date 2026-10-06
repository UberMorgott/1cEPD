<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Message from 'primevue/message'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import { api, type Anomaly } from '../api/client'

/**
 * «Это нормально»: скрыть находку с причиной или запомнить связь в реестре
 * ожидаемой топологии. Один диалог для «Находок» и карточки клиента.
 */
const props = defineProps<{ target: Anomaly | null }>()
const emit = defineEmits<{ close: []; done: [] }>()

/** Виды, которые гасит реестр топологии (service.TopologyKinds). */
const topologyKinds = new Set<Anomaly['kind']>(['orphan_idle', 'replaced'])

const reason = ref('')
const asTopology = ref(false)
const busy = ref(false)
const error = ref('')

watch(
  () => props.target,
  () => {
    reason.value = ''
    asTopology.value = false
    error.value = ''
  },
)

const topologyAllowed = computed(
  () => !!props.target && !!props.target.inn && topologyKinds.has(props.target.kind),
)

async function confirm() {
  const target = props.target
  if (!target || !reason.value.trim()) return
  busy.value = true
  try {
    if (asTopology.value && topologyAllowed.value) {
      await api.putTopology({ inn: target.inn, kpp: target.kpp, edoId: target.edoId, purpose: reason.value.trim() })
    } else {
      await api.acknowledge(target.edoId, target.fingerprint, reason.value)
    }
    emit('done')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось сохранить пометку.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Dialog
    :visible="target !== null"
    modal
    header="Скрыть находку как допустимую"
    :style="{ width: '28rem', maxWidth: 'calc(100vw - 32px)' }"
    @update:visible="emit('close')"
  >
    <p class="ack-hint">
      Пометка скроет именно это состояние. Если оно изменится — трафик пойдёт, сменится
      логин или владелец — находка появится снова.
    </p>
    <Textarea
      v-model="reason"
      class="ack-reason"
      rows="3"
      auto-resize
      placeholder="Причина, например: разные виды деятельности"
    />
    <label
      v-if="topologyAllowed"
      class="ack-topology"
    >
      <ToggleSwitch v-model="asTopology" />
      <span>
        Законная связь организации: запомнить в реестре топологии — гасит и будущие сигналы
        «без владельца» и «замена» по этой паре ИНН и идентификатора
      </span>
    </label>
    <Message
      v-if="error"
      severity="error"
      :closable="false"
    >
      {{ error }}
    </Message>
    <template #footer>
      <Button
        label="Отмена"
        text
        @click="emit('close')"
      />
      <Button
        label="Пометить"
        :loading="busy"
        :disabled="!reason.trim()"
        @click="confirm"
      />
    </template>
  </Dialog>
</template>

<style scoped>
.ack-hint {
  margin-top: 0;
  font-size: 13px;
  line-height: 18px;
  color: var(--ui-text-muted);
}

.ack-reason {
  width: 100%;
  padding: 8px 10px;
  line-height: 20px;
}

.ack-topology {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 12px 0;
  font-size: 13px;
  color: var(--ui-text-muted);
}

:deep(.p-toggleswitch-checked .p-toggleswitch-slider) {
  background: var(--ui-primary);
}
</style>
