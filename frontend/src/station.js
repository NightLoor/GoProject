import { ref } from 'vue'

export const STATIONS = [
  { code: 'BZ', name: '坝仔排涝站', shortName: '坝仔', prefix: 'BZ_' },
  { code: 'DT', name: '淡塘排涝站', shortName: '淡塘', prefix: 'DT_' },
  { code: 'DY', name: '稻元排涝站', shortName: '稻元', prefix: 'DY_' },
]

const saved = localStorage.getItem('selected_station')
export const selectedStationCode = ref(STATIONS.some((item) => item.code === saved) ? saved : 'BZ')

export function setSelectedStation(code) {
  if (!STATIONS.some((item) => item.code === code)) return
  selectedStationCode.value = code
  localStorage.setItem('selected_station', code)
  window.dispatchEvent(new CustomEvent('station-change', { detail: code }))
}

export function stationByCode(code) {
  return STATIONS.find((item) => item.code === code) || STATIONS[0]
}

export function stationByTag(tagName) {
  const name = String(tagName || '')
  return STATIONS.find((item) => name.startsWith(item.prefix)) || null
}

export function filterPointsByStation(points, code) {
  if (!code || code === 'all') return points
  return points.filter((point) => stationByTag(point.name)?.code === code)
}

export function formatValue(value, digits = 3) {
  if (value === null || value === undefined || value === '') return '--'
  const n = Number(value)
  if (!Number.isFinite(n)) return String(value)
  return Number.isInteger(n) ? String(n) : n.toFixed(digits).replace(/0+$/, '').replace(/\.$/, '')
}
