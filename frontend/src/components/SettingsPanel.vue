<script setup>
import { computed } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import AppIcon from './AppIcon.vue'

const fleet = useFleetStore()
const preferences = usePreferencesStore()
const ui = useUiStore()

const settings = computed(() => preferences.settings)

const TOGGLES = [
  { key: 'showOfflineDevices', label: 'Show offline vehicles', hint: 'Include vehicles that have stopped reporting' },
  { key: 'clusterMarkers', label: 'Cluster map markers', hint: 'Group nearby vehicles at low zoom' },
  { key: 'showTrails', label: 'Show history trails', hint: 'Draw the selected vehicle’s recent path' },
  { key: 'autoFitBounds', label: 'Auto-fit the map', hint: 'Zoom to the whole fleet on first load' },
  { key: 'animateMarkers', label: 'Animate marker moves', hint: 'Glide markers between positions' },
]

async function update(patch) {
  await preferences.updateSettings(patch)
  await fleet.fetchFeed({ silent: true })
}

async function resetAll() {
  await preferences.resetAll()
  await fleet.fetchFeed({ silent: true })
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
      v-if="ui.settingsOpen"
      class="panel absolute inset-y-0 right-0 z-40 flex w-full max-w-sm flex-col overflow-hidden rounded-none
             border-y-0 border-r-0 shadow-lifted md:inset-y-3 md:m-3 md:rounded-xl2 md:border"
      aria-label="Settings"
    >
      <header
        class="flex items-center gap-2 border-b border-sand-300/70 bg-sand-100/70 px-4 py-3 dark:border-ink-600/60 dark:bg-ink-800/70"
      >
        <AppIcon name="sliders" :size="18" class="text-clay-400" />
        <h2 class="flex-1 text-base font-semibold text-ink-800 dark:text-sand-50">Settings</h2>
        <button type="button" class="btn-ghost px-1.5 py-1" aria-label="Close settings" @click="ui.closeSettings()">
          <AppIcon name="close" :size="18" />
        </button>
      </header>

      <div class="flex-1 space-y-6 overflow-y-auto p-4">
        <section class="space-y-2">
          <h3 class="label">Appearance</h3>
          <div class="grid grid-cols-3 gap-1.5">
            <button
              v-for="option in [
                { value: 'light', label: 'Light', icon: 'sun' },
                { value: 'dark', label: 'Dark', icon: 'moon' },
                { value: 'system', label: 'System', icon: 'monitor' },
              ]"
              :key="option.value"
              type="button"
              class="flex flex-col items-center gap-1 rounded-xl border px-2 py-2.5 text-xs font-medium transition-colors duration-200"
              :class="
                settings.theme === option.value
                  ? 'border-clay-300 bg-clay-50 text-clay-600 dark:border-clay-400/60 dark:bg-clay-700/25 dark:text-clay-100'
                  : 'border-sand-300 text-ink-500 hover:border-sand-400 dark:border-ink-600 dark:text-sand-300'
              "
              :aria-pressed="settings.theme === option.value"
              @click="update({ theme: option.value })"
            >
              <AppIcon :name="option.icon" :size="16" />
              {{ option.label }}
            </button>
          </div>
        </section>

        <section class="space-y-2">
          <h3 class="label">Units</h3>
          <div class="grid grid-cols-3 gap-1.5">
            <button
              v-for="unit in [
                { value: 'mph', label: 'mph' },
                { value: 'kph', label: 'km/h' },
                { value: 'kn', label: 'knots' },
              ]"
              :key="unit.value"
              type="button"
              class="rounded-xl border px-2 py-2 text-xs font-medium transition-colors duration-200"
              :class="
                settings.speedUnit === unit.value
                  ? 'border-clay-300 bg-clay-50 text-clay-600 dark:border-clay-400/60 dark:bg-clay-700/25 dark:text-clay-100'
                  : 'border-sand-300 text-ink-500 hover:border-sand-400 dark:border-ink-600 dark:text-sand-300'
              "
              :aria-pressed="settings.speedUnit === unit.value"
              @click="update({ speedUnit: unit.value })"
            >
              {{ unit.label }}
            </button>
          </div>
        </section>

        <section class="space-y-2">
          <h3 class="label">Map style</h3>
          <div class="grid grid-cols-4 gap-1.5">
            <button
              v-for="type in ['roadmap', 'satellite', 'hybrid', 'terrain']"
              :key="type"
              type="button"
              class="rounded-xl border px-1 py-2 text-[11px] font-medium capitalize transition-colors duration-200"
              :class="
                settings.mapType === type
                  ? 'border-clay-300 bg-clay-50 text-clay-600 dark:border-clay-400/60 dark:bg-clay-700/25 dark:text-clay-100'
                  : 'border-sand-300 text-ink-500 hover:border-sand-400 dark:border-ink-600 dark:text-sand-300'
              "
              :aria-pressed="settings.mapType === type"
              @click="update({ mapType: type })"
            >
              {{ type }}
            </button>
          </div>
        </section>

        <section class="space-y-1">
          <h3 class="label mb-2">Behaviour</h3>
          <label
            v-for="toggle in TOGGLES"
            :key="toggle.key"
            class="flex cursor-pointer items-start gap-3 rounded-xl px-2 py-2 transition-colors hover:bg-sand-200/50 dark:hover:bg-ink-700/50"
          >
            <input
              type="checkbox"
              class="mt-0.5 h-4 w-4 shrink-0 accent-clay-400"
              :checked="settings[toggle.key]"
              @change="update({ [toggle.key]: $event.target.checked })"
            />
            <span class="min-w-0">
              <span class="block text-sm text-ink-700 dark:text-sand-100">{{ toggle.label }}</span>
              <span class="block text-[11px] text-ink-400 dark:text-sand-500">{{ toggle.hint }}</span>
            </span>
          </label>
        </section>

        <section class="space-y-2">
          <div class="flex items-baseline justify-between">
            <h3 class="label">Refresh interval</h3>
            <span class="text-xs tabular-nums text-ink-500 dark:text-sand-300">{{ settings.refreshSeconds }}s</span>
          </div>
          <input
            type="range"
            min="5"
            max="120"
            step="5"
            class="w-full accent-clay-400"
            :value="settings.refreshSeconds"
            aria-label="Refresh interval in seconds"
            @change="update({ refreshSeconds: Number($event.target.value) })"
          />
          <p class="text-[11px] text-ink-400 dark:text-sand-500">
            Only used when the live connection drops — pushes arrive as the server polls.
          </p>
        </section>

        <section class="space-y-2">
          <h3 class="label">Data</h3>
          <button type="button" class="btn-outline w-full text-xs" @click="fleet.exportCsv()">
            <AppIcon name="download" :size="14" />
            Export current view as CSV
          </button>
          <button type="button" class="btn-outline w-full text-xs text-clay-500" @click="resetAll">
            <AppIcon name="trash" :size="14" />
            Reset all personalisation
          </button>
        </section>

        <section class="space-y-1 border-t border-sand-300/70 pt-4 text-[11px] text-ink-400 dark:border-ink-600/60 dark:text-sand-500">
          <p class="flex justify-between"><span>Provider</span><span>{{ preferences.runtimeConfig.provider || '—' }}</span></p>
          <p class="flex justify-between"><span>Backend</span><span>{{ preferences.runtimeConfig.version || '—' }}</span></p>
          <p class="flex justify-between"><span>Mode</span><span>{{ preferences.runtimeConfig.demoMode ? 'Demo data' : 'Live' }}</span></p>
          <p class="flex justify-between"><span>Transport</span><span>{{ fleet.transport }}</span></p>
        </section>
      </div>
    </aside>
  </Transition>
</template>
