<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { MarkerClusterer } from '@googlemaps/markerclusterer'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import { MAP_STYLE_DARK, MAP_STYLE_LIGHT, useGoogleMaps } from '@/composables/useGoogleMaps'
import AppIcon from './AppIcon.vue'
import EmptyState from './EmptyState.vue'

const fleet = useFleetStore()
const preferences = usePreferencesStore()
const ui = useUiStore()
const { ready, loading, error, load } = useGoogleMaps()

const mapEl = ref(null)
let map = null
let clusterer = null
let trail = null
const markers = new Map()
const hasFitted = ref(false)

const settings = computed(() => preferences.settings)

// ponytail: classic google.maps.Marker, not AdvancedMarkerElement. Advanced
// markers require a cloud-configured Map ID, which would also disable the
// `styles` array this beige theme depends on. Swap both together if the
// deprecation ever turns into a removal.
function markerIcon(device) {
  const prefs = device.preferences || {}
  const google = window.google

  if (prefs.markerIcon === 'custom' && prefs.customIconUrl) {
    return {
      url: prefs.customIconUrl,
      scaledSize: new google.maps.Size(40, 40),
      anchor: new google.maps.Point(20, 20),
    }
  }

  const color = prefs.markerColor || '#B4643C'
  const moving = device.driveStatus === 'driving'
  const heading = Number.isFinite(device.position?.heading) ? device.position.heading : 0
  const stroke = device.online ? '#FDFBF7' : '#C4BCB0'

  // A teardrop when moving (points along the heading), a disc when parked.
  const glyph = moving
    ? `<path d="M20 4 L30 30 L20 25 L10 30 Z" fill="${color}" stroke="${stroke}" stroke-width="2.5" stroke-linejoin="round" transform="rotate(${heading} 20 20)"/>`
    : `<circle cx="20" cy="20" r="9" fill="${color}" stroke="${stroke}" stroke-width="3"/>`

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40">
      <circle cx="20" cy="20" r="15" fill="${color}" opacity="${moving ? 0.18 : 0.12}"/>${glyph}</svg>`

  return {
    url: `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(svg)}`,
    scaledSize: new google.maps.Size(40, 40),
    anchor: new google.maps.Point(20, 20),
  }
}

function syncMarkers() {
  if (!map) return
  const google = window.google
  const seen = new Set()

  for (const device of fleet.mappableDevices) {
    seen.add(device.id)
    const position = { lat: device.position.lat, lng: device.position.lng }
    let marker = markers.get(device.id)

    if (!marker) {
      marker = new google.maps.Marker({ position, title: device.name, optimized: true })
      marker.addListener('click', () => fleet.selectDevice(device.id))
      markers.set(device.id, marker)
      clusterer?.addMarker(marker)
      if (!clusterer) marker.setMap(map)
    } else {
      // Moving an existing marker (rather than recreating it) is what makes
      // the Maps API animate the vehicle instead of teleporting it.
      marker.setPosition(position)
      marker.setTitle(device.name)
    }

    marker.setIcon(markerIcon(device))
    marker.setZIndex(device.id === fleet.selectedDeviceId ? 999 : device.driveStatus === 'driving' ? 10 : 1)
  }

  for (const [id, marker] of markers) {
    if (seen.has(id)) continue
    clusterer?.removeMarker(marker)
    marker.setMap(null)
    markers.delete(id)
  }

  if (settings.value.autoFitBounds && !hasFitted.value && markers.size > 0) {
    fitBounds()
    hasFitted.value = true
  }
}

function fitBounds() {
  if (!map || markers.size === 0) return
  const bounds = new window.google.maps.LatLngBounds()
  markers.forEach((marker) => bounds.extend(marker.getPosition()))

  if (markers.size === 1) {
    map.setCenter(bounds.getCenter())
    map.setZoom(14)
    return
  }
  map.fitBounds(bounds, 64)
}

function syncClusterer() {
  const google = window.google
  if (!map || !google) return

  if (settings.value.clusterMarkers && !clusterer) {
    clusterer = new MarkerClusterer({ map, markers: [...markers.values()] })
  } else if (!settings.value.clusterMarkers && clusterer) {
    clusterer.clearMarkers()
    clusterer.setMap(null)
    clusterer = null
    markers.forEach((marker) => marker.setMap(map))
  }
}

function syncTrail() {
  const google = window.google
  if (!map || !google) return

  trail?.setMap(null)
  trail = null

  if (!settings.value.showTrails) return
  const points = fleet.selectedHistory
  if (points.length < 2) return

  trail = new google.maps.Polyline({
    map,
    path: points.map((point) => ({ lat: point.lat, lng: point.lng })),
    strokeColor: fleet.selectedDevice?.preferences?.markerColor || '#B4643C',
    strokeOpacity: 0.75,
    strokeWeight: 3,
  })
}

async function init() {
  const libraries = await load(preferences.runtimeConfig.googleMapsApiKey)
  if (!libraries || !mapEl.value) return

  map = new window.google.maps.Map(mapEl.value, {
    center: { lat: 32.7157, lng: -117.1611 },
    zoom: 11,
    mapTypeId: settings.value.mapType || 'roadmap',
    styles: ui.isDark ? MAP_STYLE_DARK : MAP_STYLE_LIGHT,
    disableDefaultUI: true,
    zoomControl: true,
    gestureHandling: 'greedy',
    clickableIcons: false,
  })

  syncClusterer()
  syncMarkers()
  syncTrail()
}

// Initialise when the key arrives, not on mount. This component mounts before
// its parent has fetched /api/v1/config, so mounting-time init would always
// see an empty key and latch the "not configured" state for good.
watch(
  () => preferences.runtimeConfig.googleMapsApiKey,
  (key) => {
    if (key && !ready.value) void init()
  },
  { immediate: true },
)

/** What the overlay should say, given config may still be in flight. */
const mapState = computed(() => {
  if (loading.value || !preferences.runtimeConfigLoaded) return 'loading'
  if (!preferences.runtimeConfig.googleMapsApiKey || error.value === 'missing-api-key') return 'missing-key'
  if (error.value) return 'load-failed'
  return 'ready'
})

onBeforeUnmount(() => {
  markers.forEach((marker) => marker.setMap(null))
  markers.clear()
  clusterer?.setMap(null)
  trail?.setMap(null)
})

watch(() => fleet.devices, syncMarkers, { deep: true })
watch(() => [settings.value.clusterMarkers], syncClusterer)
watch(() => [fleet.selectedHistory, settings.value.showTrails], syncTrail, { deep: true })
watch(() => settings.value.mapType, (type) => map?.setMapTypeId(type || 'roadmap'))
watch(() => ui.isDark, (dark) => map?.setOptions({ styles: dark ? MAP_STYLE_DARK : MAP_STYLE_LIGHT }))

// Centre on the vehicle the user just picked, without changing their zoom.
watch(
  () => fleet.selectedDeviceId,
  (id) => {
    const marker = id && markers.get(id)
    if (marker && map) map.panTo(marker.getPosition())
  },
)
</script>

<template>
  <div class="relative h-full w-full overflow-hidden">
    <div ref="mapEl" class="h-full w-full" role="application" aria-label="Fleet map" />

    <div
      v-if="mapState === 'loading'"
      class="absolute inset-0 grid place-items-center bg-sand-100/80 backdrop-blur-sm dark:bg-ink-900/80"
    >
      <div class="flex items-center gap-2 text-sm text-ink-500 dark:text-sand-300">
        <AppIcon name="refresh" :size="16" class="animate-spin" />
        Loading map…
      </div>
    </div>

    <div
      v-else-if="mapState !== 'ready'"
      class="absolute inset-0 grid place-items-center bg-sand-100 p-6 dark:bg-ink-900"
    >
      <EmptyState
        vehicle="pin"
        :title="mapState === 'missing-key' ? 'Map key not configured' : 'The map failed to load'"
        :description="
          mapState === 'missing-key'
            ? 'Set GOOGLE_MAPS_API_KEY on the server and reload. The vehicle list works without it.'
            : 'Google Maps rejected the request. Check the key’s HTTP referrer restrictions and that the Maps JavaScript API is enabled.'
        "
        action-label="Retry"
        @action="init"
      />
    </div>

    <div v-if="mapState === 'ready'" class="absolute bottom-4 right-3 flex flex-col gap-1.5">
      <button type="button" class="panel btn-ghost h-9 w-9 !px-0 shadow-soft" title="Fit all vehicles" aria-label="Fit all vehicles" @click="fitBounds">
        <AppIcon name="expand" :size="16" />
      </button>
      <button
        v-if="fleet.selectedDeviceId"
        type="button"
        class="panel btn-ghost h-9 w-9 !px-0 shadow-soft"
        title="Centre on selected vehicle"
        aria-label="Centre on selected vehicle"
        @click="fleet.selectDevice(fleet.selectedDeviceId)"
      >
        <AppIcon name="crosshair" :size="16" />
      </button>
    </div>
  </div>
</template>
