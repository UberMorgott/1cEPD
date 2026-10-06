<script setup lang="ts">
import { computed } from 'vue'

/**
 * Столбик месяца. value null — данных за месяц нет (догружается или 1С их
 * не отдала), тогда рисуется пунктирная заглушка, а note объясняет почему.
 * over — перерасход: столбик красный, а в подсказке слово, не только цвет.
 */
export interface MonthBar {
  key: string
  label: string
  value: number | null
  note?: string
  over?: boolean
}

const props = defineProps<{
  bars: MonthBar[]
  /** Горизонтальная линия лимита; null — лимита нет. */
  limit?: number | null
  /** Подпись величины в подсказке и таблице: «пакетов». */
  unit: string
  /** Название графика для чтения с экрана. */
  title: string
}>()

const scale = computed(() =>
  Math.max(1, props.limit ?? 0, ...props.bars.map((bar) => bar.value ?? 0)),
)

/** Подписываем не каждый столбик, а только последний и самый высокий. */
const labelled = computed(() => {
  const withValue = props.bars.filter((bar) => bar.value !== null)
  const last = withValue.at(-1)
  const peak = withValue.reduce<MonthBar | undefined>(
    (best, bar) => (best === undefined || (bar.value ?? 0) > (best.value ?? 0) ? bar : best),
    undefined,
  )
  return new Set([last?.key, peak?.key])
})

function height(value: number | null): string {
  return `${((value ?? 0) / scale.value) * 100}%`
}

function tip(bar: MonthBar): string {
  const value = bar.value === null ? 'нет данных' : `${bar.value} ${props.unit}`
  return [bar.label, value, bar.note].filter(Boolean).join(' · ')
}

/** Подпись «сент 25» делится на месяц и год: узкой оси год прячется. */
function monthPart(label: string): string {
  const cut = label.lastIndexOf(' ')
  return cut > 0 ? label.slice(0, cut) : label
}

function yearPart(label: string): string {
  const cut = label.lastIndexOf(' ')
  return cut > 0 ? label.slice(cut) : ''
}
</script>

<template>
  <figure class="month-bars">
    <div
      class="plot"
      role="img"
      :aria-label="title"
    >
      <div class="area">
        <div
          v-if="limit"
          class="limit"
          :style="{ bottom: height(limit) }"
        >
          <span>лимит {{ limit }}</span>
        </div>
        <div
          v-for="bar in bars"
          :key="bar.key"
          class="month-col"
          tabindex="0"
          :aria-label="tip(bar)"
        >
          <span
            v-if="bar.value !== null && labelled.has(bar.key)"
            class="value"
            :style="{ bottom: height(bar.value) }"
          >{{ bar.value }}</span>
          <div
            v-if="bar.value !== null"
            class="mark"
            :class="{ over: bar.over }"
            :style="{ height: height(bar.value) }"
          />
          <div
            v-else
            class="mark missing"
          />
          <span class="tip">{{ tip(bar) }}</span>
        </div>
      </div>
    </div>
    <div class="axis">
      <span
        v-for="bar in bars"
        :key="bar.key"
      >{{ monthPart(bar.label) }}<span class="year">{{ yearPart(bar.label) }}</span></span>
    </div>
    <details class="as-table">
      <summary>Таблицей</summary>
      <table>
        <tbody>
          <tr
            v-for="bar in bars"
            :key="bar.key"
          >
            <th>{{ bar.label }}</th>
            <td>{{ bar.value ?? '—' }}</td>
            <td>{{ bar.note }}</td>
          </tr>
        </tbody>
      </table>
    </details>
  </figure>
</template>

<style scoped>
.month-bars {
  margin: 0;
  max-width: 40rem;
  container-type: inline-size;
}

/* Сверху запас под подпись самого высокого столбика. */
.plot {
  padding-top: 16px;
  border-bottom: 1px solid var(--ui-border-accented);
}

.area {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 80px;
}

.month-col {
  position: relative;
  display: flex;
  flex: 1;
  align-items: flex-end;
  justify-content: center;
  height: 100%;
  outline: none;
}

.mark {
  width: min(100%, 28px);
  min-height: 2px;
  border-radius: 4px 4px 0 0;
  background: var(--ui-primary);
}

.mark.over {
  background: var(--ui-error);
}

/* Нет данных — пунктир, а не нулевой столбик: ноль и «не знаем» — разное. */
.mark.missing {
  height: 24px;
  border: 1px dashed var(--ui-text-dimmed);
  border-bottom: 0;
  background: transparent;
}

.month-col:hover .mark:not(.missing),
.month-col:focus-visible .mark:not(.missing) {
  filter: brightness(1.15);
}

.value {
  position: absolute;
  margin-bottom: 2px;
  font-size: 11px;
  line-height: 14px;
  color: var(--ui-text-muted);
  font-variant-numeric: tabular-nums;
}

.tip {
  position: absolute;
  bottom: calc(100% + 4px);
  z-index: 2;
  display: none;
  padding: 4px 8px;
  border: 1px solid var(--ui-border-accented);
  border-radius: var(--ui-radius-md);
  background: var(--ui-bg-elevated);
  color: var(--ui-text);
  font-size: 12px;
  line-height: 16px;
  white-space: nowrap;
  pointer-events: none;
}

.month-col:hover .tip,
.month-col:focus-visible .tip {
  display: block;
}

.limit {
  position: absolute;
  right: 0;
  left: 0;
  border-top: 1px dashed var(--ui-text-dimmed);
  pointer-events: none;
}

.limit span {
  position: absolute;
  right: 0;
  bottom: 2px;
  font-size: 11px;
  line-height: 14px;
  color: var(--ui-text-dimmed);
}

.axis {
  display: flex;
  gap: 2px;
  margin-top: 4px;
}

.axis span {
  flex: 1;
  overflow: hidden;
  font-size: 11px;
  line-height: 14px;
  text-align: center;
  color: var(--ui-text-muted);
  white-space: nowrap;
}

/* На телефоне двенадцать подписей «сент 25» не влезают и обрезались в «окт 2»,
   «дек 1». Год остаётся в подсказке и в таблице. */
@container (width < 36rem) {
  .axis span {
    font-size: 10px;
  }

  .axis .year {
    display: none;
  }
}

.as-table {
  margin-top: 8px;
  font-size: 12px;
  color: var(--ui-text-muted);
}

.as-table summary {
  cursor: pointer;
}

.as-table table {
  margin-top: 4px;
  border-collapse: collapse;
  font-variant-numeric: tabular-nums;
}

.as-table th,
.as-table td {
  padding: 2px 12px 2px 0;
  font-weight: 400;
  text-align: left;
}
</style>
