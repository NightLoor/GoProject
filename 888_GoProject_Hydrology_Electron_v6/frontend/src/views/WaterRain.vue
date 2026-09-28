<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import LineChart from '../components/LineChart.vue'
import { filterPointsByStation, siteTag, formatValue, buildPointMeta, normalizeDevices } from '../utils'
import { STATIONS, selectedStationCode, stationByCode, setSelectedStation } from '../station'

const points = ref([])
const devices = ref([])
const histories = ref({ stage: [], rainfall: [] })
const loading = ref(true)
const historyLoading = ref(false)
const error = ref('')
let timer = null

const station = computed(() => stationByCode(selectedStationCode.value))
const stationPoints = computed(() => filterPointsByStation(points.value, selectedStationCode.value))
const stagePoint = computed(() => siteTag(points.value, selectedStationCode.value, 'Stage'))
const rainfallPoint = computed(() => siteTag(points.value, selectedStationCode.value, 'Rainfall'))
const stageMeta = computed(() => buildPointMeta(devices.value).get(stagePoint.value?.name))
const rainfallMeta = computed(() => buildPointMeta(devices.value).get(rainfallPoint.value?.name))
const stationQuality = computed(() => stationPoints.value.filter((p) => p.quality === 'Good').length)
const pointMetaLabel = (name) => {
  const value = String(name || '')
  if (value.includes('Stage')) return '水位'
  if (value.includes('Rainfall')) return '降雨量'
  const match = value.match(/Point(\d+)/)
  return match ? `渗压测点${match[1]}` : '监测点'
}

async function loadBase() {
  try {
    const [io, config, status] = await Promise.all([api.getIO(), api.getConfig(), api.getDevices()])
    points.value = Array.isArray(io) ? io : []
    devices.value = normalizeDevices(config, status)
    error.value = ''
  } catch (err) { error.value = err.message || '读取监测数据失败' }
  finally { loading.value = false }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const calls = []
    if (stagePoint.value?.name) calls.push(api.getHistory(stagePoint.value.name, 120).then((v) => ({ key: 'stage', value: v || [] })))
    if (rainfallPoint.value?.name) calls.push(api.getHistory(rainfallPoint.value.name, 120).then((v) => ({ key: 'rainfall', value: v || [] })))
    const results = await Promise.all(calls)
    const next = { stage: [], rainfall: [] }
    for (const result of results) next[result.key] = result.value
    histories.value = next
  } catch (err) { error.value = err.message || '读取水雨情历史失败' }
  finally { historyLoading.value = false }
}

async function refresh() {
  await loadBase()
  await loadHistory()
}

function selectStation(code) {
  setSelectedStation(code)
  refresh()
}

function formatTime(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--' }

onMounted(() => { refresh(); timer = setInterval(loadBase, 2000) })
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div class="station-tabs">
    <button v-for="item in STATIONS" :key="item.code" :class="{ active: selectedStationCode === item.code }" @click="selectStation(item.code)">{{ item.name }}</button>
  </div>

  <section class="hero-panel water-hero">
    <div><span class="hero-kicker">WATER · RAINFALL MONITOR</span><h3>{{ station.name }} · 水雨情监测</h3><p>基于 OPC DA 实时点位监测当前水位、降雨量及通讯质量。</p></div>
    <div class="hero-status"><span class="pulse-dot"></span><strong>{{ stationQuality }}/{{ stationPoints.length }}</strong><small>有效点位</small></div>
  </section>

  <section v-if="error" class="alert">{{ error }}</section>
  <section v-if="loading" class="empty">正在读取 OPC DA 实时数据...</section>
  <template v-else>
    <section class="water-card-grid">
      <article class="metric-panel water-level">
        <div class="metric-label"><span>当前水位</span><em>{{ stagePoint?.quality || '--' }}</em></div>
        <strong>{{ formatValue(stagePoint?.value) }}</strong>
        <small>{{ stageMeta?.description || '水位监测点' }}</small>
        <div class="metric-foot">更新时间 {{ formatTime(stagePoint?.timestamp) }}</div>
      </article>
      <article class="metric-panel rainfall">
        <div class="metric-label"><span>当前降雨量</span><em>{{ rainfallPoint?.quality || '--' }}</em></div>
        <strong>{{ formatValue(rainfallPoint?.value) }}</strong>
        <small>{{ rainfallMeta?.description || '降雨监测点' }}</small>
        <div class="metric-foot">更新时间 {{ formatTime(rainfallPoint?.timestamp) }}</div>
      </article>
      <article class="summary-panel">
        <span>采集概况</span>
        <div><b>{{ stationPoints.length }}</b><small>站点点位</small></div>
        <div><b>{{ stationPoints.filter((p) => p.quality === 'Good').length }}</b><small>Good</small></div>
        <div><b>{{ stationPoints.filter((p) => p.quality === 'Bad').length }}</b><small>Bad</small></div>
      </article>
    </section>

    <section class="dual-panel-grid">
      <article class="panel analysis-panel">
        <div class="panel-title-row"><div><h3>水位趋势</h3><span class="muted">最近 120 个采样点</span></div><span class="data-chip">{{ stagePoint?.name || '--' }}</span></div>
        <div v-if="historyLoading" class="empty small-empty">正在读取趋势...</div>
        <LineChart v-else :samples="histories.stage" empty-text="暂无水位历史数据" />
      </article>
      <article class="panel analysis-panel">
        <div class="panel-title-row"><div><h3>降雨量趋势</h3><span class="muted">最近 120 个采样点</span></div><span class="data-chip">{{ rainfallPoint?.name || '--' }}</span></div>
        <div v-if="historyLoading" class="empty small-empty">正在读取趋势...</div>
        <LineChart v-else :samples="histories.rainfall" empty-text="暂无降雨历史数据" />
      </article>
    </section>

    <section class="panel">
      <div class="panel-title-row"><div><h3>站点点位状态</h3><span class="muted">水位 / 降雨量 / 渗压测点</span></div><button class="secondary" @click="refresh">刷新</button></div>
      <div class="tag-strip">
        <div v-for="point in stationPoints" :key="point.name" class="tag-mini">
          <div><span>{{ point.name }}</span><small>{{ pointMetaLabel(point.name) }}</small></div>
          <strong>{{ formatValue(point.value) }}</strong>
          <em :class="point.quality === 'Good' ? 'good' : 'bad'">{{ point.quality }}</em>
        </div>
      </div>
    </section>
  </template>
</template>

