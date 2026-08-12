<script setup>
import { computed, ref, watch } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import AppIcon from './AppIcon.vue'
import Vehicle3D from './Vehicle3D.vue'
import MarkerIconPicker from './MarkerIconPicker.vue'
import {
  driveStatusMeta,
  formatCoords,
  formatDuration,
  formatOdometer,
  formatSpeed,
  formatTimestamp,
  headingToCompass,
  timeAgo,
} from '@/utils/format'

const fleet = useFleetStore()
const preferences = usePreferencesStore()
const ui = useUiStore()

const device = computed(() => fleet.selectedDevice)
const prefs = computed(() => device.value?.preferences || {})
const position = computed(() => device.value?.position || {})
const speedUnit = computed(() => preferences.settings.speedUnit || 'mph')
const status = computed(() => driveStatusMeta(device.value?.driveStatus))

const nameDraft = ref('')
const notesDraft = ref('')
const savingName = ref(false)

// Reset the drafts whenever a different vehicle is selected, so an unsaved
// edit never leaks onto the next one.
watch(
  () => device.value?.id,
  () => {
    nameDraft.value = device.value?.renamed ? device.value.name : ''
    notesDraft.value = prefs.value.notes || ''
  },
  { immediate: true },
)

const HISTORY_WINDOWS = [
  { value: 15, label: '15m' },
  { value: 60, label: '1h' },
  { value: 240, label: '4h' },
  { value: 1440, label: '24h' },
]

const trailPoints = computed(() => fleet.selectedHistory.length)

async function saveName() {
  if (!device.value) return
  savingName.value = true
  try {
    await fleet.renameDevice(device.value.id, nameDraft.value)
    ui.notify(nameDraft.value.trim() ? 'Vehicle renamed' : 'Original name restored', {
      type: 'success',
      timeout: 2500,
    })
  } catch {
    nameDraft.value = device.value?.renamed ? device.value.name : ''
  } finally {
    savingName.value = false
  }
}

async function saveNotes() {
  if (!device.value) return
  if ((prefs.value.notes || '') === notesDraft.value) return
  await fleet.setNotes(device.value.id, notesDraft.value)
}

async function resetDevice() {
  if (!device.value) return
  await fleet.resetDevice(device.value.id)
  nameDraft.value = ''
  notesDraft.value = ''
}
</script>

