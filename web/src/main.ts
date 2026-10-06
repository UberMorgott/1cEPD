import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Aura from '@primeuix/themes/aura'
import { definePreset } from '@primeuix/themes'
import 'primeicons/primeicons.css'

import App from './App.vue'
import router from './router'

/**
 * Приложение только тёмное: класс стоит в index.html, здесь подстраховка
 * на случай, если разметку кто-то заменит.
 */
document.documentElement.classList.add('dark')

/**
 * Компоненты PrimeVue берут те же цвета, что и каркас: значения приходят
 * из переменных --ui-*, объявленных в App.vue.
 */
const preset = definePreset(Aura, {
  primitive: {
    borderRadius: { md: 'var(--ui-radius-md)', lg: 'var(--ui-radius-lg)' },
  },
  semantic: {
    primary: {
      color: 'var(--ui-primary)',
      contrastColor: 'var(--ui-bg)',
      hoverColor: 'var(--ui-primary-hover)',
      activeColor: 'var(--ui-primary-hover)',
    },
    content: {
      background: 'var(--ui-bg)',
      hoverBackground: 'var(--ui-bg-panel-hover)',
      borderColor: 'var(--ui-border)',
      borderRadius: 'var(--ui-radius-lg)',
    },
    text: {
      color: 'var(--ui-text)',
      hoverColor: 'var(--ui-text-highlighted)',
      mutedColor: 'var(--ui-text-muted)',
      hoverMutedColor: 'var(--ui-text-toned)',
    },
    formField: {
      background: 'var(--ui-bg)',
      borderColor: 'var(--ui-border)',
      hoverBorderColor: 'var(--ui-border-accented)',
      focusBorderColor: 'var(--ui-primary)',
      color: 'var(--ui-text)',
      placeholderColor: 'var(--ui-text-dimmed)',
      shadow: 'none',
    },
    /* Выпадающие списки телепортируются в body: без этих токенов они рисуются
       стоковой палитрой Aura мимо каркаса. Фон здесь только непрозрачный,
       иначе сквозь открытый список читается страница под ним. Тот же --ui-bg,
       что и у .p-dialog:
       список отделяет рамка, а подсветка строки остаётся видимой поверх. */
    overlay: {
      select: {
        background: 'var(--ui-bg)',
        borderColor: 'var(--ui-border)',
        color: 'var(--ui-text)',
      },
    },
    list: {
      option: {
        color: 'var(--ui-text)',
        focusBackground: 'var(--ui-bg-panel-hover)',
        focusColor: 'var(--ui-text-highlighted)',
        selectedBackground: 'var(--ui-bg-elevated)',
        selectedColor: 'var(--ui-text-highlighted)',
        selectedFocusBackground: 'var(--ui-bg-elevated)',
        selectedFocusColor: 'var(--ui-text-highlighted)',
      },
    },
  },
})

createApp(App)
  .use(createPinia())
  .use(router)
  .use(PrimeVue, {
    theme: { preset, options: { darkModeSelector: '.dark' } },
    // Строки, которые PrimeVue читает экранному диктору и показывает в пустых
    // списках: по умолчанию они английские. Остальные ключи сливаются с его набором.
    locale: {
      passwordPrompt: 'Введите пароль',
      emptyFilterMessage: 'Ничего не найдено',
      emptySearchMessage: 'Ничего не найдено',
      emptyMessage: 'Нет вариантов',
      emptySelectionMessage: 'Ничего не выбрано',
      searchMessage: 'Найдено: {0}',
      selectionMessage: 'Выбрано: {0}',
      aria: {
        close: 'Закрыть',
        expandRow: 'Строка раскрыта',
        collapseRow: 'Строка свёрнута',
        listLabel: 'Список вариантов',
      },
    },
  })
  .mount('#app')
