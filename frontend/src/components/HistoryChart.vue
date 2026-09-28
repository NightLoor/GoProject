<script setup>
import { computed } from 'vue'
const props = defineProps({ samples: { type: Array, default: () => [] } })
const numeric = computed(() => props.samples.filter((s) => typeof s.value === 'number' && Number.isFinite(s.value)))
const points = computed(() => {
  const data = numeric.value
  if (data.length < 2) return ''
  const min = Math.min(...data.map((s) => s.value))
  const max = Math.max(...data.map((s) => s.value))
  const range = max - min || 1
  return data.map((s, i) => `${(i / (data.length - 1)) * 100},${92 - ((s.value - min) / range) * 84}`).join(' ')
})
const min = computed(() => numeric.value.length ? Math.min(...numeric.value.map((s) => s.value)) : '--')
const max = computed(() => numeric.value.length ? Math.max(...numeric.value.map((s) => s.value)) : '--')
function timeText(v) { return v ? new Date(v).toLocaleTimeString('zh-CN', { hour12: false }) : '--' }
</script>

<template>
  <div v-if="numeric.length < 2" class="empty">有效数值样本不足，暂时无法绘制趋势</div>
  <template v-else>
    <div class="chart-wrap">
      <svg viewBox="0 0 100 100" preserveAspectRatio="none" class="chart">
        <polyline :points="points" fill="none" stroke="currentColor" stroke-width="0.8" vector-effect="non-scaling-stroke" />
      </svg>
    </div>
    <div class="chart-meta"><span>Min {{ min }}</span><span>最新 {{ timeText(numeric[numeric.length - 1].timestamp) }}</span><span>Max {{ max }}</span></div>
  </template>
</template>
