<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import { STATIONS, stationByTag, setSelectedStation } from '../station'
import { formatValue } from '../utils'
import AlarmTable from '../components/AlarmTable.vue'

const activeAlarms = ref([])
const alarmConfig = ref({ enabled: true, alarms: [] })
const events = ref([])
const points = ref([])
const draftRules = ref([])
const loading = ref(true)
const saving = ref(false)
const dirty = ref(false)
const error = ref('')
const saveMessage = ref('')
const stationFilter = ref('all')
const statusFilter = ref('all')
const search = ref('')
const alarmHistoryEvents = ref([])
const alarmHistoryTag = ref('')
const alarmHistoryStartDate = ref('')
const alarmHistoryEndDate = ref('')
const alarmHistoryLoading = ref(false)
const alarmHistoryQueried = ref(false)
let timer = null

const pointMap = computed(() => new Map(points.value.map((p) => [p.name, p])))
const activeMap = computed(() => new Map(activeAlarms.value.map((a) => [a.tag, a])))

const filteredRules = computed(() => {
  const q = search.value.trim().toLowerCase()
  return draftRules.value.filter((rule) => {
    const station = stationByTag(rule.tag)?.code
    if (stationFilter.value !== 'all' && station !== stationFilter.value) return false
    const active = activeMap.value.has(rule.tag)
    const numericConfigured = rule.data_type === 'bool'
      ? rule.alarm_value !== null && rule.alarm_value !== undefined
      : rule.low !== null && rule.low !== undefined || rule.high !== null && rule.high !== undefined
    if (statusFilter.value === 'active' && !active) return false
    if (statusFilter.value === 'configured' && !(rule.enabled && numericConfigured)) return false
    if (!q) return true
    const point = pointMap.value.get(rule.tag)
    return JSON.stringify({
      tag: rule.tag,
      data_type: rule.data_type,
      value: point?.value,
      description: point?.description,
      message: rule.message,
    }).toLowerCase().includes(q)
  })
})

const configuredCount = computed(() => draftRules.value.filter((rule) => rule.enabled).length)
const activeScoped = computed(() => activeAlarms.value.filter((alarm) => {
  const station = stationByTag(alarm.tag)?.code
  return stationFilter.value === 'all' || station === stationFilter.value
}))
const recentAlarmEvents = computed(() => events.value.filter((e) => String(e.type || '').startsWith('alarm_')))

function isBool(rule) {
  return String(rule.data_type || '').toLowerCase() === 'bool'
}

function changeStation(code) {
  stationFilter.value = code
  if (STATIONS.some((s) => s.code === code)) setSelectedStation(code)
}

function alarmFor(tag) {
  return activeMap.value.get(tag)
}

function stateText(rule) {
  if (!rule.enabled) return '未启用'
  const alarm = alarmFor(rule.tag)
  if (!alarm) return '正常'
  if (alarm.state === 'high') return '超上限'
  if (alarm.state === 'low') return '低于下限'
  if (alarm.state === 'bool_true') return 'True 报警'
  if (alarm.state === 'bool_false') return 'False 报警'
  return '报警'
}

function stateClass(rule) {
  if (!rule.enabled) return 'disabled'
  return alarmFor(rule.tag) ? 'danger' : 'normal'
}

function ensureDraftShape(rule, point) {
  const bool = isBool(rule)
  return {
    tag: rule.tag,
    data_type: rule.data_type || point?.data_type || 'float32',
    enabled: !!rule.enabled,
    alarm_value: bool ? (rule.alarm_value === true) : null,
    low: bool ? null : normalizeNullableNumber(rule.low),
    high: bool ? null : normalizeNullableNumber(rule.high),
    unit: rule.unit || '',
    message: rule.message || point?.description || '',
  }
}

