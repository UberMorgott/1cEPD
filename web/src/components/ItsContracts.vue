<script setup lang="ts">
import type { ItsSubscriber } from '../api/client'
import { contractName, contractTerm } from '../domain'
import { when } from '../format'

/**
 * Договоры 1С:ИТС абонента: вид, срок, примечание. Договоров нет — описание
 * статуса из 1С. checked — приписать, когда 1С проверяла.
 */
defineProps<{
  subscriber: ItsSubscriber
  checked?: boolean
}>()
</script>

<template>
  <div class="its-contracts">
    <ul v-if="subscriber.contracts.length">
      <li
        v-for="(contract, index) in subscriber.contracts"
        :key="index"
      >
        {{ contractName(contract) }}: {{ contractTerm(contract) }}
        <small v-if="contract.description">— {{ contract.description }}</small>
      </li>
    </ul>
    <span v-else>{{ subscriber.description || subscriber.status }}</span>
    <small
      v-if="checked && subscriber.checkedAt"
      class="checked"
    >Проверено {{ when(subscriber.checkedAt) }}.</small>
    <slot />
  </div>
</template>

<style scoped>
ul {
  margin: 0;
  padding-left: 1rem;
}

.checked {
  display: block;
  color: var(--ui-text-dimmed);
}
</style>
