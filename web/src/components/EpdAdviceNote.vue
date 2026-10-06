<script setup lang="ts">
import type { EpdAdvice } from '../api/client'
import { epdBestText, epdCurrentText } from '../domain'
import { money } from '../format'

/**
 * Совет тарифа ЭПД по расходу за 12 закрытых месяцев: сколько ушло, как клиент
 * платит сейчас и что выгоднее. Слот — действие рядом с текстом («Выбрать тариф»).
 */
defineProps<{ advice: EpdAdvice }>()
</script>

<template>
  <div class="epd-advice">
    <span>
      ЭПД за 12 мес.: исходящих {{ advice.epdOut }}, входящих {{ advice.epdIn }}.
      Сейчас {{ epdCurrentText(advice) }} — {{ money(advice.currentCost) }} в год.
      <template v-if="advice.optimal">Это самый выгодный вариант.</template>
      <template v-else>
        Выгоднее {{ epdBestText(advice) }} — {{ money(advice.bestCost) }},
        экономия <span class="gain">{{ money(advice.savings) }}</span>.
      </template>
      <small
        v-if="advice.individual"
        class="note"
      >Больше 200 000 в год — индивидуальный тариф по запросу на epd@1c.ru.</small>
    </span>
    <slot />
  </div>
</template>

<style scoped>
.epd-advice {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.gain {
  color: var(--ui-success);
  font-weight: 500;
  white-space: nowrap;
}

.note {
  display: block;
  color: var(--ui-text-muted);
}
</style>
