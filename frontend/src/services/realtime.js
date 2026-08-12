import { API_BASE, resolveUserId } from './api'

/**
 * WebSocket client for live fleet pushes.
 *
 * Design notes:
 *  - The socket is an optimisation, never a requirement. Callers keep a REST
 *    polling fallback and this class reports its state so the UI can say which
 *    one is active.
 *  - Reconnects use exponential backoff with jitter and stop escalating at
 *    30s, so a backend restart does not turn into a reconnect storm.
 *  - The browser cannot set headers on a WebSocket handshake, so the user
 *    scope travels as a query parameter (the backend accepts either).
 */

const MAX_BACKOFF_MS = 30000
const BASE_BACKOFF_MS = 1000

export const ConnectionState = {
  Idle: 'idle',
  Connecting: 'connecting',
  Open: 'open',
  Closed: 'closed',
  Unsupported: 'unsupported',
}

export class RealtimeClient {
  constructor({ onMessage, onStateChange } = {}) {
    this.onMessage = onMessage || (() => {})
    this.onStateChange = onStateChange || (() => {})
    this.socket = null
    this.attempt = 0
    this.reconnectTimer = null
    this.manuallyClosed = false
    this.state = ConnectionState.Idle
    this.lastQuery = null
  }

  get isOpen() {
    return this.socket?.readyState === WebSocket.OPEN
  }

  url() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const base = API_BASE.startsWith('http')
      ? API_BASE.replace(/^http/, 'ws')
      : `${protocol}//${window.location.host}${API_BASE}`
    return `${base}/ws?userId=${encodeURIComponent(resolveUserId())}`
  }

  setState(state) {
    if (this.state === state) return
    this.state = state
    this.onStateChange(state)
  }

  connect() {
    if (typeof WebSocket === 'undefined') {
      this.setState(ConnectionState.Unsupported)
      return
    }
    if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
      return
    }

    this.manuallyClosed = false
    this.setState(ConnectionState.Connecting)

    try {
      this.socket = new WebSocket(this.url())
    } catch {
      this.scheduleReconnect()
      return
    }

    this.socket.onopen = () => {
      this.attempt = 0
      this.setState(ConnectionState.Open)
      // Re-apply the active filters so the first push is already correct.
      if (this.lastQuery) this.sendQuery(this.lastQuery)
    }

    this.socket.onmessage = (event) => {
      let frame
      try {
        frame = JSON.parse(event.data)
      } catch {
        return
      }
      this.onMessage(frame)
    }

    this.socket.onerror = () => {
      // onclose always follows; reconnection is handled there.
    }

    this.socket.onclose = () => {
      this.socket = null
      this.setState(ConnectionState.Closed)
      if (!this.manuallyClosed) this.scheduleReconnect()
    }
  }

  scheduleReconnect() {
    if (this.manuallyClosed || this.reconnectTimer) return

    this.attempt += 1
    const exponential = Math.min(BASE_BACKOFF_MS * 2 ** (this.attempt - 1), MAX_BACKOFF_MS)
    const delay = exponential * (0.5 + Math.random() / 2)

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, delay)
  }

  send(payload) {
    if (!this.isOpen) return false
    try {
      this.socket.send(JSON.stringify(payload))
      return true
    } catch {
      return false
    }
  }

  /** Mirror the list filters onto the socket so pushes arrive pre-filtered. */
  sendQuery(query) {
    this.lastQuery = query
    return this.send({ type: 'query', data: query })
  }

  /** Ask for an immediate re-render, e.g. right after saving a preference. */
  requestRefresh() {
    return this.send({ type: 'refresh' })
  }

  close() {
    this.manuallyClosed = true
    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.socket) {
      this.socket.onclose = null
      this.socket.close()
      this.socket = null
    }
    this.setState(ConnectionState.Idle)
  }
}
