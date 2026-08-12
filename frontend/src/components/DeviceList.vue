<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { usePreferencesStore } from '@/stores/preferences'
import DeviceCard from './DeviceCard.vue'
import EmptyState from './EmptyState.vue'

const emit = defineEmits(['edit'])

const fleet = useFleetStore()
const preferences = usePreferencesStore()

// One shared clock for every row, rather than a timer per card.
const now = ref(Date.now())
const ticker = window.setInterval(() => {
  now.value = Date.now()
}, 5000)
onBeforeUnmount(() => window.clearInterval(ticker))

const speedUnit = computed(() => preferences.settings.speedUnit || 'mph')
const reorderable = computed(() => (fleet.filters.sortKey || preferences.settings.sortKey) === 'custom')
const showSkeletons = computed(() => fleet.loading && !fleet.initialised)

// ---- Drag to reorder -------------------------------------------------------
const dragId = ref(null)
const dragOverId = ref(null)

function onDragStart(event, device) {
  if (!reorderable.value) return
  dragId.value = device.id
  event.dataTransfer.effectAllowed = 'move'
  // Firefox requires data to be set for a drag to start at all.
  event.dataTransfer.setData('text/plain', device.id)
}

function onDragOver(event, device) {
  if (!reorderable.value || !dragId.value) return
  event.preventDefault()
  dragOverId.value = device.id
}

async function onDrop(device) {
  const sourceId = dragId.value
  dragId.value = null
  dragOverId.value = null
  if (!sourceId || sourceId === device.id) return

  const ids = fleet.devices.map((d) => d.id)
  const from = ids.indexOf(sourceId)
  const to = ids.indexOf(device.id)
  if (from === -1 || to === -1) return

  ids.splice(to, 0, ids.splice(from, 1)[0])
  await fleet.reorderDevices(ids)
}

function onDragEnd() {
  dragId.value = null
  dragOverId.value = null
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div v-if="showSkeletons" class="space-y-2 p-3" aria-busy="true" aria-label="Loading vehicles">
      <div v-for="n in 5" :key="n" class="panel-flush rounded-xl2 p-3">
        <div class="flex gap-3">
          <div class="skeleton h-12 w-12 rounded-xl" />
          <div class="flex-1 space-y-2">
            <div class="skeleton h-3.5 w-2/3" />
            <div class="skeleton h-3 w-1/3" />
            <div class="skeleton h-3 w-1/2" />
          </div>
        </div>
      </div>
    </div>

    <EmptyState
      v-else-if="fleet.isFleetEmpty"
      title="No vehicles yet"
      description="The GPS provider returned an empty fleet. Check that the API key belongs to an account with active devices."
      vehicle="truck"
    />

    <EmptyState
      v-else-if="fleet.isFilteredEmpty"
      title="No vehicles match"
      description="Nothing in the fleet fits the current search and filters."
      action-label="Clear filters"
      vehicle="car"
      @action="fleet.resetFilters()"
    />

    <ul v-else class="flex-1 space-y-2 overflow-y-auto p-3" aria-label="Vehicles">
      <li
        v-for="device in fleet.devices"
        :key="device.id"
        class="animate-fade-up transition-transform duration-200"
        :class="[
          dragOverId === device.id && dragId !== device.id ? 'translate-y-0.5 opacity-80' : '',
          dragId === device.id ? 'opacity-40' : '',
        ]"
        @dragover="onDragOver($event, device)"
        @drop.prevent="onDrop(device)"
      >
        <DeviceCard
          :device="device"
          :selected="fleet.selectedDeviceId === device.id"
          :speed-unit="speedUnit"
          :draggable="reorderable"
          :now="now"
          @dragstart="onDragStart($event, device)"
          @dragend="onDragEnd"
          @select="fleet.selectDevice($event)"
          @toggle-pin="fleet.setPinned(device.id, !device.preferences.pinned)"
          @toggle-hidden="fleet.setHidden(device.id, !device.preferences.hidden)"
          @edit="emit('edit', $event)"
        />
      </li>
    </ul>

    <p
      v-if="reorderable && fleet.devices.length > 1"
      class="border-t border-sand-300/70 px-3 py-2 text-center text-xs text-ink-400 dark:border-ink-600/60 dark:text-sand-500"
    >
      Drag rows to set your own order
    </p>
  </div>
</template>
