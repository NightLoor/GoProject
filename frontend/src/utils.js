import { stationByTag, formatValue, filterPointsByStation } from './station'

// Re-export station helpers for legacy page imports.
export { formatValue, filterPointsByStation }

export function normalizeDevices(config, statuses = []) {
  const items = []
  const modbusEnabled = config?.enabled !== false
  const opcdaEnabled = config?.opcda_enabled !== false
  for (const device of (modbusEnabled && Array.isArray(config?.devices) ? config.devices : [])) {
    items.push({
      ...device,
      protocol: 'Modbus TCP',
      address: device.address || '',
      tag_count: Array.isArray(device.tags) ? device.tags.length : 0,
    })
  }
  for (const device of (opcdaEnabled && Array.isArray(config?.opcda_devices) ? config.opcda_devices : [])) {
    items.push({
      ...device,
      protocol: 'OPC DA',
      address: device.node && device.node !== 'localhost' ? `${device.node} · ${device.prog_id}` : (device.prog_id || device.node || ''),
      tag_count: Array.isArray(device.tags) ? device.tags.length : 0,
    })
  }
  const statusByName = new Map((Array.isArray(statuses) ? statuses : []).map((s) => [s.name, s]))
  return items.map((device) => ({ ...device, ...(statusByName.get(device.name) || {}) }))
}

export function buildPointMeta(devices) {
  const map = new Map()
  for (const device of Array.isArray(devices) ? devices : []) {
    for (const tag of Array.isArray(device.tags) ? device.tags : []) {
      const station = stationByTag(tag.name)
      map.set(tag.name, {
        ...tag,
        device: device.name,
        protocol: device.protocol,
        stationCode: station?.code || '',
        stationName: station?.name || '',
      })
    }
  }
  return map
}

export function siteTag(points, code, type) {
  return points.find((p) => {
    const name = String(p.name || '')
    return name.startsWith(`${code}_`) && name.includes(type)
  }) || null
}

export function sitePressurePoints(points, code) {
  return points
    .filter((p) => new RegExp(`^${code}_Point\\d+$`).test(String(p.name || '')))
    .sort((a, b) => Number(a.name.replace(`${code}_Point`, '')) - Number(b.name.replace(`${code}_Point`, '')))
}
