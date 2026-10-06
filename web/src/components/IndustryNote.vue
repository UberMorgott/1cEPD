<script setup lang="ts">
import { computed } from 'vue'
import type { ItsIndustry } from '../api/client'
import { industryText } from '../domain'

/**
 * ИТС Отраслевой абонента одной строкой; не оформлен, а нужен — красным.
 * 1С отвечает и пустым статусом без описания: тогда блок не рисуется.
 */
const props = defineProps<{
  industry?: ItsIndustry
  /** Подпись перед текстом, когда блок стоит без своей колонки «ИТС Отраслевой». */
  label?: string
}>()

const text = computed(() => industryText(props.industry))
</script>

<template>
  <span
    v-if="text"
    class="industry"
    :class="{ missing: industry?.missing.length }"
  ><template v-if="label">{{ label }}: </template>{{ text }}</span>
</template>

<style scoped>
.missing {
  color: var(--ui-error);
}
</style>
