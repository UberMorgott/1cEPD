<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { requisites, shortName } from '../format'

/**
 * Клиент в ячейке таблицы — одна вёрстка на всех экранах: сокращённое название
 * (полное — в подсказке), под ним реквизиты и, если переданы, оператор, логин,
 * идентификатор ЭДО и владелец. Необязательное не передано — строки нет.
 */
const props = defineProps<{
  name: string
  inn?: string
  kpp?: string
  /** Оператор ЭДО — через точку после реквизитов. */
  operator?: string
  login?: string
  edoId?: string
  owner?: string
  /** Длинные строки — в одну с многоточием (узкие колонки фиксированной ширины). */
  ellipsis?: boolean
  /** Ключ карточки клиента (domain.ts clientKey): название становится ссылкой на неё. */
  to?: string
}>()

const requisitesLine = computed(() => {
  const line = requisites({ inn: props.inn, kpp: props.kpp })
  return props.operator ? `${line} · ${props.operator}` : line
})
</script>

<template>
  <div
    class="client-summary"
    :class="{ ellipsis }"
  >
    <!-- Строки таблиц раскрываются и открываются щелчком: ссылка его не отдаёт. -->
    <RouterLink
      v-if="to"
      :to="{ name: 'client', params: { key: to } }"
      :title="`Карточка клиента: ${name}`"
      class="card-link"
      @click.stop
    >
      {{ shortName(name) || '—' }}
    </RouterLink>
    <span
      v-else
      :title="name"
    >{{ shortName(name) || '—' }}</span>
    <small>{{ requisitesLine }}</small>
    <small
      v-if="login"
      class="login"
    >{{ login }}</small>
    <small v-if="edoId"><code>{{ edoId }}</code></small>
    <small v-if="owner">{{ owner }}</small>
  </div>
</template>

<style scoped>
.client-summary {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.client-summary small {
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

.card-link {
  color: inherit;
  text-decoration: underline;
  text-decoration-color: var(--ui-border-accented);
  text-underline-offset: 3px;
}

.card-link:hover {
  color: var(--ui-primary);
  text-decoration-color: currentcolor;
}

/* Логин — почта без пробелов: без разрешённого переноса он распирает колонку. */
.login {
  overflow-wrap: anywhere;
}

code {
  font-size: 12px;
  color: var(--ui-text-muted);
  white-space: nowrap;
}

/* Длинные названия — одной строкой с многоточием, полностью в подсказке. */
.ellipsis > * {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
