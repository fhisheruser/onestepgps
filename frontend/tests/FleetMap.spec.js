import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// Google Maps is never really loaded in jsdom; we only care about *when* the
// component asks for it.
const importLibrary = vi.hoisted(() => vi.fn().mockResolvedValue({}))
vi.mock('@googlemaps/js-api-loader', () => ({
  Loader: class {
    importLibrary = importLibrary
  },
}))
vi.mock('@googlemaps/markerclusterer', () => ({ MarkerClusterer: class {} }))

import FleetMap from '@/components/FleetMap.vue'
import { usePreferencesStore } from '@/stores/preferences'

function stubGoogleGlobals() {
  window.google = {
    maps: {
      Map: class {
        setOptions() {}
        setMapTypeId() {}
      },
      Marker: class {
        addListener() {}
        setIcon() {}
        setZIndex() {}
        setMap() {}
        getPosition() {
          return {}
        }
      },
      Size: class {},
      Point: class {},
      Polyline: class {},
      LatLngBounds: class {
        extend() {}
      },
    },
  }
}

describe('FleetMap key timing', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    stubGoogleGlobals()
  })

  it('waits for the runtime config instead of latching an error on mount', async () => {
    const preferences = usePreferencesStore()
    // Mirrors reality: this child mounts before the parent has fetched /config.
    preferences.runtimeConfigLoaded = false
    preferences.runtimeConfig.googleMapsApiKey = ''

    const wrapper = mount(FleetMap)
    await wrapper.vm.$nextTick()

    expect(importLibrary).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Loading map')
    expect(wrapper.text()).not.toContain('Map key not configured')
  })

  it('initialises the map once the key arrives', async () => {
    const preferences = usePreferencesStore()
    preferences.runtimeConfigLoaded = false
    preferences.runtimeConfig.googleMapsApiKey = ''

    const wrapper = mount(FleetMap)
    await wrapper.vm.$nextTick()

    // The config request lands.
    preferences.runtimeConfig.googleMapsApiKey = 'AIza-test-key'
    preferences.runtimeConfigLoaded = true
    await new Promise((resolve) => setTimeout(resolve, 0))
    await wrapper.vm.$nextTick()

    expect(importLibrary).toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('Map key not configured')
  })

  it('still reports a genuinely missing key once config has loaded', async () => {
    const preferences = usePreferencesStore()
    preferences.runtimeConfigLoaded = true
    preferences.runtimeConfig.googleMapsApiKey = ''

    const wrapper = mount(FleetMap)
    await wrapper.vm.$nextTick()

    expect(importLibrary).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Map key not configured')
  })
})
