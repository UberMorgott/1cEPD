import { defineStore } from 'pinia'
import { ref } from 'vue'

/** Событие из потока сервера. */
export interface LiveEvent {
  kind: string
  payload: Record<string, unknown>
}

export const useLiveStore = defineStore('live', () => {
  const connected = ref(false)
  const lastEvent = ref<LiveEvent | null>(null)
  /** Время, когда экран последний раз получил свежие данные, а не время события. */
  const lastRefreshAt = ref<Date | null>(null)

  let source: EventSource | null = null
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let retryDelay = 1000

  function handle(kind: string, event: MessageEvent) {
    let payload: Record<string, unknown> = {}
    try {
      payload = JSON.parse(event.data) as Record<string, unknown>
    } catch {
      // Событие без разбираемых данных — не повод падать, важен сам факт.
    }
    lastEvent.value = { kind, payload }
  }

  /** Вьюха отмечает успешную загрузку: только она знает, что данные на экране свежие. */
  function markRefreshed() {
    lastRefreshAt.value = new Date()
  }

  function connect() {
    if (source) return
    retryTimer = null

    source = new EventSource('/api/events', { withCredentials: true })

    source.onopen = () => {
      connected.value = true
      retryDelay = 1000
    }

    for (const kind of ['snapshot.completed', 'identifier.alert', 'programs.synced']) {
      source.addEventListener(kind, (event) => handle(kind, event as MessageEvent))
    }

    source.onerror = () => {
      connected.value = false
      source?.close()
      source = null
      // Переподключаемся с нарастающей паузой: сервер мог перезапуститься,
      // а долбить его каждую секунду незачем.
      retryTimer = setTimeout(connect, retryDelay)
      retryDelay = Math.min(retryDelay * 2, 30000)
    }
  }

  function disconnect() {
    // Без отмены таймера соединение воскресает уже после выхода из системы.
    if (retryTimer !== null) {
      clearTimeout(retryTimer)
      retryTimer = null
    }
    source?.close()
    source = null
    connected.value = false
  }

  return { connected, lastEvent, lastRefreshAt, markRefreshed, connect, disconnect }
})
