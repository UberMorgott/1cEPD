<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

/**
 * Хвост бесконечного списка. Следит за собой через IntersectionObserver с запасом
 * в экран: следующая порция просится заранее, до того как пользователь дошёл до конца.
 */
const props = withDefaults(
  defineProps<{
    hasMore: boolean
    shown: number
    total: number
    /** Запас до конца списка, на котором грузим дальше (rootMargin снизу). */
    buffer?: string
  }>(),
  { buffer: '100%' },
)

const emit = defineEmits<{ more: [] }>()

const el = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      if (entries.at(-1)?.isIntersecting && props.hasMore) emit('more')
    },
    { rootMargin: `0px 0px ${props.buffer} 0px` },
  )
  if (el.value) observer.observe(el.value)
})

onBeforeUnmount(() => observer?.disconnect())

// Порция дорисовалась, а хвост всё ещё в зоне запаса (короткие строки, быстрая
// прокрутка) — наблюдатель молчит, пока состояние не сменилось. Повторное observe
// всегда присылает текущее состояние, поэтому переподписываемся.
watch(
  () => [props.shown, props.hasMore],
  async () => {
    await nextTick()
    if (!el.value || !observer) return
    observer.unobserve(el.value)
    observer.observe(el.value)
  },
)
</script>

<template>
  <div
    ref="el"
    class="tail"
    role="status"
  >
    <template v-if="hasMore">
      <i class="pi pi-spin pi-spinner" />
      <span>Показано {{ shown }} из {{ total }}, подгружаем…</span>
    </template>
    <!-- Короткий список виден целиком и так: итог нужен, только когда листали. -->
    <span v-else-if="total > 20">Показаны все {{ total }}</span>
  </div>
</template>

<style scoped>
.tail {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 1px;
  padding: 4px 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--ui-text-muted);
}
</style>
