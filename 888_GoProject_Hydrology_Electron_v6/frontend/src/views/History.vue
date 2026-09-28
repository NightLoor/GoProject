<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import HistoryChart from '../components/HistoryChart.vue'
import { buildPointMeta, normalizeDevices, formatValue } from '../utils'
import { STATIONS, stationByTag } from '../station'

const points = ref([])
const devices = ref([])
const systemConfig = ref(null)
const selected = ref('')
const stationFilter = ref('BZ')
const category = ref('all')
const startDate = ref('')
const endDate = ref('')
const samples = ref([])
const loading = ref(false)
const error = ref('')
const hasQueried = ref(false)

const MAX_DAYS = 10

const pointMeta = computed(() => buildPointMeta(devices.value))
const selectedMeta = computed(() => pointMeta.value.get(selected.value))
const selectedPoint = computed(() => points.value.find((p) => p.name === selected.value))
const historyPolicy = computed(() => systemConfig.value?.history || {})
const sampleIntervalText = computed(() => {
  const minutes = Number(historyPolicy.value.sample_interval_minutes || 1)
  return minutes === 1 ? '1 分钟' : `${minutes} 分钟`
})
const queryMaxPoints = computed(() => Number(historyPolicy.value.query_max_points || 2000))

const filteredPoints = computed(() => points.value.filter((point) => {
  if (point.history !== true) return false
  const station = stationByTag(point.name)?.code
  if (stationFilter.value !== 'all' && station !== stationFilter.value) return false
  if (category.value === 'pressure' && !/_Point\d+$/.test(point.name)) return false
  if (category.value === 'stage' && !point.name.includes('Stage')) return false
  if (category.value === 'rainfall' && !point.name.includes('Rainfall')) return false
  return true
}))

const numericSamples = computed(() => samples.value.map((s) => Number(s.value)).filter(Number.isFinite))
const stats = computed(() => {
  const values = numericSamples.value
  if (!values.length) return { min: '--', max: '--', avg: '--', count: samples.value.length }
  return {
    min: Math.min(...values),
    max: Math.max(...values),
    avg: values.reduce((a, b) => a + b, 0) / values.length,
    count: samples.value.length,
  }
})

const rangeDays = computed(() => {
  if (!startDate.value || !endDate.value) return null
  const start = parseDateInput(startDate.value)
  const end = parseDateInput(endDate.value)
  if (!start || !end) return null
  return Math.round((end.getTime() - start.getTime()) / 86400000) + 1
})

