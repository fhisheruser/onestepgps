import axios from 'axios'

export const API_BASE = import.meta.env.VITE_API_BASE_URL || '/api/v1'

const USER_ID_STORAGE_KEY = 'fleetview.userId'

export function resolveUserId() {
  try {
    const existing = window.localStorage.getItem(USER_ID_STORAGE_KEY)
    if (existing && /^[A-Za-z0-9._-]{1,64}$/.test(existing)) return existing

    const generated =
      typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : `u-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`

    window.localStorage.setItem(USER_ID_STORAGE_KEY, generated)
    return generated
  } catch {
   
    return 'default'
  }
}

export const http = axios.create({
  baseURL: API_BASE,
  timeout: 15000,
  headers: { Accept: 'application/json' },
})

http.interceptors.request.use((config) => {
  config.headers['X-User-Id'] = resolveUserId()
  return config
})

export class ApiError extends Error {
  constructor({ message, code = 'unknown', field = '', status = 0, retryable = false }) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.field = field
    this.status = status
    this.retryable = retryable
  }
}

function normaliseError(error) {
  if (axios.isCancel?.(error) || error.code === 'ERR_CANCELED') {
    return new ApiError({ message: 'Request cancelled', code: 'cancelled' })
  }

  if (!error.response) {
    return new ApiError({
      message: 'Cannot reach the FleetView server. Check your connection.',
      code: 'network_error',
      retryable: true,
    })
  }

  const { status, data } = error.response
  const payload = data?.error
  return new ApiError({
    message: payload?.message || defaultMessageFor(status),
    code: payload?.code || `http_${status}`,
    field: payload?.field || '',
    status,
    retryable: status >= 500 || status === 429,
  })
}

function defaultMessageFor(status) {
  switch (status) {
    case 400:
      return 'The request was malformed.'
    case 404:
      return 'That resource no longer exists.'
    case 413:
      return 'That file is too large.'
    case 422:
      return 'Some values were rejected.'
    case 429:
      return 'Too many requests — slow down for a moment.'
    case 502:
      return 'The GPS provider is unreachable.'
    case 503:
      return 'Live data is not ready yet.'
    default:
      return 'Something went wrong.'
  }
}

http.interceptors.response.use(
  (response) => response,
  (error) => Promise.reject(normaliseError(error)),
)


function cleanParams(params = {}) {
  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => value !== '' && value !== null && value !== undefined && value !== false),
  )
}

export const fleetApi = {
  async runtimeConfig() {
    const { data } = await http.get('/config')
    return data
  },

  async devices(params = {}, options = {}) {
    const { data } = await http.get('/devices', { params: cleanParams(params), ...options })
    return data
  },

  async device(deviceId) {
    const { data } = await http.get(`/devices/${encodeURIComponent(deviceId)}`)
    return data
  },

  async history(deviceId, minutes = 60, limit = 500) {
    const { data } = await http.get(`/devices/${encodeURIComponent(deviceId)}/history`, {
      params: { minutes, limit },
    })
    return data
  },

  async summary(params = {}) {
    const { data } = await http.get('/fleet/summary', { params: cleanParams(params) })
    return data
  },

  async preferences() {
    const { data } = await http.get('/preferences')
    return data
  },

  async updateSettings(patch) {
    const { data } = await http.put('/preferences/settings', patch)
    return data
  },

  async updateDevicePreference(deviceId, patch) {
    const { data } = await http.put(`/preferences/devices/${encodeURIComponent(deviceId)}`, patch)
    return data
  },

  async deleteDevicePreference(deviceId) {
    await http.delete(`/preferences/devices/${encodeURIComponent(deviceId)}`)
  },

  async reorder(deviceIds) {
    await http.post('/preferences/reorder', { deviceIds })
  },

  async resetPreferences() {
    await http.post('/preferences/reset')
  },

  async uploadIcon(deviceId, file) {
    const form = new FormData()
    form.append('icon', file)
    const { data } = await http.post(`/preferences/devices/${encodeURIComponent(deviceId)}/icon`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000,
    })
    return data
  },

  async deleteIcon(deviceId) {
    const { data } = await http.delete(`/preferences/devices/${encodeURIComponent(deviceId)}/icon`)
    return data
  },

 
  exportCsvUrl(params = {}) {
    const query = new URLSearchParams({ ...cleanParams(params), userId: resolveUserId() })
    return `${API_BASE}/export/devices.csv?${query.toString()}`
  },
}