function normalizeNullableNumber(value) {
  if (value === '' || value === null || value === undefined) return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

function rebuildDraftRules(ioPoints, config) {
  const configMap = new Map((config?.alarms || []).map((rule) => [rule.tag, rule]))
  draftRules.value = (Array.isArray(ioPoints) ? ioPoints : []).map((point) => {
    const rule = configMap.get(point.name) || {
      tag: point.name,
      data_type: point.data_type,
      enabled: false,
      alarm_value: point.data_type === 'bool' ? false : null,
      low: null,
      high: null,
      unit: '',
      message: point.description || '',
    }
    return ensureDraftShape(rule, point)
  })
}

function validateDrafts() {
  for (const rule of draftRules.value) {
    if (!rule.enabled) continue
    if (isBool(rule)) {
      if (rule.alarm_value !== true && rule.alarm_value !== false) {
        return `${rule.tag}：布尔变量必须选择 True 或 False 作为报警条件`
      }
      continue
    }
    const low = normalizeNullableNumber(rule.low)
    const high = normalizeNullableNumber(rule.high)
    if (low === null && high === null) {
      return `${rule.tag}：启用报警后至少设置一个上限或下限`
    }
    if (low !== null && high !== null && low > high) {
      return `${rule.tag}：下限不能大于上限`
    }
  }
  return ''
}

function markDirty() {
  dirty.value = true
  saveMessage.value = ''
}

function pad(n) { return String(n).padStart(2, '0') }
function toInputDate(date) { return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` }
function initAlarmHistoryDates() {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - 6)
  alarmHistoryStartDate.value = toInputDate(start)
  alarmHistoryEndDate.value = toInputDate(end)
}

async function queryAlarmHistory() {
  if (!alarmHistoryStartDate.value || !alarmHistoryEndDate.value) {
    error.value = '请选择历史报警查询的开始日期和结束日期。'
    return
  }
  const start = new Date(`${alarmHistoryStartDate.value}T00:00:00`)
  const end = new Date(`${alarmHistoryEndDate.value}T00:00:00`)
  const days = Math.round((end.getTime() - start.getTime()) / 86400000) + 1
  if (Number.isNaN(days) || days <= 0) {
    error.value = '结束日期不能早于开始日期，请重新选择。'
    return
  }
  alarmHistoryLoading.value = true
  try {
    alarmHistoryEvents.value = await api.getAlarmHistory(alarmHistoryTag.value, alarmHistoryStartDate.value, alarmHistoryEndDate.value) || []
    alarmHistoryQueried.value = true
    error.value = ''
  } catch (err) {
    alarmHistoryEvents.value = []
    alarmHistoryQueried.value = false
    error.value = err.message || '历史报警查询失败'
  } finally {
    alarmHistoryLoading.value = false
  }
}

function buildPayload() {
  return {
    enabled: true,
    alarms: draftRules.value.map((rule) => ({
      tag: rule.tag,
      data_type: rule.data_type,
      enabled: !!rule.enabled,
      alarm_value: isBool(rule) ? !!rule.alarm_value : null,
      low: isBool(rule) ? null : normalizeNullableNumber(rule.low),
      high: isBool(rule) ? null : normalizeNullableNumber(rule.high),
      unit: String(rule.unit || '').trim(),
      message: String(rule.message || '').trim(),
    })),
  }
}

async function saveAll() {
  error.value = ''
  saveMessage.value = ''
  const validationError = validateDrafts()
  if (validationError) {
    error.value = validationError
    return
  }
  saving.value = true
  try {
    const saved = await api.updateAlarmConfig(buildPayload())
    alarmConfig.value = saved || buildPayload()
    dirty.value = false
    rebuildDraftRules(points.value, alarmConfig.value)
    saveMessage.value = `报警配置已保存，共 ${draftRules.value.length} 个变量`
    await refresh(false, false)
  } catch (err) {
    error.value = err.message || '保存报警配置失败'
  } finally {
    saving.value = false
  }
}

async function refresh(syncDraft = false, clearMessage = true) {
  try {
    const [alarms, config, io, eventList] = await Promise.all([
      api.getAlarms(), api.getAlarmConfig(), api.getIO(), api.getEvents(100),
    ])
    activeAlarms.value = Array.isArray(alarms) ? alarms : []
    alarmConfig.value = config || { enabled: true, alarms: [] }
    points.value = Array.isArray(io) ? io : []
    events.value = Array.isArray(eventList) ? eventList : []
    if (syncDraft || !dirty.value) rebuildDraftRules(points.value, alarmConfig.value)
    error.value = ''
    if (clearMessage) saveMessage.value = ''
  } catch (err) {
    error.value = err.message || '读取报警数据失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  initAlarmHistoryDates()
  refresh(true)
  timer = setInterval(() => refresh(false, false), 1500)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <section class="alarm-banner">
    <div>
      <span class="hero-kicker">VARIABLE ALARM CONFIGURATION</span>
      <h3>设备变量报警</h3>
      <p>报警配置来自 IOManager 当前变量。布尔变量选择 True / False 作为报警条件；整数、浮点变量使用上限 / 下限判断。</p>
    </div>
    <div class="alarm-banner-status enabled">
      <strong>报警功能已启用</strong>
      <small>{{ configuredCount }} 个变量已启用报警</small>
    </div>
  </section>

  <section class="stats-grid alarm-stats">
    <article class="stat-card bad"><span>当前活动报警</span><strong>{{ activeScoped.length }}</strong><small>按当前站点筛选</small></article>
    <article class="stat-card"><span>IOManager变量</span><strong>{{ draftRules.length }}</strong><small>当前已加载变量</small></article>
    <article class="stat-card"><span>已启用报警</span><strong>{{ configuredCount }}</strong><small>可实时参与报警判定</small></article>
    <article class="stat-card"><span>报警事件</span><strong>{{ recentAlarmEvents.length }}</strong><small>最近 100 条事件</small></article>
  </section>

  <section class="panel alarm-editor-panel">
    <div class="panel-title-row">
      <div>
        <h3>报警参数配置</h3>
        <span class="muted">修改后点击“保存全部配置”，后端会同步写入 configs/Alarm.csv 并立即生效</span>
      </div>
      <div class="panel-actions">
        <span v-if="saveMessage" class="save-ok">{{ saveMessage }}</span>
        <button class="primary" :disabled="saving || loading" @click="saveAll">{{ saving ? '保存中...' : '保存全部配置' }}</button>
      </div>
    </div>

    <div v-if="error" class="alert">{{ error }}</div>
    <div v-if="loading" class="empty">正在读取 IOManager 变量与报警配置...</div>
    <template v-else>
      <div class="toolbar alarm-toolbar">
        <div class="filters">
          <select v-model="stationFilter" @change="changeStation(stationFilter)">
            <option value="all">全部站点</option>
            <option v-for="station in STATIONS" :key="station.code" :value="station.code">{{ station.name }}</option>
          </select>
          <select v-model="statusFilter">
            <option value="all">全部变量</option>
            <option value="active">仅活动报警</option>
            <option value="configured">仅启用报警</option>
          </select>
          <input v-model="search" placeholder="搜索变量、站点或说明" />
        </div>
      </div>

      <div class="alarm-editor-table table-wrap">
        <table>
          <thead>
            <tr>
              <th>站点</th>
              <th>变量</th>
              <th>类型</th>
              <th>当前值</th>
              <th>启用报警</th>
              <th>报警条件</th>
              <th>状态</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in filteredRules" :key="rule.tag" :class="{ 'alarm-row': !!alarmFor(rule.tag) }">
              <td>{{ stationByTag(rule.tag)?.shortName || '设备' }}</td>
              <td><strong>{{ rule.tag }}</strong></td>
              <td><span class="type-chip">{{ rule.data_type }}</span></td>
              <td class="value">{{ formatValue(pointMap.get(rule.tag)?.value) }} {{ rule.unit }}</td>
              <td>
                <label class="switch-inline">
                  <input type="checkbox" v-model="rule.enabled" @change="markDirty" />
                  <span>{{ rule.enabled ? '启用' : '停用' }}</span>
                </label>
              </td>
              <td class="alarm-condition-cell">
                <template v-if="isBool(rule)">
                  <select v-model="rule.alarm_value" :disabled="!rule.enabled" class="condition-select" @change="markDirty">
                    <option :value="true">True 报警</option>
                    <option :value="false">False 报警</option>
                  </select>
                </template>
                <template v-else>
                  <div class="limit-editor">
                    <label>下限<input v-model="rule.low" :disabled="!rule.enabled" type="number" step="any" placeholder="不设" @input="markDirty" /></label>
                    <label>上限<input v-model="rule.high" :disabled="!rule.enabled" type="number" step="any" placeholder="不设" @input="markDirty" /></label>
                  </div>
                </template>
              </td>
              <td><span class="alarm-state" :class="stateClass(rule)">{{ stateText(rule) }}</span></td>
              <td><input v-model="rule.message" class="message-input" placeholder="变量说明 / 报警说明" @input="markDirty" /></td>
            </tr>
            <tr v-if="!filteredRules.length"><td colspan="8" class="table-empty">当前筛选条件下暂无变量</td></tr>
          </tbody>
        </table>
      </div>
    </template>
  </section>

  <section class="panel">
    <div class="panel-title-row"><div><h3>活动报警</h3><span class="muted">仅显示尚未恢复的变量报警</span></div><span class="data-chip danger-chip">{{ activeScoped.length }} 条</span></div>
    <div v-if="activeScoped.length" class="active-alarm-grid">
      <article v-for="alarm in activeScoped" :key="alarm.tag" class="active-alarm-card">
        <div class="active-alarm-head"><span>{{ stationByTag(alarm.tag)?.shortName || '设备变量' }}</span><strong>{{ alarm.state === 'high' ? '高限报警' : alarm.state === 'low' ? '低限报警' : alarm.state === 'bool_true' ? 'True报警' : 'False报警' }}</strong></div>
        <h4>{{ alarm.message || alarm.tag }}</h4>
        <div class="active-alarm-value"><strong>{{ formatValue(alarm.value) }}</strong><span v-if="alarm.limit !== null && alarm.limit !== undefined">限值 {{ formatValue(alarm.limit) }} {{ alarm.unit }}</span><span v-else>期望值 {{ alarm.expected ? 'True' : 'False' }}</span></div>
        <small>{{ alarm.timestamp ? new Date(alarm.timestamp).toLocaleString('zh-CN', { hour12: false }) : '--' }}</small>
      </article>
    </div>
    <div v-else class="empty">当前没有活动变量报警</div>
  </section>

  <section class="panel alarm-history-panel">
    <div class="panel-title-row">
      <div>
        <h3>历史报警查询</h3>
        <span class="muted">报警触发、恢复事件已持久化到 SQLite，可按点位和日期范围查询。</span>
      </div>
      <span class="data-chip">{{ alarmHistoryQueried ? `${alarmHistoryEvents.length} 条` : '未查询' }}</span>
    </div>
    <div class="alarm-history-filter-grid">
      <label>
        <span>报警点位</span>
        <select v-model="alarmHistoryTag">
          <option value="">全部点位</option>
          <option v-for="rule in draftRules" :key="`history-${rule.tag}`" :value="rule.tag">{{ rule.tag }}</option>
        </select>
      </label>
      <label>
        <span>开始日期</span>
        <input v-model="alarmHistoryStartDate" type="date" :max="alarmHistoryEndDate || undefined" />
      </label>
      <label>
        <span>结束日期</span>
        <input v-model="alarmHistoryEndDate" type="date" :min="alarmHistoryStartDate || undefined" />
      </label>
      <button class="primary" :disabled="alarmHistoryLoading" @click="queryAlarmHistory">
        {{ alarmHistoryLoading ? '查询中...' : '查询历史报警' }}
      </button>
    </div>
    <div class="alarm-history-tip">
      <span>查询结果包含报警触发与恢复事件，不包含通讯质量事件。</span>
      <span v-if="alarmHistoryQueried">{{ alarmHistoryStartDate }} ～ {{ alarmHistoryEndDate }}</span>
    </div>
    <AlarmTable :events="alarmHistoryEvents" />
  </section>

  <section class="panel">
    <div class="panel-title-row"><div><h3>报警事件记录</h3><span class="muted">包括超限、布尔报警、恢复以及通讯质量事件</span></div></div>
    <AlarmTable :events="events" />
  </section>
</template>
