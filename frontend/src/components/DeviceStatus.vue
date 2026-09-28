<script setup>
defineProps({ devices: { type: Array, default: () => [] } })
function timeText(value) { return value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '--' }
</script>

<template>
  <div v-if="!devices.length" class="empty">暂无设备配置</div>
  <div v-else class="device-list">
    <article v-for="device in devices" :key="device.name" class="device-card">
      <div class="device-title">
        <div><strong>{{ device.name }}</strong><span class="device-protocol">{{ device.protocol || 'Unknown' }}</span><span class="device-address">{{ device.address || device.endpoint || '--' }}</span></div>
        <span class="quality" :class="device.connected ? 'good' : 'bad'">{{ device.connected ? '在线' : '离线' }}</span>
      </div>
      <div class="device-meta">
        <span>点位：{{ device.tag_count ?? (device.tags?.length || 0) }}</span>
        <span>最近连接：{{ timeText(device.last_connected) }}</span>
        <span>最近采集：{{ timeText(device.last_poll) }}</span>
      </div>
      <div v-if="device.error" class="device-error">{{ device.error }}</div>
    </article>
  </div>
</template>
