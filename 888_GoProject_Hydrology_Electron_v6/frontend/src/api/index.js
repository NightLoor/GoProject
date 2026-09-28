const API_BASE = window.desktopApp?.apiBase || ''

async function request(url, options) {
  const target = `${API_BASE}${url}`
  const response = await fetch(target, options)
  const text = await response.text()
  let data = null
  try { data = text ? JSON.parse(text) : null } catch { data = text }
  if (!response.ok) {
    throw new Error(typeof data === 'string' ? data : data?.error || `HTTP ${response.status}`)
  }
  return data?.data ?? data
}

export const api = {
  getIO: () => request('/api/io'),
  getConfig: () => request('/api/config'),
  getDevices: () => request('/api/devices'),
  getEvents: (limit = 100) => request(`/api/events?limit=${limit}`),
  getAlarmHistory: (tag, startDate, endDate) => request(`/api/alarm-history?tag=${encodeURIComponent(tag || '')}&start_date=${encodeURIComponent(startDate)}&end_date=${encodeURIComponent(endDate)}`),
  getAlarms: () => request('/api/alarms'),
  getAlarmConfig: () => request('/api/alarm-config'),
  updateAlarmConfig: (config) => request('/api/alarm-config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(config) }),
  getHistory: (name, limit = 120) => request(`/api/io/${encodeURIComponent(name)}/history?limit=${limit}`),
  getHistoryRange: (name, startDate, endDate) => request(`/api/io/${encodeURIComponent(name)}/history?start_date=${encodeURIComponent(startDate)}&end_date=${encodeURIComponent(endDate)}`),
  write: (tag, value) => request('/api/io/write', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tag, value })
  })
}
