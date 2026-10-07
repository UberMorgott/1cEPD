<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useSessionStore } from './stores/session'
import { useLiveStore } from './stores/live'
import VersionBadge from './components/VersionBadge.vue'

const session = useSessionStore()
const live = useLiveStore()
const route = useRoute()
const router = useRouter()

// Поток событий держим только под сессией: без неё сервер ответит 401
// и клиент уйдёт в бесконечное переподключение.
watch(
  () => session.authenticated,
  (authenticated) => (authenticated ? live.connect() : live.disconnect()),
)

onMounted(() => {
  if (session.authenticated) live.connect()
})
onUnmounted(() => live.disconnect())

async function logout() {
  await session.logout()
  await router.push({ name: 'login' })
}

/** Пункты бокового меню: маршрут, подпись и иконка PrimeIcons. */
const navItems = [
  { to: '/clients', label: 'Клиенты', icon: 'pi-users' },
  { to: '/requests', label: 'Заявки', icon: 'pi-inbox' },
  { to: '/settings', label: 'Настройки', icon: 'pi-cog' },
]

// Заголовок верхней панели: карта имени маршрута в подпись, чтобы не
// заводить meta в роутере ради трёх строк.
const titles: Record<string, string> = {
  clients: 'Клиенты',
  requests: 'Заявки',
  'request-new': 'Заявки',
  request: 'Заявки',
  settings: 'Настройки',
  client: 'Карточка клиента',
}

/** Пункт меню подсвечен и на вложенных адресах: /requests/new, /requests/12. */
function navActive(to: string) {
  return route.path === to || route.path.startsWith(`${to}/`)
}
const pageTitle = computed(() => titles[String(route.name ?? '')] ?? '')

// Боковая панель на узком экране выезжает поверх контента.
const navOpen = ref(false)
watch(() => route.fullPath, () => (navOpen.value = false))
</script>

<template>
  <div
    v-if="route.name !== 'login'"
    class="shell"
    :class="{ 'shell-open': navOpen }"
  >
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark" />
        <span class="brand-name">Пульт ЭПД</span>
        <VersionBadge />
      </div>

      <nav class="nav">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="nav-item"
          :class="{ 'router-link-active': navActive(item.to) }"
        >
          <i
            class="pi"
            :class="item.icon"
          />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="sidebar-foot">
        <button
          type="button"
          class="nav-item"
          @click="logout"
        >
          <i class="pi pi-sign-out" />
          <span>Выйти</span>
        </button>
        <div class="user">
          <span class="avatar"><i class="pi pi-user" /></span>
          <span class="user-name">Пользователь</span>
        </div>
      </div>
    </aside>

    <div
      class="backdrop"
      @click="navOpen = false"
    />

    <div class="column">
      <header class="topbar">
        <button
          type="button"
          class="burger"
          aria-label="Меню"
          @click="navOpen = !navOpen"
        >
          <i class="pi pi-bars" />
        </button>
        <h1 class="page-title">
          {{ pageTitle }}
        </h1>
        <!-- Связь — свойство приложения, а не экрана: индикатор живёт здесь,
             иначе он пропадает там, где нет кнопки обновления. -->
        <span
          v-if="!live.connected"
          class="offline"
        >Нет связи</span>
      </header>
      <main class="content">
        <RouterView />
      </main>
    </div>
  </div>
  <RouterView v-else />
</template>

<style>
/* Шрифт лежит рядом с кодом: на закрытом контуре в интернет ходить некуда.
   Файл — латинский срез переменного Public Sans (ось веса 400–600), кириллицы
   в этом шрифте нет вообще, русский текст набирает системный шрифт из списка
   ниже — так было и со шрифтом из сети. */
@font-face {
  font-family: 'Public Sans';
  font-style: normal;
  font-weight: 400 600;
  font-display: swap;
  src: url('./assets/fonts/public-sans-latin.woff2') format('woff2');
  unicode-range: U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304,
    U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD;
}

