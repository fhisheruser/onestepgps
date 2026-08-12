import { defineStore } from 'pinia'
import { fleetApi } from '@/services/api'
import { useUiStore } from './ui'

const DEFAULT_SETTINGS = {
  theme: 'system',
  sortKey: 'name',
  sortDirection: 'asc',
  speedUnit: 'mph',
  mapType: 'roadmap',
  showOfflineDevices: true,
  clusterMarkers: true,
  showTrails: false,
  animateMarkers: true,
  autoFitBounds: true,
  refreshSeconds: 10,
}

/**
 * Owns everything the user personalises. Writes are optimistic — the UI
 * updates immediately and rolls back if the server rejects the change — because
 * a rename that lags behind the keystroke feels broken.
 */
export const usePreferencesStore = defineStore('preferences', {
  state: () => ({
    settings: { ...DEFAULT_SETTINGS },
    devicePreferences: {},
    runtimeConfig: {
      googleMapsApiKey: '',
      googleMapsMapId: '',
      refreshSeconds: 10,
      realtimeEnabled: true,
      demoMode: false,
      provider: '',
      version: '',
      maxIconBytes: 262144,
    },
    loading: false,
    saving: false,
    loaded: false,
    error: null,
  }),

  getters: {
    mapsConfigured: (state) => Boolean(state.runtimeConfig.googleMapsApiKey),
    maxIconKb: (state) => Math.round((state.runtimeConfig.maxIconBytes || 262144) / 1024),
    preferenceFor: (state) => (deviceId) => state.devicePreferences[deviceId] || null,
  },

  actions: {
    async loadRuntimeConfig() {
      try {
        this.runtimeConfig = { ...this.runtimeConfig, ...(await fleetApi.runtimeConfig()) }
      } catch (error) {
        // A missing runtime config is not fatal: the dashboard still works,
        // just without the map.
        this.error = error.message
      }
      return this.runtimeConfig
    },

    async load() {
      this.loading = true
      try {
        const data = await fleetApi.preferences()
        this.applySettings(data.settings)
        this.devicePreferences = Object.fromEntries((data.devices || []).map((pref) => [pref.deviceId, pref]))
        this.loaded = true
        this.error = null
      } catch (error) {
        this.error = error.message
        throw error
      } finally {
        this.loading = false
      }
    },

    /** Merge server-owned settings in, keeping the local theme in sync. */
    applySettings(settings) {
      if (!settings) return
      this.settings = { ...DEFAULT_SETTINGS, ...settings }

      const ui = useUiStore()
      if (ui.theme !== this.settings.theme) ui.setTheme(this.settings.theme)
    },

    async updateSettings(patch) {
      const previous = { ...this.settings }
      this.settings = { ...this.settings, ...patch }
      this.saving = true

      try {
        const saved = await fleetApi.updateSettings(patch)
        this.applySettings(saved)
        return saved
      } catch (error) {
        this.settings = previous
        useUiStore().notify(error.message, { type: 'error' })
        throw error
      } finally {
        this.saving = false
      }
    },

    async updateDevicePreference(deviceId, patch) {
      const previous = this.devicePreferences[deviceId]
      this.devicePreferences = {
        ...this.devicePreferences,
        [deviceId]: { deviceId, ...(previous || {}), ...patch },
      }
      this.saving = true

      try {
        const saved = await fleetApi.updateDevicePreference(deviceId, patch)
        this.devicePreferences = { ...this.devicePreferences, [deviceId]: saved }
        return saved
      } catch (error) {
        // Roll back so the UI never shows a value the server rejected.
        const rolledBack = { ...this.devicePreferences }
        if (previous) rolledBack[deviceId] = previous
        else delete rolledBack[deviceId]
        this.devicePreferences = rolledBack

        useUiStore().notify(error.message, { type: 'error' })
        throw error
      } finally {
        this.saving = false
      }
    },

    async resetDevicePreference(deviceId) {
      const previous = this.devicePreferences[deviceId]
      const next = { ...this.devicePreferences }
      delete next[deviceId]
      this.devicePreferences = next

      try {
        await fleetApi.deleteDevicePreference(deviceId)
      } catch (error) {
        if (previous) this.devicePreferences = { ...this.devicePreferences, [deviceId]: previous }
        useUiStore().notify(error.message, { type: 'error' })
        throw error
      }
    },

    async uploadIcon(deviceId, file) {
      this.saving = true
      try {
        const saved = await fleetApi.uploadIcon(deviceId, file)
        this.devicePreferences = { ...this.devicePreferences, [deviceId]: saved }
        useUiStore().notify('Custom marker uploaded', { type: 'success' })
        return saved
      } catch (error) {
        useUiStore().notify(error.message, { type: 'error' })
        throw error
      } finally {
        this.saving = false
      }
    },

    async removeIcon(deviceId) {
      try {
        const saved = await fleetApi.deleteIcon(deviceId)
        this.devicePreferences = { ...this.devicePreferences, [deviceId]: saved }
        return saved
      } catch (error) {
        useUiStore().notify(error.message, { type: 'error' })
        throw error
      }
    },

    async reorder(deviceIds) {
      try {
        await fleetApi.reorder(deviceIds)
        this.settings = { ...this.settings, sortKey: 'custom' }
        await this.load()
      } catch (error) {
        useUiStore().notify(error.message, { type: 'error' })
        throw error
      }
    },

    async resetAll() {
      try {
        await fleetApi.resetPreferences()
        this.settings = { ...DEFAULT_SETTINGS }
        this.devicePreferences = {}
        useUiStore().notify('All personalisation reset', { type: 'success' })
      } catch (error) {
        useUiStore().notify(error.message, { type: 'error' })
        throw error
      }
    },
  },
})

export { DEFAULT_SETTINGS }
