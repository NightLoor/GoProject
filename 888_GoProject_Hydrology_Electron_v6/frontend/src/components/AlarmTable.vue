<script setup>
defineProps({ events: { type: Array, default: () => [] } })
function timeText(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--' }
function eventClass(type) { return String(type || '').toLowerCase().replaceAll(' ', '_') }
function eventLabel(type) {
  return ({
    alarm_high: '变量超上限',
    alarm_low: '变量低下限',
    alarm_bool_true: 'True变量报警',
    alarm_bool_false: 'False变量报警',
    alarm_recovered: '报警恢复',
    quality_bad: '点位质量异常',
    quality_recovered: '点位质量恢复',
    device_connected: '设备上线',
    device_disconnected: '设备离线',
  })[type] || type || '系统事件'
}
function valueText(event) {
  if (['alarm_bool_true', 'alarm_bool_false'].includes(event.type)) {
    const value = event.value === true ? 'True' : event.value === false ? 'False' : event.value ?? '--'
    const expected = event.expected === true ? 'True' : event.expected === false ? 'False' : '--'
    return `当前值 ${value} · 报警值 ${expected}`
  }
  if (!['alarm_high','alarm_low','alarm_recovered'].includes(event.type)) return ''
  const value = event.value ?? '--'
  const limit = event.limit ?? '--'
  const unit = event.unit || ''
  return `值 ${value}${unit ? ` ${unit}` : ''} · 限值 ${limit}${unit ? ` ${unit}` : ''}`
}
</script>

<template>
  <div v-if="!events.length" class="empty">暂无报警/事件</div>
  <div v-else class="event-list">
    <div v-for="(event, index) in events" :key="`${event.timestamp}-${event.tag}-${index}`" class="event-row">
      <span class="event-time">{{ timeText(event.timestamp) }}</span>
      <span class="event-type" :class="eventClass(event.type)">{{ eventLabel(event.type) }}</span>
      <strong>{{ event.tag || event.device || '--' }}</strong>
      <span><span>{{ event.message || event.error || event.description || '--' }}</span><small v-if="valueText(event)" class="event-value">{{ valueText(event) }}</small></span>
    </div>
  </div>
</template>
