import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import { ConnectionState, RealtimeClient } from '@/services/realtime'


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
      
      if (document.visibilityState === 'hidden') return
      void fleet.fetchFeed({ silent: true })
    }, refreshSeconds() * 1000)
  }

  function startHeartbeat() {
    if (heartbeatTimer.value) window.clearInterval(heartbeatTimer.value)
   
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
   
    void fleet.fetchFeed({ silent: true })
    if (fleet.transport !== 'realtime') client.value?.connect()
  }

  async function bootstrap() {
    await preferences.loadRuntimeConfig()
    try {
      await preferences.load()
    } catch {
     
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
