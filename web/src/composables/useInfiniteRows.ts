import { computed, nextTick, ref, watch, type WatchSource } from 'vue'
import { localeComparator, resolveFieldData, sort } from '@primeuix/utils/object'

export interface InfiniteRowsOptions {
  /** Сколько строк добавлять за раз. */
  chunk?: number
  /** Смена этих значений (фильтр, поиск) начинает список заново. */
  resetOn?: WatchSource[]
  /** Верх списка: после сброса возвращаем к нему, если он уехал вверх за экран. */
  anchor?: () => Element | null | undefined
}

/**
 * Бесконечная лента поверх уже загруженного списка: строки дорисовываются
 * порциями, пока пользователь листает, без страниц. Таблица работает в lazy-режиме:
 * сортирует здесь весь список, а не только отрисованную часть, тем же сравнением,
 * что и DataTable (localeComparator + sort из @primeuix/utils).
 */
export function useInfiniteRows<T>(source: () => readonly T[], options: InfiniteRowsOptions = {}) {
  const chunk = options.chunk ?? 50
  // Верх списка знает таблица, которая его рисует (InfiniteTable), — она и
  // подставляет его после монтирования, если экран не задал свой.
  let anchor = options.anchor
  const limit = ref(chunk)
  const sortField = ref<string>()
  const sortOrder = ref<number>()

  const sorted = computed<readonly T[]>(() => {
    const all = source()
    const field = sortField.value
    const order = sortOrder.value
    if (!field || !order) return all
    const comparer = localeComparator()
    const keyed = all.map((item) => ({ item, key: resolveFieldData(item, field) }))
    keyed.sort((a, b) => sort(a.key, b.key, order, comparer, 1))
    return keyed.map((entry) => entry.item)
  })

  const rows = computed(() => sorted.value.slice(0, limit.value) as T[])
  const total = computed(() => sorted.value.length)
  const hasMore = computed(() => limit.value < total.value)

  function more() {
    if (hasMore.value) limit.value += chunk
  }

  function reset() {
    limit.value = chunk
    void nextTick(() => {
      const top = anchor?.()
      if (!top) return
      // Сверху липнут шапка и панель поиска: верх списка должен встать под ними.
      const cover = document.querySelector('main .bar')?.getBoundingClientRect().bottom ?? 0
      const offset = top.getBoundingClientRect().top - cover
      if (offset < 0) window.scrollBy({ top: offset - 8 })
    })
  }

  function bindAnchor(next: () => Element | null | undefined) {
    anchor ??= next
  }

  /** Сортировка из шапки таблицы (update:sort-field / update:sort-order). */
  function setSortField(field: unknown) {
    sortField.value = typeof field === 'string' ? field : undefined
  }
  function setSortOrder(order: number | null | undefined) {
    sortOrder.value = order ?? undefined
  }

  watch([...(options.resetOn ?? []), sortField, sortOrder], reset)

  return {
    rows, total, hasMore, more, reset, sortField, sortOrder, bindAnchor, setSortField, setSortOrder,
  }
}

export type InfiniteRows<T> = ReturnType<typeof useInfiniteRows<T>>
