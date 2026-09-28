<script setup>
import { computed } from 'vue'

const props = defineProps({
  samples: { type: Array, default: () => [] },
  height: { type: Number, default: 220 },
  emptyText: { type: String, default: '暂无有效历史数据' },
})

const numeric = computed(() => props.samples
  .map((sample) => ({ ...sample, value: Number(sample.value) }))
  .filter((sample) => Number.isFinite(sample.value)))

const stats = computed(() => {
  if (!numeric.value.length) return { min: '--', max: '--', latest: '--' }
  const values = numeric.value.map((item) => item.value)
  return {
    min: Math.min(...values),
    max: Math.max(...values),
    latest: values[values.length - 1],
  }
})

const points = computed(() => {
  const data = numeric.value
  if (data.length < 2) return ''
  const min = Math.min(...data.map((s) => s.value))
  const max = Math.max(...data.map((s) => s.value))
  const range = max - min || 1
  return data.map((s, i) => `${(i / (data.length - 1)) * 100},${92 - ((s.value - min) / range) * 78}`).join(' ')
})

function timeText(v) {
  return v ? new Date(v).toLocaleTimeString('zh-CN', { hour12: false }).slice(0, 5) : '--'
}
</script>

<template>
  <div v-if="numeric.length < 2" class="chart-empty">{{ emptyText }}</div>
  <div v-else>
    <div class="line-chart" :style="{ height: `${height}px` }">
      <div class="chart-gridline g1"></div>
      <div class="chart-gridline g2"></div>
      <div class="chart-gridline g3"></div>
      <svg viewBox="0 0 100 100" preserveAspectRatio="none">
        <polyline :points="points" fill="none" stroke="currentColor" stroke-width="1.2" vector-effect="non-scaling-stroke" />
      </svg>
    </div>
    <div class="chart-axis"><span>{{ timeText(numeric[0]?.timestamp) }}</span><span>{{ timeText(numeric[Math.floor(numeric.length / 2)]?.timestamp) }}</span><span>{{ timeText(numeric[numeric.length - 1]?.timestamp) }}</span></div>
    <div class="chart-kpis"><span>最小 {{ Number(stats.min).toFixed(3) }}</span><span>最新 {{ Number(stats.latest).toFixed(3) }}</span><span>最大 {{ Number(stats.max).toFixed(3) }}</span></div>
  </div>
</template>
