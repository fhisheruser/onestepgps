<script setup>
import { computed } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import { formatSpeed } from '@/utils/format'
import AppIcon from './AppIcon.vue'
import StatTile from './StatTile.vue'
import ConnectionBadge from './ConnectionBadge.vue'
import Vehicle3D from './Vehicle3D.vue'

defineProps({ refreshing: { type: Boolean, default: false } })
const emit = defineEmits(['refresh'])

const fleet = useFleetStore()
const preferences = usePreferencesStore()
const ui = useUiStore()

const summary = computed(() => fleet.summary)
const avgSpeed = computed(() =>
  formatSpeed(summary.value.avgSpeed, summary.value.speedUnit, preferences.settings.speedUnit || 'mph'),
)
</script>

<template>
  <header
    class="z-20 flex flex-wrap items-center gap-3 border-b border-sand-300/80 bg-sand-100/85 px-3 py-2.5
           backdrop-blur-md dark:border-ink-700/70 dark:bg-ink-900/85 sm:px-4"
  >
    <button
      type="button"
      class="btn-ghost px-1.5 py-1.5 lg:hidden"
      :aria-label="ui.sidebarOpen ? 'Hide vehicle list' : 'Show vehicle list'"
      @click="ui.toggleSidebar()"
    >
      <AppIcon :name="ui.sidebarOpen ? 'close' : 'menu'" :size="20" />
    </button>

    <div class="flex min-w-0 items-center gap-2">
      <Vehicle3D type="truck" color="#B4643C" :size="44" :heading="-36" :shadow="false" />
      <div class="min-w-0">
        <h1 class="font-display text-lg font-semibold leading-none tracking-tight text-ink-800 dark:text-sand-50">
          FleetView
        </h1>
        <p class="mt-0.5 truncate text-[11px] text-ink-400 dark:text-sand-500">
          {{ summary.total }} vehicles tracked
        </p>
      </div>
    </div>

    <div class="ml-auto flex items-center gap-2 lg:order-last">
      <ConnectionBadge />

      <button
        type="button"
        class="btn-ghost px-1.5 py-1.5"
        :disabled="refreshing"
        title="Refresh now"
        aria-label="Refresh now"
        @click="emit('refresh')"
      >
        <AppIcon name="refresh" :size="18" :class="refreshing ? 'animate-spin' : ''" />
      </button>

      <button
        type="button"
        class="btn-ghost hidden px-1.5 py-1.5 sm:inline-flex"
        title="Export CSV"
        aria-label="Export current view as CSV"
        @click="fleet.exportCsv()"
      >
        <AppIcon name="download" :size="18" />
      </button>

      <button
        type="button"
        class="btn-ghost px-1.5 py-1.5"
        :title="ui.isDark ? 'Switch to light' : 'Switch to dark'"
        :aria-label="ui.isDark ? 'Switch to light theme' : 'Switch to dark theme'"
        @click="preferences.updateSettings({ theme: ui.isDark ? 'light' : 'dark' })"
      >
        <AppIcon :name="ui.isDark ? 'sun' : 'moon'" :size="18" />
      </button>

      <button
        type="button"
        class="btn-ghost px-1.5 py-1.5"
        title="Settings"
        aria-label="Open settings"
        @click="ui.openSettings()"
      >
        <AppIcon name="sliders" :size="18" />
      </button>
    </div>

    <div class="order-last flex w-full gap-2 overflow-x-auto pb-0.5 lg:order-none lg:w-auto lg:flex-1 lg:justify-center">
      <StatTile label="Driving" :value="summary.driving" icon="route" tone="positive" :loading="!fleet.initialised" />
      <StatTile label="Idle" :value="summary.idle" icon="clock" tone="warning" :loading="!fleet.initialised" />
      <StatTile label="Parked" :value="summary.off" icon="pin" :loading="!fleet.initialised" />
      <StatTile label="Offline" :value="summary.offline" icon="offline" :loading="!fleet.initialised" />
      <StatTile label="Avg speed" :value="avgSpeed" icon="gauge" tone="accent" :loading="!fleet.initialised" />
    </div>
  </header>
</template>
