<script setup>
defineProps({
  rows: { type: Array, default: () => [] },
  pointMeta: { type: Map, required: true }
})
const emit = defineEmits(['select', 'write'])

function valueText(value) {
  if (value === null || value === undefined) return '--'
  if (typeof value === 'number') return Number.isInteger(value) ? value : Number(value.toFixed(4))
  return value
}
function timeText(value) {
  return value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '--'
}
</script>

<template>
  <div v-if="!rows.length" class="empty">暂无匹配点位</div>
  <div v-else class="table-wrap">
    <table>
      <thead><tr><th>点位</th><th>协议</th><th>设备</th><th>值</th><th>质量</th><th>更新时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="point in rows" :key="point.name">
          <td>
            <button class="link-btn" @click="emit('select', point)"><strong>{{ point.name }}</strong></button>
            <small>{{ pointMeta.get(point.name)?.address || '--' }}</small>
          </td>
          <td><span class="protocol-badge" :class="String(pointMeta.get(point.name)?.protocol || '').toLowerCase().replaceAll(' ', '-')">{{ pointMeta.get(point.name)?.protocol || '--' }}</span></td>
          <td>{{ pointMeta.get(point.name)?.device || '--' }}</td>
          <td class="value">{{ valueText(point.value) }}</td>
          <td>
            <span class="quality" :class="String(point.quality || '').toLowerCase()">{{ point.quality || '--' }}</span>
            <small v-if="point.error">{{ point.error }}</small>
          </td>
          <td>{{ timeText(point.timestamp) }}</td>
          <td>
            <button v-if="pointMeta.get(point.name)?.writable" class="write-btn" @click="emit('write', point)">写入</button>
            <button class="secondary" @click="emit('select', point)">趋势</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
