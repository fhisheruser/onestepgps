import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  devices: vi.fn(),
  history: vi.fn(),
  updateDevicePreference: vi.fn(),
  preferences: vi.fn(),
  runtimeConfig: vi.fn(),
  exportCsvUrl: vi.fn(() => '/api/v1/export/devices.csv'),
}))

vi.mock('@/services/api', () => ({ fleetApi: api, API_BASE: '/api/v1', resolveUserId: () => 'test-user' }))

import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'

const FEED = {
  devices: [
    {
      id: 'd1',
      name: 'Truck 04',
      providerName: 'Truck 04',
      renamed: false,
      online: true,
      driveStatus: 'driving',
      position: { lat: 32.7, lng: -117.1, speed: 55, speedUnit: 'km/h', valid: true },
      preferences: { hidden: false, pinned: false, markerIcon: 'truck', markerColor: '#B4643C' },
    },
    {
      id: 'd2',
      name: 'Van 12',
      providerName: 'Van 12',
      renamed: false,
      online: false,
      driveStatus: 'off',
      position: { lat: 0, lng: 0, speed: 0, speedUnit: 'km/h', valid: false },
      preferences: { hidden: false, pinned: false, markerIcon: 'van', markerColor: '#B4643C' },
    },
  ],
  summary: { total: 2, visible: 2, driving: 1, off: 1, offline: 1, speedUnit: 'mph' },
  meta: { fetchedAt: '2026-08-11T12:00:00Z', ageSeconds: 2, stale: false, error: '' },
  settings: { speedUnit: 'kph', sortKey: 'name', theme: 'system' },
}

describe('fleet store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    api.devices.mockResolvedValue(structuredClone(FEED))
  })

  it('applies a feed from either transport identically', async () => {
    const fleet = useFleetStore()
    await fleet.fetchFeed()

    expect(fleet.devices).toHaveLength(2)
    expect(fleet.summary.driving).toBe(1)
    expect(fleet.initialised).toBe(true)
    expect(fleet.error).toBeNull()

    // A WebSocket push carries the same envelope.
    fleet.applyFeed({ ...structuredClone(FEED), summary: { ...FEED.summary, driving: 2 } })
    expect(fleet.summary.driving).toBe(2)
  })

  it('adopts the settings that ride along with the feed', async () => {
    const fleet = useFleetStore()
    await fleet.fetchFeed()
    expect(usePreferencesStore().settings.speedUnit).toBe('kph')
  })

  it('only offers mappable devices to the map', async () => {
    const fleet = useFleetStore()
    await fleet.fetchFeed()
    expect(fleet.mappableDevices.map((d) => d.id)).toEqual(['d1'])
  })

  it('distinguishes an empty fleet from an over-filtered one', async () => {
    const fleet = useFleetStore()

    api.devices.mockResolvedValueOnce({ devices: [], summary: { total: 0, visible: 0 }, meta: {} })
    await fleet.fetchFeed()
    expect(fleet.isFleetEmpty).toBe(true)
    expect(fleet.isFilteredEmpty).toBe(false)

    api.devices.mockResolvedValueOnce({ devices: [], summary: { total: 9, visible: 0 }, meta: {} })
    await fleet.fetchFeed()
    expect(fleet.isFleetEmpty).toBe(false)
    expect(fleet.isFilteredEmpty).toBe(true)
  })

  it('keeps the last good data when a background refresh fails', async () => {
    const fleet = useFleetStore()
    await fleet.fetchFeed()

    api.devices.mockRejectedValueOnce(Object.assign(new Error('Cannot reach the FleetView server.'), { code: 'network_error' }))
    await fleet.fetchFeed({ silent: true })

    expect(fleet.error).toBe('Cannot reach the FleetView server.')
    expect(fleet.devices).toHaveLength(2)
    expect(useUiStore().toasts).toHaveLength(0)
  })

  it('translates filters into the query the API expects', () => {
    const fleet = useFleetStore()
    fleet.setFilters({ search: 'ford', status: 'driving', onlyPinned: true })

    expect(fleet.queryParams).toMatchObject({ search: 'ford', status: 'driving', pinned: true })
    expect(fleet.isFiltering).toBe(true)

    fleet.resetFilters()
    expect(fleet.isFiltering).toBe(false)
  })

  it('hides a device optimistically and drops the selection with it', async () => {
    const fleet = useFleetStore()
    await fleet.fetchFeed()

    api.history.mockResolvedValue({ points: [] })
    fleet.selectDevice('d1')
    api.updateDevicePreference.mockResolvedValue({ deviceId: 'd1', hidden: true })

    await fleet.setHidden('d1', true)

    expect(api.updateDevicePreference).toHaveBeenCalledWith('d1', { hidden: true })
    expect(fleet.selectedDeviceId).toBeNull()
  })

  it('rolls a rejected preference back instead of leaving a lie on screen', async () => {
    const fleet = useFleetStore()
    const preferences = usePreferencesStore()
    await fleet.fetchFeed()

    api.updateDevicePreference.mockRejectedValueOnce(Object.assign(new Error('markerColor must be a hex colour'), { code: 'validation_failed' }))

    await expect(preferences.updateDevicePreference('d1', { markerColor: 'puce' })).rejects.toThrow()
    expect(preferences.devicePreferences.d1).toBeUndefined()
    expect(useUiStore().toasts[0].type).toBe('error')
  })

  it('records a stale snapshot without discarding the vehicles', async () => {
    const fleet = useFleetStore()
    fleet.applyFeed({ ...structuredClone(FEED), meta: { stale: true, error: 'provider timeout' } })

    expect(fleet.meta.stale).toBe(true)
    expect(fleet.meta.error).toBe('provider timeout')
    expect(fleet.devices).toHaveLength(2)
  })

  it('never lets a failed history fetch break tracking', async () => {
    const fleet = useFleetStore()
    api.history.mockRejectedValueOnce(new Error('boom'))

    await expect(fleet.loadHistory('d1')).resolves.toEqual([])
    expect(fleet.historyLoading).toBe(false)
  })
})
