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
    { path: '/', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    { path: '/anomalies', name: 'anomalies', component: () => import('../views/AnomaliesView.vue') },
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
    return { name: 'dashboard' }
  }
  return true
})

export default router