:root {
  /* surfaces */
  --ui-bg: oklch(21% 0.006 285.885deg);
  --ui-bg-muted: oklch(27.4% 0.006 286.033deg);
  --ui-bg-elevated: oklch(27.4% 0.006 286.033deg);
  --ui-bg-accented: oklch(37% 0.013 285.805deg);

  /* Поверхности с содержимым — боковая панель, карточки, таблицы — светлее
     страницы: тот же цвет, что и подсветка, но разбавленный. Наведение берёт
     половину, выбранное состояние — полный цвет. Так глубина набирается одним
     оттенком, а не тремя разными. Все поверхности непрозрачные: четверть
     подсветки, смешанная с фоном страницы, а не с прозрачностью, — тот же
     тон, что был у полупрозрачной карточки над фоном, но узор фона виден
     только между панелями и никогда сквозь содержимое. */
  --ui-bg-panel: color-mix(in oklab, var(--ui-bg-elevated) 25%, var(--ui-bg));
  --ui-bg-panel-hover: color-mix(in oklab, var(--ui-bg-elevated) 50%, var(--ui-bg));

  /* text */
  --ui-text-highlighted: #fff;
  --ui-text: oklch(92% 0.004 286.32deg);
  --ui-text-toned: oklch(87.1% 0.006 286.286deg);
  --ui-text-muted: oklch(70.5% 0.015 286.067deg);
  --ui-text-dimmed: oklch(55.2% 0.016 285.938deg);

  /* borders */
  --ui-border: oklch(27.4% 0.006 286.033deg);
  --ui-border-accented: oklch(37% 0.013 285.805deg);

  /* accent */
  --ui-primary: oklch(70.253% 0.1320 160.37deg);
  --ui-primary-hover: oklch(74.927% 0.1183 162.26deg);

  /* status */
  --ui-success: oklch(70.253% 0.1320 160.37deg);
  --ui-warning: #f59e0b;
  --ui-error: #ef4444;
  --ui-info: #4d8cfd;

  /* geometry */
  --ui-radius: 0.25rem;
  --ui-radius-md: 6px;
  --ui-radius-lg: 8px;
  --app-sidebar-width: 16rem;
  --app-header-height: 4rem;

  color-scheme: dark;
}

body {
  margin: 0;
  font-family: 'Public Sans', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;

  /* Типографика задана в пикселях: три размера на всё приложение. */
  font-size: 14px;
  line-height: 20px;

  /* Розовый узор под всеми окнами: сплошной тон зашит в сам SVG, фон
     страницы остаётся прежним цветом под ним. */
  background: var(--ui-bg) url('./assets/bg-pattern.svg') repeat fixed;
  background-size: 180px 180px;
  color: var(--ui-text);
}

/* Заголовок экрана печатает только верхняя панель. */
h1 {
  margin: 0;
  font-size: 24px;
  line-height: 32px;
  font-weight: 600;
}

small,
.caption {
  font-size: 12px;
  line-height: 16px;
}

/* Полосы прокрутки: без выпуклостей, в цвет рамок. */
* {
  scrollbar-width: thin;
  scrollbar-color: var(--ui-border-accented) transparent;
}

::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-thumb {
  background: var(--ui-border-accented);
  border-radius: 999px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

/* Плотные таблицы: селектор с .p-datatable перебивает тему независимо от
   порядка вставки её стилей в head. */
.p-datatable .p-datatable-thead > tr > th,
.p-datatable .p-datatable-tbody > tr > td {
  padding: 10px 12px;
}

.p-datatable .p-datatable-column-title {
  font-size: 14px;
  line-height: 20px;
  font-weight: 600;
}

/* Колонка с классом num: числа вправо, цифры одной ширины — разряды в столбик.
   Класс Column уезжает и на th, и на td, поэтому правим обе стороны. */
.p-datatable .p-datatable-thead > tr > th.num,
.p-datatable .p-datatable-tbody > tr > td.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.p-datatable .p-datatable-thead > tr > th.num .p-datatable-column-header-content {
  justify-content: flex-end;
}

/* Диалоги телепортируются в body, поэтому scoped-стили представлений до них
   не дотягиваются — правила живут здесь. */
.p-dialog {
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-bg);
}

.p-dialog-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

/* ---- Каркас: боковое меню слева, колонка контента справа ---- */
.shell {
  min-height: 100vh;
}

