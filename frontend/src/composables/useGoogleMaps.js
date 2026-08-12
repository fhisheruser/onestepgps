import { ref, shallowRef } from 'vue'
import { Loader } from '@googlemaps/js-api-loader'

/**
 * Loads the Google Maps JavaScript API exactly once per page.
 *
 * The key is fetched from the backend at runtime (GET /api/v1/config) instead
 * of being baked into the bundle, so the same built image can be promoted
 * between environments. A Maps browser key is public by nature — restrict it
 * by HTTP referrer in Cloud Console; that, not secrecy, is what protects it.
 */

let loaderPromise = null
let loadedLibraries = null

export async function loadGoogleMaps(apiKey, { libraries = ['marker'] } = {}) {
  if (!apiKey) throw new Error('missing-api-key')
  if (loaderPromise) return loaderPromise

  const loader = new Loader({ apiKey, version: 'weekly', libraries })

  loaderPromise = (async () => {
    const [maps, marker] = await Promise.all([
      loader.importLibrary('maps'),
      libraries.includes('marker') ? loader.importLibrary('marker') : Promise.resolve(null),
    ])
    loadedLibraries = { maps, marker, google: window.google }
    return loadedLibraries
  })()

  try {
    return await loaderPromise
  } catch (error) {
    // Allow a later retry (e.g. after the user fixes their key/referrer).
    loaderPromise = null
    throw error
  }
}

export function useGoogleMaps() {
  const libraries = shallowRef(loadedLibraries)
  const ready = ref(Boolean(loadedLibraries))
  const loading = ref(false)
  const error = ref(null)

  async function load(apiKey) {
    if (ready.value) return libraries.value
    if (!apiKey) {
      error.value = 'missing-api-key'
      return null
    }

    loading.value = true
    error.value = null
    try {
      libraries.value = await loadGoogleMaps(apiKey)
      ready.value = true
      return libraries.value
    } catch (err) {
      error.value = err?.message === 'missing-api-key' ? 'missing-api-key' : 'load-failed'
      return null
    } finally {
      loading.value = false
    }
  }

  return { libraries, ready, loading, error, load }
}

/**
 * A warm, low-contrast map style that matches the beige UI and, crucially,
 * keeps vehicle markers as the highest-contrast thing on screen.
 */
export const MAP_STYLE_LIGHT = [
  { elementType: 'geometry', stylers: [{ color: '#f7f2e8' }] },
  { elementType: 'labels.text.fill', stylers: [{ color: '#7a6f60' }] },
  { elementType: 'labels.text.stroke', stylers: [{ color: '#faf6ee' }] },
  { featureType: 'administrative', elementType: 'geometry.stroke', stylers: [{ color: '#e0d3bd' }] },
  { featureType: 'landscape.natural', elementType: 'geometry', stylers: [{ color: '#f1ead9' }] },
  { featureType: 'poi', elementType: 'labels', stylers: [{ visibility: 'off' }] },
  { featureType: 'poi.park', elementType: 'geometry', stylers: [{ color: '#e4ead9' }] },
  { featureType: 'road', elementType: 'geometry', stylers: [{ color: '#ffffff' }] },
  { featureType: 'road.arterial', elementType: 'geometry', stylers: [{ color: '#fdfbf7' }] },
  { featureType: 'road.highway', elementType: 'geometry', stylers: [{ color: '#f0dcc8' }] },
  { featureType: 'road.highway', elementType: 'geometry.stroke', stylers: [{ color: '#e2c3a5' }] },
  { featureType: 'road', elementType: 'labels.icon', stylers: [{ visibility: 'off' }] },
  { featureType: 'transit', stylers: [{ visibility: 'off' }] },
  { featureType: 'water', elementType: 'geometry', stylers: [{ color: '#cfdde3' }] },
  { featureType: 'water', elementType: 'labels.text.fill', stylers: [{ color: '#8fa3ac' }] },
]

export const MAP_STYLE_DARK = [
  { elementType: 'geometry', stylers: [{ color: '#1d1916' }] },
  { elementType: 'labels.text.fill', stylers: [{ color: '#9c9184' }] },
  { elementType: 'labels.text.stroke', stylers: [{ color: '#141110' }] },
  { featureType: 'administrative', elementType: 'geometry.stroke', stylers: [{ color: '#3a322b' }] },
  { featureType: 'landscape.natural', elementType: 'geometry', stylers: [{ color: '#221d19' }] },
  { featureType: 'poi', elementType: 'labels', stylers: [{ visibility: 'off' }] },
  { featureType: 'poi.park', elementType: 'geometry', stylers: [{ color: '#26302a' }] },
  { featureType: 'road', elementType: 'geometry', stylers: [{ color: '#2e2823' }] },
  { featureType: 'road.highway', elementType: 'geometry', stylers: [{ color: '#453a30' }] },
  { featureType: 'road', elementType: 'labels.icon', stylers: [{ visibility: 'off' }] },
  { featureType: 'transit', stylers: [{ visibility: 'off' }] },
  { featureType: 'water', elementType: 'geometry', stylers: [{ color: '#16232a' }] },
]
