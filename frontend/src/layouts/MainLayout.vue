<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { STATIONS, selectedStationCode, setSelectedStation } from '../station'

const route = useRoute()
const now = ref(new Date())
let clockTimer = null

const pageTitle = computed(() => ({
  '/water-rain': '水雨情监测',
  '/seepage': '渗压分析',
  '/alarms': '设备变量报警',
  '/history': '历史数据查询',
}[route.path] || '排涝站监测'))
const stationName = computed(() => STATIONS.find((x) => x.code === selectedStationCode.value)?.name || '全部站点')
const displayDate = computed(() => now.value.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'short' }))
const displayClock = computed(() => now.value.toLocaleTimeString('zh-CN', { hour12: false }))

function chooseStation(code) { setSelectedStation(code) }

onMounted(() => { clockTimer = setInterval(() => { now.value = new Date() }, 1000) })
onBeforeUnmount(() => clearInterval(clockTimer))
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
        <div class="header-weather-mark">◷</div>
        <div class="header-date"><span>{{ displayDate }}</span><small>系统日期</small></div>
        <div class="header-clock">{{ displayClock }}</div>
      </div>
    </header>

    <div class="dashboard-workspace">
      <aside class="dashboard-sidebar">
        <div class="sidebar-section-label">排涝站选择 <span>STATIONS</span></div>
        <nav class="station-selector" aria-label="排涝站选择">
          <button v-for="(station, index) in STATIONS" :key="station.code"
            class="station-select-item" :class="{ active: selectedStationCode === station.code }"
            @click="chooseStation(station.code)">
            <span class="station-select-index">0{{ index + 1 }}</span>
            <span class="station-select-copy"><strong>{{ station.name }}</strong><small>{{ station.shortName }}排涝站</small></span>
            <span class="station-select-check">{{ selectedStationCode === station.code ? '✓' : '›' }}</span>
          </button>
        </nav>

        <div class="sidebar-section-label business-label">业务功能 <span>MONITOR</span></div>
        <nav class="dashboard-nav">
          <RouterLink to="/water-rain" class="nav-item"><span class="nav-icon">◉</span><span class="nav-copy"><strong>水雨情监测</strong><small>Water & Rainfall</small></span><span class="nav-arrow">›</span></RouterLink>
          <RouterLink to="/seepage" class="nav-item"><span class="nav-icon">⌁</span><span class="nav-copy"><strong>渗压分析</strong><small>Seepage Analysis</small></span><span class="nav-arrow">›</span></RouterLink>
          <RouterLink to="/alarms" class="nav-item"><span class="nav-icon">!</span><span class="nav-copy"><strong>设备变量报警</strong><small>Alarm Center</small></span><span class="nav-arrow">›</span></RouterLink>
          <RouterLink to="/history" class="nav-item"><span class="nav-icon">▤</span><span class="nav-copy"><strong>历史数据查询</strong><small>Historical Data</small></span><span class="nav-arrow">›</span></RouterLink>
        </nav>

        <div class="sidebar-footer"><span class="footer-status-dot"></span> 数据采集与监控系统 <small>OPC DA · Modbus TCP</small></div>
      </aside>

      <main class="dashboard-main">
        <div class="dashboard-page-heading">
          <div class="page-heading-left">
            <div class="page-breadcrumb"><span>监测中心</span><b>/</b><strong>{{ pageTitle }}</strong></div>
            <h2>{{ pageTitle }}</h2>
            <p>当前监测站点：<strong>{{ stationName }}</strong><span class="heading-divider">·</span>实时监测数据</p>
          </div>
          <div class="page-heading-status"><span class="heading-status-icon">⌖</span><div><strong>{{ stationName }}</strong><small>当前选中站点</small></div></div>
        </div>
        <div class="dashboard-page-content"><RouterView /></div>
        <footer class="dashboard-footer"><span>智慧水务监测平台</span><span>实时采集 · 数据分析 · 报警管理</span></footer>
      </main>
    </div>
  </div>
</template>
