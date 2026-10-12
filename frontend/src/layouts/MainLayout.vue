<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { api } from '../api'
import { normalizeDevices } from '../utils'
import { STATIONS, selectedStationCode, setSelectedStation } from '../station'

const route = useRoute()
const devices = ref([])
const points = ref([])
const error = ref('')
const lastUpdated = ref(null)
const now = ref(new Date())
let timer = null
let clockTimer = null

const pageTitle = computed(() => ({
  '/water-rain': '水雨情监测',
  '/seepage': '渗压分析',
  '/alarms': '设备变量报警',
  '/history': '历史数据查询',
}[route.path] || '排涝站监测'))
const connectedCount = computed(() => devices.value.filter((d) => d.connected).length)
const goodCount = computed(() => points.value.filter((p) => p.quality === 'Good').length)
const stationName = computed(() => STATIONS.find((x) => x.code === selectedStationCode)?.name || '全部站点')
const displayDate = computed(() => now.value.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'short' }))
const displayClock = computed(() => now.value.toLocaleTimeString('zh-CN', { hour12: false }))

async function refreshStatus() {
  try {
    const [config, status, io] = await Promise.all([api.getConfig(), api.getDevices(), api.getIO()])
    devices.value = normalizeDevices(config, status)
    points.value = Array.isArray(io) ? io : []
    error.value = ''
    lastUpdated.value = new Date()
  } catch (err) { error.value = err.message || '后端连接异常' }
}

function chooseStation(code) { setSelectedStation(code) }
function formatTime(value) { return value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '--' }

onMounted(() => {
  refreshStatus()
  timer = setInterval(refreshStatus, 3000)
  clockTimer = setInterval(() => { now.value = new Date() }, 1000)
})
onBeforeUnmount(() => { clearInterval(timer); clearInterval(clockTimer) })
</script>

<template>
  <div class="app-shell hydrology-shell dashboard-layout">
    <header class="dashboard-header">
      <div class="dashboard-brand">
        <div class="brand-mark"><span>水</span></div>
        <div class="brand-copy">
          <div class="brand-eyebrow">WATER RESOURCES · INTELLIGENT MONITORING</div>
          <h1>智慧水务监测平台</h1>
          <p>排涝站水雨情 · 渗压分析 · 设备运行监控</p>
        </div>
      </div>
      <div class="dashboard-header-center"><span class="header-live-dot"></span><span>实时监测中心</span></div>
      <div class="dashboard-header-meta">
        <div class="header-weather-mark">☀</div>
        <div class="header-date"><span>{{ displayDate }}</span><small>系统日期</small></div>
        <div class="header-clock">{{ displayClock }}</div>
      </div>
    </header>

    <div class="dashboard-stationbar">
      <div class="stationbar-heading"><span class="stationbar-icon">⌖</span><div><strong>监测站点</strong><small>MONITORING STATIONS</small></div></div>
      <div class="stationbar-tabs">
        <button v-for="station in STATIONS" :key="station.code" class="station-tab" :class="{ active: selectedStationCode === station.code }" @click="chooseStation(station.code)">
          <span class="station-tab-code">{{ station.code }}</span>{{ station.name }}
        </button>
      </div>
      <div class="stationbar-health" :class="{ offline: error }"><i></i><span>{{ error ? '接口异常' : '数据接口正常' }}</span><small>最近更新 {{ formatTime(lastUpdated) }}</small></div>
    </div>

    <div class="dashboard-workspace">
      <aside class="dashboard-sidebar">
        <div class="sidebar-section-label">监测业务 <span>MONITOR</span></div>
        <nav class="dashboard-nav">
          <RouterLink to="/water-rain" class="nav-item"><span class="nav-icon">◉</span><span class="nav-copy"><strong>水雨情监测</strong><small>Water & Rainfall</small></span><span class="nav-arrow">›</span></RouterLink>
          <RouterLink to="/seepage" class="nav-item"><span class="nav-icon">⌁</span><span class="nav-copy"><strong>渗压分析</strong><small>Seepage Analysis</small></span><span class="nav-arrow">›</span></RouterLink>
          <RouterLink to="/alarms" class="nav-item"><span class="nav-icon">!</span><span class="nav-copy"><strong>设备变量报警</strong><small>Alarm Center</small></span><span class="nav-arrow">›</span></RouterLink>
          <RouterLink to="/history" class="nav-item"><span class="nav-icon">▤</span><span class="nav-copy"><strong>历史数据查询</strong><small>Historical Data</small></span><span class="nav-arrow">›</span></RouterLink>
        </nav>

        <div class="sidebar-section-label system-label">系统概况 <span>STATUS</span></div>
        <section class="sidebar-system-card">
          <div class="system-card-title"><span class="system-pulse"></span><strong>采集服务状态</strong><small>{{ error ? '异常' : '运行中' }}</small></div>
          <div class="system-mini-metrics">
            <div><span>设备在线</span><strong>{{ connectedCount }}<small> / {{ devices.length }}</small></strong></div>
            <div><span>有效点位</span><strong>{{ goodCount }}<small> / {{ points.length }}</small></strong></div>
          </div>
          <div class="system-refresh-row"><span>自动刷新周期</span><strong>3 秒</strong></div>
          <div v-if="error" class="sidebar-error">{{ error }}</div>
        </section>
        <div class="sidebar-footer"><span class="footer-status-dot"></span> 数据采集与监控系统 <small>OPC DA · Modbus TCP</small></div>
      </aside>

      <main class="dashboard-main">
        <div class="dashboard-page-heading">
          <div class="page-heading-left">
            <div class="page-breadcrumb"><span>监测中心</span><b>/</b><strong>{{ pageTitle }}</strong></div>
            <h2>{{ pageTitle }}</h2>
            <p>当前监测范围：<strong>{{ stationName }}</strong><span class="heading-divider">·</span>实时数据自动刷新</p>
          </div>
          <div class="page-heading-status"><span class="heading-status-icon">◷</span><div><strong>{{ formatTime(lastUpdated) }}</strong><small>最近数据更新时间</small></div></div>
        </div>
        <div class="dashboard-page-content"><RouterView /></div>
        <footer class="dashboard-footer"><span>智慧水务监测平台</span><span>实时采集 · 数据分析 · 报警管理</span></footer>
      </main>
    </div>
  </div>
</template>
