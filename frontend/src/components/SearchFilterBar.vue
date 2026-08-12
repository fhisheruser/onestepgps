<script setup>
import { computed } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import AppIcon from './AppIcon.vue'

const fleet = useFleetStore()
const preferences = usePreferencesStore()

const STATUS_FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'driving', label: 'Driving' },
  { value: 'idle', label: 'Idle' },
  { value: 'off', label: 'Parked' },
  { value: 'offline', label: 'Offline' },
]

const SORT_OPTIONS = [
  { value: 'name', label: 'Name' },
  { value: 'status', label: 'Status' },
  { value: 'speed', label: 'Speed' },
  { value: 'updated', label: 'Last update' },
  { value: 'custom', label: 'Custom order' },
]

const search = computed({
  get: () => fleet.filters.search,
  set: (value) => fleet.setFilters({ search: value }),
})


const activeSort = computed({
  get: () => fleet.filters.sortKey || preferences.settings.sortKey || 'name',
  set: (value) => fleet.setFilters({ sortKey: value }),
})

const direction = computed(() => fleet.filters.sortDirection || preferences.settings.sortDirection || 'asc')

function toggleDirection() {
  fleet.setFilters({ sortDirection: direction.value === 'asc' ? 'desc' : 'asc' })
}

const counts = computed(() => ({
  all: fleet.summary.visible,
  driving: fleet.summary.driving,
  idle: fleet.summary.idle,
  off: fleet.summary.off,
  offline: fleet.summary.offline,
}))
</script>

<template>
  <div class="space-y-2.5">
    <div class="relative">
      <AppIcon
        name="search"
        :size="16"
        class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-300 dark:text-sand-500"
      />
      <input
        v-model="search"
        type="search"
        class="field pl-9 pr-9"
        placeholder="Search name, make, group…"
        aria-label="Search vehicles"
      />
      <button
        v-if="search"
        type="button"
        class="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1 text-ink-300 transition hover:text-ink-500 dark:text-sand-500 dark:hover:text-sand-200"
        aria-label="Clear search"
        @click="search = ''"
      >
        <AppIcon name="close" :size="15" />
      </button>
    </div>

    <div class="-mx-0.5 flex gap-1 overflow-x-auto pb-1" role="group" aria-label="Filter by status">
      <button
        v-for="filter in STATUS_FILTERS"
        :key="filter.value"
        type="button"
        class="chip shrink-0 border transition-colors duration-200"
        :class="
          fleet.filters.status === filter.value
            ? 'border-clay-300 bg-clay-100 text-clay-600 dark:border-clay-400/50 dark:bg-clay-700/40 dark:text-clay-100'
            : 'border-transparent bg-sand-200/70 text-ink-500 hover:bg-sand-300/70 dark:bg-ink-700/70 dark:text-sand-300 dark:hover:bg-ink-600/70'
        "
        :aria-pressed="fleet.filters.status === filter.value"
        @click="fleet.setFilters({ status: filter.value })"
      >
        {{ filter.label }}
        <span class="tabular-nums opacity-60">{{ counts[filter.value] ?? 0 }}</span>
      </button>
    </div>

    <div class="flex items-center gap-2">
      <label class="sr-only" for="sort-select">Sort vehicles by</label>
      <div class="relative flex-1">
        <select id="sort-select" v-model="activeSort" class="field appearance-none pr-8 text-xs">
          <option v-for="option in SORT_OPTIONS" :key="option.value" :value="option.value">
            Sort: {{ option.label }}
          </option>
        </select>
        <AppIcon
          name="chevron"
          :size="14"
          class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-300 dark:text-sand-500"
        />
      </div>

      <button
        type="button"
        class="btn-outline px-2.5 py-2"
        :title="direction === 'asc' ? 'Ascending' : 'Descending'"
        :aria-label="`Sort direction: ${direction === 'asc' ? 'ascending' : 'descending'}`"
        @click="toggleDirection"
      >
        <AppIcon name="chevron" :size="15" :class="direction === 'asc' ? 'rotate-180' : ''" class="transition-transform" />
      </button>

      <button
        type="button"
        class="btn-outline px-2.5 py-2"
        :class="fleet.filters.onlyPinned ? 'border-clay-300 text-clay-500' : ''"
        :aria-pressed="fleet.filters.onlyPinned"
        title="Show pinned only"
        aria-label="Show pinned vehicles only"
        @click="fleet.setFilters({ onlyPinned: !fleet.filters.onlyPinned })"
      >
        <AppIcon name="pin" :size="15" />
      </button>

      <button
        type="button"
        class="btn-outline px-2.5 py-2"
        :class="fleet.filters.includeHidden ? 'border-clay-300 text-clay-500' : ''"
        :aria-pressed="fleet.filters.includeHidden"
        :title="fleet.filters.includeHidden ? 'Hiding hidden vehicles' : 'Show hidden vehicles'"
        aria-label="Toggle hidden vehicles"
        @click="fleet.setFilters({ includeHidden: !fleet.filters.includeHidden })"
      >
        <AppIcon :name="fleet.filters.includeHidden ? 'eye' : 'eye-off'" :size="15" />
      </button>
    </div>
  </div>
</template>
