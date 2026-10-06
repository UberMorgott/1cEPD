<script setup lang="ts">
import type { ForecastClient } from '../api/client'
import { forecastText } from '../domain'

/** Прогноз расхода лимита к концу месяца; выйдет за лимит — красным, с перерасходом. */
defineProps<{ item: ForecastClient }>()
</script>

<template>
  <span :class="{ warn: item.warn }">
    {{ forecastText(item) }}<template v-if="item.warn"> — сверх лимита ~{{ item.overBy }}</template>
  </span>
</template>

<style scoped>
.warn {
  color: var(--ui-error);
}
</style>
