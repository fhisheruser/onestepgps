import { defineStore } from 'pinia'
import { fleetApi } from '@/services/api'
import { usePreferencesStore } from './preferences'
import { useUiStore } from './ui'

const EMPTY_SUMMARY = {
  total: 0,
  visible: 0,
  hidden: 0,
  online: 0,
  offline: 0,
  driving: 0,
  idle: 0,
  off: 0,
  avgSpeed: 0,
  speedUnit: 'mph',
  lastUpdated: null,
  stale: false,
}


export const useFleetStore = defineStore('fleet', {
  state: () => ({
    devices: [],
    summary: { ...EMPTY_SUMMARY },
    meta: { fetchedAt: null, ageSeconds: 0, stale: false, error: '', count: 0, serverTime: null },

    filters: {
      search: '',
      status: 'all',
      sortKey: '',
      sortDirection: '',
      includeHidden: false,
      onlyPinned: false,
    },

    loading: false,
    initialised: false,
    error: null,

    selectedDeviceId: null,
    history: {},
    historyLoading: false,
    historyWindowMinutes: 60,

    transport: 'polling',
    connectionState: 'idle',
    lastUpdateAt: null,
  }),

  getters: {
    selectedDevice: (state) => state.devices.find((device) => device.id === state.selectedDeviceId) || null,

    hasDevices: (state) => state.devices.length > 0,

   
    isFleetEmpty: (state) => state.initialised && state.summary.total === 0,

   
    isFilteredEmpty: (state) => state.initialised && state.summary.total > 0 && state.devices.length === 0,

    isFiltering: (state) =>
      Boolean(state.filters.search) || state.filters.status !== 'all' || state.filters.onlyPinned,

    mappableDevices: (state) => state.devices.filter((device) => device.position?.valid),

    selectedHistory: (state) => (state.selectedDeviceId ? state.history[state.selectedDeviceId] || [] : []),

    queryParams: (state) => ({
      search: state.filters.search,
      status: state.filters.status,
      sort: state.filters.sortKey,
      dir: state.filters.sortDirection,
      includeHidden: state.filters.includeHidden,
      pinned: state.filters.onlyPinned,
    }),

   
    realtimeQuery: (state) => ({
      search: state.filters.search,
      status: state.filters.status,
      sortKey: state.filters.sortKey,
      sortDirection: state.filters.sortDirection,
      includeHidden: state.filters.includeHidden,
      onlyPinned: state.filters.onlyPinned,
    }),
  },

  actions: {
    
    async fetchFeed({ silent = false } = {}) {
      if (!silent) this.loading = true
      try {
        this.applyFeed(await fleetApi.devices(this.queryParams))
        this.error = null
      } catch (error) {
        this.error = error.message
       
        if (!silent) useUiStore().notify(error.message, { type: 'error' })
      } finally {
        this.loading = false
        this.initialised = true
      }
    },

   
    applyFeed(feed) {
      if (!feed) return
      this.devices = feed.devices || []
      this.summary = { ...EMPTY_SUMMARY, ...(feed.summary || {}) }
      this.meta = { ...this.meta, ...(feed.meta || {}) }
      this.lastUpdateAt = Date.now()
      this.initialised = true

      if (feed.settings) usePreferencesStore().applySettings(feed.settings)
    },

    setFilters(patch) {
      this.filters = { ...this.filters, ...patch }
    },

    resetFilters() {
      this.filters = {
        ...this.filters,
        search: '',
        status: 'all',
        onlyPinned: false,
        includeHidden: false,
      }
    },

    selectDevice(deviceId) {
      this.selectedDeviceId = deviceId
      if (deviceId) {
        useUiStore().openDetail()
        void this.loadHistory(deviceId)
      }
    },

    clearSelection() {
      this.selectedDeviceId = null
      useUiStore().closeDetail()
    },

    async loadHistory(deviceId, minutes = this.historyWindowMinutes) {
      if (!deviceId) return []
      this.historyLoading = true
      this.historyWindowMinutes = minutes
      try {
        const data = await fleetApi.history(deviceId, minutes)
        this.history = { ...this.history, [deviceId]: data.points || [] }
        return this.history[deviceId]
      } catch {
       
        this.history = { ...this.history, [deviceId]: [] }
        return []
      } finally {
        this.historyLoading = false
      }
    },

   
    patchDevice(deviceId, patch) {
      this.devices = this.devices.map((device) =>
        device.id === deviceId
          ? { ...device, ...patch, preferences: { ...device.preferences, ...(patch.preferences || {}) } }
          : device,
      )
    },


    async renameDevice(deviceId, displayName) {
      const trimmed = String(displayName || '').trim()
      const device = this.devices.find((d) => d.id === deviceId)
      this.patchDevice(deviceId, { name: trimmed || device?.providerName, renamed: Boolean(trimmed) })

      await usePreferencesStore().updateDevicePreference(deviceId, { displayName: trimmed })
      await this.fetchFeed({ silent: true })
    },

    async setHidden(deviceId, hidden) {
      this.patchDevice(deviceId, { preferences: { hidden } })
      await usePreferencesStore().updateDevicePreference(deviceId, { hidden })

      if (hidden && this.selectedDeviceId === deviceId && !this.filters.includeHidden) {
        this.clearSelection()
      }
      await this.fetchFeed({ silent: true })
      useUiStore().notify(hidden ? 'Vehicle hidden' : 'Vehicle restored', { type: 'success', timeout: 2500 })
    },

    async setPinned(deviceId, pinned) {
      this.patchDevice(deviceId, { preferences: { pinned } })
      await usePreferencesStore().updateDevicePreference(deviceId, { pinned })
      await this.fetchFeed({ silent: true })
    },

    async setMarkerColor(deviceId, markerColor) {
      this.patchDevice(deviceId, { preferences: { markerColor } })
      await usePreferencesStore().updateDevicePreference(deviceId, { markerColor })
    },

    async setMarkerIcon(deviceId, markerIcon) {
      this.patchDevice(deviceId, { preferences: { markerIcon } })
      await usePreferencesStore().updateDevicePreference(deviceId, { markerIcon })
    },

    async setNotes(deviceId, notes) {
      this.patchDevice(deviceId, { preferences: { notes } })
      await usePreferencesStore().updateDevicePreference(deviceId, { notes })
    },

    async uploadIcon(deviceId, file) {
      const saved = await usePreferencesStore().uploadIcon(deviceId, file)
      this.patchDevice(deviceId, {
        preferences: { markerIcon: saved.markerIcon, customIconUrl: saved.customIconUrl },
      })
      await this.fetchFeed({ silent: true })
      return saved
    },

    async removeIcon(deviceId) {
      const saved = await usePreferencesStore().removeIcon(deviceId)
      this.patchDevice(deviceId, {
        preferences: { markerIcon: saved.markerIcon, customIconUrl: '' },
      })
      await this.fetchFeed({ silent: true })
    },

    async resetDevice(deviceId) {
      await usePreferencesStore().resetDevicePreference(deviceId)
      await this.fetchFeed({ silent: true })
      useUiStore().notify('Vehicle reset to defaults', { type: 'success', timeout: 2500 })
    },

   
    async reorderDevices(deviceIds) {
      await usePreferencesStore().reorder(deviceIds)
      this.setFilters({ sortKey: 'custom', sortDirection: 'asc' })
      await this.fetchFeed({ silent: true })
    },

    exportCsv() {
      const url = fleetApi.exportCsvUrl(this.queryParams)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.rel = 'noopener'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      useUiStore().notify('Export started', { type: 'success', timeout: 2500 })
    },
  },
})
