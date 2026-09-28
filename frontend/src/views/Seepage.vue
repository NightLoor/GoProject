<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import LineChart from '../components/LineChart.vue'
import { filterPointsByStation, sitePressurePoints, siteTag, formatValue } from '../utils'
import { STATIONS, selectedStationCode, stationByCode, setSelectedStation } from '../station'

const points = ref([])
const selectedTag = ref('')
const history = ref([])
const loading = ref(true)
const historyLoading = ref(false)
const error = ref('')
let timer = null

const station = computed(() => stationByCode(selectedStationCode.value))
const stationPoints = computed(() => filterPointsByStation(points.value, selectedStationCode.value))
const pressurePoints = computed(() => sitePressurePoints(points.value, selectedStationCode.value))
const stage = computed(() => siteTag(points.value, selectedStationCode.value, 'Stage'))
const selectedPoint = computed(() => pressurePoints.value.find((p) => p.name === selectedTag.value) || pressurePoints.value[0])
const selectedHistoryValues = computed(() => history.value.map((s) => Number(s.value)).filter(Number.isFinite))
const analysis = computed(() => {
  const values = selectedHistoryValues.value
  if (!values.length) return { min: '--', max: '--', avg: '--', range: '--' }
  const min = Math.min(...values); const max = Math.max(...values); const avg = values.reduce((a, b) => a + b, 0) / values.length
  return { min, max, avg, range: max - min }
})

async function loadBase() {
  try { points.value = await api.getIO() || []; error.value = '' }
  catch (err) { error.value = err.message || '读取渗压数据失败' }
  finally { loading.value = false }
}
async function loadHistory() {
  const target = selectedPoint.value
  if (!target?.name) return
  historyLoading.value = true
  try { history.value = await api.getHistory(target.name, 180) || [] }
  catch (err) { error.value = err.message || '读取渗压历史失败'; history.value = [] }
  finally { historyLoading.value = false }
}
async function refresh() { await loadBase(); await loadHistory() }
function selectStation(code) { setSelectedStation(code); selectedTag.value = ''; refresh() }
function selectPoint(name) { selectedTag.value = name; loadHistory() }
function qualityClass(point) { return point?.quality === 'Good' ? 'good' : 'bad' }

onMounted(() => { refresh(); timer = setInterval(loadBase, 2000) })
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div class="station-tabs">
    <button v-for="item in STATIONS" :key="item.code" :class="{ active: selectedStationCode === item.code }" @click="selectStation(item.code)">{{ item.name }}</button>
  </div>

  <section class="hero-panel pressure-hero">
    <div><span class="hero-kicker">SEEPAGE PRESSURE ANALYSIS</span><h3>{{ station.name }} · 渗压分析</h3><p>8 个渗压监测点实时对比，结合历史样本观察趋势变化。</p></div>
    <div class="hero-side-kpi"><small>当前水位</small><strong>{{ formatValue(stage?.value) }}</strong></div>
  </section>

  <section v-if="error" class="alert">{{ error }}</section>
  <section v-if="loading" class="empty">正在读取渗压监测数据...</section>
  <template v-else>
    <section class="pressure-grid">
      <button v-for="(point, index) in pressurePoints" :key="point.name" class="pressure-card" :class="{ selected: selectedPoint?.name === point.name }" @click="selectPoint(point.name)">
        <div class="pressure-head"><span>测点 {{ index + 1 }}</span><em :class="qualityClass(point)">{{ point.quality }}</em></div>
        <strong>{{ formatValue(point.value) }}</strong>
        <small>{{ point.name }}</small>
      </button>
    </section>

    <section class="dual-panel-grid seepage-main">
      <article class="panel analysis-panel">
        <div class="panel-title-row"><div><h3>{{ selectedPoint?.name || '请选择渗压测点' }}</h3><span class="muted">历史趋势分析 · 最近 180 个采样点</span></div><span class="data-chip">{{ selectedPoint?.quality || '--' }}</span></div>
        <div v-if="historyLoading" class="empty small-empty">正在读取渗压历史...</div>
        <LineChart v-else :samples="history" :height="300" empty-text="暂无该测点历史样本" />
      </article>

      <article class="panel analysis-panel analysis-side">
        <div class="panel-title-row"><div><h3>统计分析</h3><span class="muted">基于当前测点历史样本</span></div></div>
        <div class="analysis-list">
          <div><span>当前值</span><strong>{{ formatValue(selectedPoint?.value) }}</strong></div>
          <div><span>历史最小</span><strong>{{ formatValue(analysis.min) }}</strong></div>
          <div><span>历史平均</span><strong>{{ formatValue(analysis.avg) }}</strong></div>
          <div><span>历史最大</span><strong>{{ formatValue(analysis.max) }}</strong></div>
          <div><span>波动范围</span><strong>{{ formatValue(analysis.range) }}</strong></div>
        </div>
        <div class="analysis-note">当前页面只基于点位实际采集值做趋势与统计展示；阈值、单位和报警判定以现场配置为准。</div>
      </article>
    </section>

    <section class="panel">
      <div class="panel-title-row"><div><h3>渗压点位明细</h3><span class="muted">数据源：OPC DA Tags.csv</span></div><span class="muted">共 {{ pressurePoints.length }} 个测点</span></div>
      <div class="pressure-table">
        <table><thead><tr><th>测点</th><th>当前值</th><th>质量</th><th>更新时间</th><th>ItemID</th></tr></thead>
          <tbody><tr v-for="(point, index) in pressurePoints" :key="point.name"><td><button class="link-btn" @click="selectPoint(point.name)">测点 {{ index + 1 }} · {{ point.name }}</button></td><td class="value">{{ formatValue(point.value) }}</td><td><span class="quality" :class="qualityClass(point)">{{ point.quality }}</span></td><td>{{ point.timestamp ? new Date(point.timestamp).toLocaleString('zh-CN', { hour12: false }) : '--' }}</td><td><small>{{ point.item_id || '--' }}</small></td></tr></tbody>
        </table>
      </div>
    </section>
  </template>
</template>