<template>
  <Transition
    enter-active-class="transition duration-300 ease-smooth"
    enter-from-class="translate-x-full opacity-0"
    leave-active-class="transition duration-200 ease-smooth"
    leave-to-class="translate-x-full opacity-0"
  >
    <aside
      v-if="ui.detailOpen && device"
      class="panel absolute inset-y-0 right-0 z-30 flex w-full max-w-sm flex-col overflow-hidden rounded-none
             border-y-0 border-r-0 shadow-lifted sm:rounded-l-xl2 md:m-3 md:inset-y-3 md:rounded-xl2 md:border"
      aria-label="Vehicle details"
    >
      <header
        class="flex items-start gap-2 border-b border-sand-300/70 bg-sand-100/70 px-4 py-3 dark:border-ink-600/60 dark:bg-ink-800/70"
      >
        <div class="min-w-0 flex-1">
          <h2 class="truncate text-base font-semibold text-ink-800 dark:text-sand-50">{{ device.name }}</h2>
          <p class="truncate text-xs text-ink-400 dark:text-sand-500">
            {{ device.make }} {{ device.model }}
            <span v-if="device.renamed"> · originally {{ device.providerName }}</span>
          </p>
        </div>
        <button type="button" class="btn-ghost px-1.5 py-1" aria-label="Close details" @click="fleet.clearSelection()">
          <AppIcon name="close" :size="18" />
        </button>
      </header>

      <div class="flex-1 space-y-5 overflow-y-auto p-4">
        <!-- Hero: a 3D model of the vehicle in its own marker colour, turned
             to match the heading the tracker last reported. -->
        <div
          class="relative grid place-items-center rounded-xl2 bg-gradient-to-b from-sand-200/70 to-sand-100/40 py-3 dark:from-ink-700/60 dark:to-ink-800/40"
        >
          <Vehicle3D
            :type="prefs.markerIcon === 'custom' || prefs.markerIcon === 'pin' ? 'van' : prefs.markerIcon || 'car'"
            :color="prefs.markerColor || '#B4643C'"
            :size="150"
            :heading="position.valid ? position.heading - 90 : -30"
            :spin="device.driveStatus !== 'driving'"
          />
          <span class="chip absolute right-2 top-2 bg-sand-50/80 text-ink-500 dark:bg-ink-900/70 dark:text-sand-300">
            <span class="h-1.5 w-1.5 rounded-full" :style="{ background: status.dot }" />
            {{ status.label }}
          </span>
        </div>

        <dl class="grid grid-cols-2 gap-2">
          <div class="panel-flush rounded-xl px-3 py-2">
            <dt class="label">Speed</dt>
            <dd class="text-base font-semibold tabular-nums text-ink-700 dark:text-sand-100">
              {{ formatSpeed(position.speed, position.speedUnit, speedUnit) }}
            </dd>
          </div>
          <div class="panel-flush rounded-xl px-3 py-2">
            <dt class="label">Heading</dt>
            <dd class="text-base font-semibold tabular-nums text-ink-700 dark:text-sand-100">
              {{ position.valid ? `${Math.round(position.heading)}° ${headingToCompass(position.heading)}` : '—' }}
            </dd>
          </div>
          <div class="panel-flush rounded-xl px-3 py-2">
            <dt class="label">In state for</dt>
            <dd class="text-base font-semibold text-ink-700 dark:text-sand-100">
              {{ formatDuration(device.driveStatusSeconds) }}
            </dd>
          </div>
          <div class="panel-flush rounded-xl px-3 py-2">
            <dt class="label">Odometer</dt>
            <dd class="text-base font-semibold tabular-nums text-ink-700 dark:text-sand-100">
              {{ formatOdometer(device.odometer, device.odometerUnit) }}
            </dd>
          </div>
        </dl>

        <div class="panel-flush space-y-1.5 rounded-xl px-3 py-2.5 text-xs">
          <div class="flex justify-between gap-3">
            <span class="text-ink-400 dark:text-sand-500">Coordinates</span>
            <span class="font-mono tabular-nums text-ink-600 dark:text-sand-200">
              {{ position.valid ? formatCoords(position.lat, position.lng) : 'No fix' }}
            </span>
          </div>
          <div class="flex justify-between gap-3">
            <span class="text-ink-400 dark:text-sand-500">Last report</span>
            <span class="text-ink-600 dark:text-sand-200" :title="formatTimestamp(position.recordedAt)">
              {{ timeAgo(position.recordedAt) }}
            </span>
          </div>
          <div class="flex justify-between gap-3">
            <span class="text-ink-400 dark:text-sand-500">Connection</span>
            <span :class="device.online ? 'text-sage-700 dark:text-sage-300' : 'text-clay-500 dark:text-clay-300'">
              {{ device.online ? 'Online' : 'Offline' }}
            </span>
          </div>
          <div v-if="device.groups?.length" class="flex justify-between gap-3">
            <span class="text-ink-400 dark:text-sand-500">Groups</span>
            <span class="text-right text-ink-600 dark:text-sand-200">{{ device.groups.join(', ') }}</span>
          </div>
          <div class="flex justify-between gap-3">
            <span class="text-ink-400 dark:text-sand-500">Device ID</span>
            <span class="truncate font-mono text-[11px] text-ink-500 dark:text-sand-400">{{ device.id }}</span>
          </div>
        </div>

        <section class="space-y-2">
          <h3 class="label">Rename</h3>
          <div class="flex gap-2">
            <input
              v-model="nameDraft"
              class="field"
              :placeholder="device.providerName"
              maxlength="64"
              aria-label="Custom vehicle name"
              @keyup.enter="saveName"
            />
            <button type="button" class="btn-primary shrink-0" :disabled="savingName" @click="saveName">
              <AppIcon name="check" :size="15" />
            </button>
          </div>
          <p class="text-[11px] text-ink-400 dark:text-sand-500">Leave empty to restore the provider's name.</p>
        </section>

        <section class="space-y-2">
          <h3 class="label">Notes</h3>
          <textarea
            v-model="notesDraft"
            class="field min-h-[68px] resize-y"
            maxlength="500"
            placeholder="Trailer type, driver, service due…"
            aria-label="Vehicle notes"
            @blur="saveNotes"
          />
        </section>

        <section class="space-y-2">
          <h3 class="label">Map marker</h3>
          <MarkerIconPicker
            :marker-icon="prefs.markerIcon"
            :marker-color="prefs.markerColor"
            :custom-icon-url="prefs.customIconUrl"
            @update:marker-icon="fleet.setMarkerIcon(device.id, $event)"
            @update:marker-color="fleet.setMarkerColor(device.id, $event)"
            @upload="fleet.uploadIcon(device.id, $event)"
            @remove-icon="fleet.removeIcon(device.id)"
          />
        </section>

        <section class="space-y-2">
          <div class="flex items-center justify-between">
            <h3 class="label">History trail</h3>
            <span class="text-[11px] text-ink-400 dark:text-sand-500">
              {{ fleet.historyLoading ? 'loading…' : `${trailPoints} points` }}
            </span>
          </div>
          <div class="flex gap-1.5">
            <button
              v-for="window in HISTORY_WINDOWS"
              :key="window.value"
              type="button"
              class="chip flex-1 justify-center border transition-colors duration-200"
              :class="
                fleet.historyWindowMinutes === window.value
                  ? 'border-clay-300 bg-clay-100 text-clay-600 dark:border-clay-400/50 dark:bg-clay-700/40 dark:text-clay-100'
                  : 'border-sand-300 text-ink-500 dark:border-ink-600 dark:text-sand-300'
              "
              @click="fleet.loadHistory(device.id, window.value)"
            >
              {{ window.label }}
            </button>
          </div>
          <p v-if="!preferences.settings.showTrails" class="text-[11px] text-ink-400 dark:text-sand-500">
            Enable “Show history trails” in settings to draw this on the map.
          </p>
        </section>
      </div>

      <footer
        class="flex items-center gap-2 border-t border-sand-300/70 bg-sand-100/70 px-4 py-2.5 dark:border-ink-600/60 dark:bg-ink-800/70"
      >
        <button
          type="button"
          class="btn-outline flex-1 text-xs"
          @click="fleet.setHidden(device.id, !prefs.hidden)"
        >
          <AppIcon :name="prefs.hidden ? 'eye' : 'eye-off'" :size="14" />
          {{ prefs.hidden ? 'Show' : 'Hide' }}
        </button>
        <button type="button" class="btn-outline flex-1 text-xs" @click="fleet.setPinned(device.id, !prefs.pinned)">
          <AppIcon name="pin" :size="14" />
          {{ prefs.pinned ? 'Unpin' : 'Pin' }}
        </button>
        <button type="button" class="btn-ghost text-xs text-clay-500" title="Reset all customisation" @click="resetDevice">
          <AppIcon name="refresh" :size="14" />
          Reset
        </button>
      </footer>
    </aside>
  </Transition>
</template>
