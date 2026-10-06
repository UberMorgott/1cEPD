import { computed, onMounted, ref, watch } from 'vue'
import { useLiveStore } from '../stores/live'
import { clockTime } from '../format'

export interface RefreshableOptions {
  /** Текст ошибки, если сервер не объяснил отказ. */
  failText: string
  /** Загрузить при открытии экрана и перечитывать по событию сервера (новый снимок). */
  auto?: boolean
}

/**
 * Загрузка данных экрана с фоновым обновлением: первая загрузка крутит таблицу,
 * фоновая оставляет данные на экране, а её сбой не стирает их, а только
 * помечает «Не удалось обновить». Отметка «Обновлено в» — общая для экранов.
 */
export function useRefreshable(loader: () => Promise<void>, options: RefreshableOptions) {
  const live = useLiveStore()
  const loading = ref(true)
  /** Фоновое обновление: данные на экране остаются, таблица не мигает. */
  const refreshing = ref(false)
  const error = ref('')
  const refreshFailed = ref(false)

  async function load(background = false) {
    if (background) refreshing.value = true
    else loading.value = true
    try {
      await loader()
      error.value = ''
      refreshFailed.value = false
      live.markRefreshed()
    } catch (err) {
      const text = err instanceof Error ? err.message : options.failText
      // Сорванное фоновое обновление не повод стирать с экрана рабочие данные.
      if (background) refreshFailed.value = true
      else error.value = text
    } finally {
      loading.value = false
      refreshing.value = false
    }
  }

  const dataAsOf = computed(() => clockTime(live.lastRefreshAt))

  if (options.auto !== false) {
    onMounted(() => load())
    // Снялся новый снимок — данные могли измениться, перечитываем сами.
    watch(() => live.lastEvent, () => load(true))
  }

  return { loading, refreshing, error, refreshFailed, dataAsOf, load }
}
