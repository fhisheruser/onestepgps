import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import { ConnectionState, RealtimeClient } from '@/services/realtime'

/**
 * Keeps the dashboard live.
 *
 * Strategy: prefer a WebSocket push (instant, one message per poll cycle) and
 * fall back to REST polling when the socket is not open. Both paths produce
 * the same feed shape, so the rest of the app cannot tell them apart. A slow
 * heartbeat poll runs even while the socket is healthy, which repairs the view
 * if a push is ever dropped.
 */
export function useFleetSync() {
  const fleet = useFleetStore()
  const preferences = usePreferencesStore()
  const ui = useUiStore()

  const client = ref(null)
  const pollTimer = ref(null)
  const heartbeatTimer = ref(null)
  const filterDebounce = ref(null)

  const refreshSeconds = () => Math.max(5, Number(preferences.settings.refreshSeconds) || 10)

  function stopPolling() {
    if (pollTimer.value) {
      window.clearInterval(pollTimer.value)
      pollTimer.value = null
    }
  }

  function startPolling() {
    stopPolling()
    fleet.transport = 'polling'
    pollTimer.value = window.setInterval(() => {
      // Skip work the user cannot see; the visibilitychange handler catches up.
      if (document.visibilityState === 'hidden') return
      void fleet.fetchFeed({ silent: true })
    }, refreshSeconds() * 1000)
  }

  function startHeartbeat() {
    if (heartbeatTimer.value) window.clearInterval(heartbeatTimer.value)
    // Six poll intervals: cheap insurance against a lost push, not a poll loop.
    heartbeatTimer.value = window.setInterval(() => {
      if (fleet.transport === 'realtime' && document.visibilityState === 'visible') {
        void fleet.fetchFeed({ silent: true })
      }
    }, refreshSeconds() * 6000)
  }

  function handleFrame(frame) {
    if (!frame?.type) return

    switch (frame.type) {
      case 'fleet.updated':
        if (frame.data) fleet.applyFeed(frame.data)
        break
      case 'fleet.error':
        fleet.meta = { ...fleet.meta, stale: true, error: frame.data?.detail || frame.data?.message || '' }
        break
      default:
        break
    }
  }

  function handleStateChange(state) {
    fleet.connectionState = state

    if (state === ConnectionState.Open) {
      fleet.transport = 'realtime'
      stopPolling()
      client.value?.sendQuery(fleet.realtimeQuery)
      return
    }

    // Any non-open state means we cannot rely on pushes: poll instead.
    if (fleet.transport !== 'polling') {
      fleet.transport = 'polling'
    }
    startPolling()
  }

  function connectRealtime() {
    if (!preferences.runtimeConfig.realtimeEnabled) {
      startPolling()
      return
    }
    client.value = new RealtimeClient({
      onMessage: handleFrame,
      onStateChange: handleStateChange,
    })
    client.value.connect()
  }

  function onVisibilityChange() {
    if (document.visibilityState !== 'visible') return
    // Coming back to the tab: repaint immediately rather than waiting a tick.
    void fleet.fetchFeed({ silent: true })
    if (fleet.transport !== 'realtime') client.value?.connect()
  }

  async function bootstrap() {
    await preferences.loadRuntimeConfig()
    try {
      await preferences.load()
    } catch {
      // Preferences are optional for a first render; the feed carries settings.
    }
    await fleet.fetchFeed()

    connectRealtime()
    startPolling()
    startHeartbeat()

    if (preferences.runtimeConfig.demoMode) {
      ui.notify('Demo mode: showing simulated vehicles. Add a OneStepGPS API key for live data.', {
        type: 'info',
        timeout: 8000,
      })
    }
  }

  // Filter changes go to both transports: the socket so pushes arrive
  // pre-filtered, and a debounced fetch so typing feels instant.
  watch(
    () => fleet.realtimeQuery,
    (query) => {
      client.value?.sendQuery(query)
      if (filterDebounce.value) window.clearTimeout(filterDebounce.value)
      filterDebounce.value = window.setTimeout(() => {
        void fleet.fetchFeed({ silent: true })
      }, 220)
    },
    { deep: true },
  )

  // Honour a changed refresh interval without a page reload.
  watch(
    () => preferences.settings.refreshSeconds,
    () => {
      if (fleet.transport === 'polling') startPolling()
      startHeartbeat()
    },
  )

  onMounted(() => {
    document.addEventListener('visibilitychange', onVisibilityChange)
    void bootstrap()
  })

  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', onVisibilityChange)
    stopPolling()
    if (heartbeatTimer.value) window.clearInterval(heartbeatTimer.value)
    if (filterDebounce.value) window.clearTimeout(filterDebounce.value)
    client.value?.close()
  })

  return {
    refreshNow: () => fleet.fetchFeed({ silent: false }),
    requestPush: () => client.value?.requestRefresh(),
  }
}
