import { createRouter, createWebHistory, type LocationQuery } from 'vue-router'
import { useSessionStore } from '../stores/session'

/** Новая и сохранённая заявка — один экран: переход между ними его не пересоздаёт. */
const RequestForm = () => import('../views/RequestFormView.vue')

/** Только перечисленные параметры адреса: чужие старому экрану не переносим. */
function pick(query: LocationQuery, ...names: string[]): LocationQuery {
  return Object.fromEntries(Object.entries(query).filter(([name]) => names.includes(name)))
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Сводка и Находки слились в Клиентов: счётчики поводов и находки ЭДО
    // живут там (?show=anomalies — находки списком), старые адреса ведут туда.
    { path: '/', redirect: (to) => ({ name: 'clients', query: pick(to.query, 'q', 'show') }) },
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    { path: '/anomalies', redirect: (to) => ({ name: 'clients', query: { ...pick(to.query, 'q'), show: 'anomalies' } }) },
    { path: '/clients', name: 'clients', component: () => import('../views/ClientsView.vue') },
    // Биллинг, Абоненты и Реестр слились в Клиентов: старые ссылки ведут туда
    // же, поиск (?q=) и счётчик (?show=) — те же ключи.
    { path: '/billing', redirect: (to) => ({ name: 'clients', query: pick(to.query, 'q', 'show') }) },
    { path: '/subscribers', redirect: (to) => ({ name: 'clients', query: pick(to.query, 'q') }) },
    { path: '/identifiers', redirect: (to) => ({ name: 'clients', query: pick(to.query, 'q') }) },
    {
      path: '/requests',
      name: 'requests',
      component: () => import('../views/RequestsListView.vue'),
      // Старые ссылки «Заявка на продление» (?inn&kpp&start&tariff) вели на форму.
      beforeEnter: (to) => (to.query.inn ? { name: 'request-new', query: to.query } : true),
    },
    { path: '/requests/new', name: 'request-new', component: RequestForm },
    // Карточка клиента: ключ — ИНН-КПП (domain.ts clientKey).
    { path: '/clients/:key', name: 'client', component: () => import('../views/ClientCardView.vue') },
    { path: '/requests/:id(\\d+)', name: 'request', component: RequestForm },
    { path: '/settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
    // Опечатка в адресе или устаревшая ссылка — не пустой экран, а главная.
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  const session = useSessionStore()
  if (to.name !== 'login' && !session.authenticated) {
    return { name: 'login' }
  }
  if (to.name === 'login' && session.authenticated) {
    return { name: 'clients' }
  }
  return true
})

/**
 * Кусок экрана не загрузился — вкладка открыта до обновления программы, и
 * файлов прошлой сборки на сервере уже нет. Без этого переход молча срывался,
 * и пункты меню показывали прежний экран. Загружаем адрес заново — уже с
 * новой сборкой; один раз, чтобы при настоящем сбое не уйти в цикл.
 */
const reloadMark = 'pult-epd:chunk-reload'

function isChunkError(error: unknown): boolean {
  const text = error instanceof Error ? error.message : String(error)
  return /dynamically imported module|Importing a module script failed|error loading dynamically imported module/i.test(text)
}

router.onError((error, to) => {
  if (!isChunkError(error)) return
  let reloaded = false
  try {
    reloaded = sessionStorage.getItem(reloadMark) === to.fullPath
    sessionStorage.setItem(reloadMark, to.fullPath)
  } catch {
    // Хранилище недоступно — перезагружаем всё равно.
  }
  if (!reloaded) window.location.assign(to.fullPath)
})

router.afterEach(() => {
  try {
    sessionStorage.removeItem(reloadMark)
  } catch {
    // Нечего снимать.
  }
})

export default router
