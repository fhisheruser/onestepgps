
const SPEED_TO_KPH = {
  'km/h': 1,
  kph: 1,
  kmh: 1,
  mph: 1.609344,
  'mi/h': 1.609344,
  kn: 1.852,
  knots: 1.852,
  'm/s': 3.6,
}

const KPH_TO_UNIT = {
  kph: 1,
  mph: 1 / 1.609344,
  kn: 1 / 1.852,
}

export const SPEED_UNIT_LABELS = {
  kph: 'km/h',
  mph: 'mph',
  kn: 'kn',
}


export function convertSpeed(value, fromUnit, toUnit = 'mph') {
  if (!Number.isFinite(value)) return 0
  const from = SPEED_TO_KPH[String(fromUnit || '').toLowerCase()]
  const to = KPH_TO_UNIT[String(toUnit || 'mph').toLowerCase()]
  if (!from || !to) return value
  return value * from * to
}

export function formatSpeed(value, fromUnit, toUnit = 'mph', { withUnit = true } = {}) {
  const converted = convertSpeed(value, fromUnit, toUnit)
  const rounded = converted >= 100 ? Math.round(converted) : Math.round(converted * 10) / 10
  const label = SPEED_UNIT_LABELS[String(toUnit).toLowerCase()] || toUnit
  return withUnit ? `${rounded} ${label}` : String(rounded)
}


export function timeAgo(input, now = Date.now()) {
  if (!input) return 'never'
  const then = input instanceof Date ? input.getTime() : Date.parse(input)
  if (Number.isNaN(then)) return 'unknown'

  const seconds = Math.max(0, Math.round((now - then) / 1000))
  if (seconds < 10) return 'just now'
  if (seconds < 60) return `${seconds}s ago`

  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`

  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours}h ago`

  const days = Math.round(hours / 24)
  if (days < 30) return `${days}d ago`
  return new Date(then).toLocaleDateString()
}


export function formatDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—'
  const total = Math.round(seconds)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)

  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m`
  return `${total}s`
}


export function formatTimestamp(input) {
  if (!input) return '—'
  const date = input instanceof Date ? input : new Date(input)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}


export function formatCoords(lat, lng) {
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) return '—'
  return `${lat.toFixed(5)}, ${lng.toFixed(5)}`
}


export function headingToCompass(deg) {
  if (!Number.isFinite(deg)) return '—'
  const points = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW']
  return points[Math.round((((deg % 360) + 360) % 360) / 45) % 8]
}

export const DRIVE_STATUS_META = {
  driving: { label: 'Driving', tone: 'sage', dot: '#4C8C4A' },
  idle: { label: 'Idle', tone: 'amber', dot: '#C9A227' },
  off: { label: 'Parked', tone: 'neutral', dot: '#9C9184' },
  unknown: { label: 'Unknown', tone: 'neutral', dot: '#C4BCB0' },
}

export function driveStatusMeta(status) {
  return DRIVE_STATUS_META[status] || DRIVE_STATUS_META.unknown
}


export function formatOdometer(value, unit) {
  if (!Number.isFinite(value) || value <= 0) return '—'
  return `${Math.round(value).toLocaleString()}${unit ? ` ${unit}` : ''}`
}


export function truncate(text, max = 60) {
  const value = String(text ?? '')
  if (value.length <= max) return value
  const clipped = value.slice(0, max)
  const lastSpace = clipped.lastIndexOf(' ')
  return `${(lastSpace > max * 0.6 ? clipped.slice(0, lastSpace) : clipped).trimEnd()}…`
}
