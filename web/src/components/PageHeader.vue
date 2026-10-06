<script setup lang="ts">
import { computed } from 'vue'
import Button from 'primevue/button'
import Message from 'primevue/message'
import SearchBar from './SearchBar.vue'

/**
 * Шапка рабочего экрана: поиск, сведения о данных, отметка свежести и
 * обновление, а под ней — ошибка загрузки или «Не удалось обновить… Повторить».
 * Липкость и раскладку панели задаёт App.vue (main .bar), здесь только состав.
 *
 * Слоты: meta — сведения о данных после поиска; tools — кнопки перед отметкой;
 * actions — вместо кнопки обновления (например, «Выгрузить сейчас»);
 * error — дополнение к тексту ошибки (ссылка на настройки).
 */
const props = withDefaults(
  defineProps<{
    placeholder?: string
    /** Идёт фоновое обновление: «Обновляем…» вместо отметки. */
    refreshing?: boolean
    /** Время последней удачной загрузки, «10:15». */
    dataAsOf?: string
    /** Своя отметка вместо «Обновлено в …», например «Выгружено из 1С …». */
    stamp?: string
    refreshFailed?: boolean
    error?: string
    /** Кнопка ⟳ в шапке; без неё обновление — через слот actions. */
    refreshable?: boolean
  }>(),
  {
    placeholder: undefined,
    dataAsOf: '',
    stamp: undefined,
    error: '',
    refreshable: true,
  },
)

const emit = defineEmits<{ refresh: [] }>()

/** Поиск есть, только если экран его привязал: у формы заявки поиска нет. */
const search = defineModel<string>('search')

const stampText = computed(() => props.stamp ?? (props.dataAsOf ? `Обновлено в ${props.dataAsOf}` : ''))
</script>

<template>
  <header class="bar">
    <SearchBar
      v-if="search !== undefined"
      v-model="search"
      :placeholder="placeholder"
    />
    <slot name="meta" />
    <div class="tools">
      <slot name="tools" />
      <span
        v-if="refreshing && refreshable"
        class="stamp"
      >Обновляем…</span>
      <span
        v-else-if="stampText"
        class="stamp"
      >{{ stampText }}</span>
      <slot name="actions">
        <Button
          v-if="refreshable"
          icon="pi pi-refresh"
          text
          aria-label="Обновить"
          :loading="refreshing"
          @click="emit('refresh')"
        />
      </slot>
    </div>
  </header>

  <Message
    v-if="error"
    severity="error"
    :closable="false"
  >
    {{ error }}
    <slot name="error" />
  </Message>
  <Message
    v-else-if="refreshFailed"
    severity="warn"
    :closable="false"
  >
    <span class="stale">
      Не удалось обновить, данные от {{ dataAsOf || 'последней загрузки' }}
      <Button
        label="Повторить"
        size="small"
        text
        @click="emit('refresh')"
      />
    </span>
  </Message>
</template>

<style scoped>
.stale {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