.sidebar {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: 40;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  width: var(--app-sidebar-width);
  padding: 12px;
  gap: 6px;
  background: var(--ui-bg-panel);
  border-right: 1px solid var(--ui-border);
  transition: transform 0.2s ease;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

.brand-mark {
  width: 20px;
  height: 20px;
  border-radius: var(--ui-radius-md);
  background: var(--ui-primary);
}

/* Версия стоит вплотную к названию, как в agent-link. */
.brand-name {
  flex: none;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 32px;
  padding: 6px 8px;
  box-sizing: border-box;
  border: 0;
  border-radius: var(--ui-radius-md);
  background: transparent;
  font: inherit;
  font-weight: 500;
  color: var(--ui-text-muted);
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.nav-item .pi {
  font-size: 16px;
}

.nav-item:hover {
  background: var(--ui-bg-panel-hover);
  color: var(--ui-text);
}

.nav-item.router-link-active {
  background: var(--ui-bg-elevated);
  color: var(--ui-text-highlighted);
}

.nav-item.router-link-active .pi {
  color: var(--ui-primary);
}

.sidebar-foot {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.user {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-top: 1px solid var(--ui-border);
  color: var(--ui-text);
}

.user-name {
  flex: 1;
}

.avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--ui-bg-accented);
  color: var(--ui-text-muted);
  font-size: 12px;
}

.backdrop {
  display: none;
}

.column {
  margin-left: var(--app-sidebar-width);
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  gap: 12px;
  height: var(--app-header-height);
  padding: 0 24px;
  background: var(--ui-bg);
  border-bottom: 1px solid var(--ui-border);
}

.page-title {
  flex: 1;
  font-size: 14px;
  line-height: 20px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

/* Норма молчит: цветом отмечаем только потерю связи. */
.offline {
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-error);
  white-space: nowrap;
}

.burger {
  display: none;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  background: transparent;
  color: var(--ui-text);
  cursor: pointer;
}

.content {
  padding: 24px;
}

/* Панель заголовка экрана с поиском липнет под верхней панелью.
   Правило общее для всех экранов, поэтому живёт здесь, а не в каждой вьюхе.
   Отрицательные поля с равным паддингом закрывают просвет по бокам,
   через который иначе просвечивали бы уезжающие строки. */
main .bar {
  position: sticky;
  top: var(--app-header-height);
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 16px;
  margin: -24px -24px 0;
  padding: 16px 24px;
  background: var(--ui-bg);
  border-bottom: 1px solid var(--ui-border);
}

/* Фильтр прижат к левому краю над таблицей, всё остальное — к правому. */
main .bar .filter {
  width: 16rem;
  flex: none;
}

main .bar .meta {
  min-width: 0;
  font-size: 13px;
  line-height: 16px;
  color: var(--ui-text-muted);
}

main .bar .tools {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

/* Отметка о свежести данных стоит вплотную к кнопке обновления. */
main .bar .stamp {
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
  white-space: nowrap;
}

/* Надпись кнопки — одной строкой: кнопки фиксированной высоты, и перенесённая
   надпись вылезала за их края («Проверить в 1С», «Скачать файл»). */
.p-button {
  white-space: nowrap;
}

/* Заголовки таблицы стоят на своём месте, в начале таблицы. Липкими их
   делать нечем: отступ пришлось бы считать от высоты панели поиска, а её
   никто не измеряет — от подставленного значения заголовок отрывался и
   первая строка проезжала в просвет. */

/* Шапка таблицы — единственная полоса светлее панели: половина подсветки,
   скруглённая по краям строки, текст в полный контраст. */
.p-datatable .p-datatable-thead > tr > th {
  background: var(--ui-bg-panel-hover);
  font-size: 14px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}

.p-datatable .p-datatable-thead > tr > th:first-child {
  border-top-left-radius: var(--ui-radius-lg);
  border-bottom-left-radius: var(--ui-radius-lg);
}

.p-datatable .p-datatable-thead > tr > th:last-child {
  border-top-right-radius: var(--ui-radius-lg);
  border-bottom-right-radius: var(--ui-radius-lg);
}

@media (width <= 900px) {
  .sidebar {
    transform: translateX(-100%);
  }

  .shell-open .sidebar {
    transform: none;
  }

  .shell-open .backdrop {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 35;
    background: rgb(0 0 0 / 60%);
  }

  .column {
    margin-left: 0;
  }

  .topbar {
    padding: 0 16px;
  }

  .content {
    padding: 16px;
  }

  /* Панель инструментов вытягивается ровно на поля контента, иначе на узком
     экране она вылезает за край на 8px с каждой стороны. */
  main .bar {
    margin: -16px -16px 0;
    padding: 16px;
    flex-wrap: wrap;
  }

  main .bar .filter {
    width: 100%;
  }

  /* Кнопки режимов, отметка и обновление не влезают в 343px одной строкой. */
  main .bar .tools {
    flex-wrap: wrap;
  }

  .burger {
    display: flex;
  }
}
</style>
