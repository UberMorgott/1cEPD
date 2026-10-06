import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { api, ApiError } from '../api/client'

/**
 * Ключ, под которым помним факт входа.
 *
 * Сама сессия живёт в cookie и известна только серверу, а маршрутизатору ответ
 * нужен до первого запроса — иначе после перезагрузки страницы он уводит на
 * форму входа при живой сессии. Если cookie всё-таки протухла, первый же запрос
 * вернёт 401 и клиент API вызовет expire().
 */
const AUTHENTICATED_KEY = 'session.authenticated'

export const useSessionStore = defineStore('session', () => {
  const authenticated = ref(localStorage.getItem(AUTHENTICATED_KEY) === '1')
  const error = ref('')
  const busy = ref(false)

  watch(authenticated, (value) => {
    if (value) {
      localStorage.setItem(AUTHENTICATED_KEY, '1')
    } else {
      localStorage.removeItem(AUTHENTICATED_KEY)
    }
  })

  async function login(name: string, password: string): Promise<boolean> {
    busy.value = true
    error.value = ''
    try {
      await api.login(name, password)
      authenticated.value = true
      return true
    } catch (err) {
      error.value = err instanceof ApiError ? err.message : 'Не удалось войти.'
      return false
    } finally {
      busy.value = false
    }
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      authenticated.value = false
    }
  }

  /**
   * Помечает сессию потерянной. Вызывается из клиента API при 401.
   *
   * Охранник маршрутов срабатывает только на переходах, поэтому уводим на форму
   * входа сами. Маршрутизатор берём отложенно: он импортирует это хранилище.
   */
  async function expire() {
    authenticated.value = false
    const { default: router } = await import('../router')
    if (router.currentRoute.value.name !== 'login') {
      await router.replace({ name: 'login' })
    }
  }

  return { authenticated, error, busy, login, logout, expire }
})
