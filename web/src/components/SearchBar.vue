<script setup lang="ts">
import InputText from 'primevue/inputtext'

/** Поиск над списком: липкая панель экрана прижимает его к левому краю (класс filter). */
withDefaults(defineProps<{ placeholder?: string }>(), {
  placeholder: 'Клиент, ИНН, логин, регномер, идентификатор',
})

const query = defineModel<string>({ required: true })
</script>

<template>
  <InputText
    v-model="query"
    class="filter"
    :placeholder="placeholder"
    :aria-label="placeholder"
  />
</template>