function pad(n) { return String(n).padStart(2, '0') }
function toInputDate(date) { return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` }
function parseDateInput(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null
  const [year, month, day] = value.split('-').map(Number)
  const date = new Date(year, month - 1, day)
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day ? date : null
}

function initDates() {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - 6)
  startDate.value = toInputDate(start)
  endDate.value = toInputDate(end)
}

function ensureSelection() {
  if (!filteredPoints.value.some((p) => p.name === selected.value)) selected.value = filteredPoints.value[0]?.name || ''
}

function validateRange() {
  error.value = ''
  const start = parseDateInput(startDate.value)
  const end = parseDateInput(endDate.value)
  if (!start || !end) {
    error.value = '请选择有效的开始日期和结束日期。'
    return false
  }
  if (end < start) {
    error.value = '结束日期不能早于开始日期，请重新选择。'
    return false
  }
  const days = Math.round((end.getTime() - start.getTime()) / 86400000) + 1
  if (days > MAX_DAYS) {
    error.value = `查询时间跨度为 ${days} 天，已超过 ${MAX_DAYS} 天，请重新选择日期。`
    return false
  }
  return true
}

async function loadBase() {
  try {
    const [io, config] = await Promise.all([api.getIO(), api.getConfig()])
    points.value = Array.isArray(io) ? io : []
    systemConfig.value = config
    devices.value = normalizeDevices(config, [])
    ensureSelection()
    error.value = ''
  } catch (err) { error.value = err.message || '读取点位失败' }
}

async function query() {
  if (!selected.value) {
    error.value = '请先选择需要查询的监测变量。'
    return
  }
  if (!validateRange()) {
    samples.value = []
    hasQueried.value = false
    return
  }
  loading.value = true
  error.value = ''
  try {
    samples.value = await api.getHistoryRange(selected.value, startDate.value, endDate.value) || []
    hasQueried.value = true
  } catch (err) {
    error.value = err.message || '历史查询失败'
    samples.value = []
    hasQueried.value = false
  } finally { loading.value = false }
}

function changeFilter() {
  ensureSelection()
  samples.value = []
  hasQueried.value = false
}

function pointDescription() { return selectedMeta.value?.description || selectedPoint.value?.description || '监测变量' }
function formatTime(v) { return v ? new Date(v).toLocaleString('zh-CN', { hour12: false }) : '--' }

onMounted(async () => {
  initDates()
  await loadBase()
})
</script>

<template>
  <section class="history-hero">
    <div>
      <span class="hero-kicker">HISTORICAL DATA ANALYSIS</span>
      <h3>历史数据查询</h3>
      <p>历史变量按 {{ sampleIntervalText }} 采样；按站点、变量类型、监测变量和日期范围查询。单次查询最多支持 10 天，超出 {{ queryMaxPoints }} 个点会自动降采样。</p>
    </div>
    <div class="history-hero-value">
      <small>查询周期</small>
      <strong>{{ startDate || '--' }} ～ {{ endDate || '--' }}</strong>
      <em v-if="rangeDays" :class="rangeDays > MAX_DAYS ? 'danger' : 'ok'">{{ rangeDays }} 天</em>
    </div>
  </section>

  <section class="panel history-query-panel">
    <div class="history-query-heading">
      <div>
        <h3>历史查询条件</h3>
        <span>选择一个历史变量，再设置开始和结束日期。</span>
      </div>
      <div class="history-range-badge" :class="{ warning: rangeDays && rangeDays > MAX_DAYS }">
        <span>最大跨度</span><strong>{{ MAX_DAYS }} 天</strong>
      </div>
    </div>

    <div class="history-filter-grid">
      <label>
        <span>监测站点</span>
        <select v-model="stationFilter" @change="changeFilter">
          <option v-for="station in STATIONS" :key="station.code" :value="station.code">{{ station.name }}</option>
          <option value="all">全部站点</option>
        </select>
      </label>
      <label>
        <span>变量类型</span>
        <select v-model="category" @change="changeFilter">
          <option value="all">全部变量</option>
          <option value="pressure">渗压</option>
          <option value="stage">水位</option>
          <option value="rainfall">降雨量</option>
        </select>
      </label>
      <label class="history-point-field">
        <span>监测变量</span>
        <select v-model="selected">
          <option value="">请选择变量</option>
          <option v-for="p in filteredPoints" :key="p.name" :value="p.name">{{ p.name }} · {{ pointMeta.get(p.name)?.description || p.description || '' }}</option>
        </select>
      </label>
      <label>
        <span>开始日期</span>
        <input v-model="startDate" type="date" :max="endDate || undefined" />
      </label>
      <label>
        <span>结束日期</span>
        <input v-model="endDate" type="date" :min="startDate || undefined" />
      </label>
      <div class="history-action">
        <button class="primary large-button" :disabled="loading || !selected" @click="query">
          {{ loading ? '查询中...' : '查询历史数据' }}
        </button>
      </div>
    </div>

    <div class="history-query-tip">
      <span>数据库每 {{ sampleIntervalText }} 保存一条启用 history 的变量记录，结束日期当天数据也会包含；查询结果上限 {{ queryMaxPoints }} 点，超出自动降采样。</span>
      <span v-if="rangeDays && rangeDays <= MAX_DAYS" class="ok">当前 {{ rangeDays }} 天</span>
      <span v-else-if="rangeDays" class="danger">超过 10 天</span>
    </div>
  </section>

  <div v-if="error" class="alert history-error">{{ error }}</div>

  <section class="history-kpi-grid">
    <article><span>变量</span><strong>{{ selected || '--' }}</strong><small>{{ pointDescription() }}</small></article>
    <article><span>当前值</span><strong>{{ formatValue(selectedPoint?.value) }}</strong><small>{{ selectedPoint?.quality || '--' }}</small></article>
    <article><span>历史最小</span><strong>{{ formatValue(stats.min) }}</strong><small>{{ startDate }} ～ {{ endDate }}</small></article>
    <article><span>历史最大</span><strong>{{ formatValue(stats.max) }}</strong><small>查询日期范围内</small></article>
    <article><span>历史平均</span><strong>{{ formatValue(stats.avg) }}</strong><small>{{ stats.count }} 条记录</small></article>
  </section>

  <section class="panel history-chart-panel">
    <div class="panel-title-row">
      <div><h3>趋势曲线</h3><span class="muted">{{ selectedMeta?.device || '--' }} · {{ selectedMeta?.protocol || '--' }}</span></div>
      <div class="history-result-meta">
        <span v-if="hasQueried">{{ samples.length }} 条记录</span>
        <span v-else>等待查询</span>
      </div>
    </div>
    <HistoryChart v-if="hasQueried" :samples="samples" />
    <div v-else class="empty history-empty-state">设置日期范围后点击“查询历史数据”</div>
  </section>

  <section class="panel history-table-panel">
    <div class="panel-title-row">
      <div><h3>历史采样明细</h3><span class="muted">按时间升序查看查询结果</span></div>
      <span class="data-chip">{{ hasQueried ? `${samples.length} 条` : '未查询' }}</span>
    </div>
    <div v-if="samples.length" class="table-wrap history-table enhanced-history-table">
      <table>
        <thead><tr><th>时间</th><th>数值</th><th>质量</th><th>状态</th><th>说明</th></tr></thead>
        <tbody>
          <tr v-for="(s, i) in samples" :key="`${s.timestamp}-${i}`">
            <td>{{ formatTime(s.timestamp) }}</td>
            <td class="value strong-value">{{ formatValue(s.value, 4) }}</td>
            <td><span class="quality" :class="String(s.quality || '').toLowerCase()">{{ s.quality || '--' }}</span></td>
            <td><span class="sample-state" :class="s.quality === 'Good' ? 'normal' : 'bad'">{{ s.quality === 'Good' ? '有效' : '无效' }}</span></td>
            <td>{{ s.error || pointDescription() }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty history-empty-state">{{ hasQueried ? '当前日期范围内暂无历史记录' : '请选择条件后查询历史数据' }}</div>
  </section>
</template>
