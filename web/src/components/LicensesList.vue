<script setup lang="ts">
import type { LicenseTariff } from '../api/client'
import { optionText, serviceName, tariffServices } from '../domain'
import { moscowDate, shortName } from '../format'

/** Тарифы сервисов абонента (1С-Отчетность, 1С:Подпись…): срок, организация, остатки опций. */
defineProps<{ tariffs: LicenseTariff[] }>()
</script>

<template>
  <ul class="licenses">
    <li
      v-for="(tariff, index) in tariffs"
      :key="index"
    >
      {{ tariff.name }} ({{ tariffServices(tariff) }}): {{ moscowDate(tariff.start) }} — {{ moscowDate(tariff.end) }}
      <small v-if="tariff.orgName">— {{ shortName(tariff.orgName) }}</small>
      <small
        v-for="(option, at) in tariff.options"
        :key="at"
        class="note"
      >{{ serviceName(option.type) }}: {{ optionText(option) }}</small>
    </li>
  </ul>
</template>

<style scoped>
.licenses {
  margin: 0;
  padding-left: 1rem;
}

.note {
  display: block;
  font-size: 12px;
  color: var(--ui-text-muted);
}
</style>
