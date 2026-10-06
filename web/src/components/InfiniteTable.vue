<script setup lang="ts" generic="T">
import { onMounted, ref, type ComponentPublicInstance } from 'vue'
import DataTable from 'primevue/datatable'
import InfiniteSentinel from './InfiniteSentinel.vue'
import type { InfiniteRows } from '../composables/useInfiniteRows'

/**
 * Лента-таблица: DataTable в lazy-режиме поверх useInfiniteRows и хвост,
 * который догружает порции при прокрутке. Колонки, #empty и #expansion
 * передаются как в DataTable; остальные атрибуты (data-key, expanded-rows,
 * row-click, table-style…) уходят в саму таблицу.
 */
defineOptions({ inheritAttrs: false })

const props = defineProps<{
  feed: InfiniteRows<T>
  loading?: boolean
}>()

const table = ref<ComponentPublicInstance | null>(null)

// После сброса ленты (поиск, фильтр) экран возвращается к верху таблицы.
onMounted(() => props.feed.bindAnchor(() => table.value?.$el as Element | undefined))
</script>

<template>
  <DataTable
    ref="table"
    v-bind="$attrs"
    :sort-field="feed.sortField.value"
    :sort-order="feed.sortOrder.value"
    :value="feed.rows.value"
    :loading="loading"
    lazy
    removable-sort
    @update:sort-field="feed.setSortField"
    @update:sort-order="feed.setSortOrder"
  >
    <template
      v-for="(_, name) in $slots"
      #[name]="scope"
    >
      <slot
        :name="name"
        v-bind="scope ?? {}"
      />
    </template>
  </DataTable>
  <InfiniteSentinel
    :has-more="feed.hasMore.value"
    :shown="feed.rows.value.length"
    :total="feed.total.value"
    @more="feed.more"
  />
</template>
