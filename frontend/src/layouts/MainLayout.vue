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
let timer = null

const pageTitle = computed(() => ({
  '/water-rain': '水雨情监测',
  '/seepage': '渗压分析',
  '/alarms': '设备变量报警',
  '/history': '历史数据查询',
}[route.path] || '排涝站监测'))
const connectedCount = computed(() => devices.value.filter((d) => d.connected).length)
const goodCount = computed(() => points.value.filter((p) => p.quality === 'Good').length)

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

onMounted(() => { refreshStatus(); timer = setInterval(refreshStatus, 3000) })
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div class="app-shell hydrology-shell">
    <header class="topbar hydrology-topbar">
      <div class="brand-block">
        <div class="brand-mark">水</div>
        <div><div class="eyebrow">DRAINAGE STATION MONITORING PLATFORM</div><h1>排涝站水雨情与渗压监测平台</h1><p>OPC DA · Modbus TCP · 实时采集与趋势分析</p></div>
      </div>
      <div class="top-actions">
        <div class="system-health"><span class="pulse-dot"></span><div><strong>{{ connectedCount }}/{{ devices.length || 0 }} 在线</strong><small>{{ goodCount }} 个有效点位</small></div></div>
        <div class="clock">{{ formatTime(lastUpdated) }}</div>
      </div>
    </header>

    <div class="station-bar">
      <div class="station-bar-label">监测站点</div>
      <button v-for="station in STATIONS" :key="station.code" class="station-tab" :class="{ active: selectedStationCode === station.code }" @click="chooseStation(station.code)"><span>{{ station.code }}</span>{{ station.name }}</button>
      <div class="station-bar-right">{{ error || '数据接口正常' }}</div>
    </div>

    <div class="shell-body">
      <aside class="sidebar">
        <div class="sidebar-title">监测功能</div>
        <nav>
          <RouterLink to="/water-rain" class="nav-item"><span class="nav-icon">◉</span><span>水雨情监测</span></RouterLink>
          <RouterLink to="/seepage" class="nav-item"><span class="nav-icon">≈</span><span>渗压分析</span></RouterLink>
          <RouterLink to="/alarms" class="nav-item"><span class="nav-icon">!</span><span>设备变量报警</span></RouterLink>
          <RouterLink to="/history" class="nav-item"><span class="nav-icon">↗</span><span>历史数据</span></RouterLink>
        </nav>
        <div class="sidebar-status">
          <div class="status-label">系统状态</div>
          <div class="status-row"><span>设备在线</span><strong>{{ connectedCount }}/{{ devices.length }}</strong></div>
          <div class="status-row"><span>点位 Good</span><strong>{{ goodCount }}/{{ points.length }}</strong></div>
          <div class="status-row"><span>刷新周期</span><strong>3 s</strong></div>
        </div>
      </aside>

      <main class="page-content">
        <div class="page-heading"><div><div class="eyebrow">LIVE MONITORING</div><h2>{{ pageTitle }}</h2><span class="muted">当前站点：{{ STATIONS.find((x) => x.code === selectedStationCode)?.name }}</span></div></div>
        <RouterView />
      </main>
    </div>
  </div>
</template>
