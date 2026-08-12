<script setup>
import { computed } from 'vue'
import Vehicle3D from './Vehicle3D.vue'
import AppIcon from './AppIcon.vue'
import { driveStatusMeta, formatCoords, formatSpeed, headingToCompass, timeAgo, truncate } from '@/utils/format'

const props = defineProps({
  device: { type: Object, required: true },
  selected: { type: Boolean, default: false },
  speedUnit: { type: String, default: 'mph' },
  draggable: { type: Boolean, default: false },
  now: { type: Number, default: () => Date.now() },
})

const emit = defineEmits(['select', 'toggle-pin', 'toggle-hidden', 'edit'])

const prefs = computed(() => props.device.preferences || {})
const status = computed(() => driveStatusMeta(props.device.driveStatus))
const position = computed(() => props.device.position || {})

const STATUS_CLASSES = {
  driving: 'bg-sage-100 text-sage-700 dark:bg-sage-700/35 dark:text-sage-100',
  idle: 'bg-amberish-100 text-amberish-700 dark:bg-amberish-700/30 dark:text-amberish-100',
  off: 'bg-sand-200 text-ink-500 dark:bg-ink-700 dark:text-sand-300',
  unknown: 'bg-sand-200 text-ink-400 dark:bg-ink-700 dark:text-sand-400',
}

const statusClass = computed(() => STATUS_CLASSES[props.device.driveStatus] || STATUS_CLASSES.unknown)

const vehicleType = computed(() => {
  const icon = prefs.value.markerIcon

  return !icon || icon === 'custom' || icon === 'pin' ? 'van' : icon
})

const updatedLabel = computed(() => timeAgo(position.value.recordedAt, props.now))


const isCold = computed(() => {
  const at = Date.parse(position.value.recordedAt || '')
  return Number.isNaN(at) ? true : props.now - at > 15 * 60 * 1000
})
</script>

<template>
  <article
    class="group relative w-full rounded-xl2 border p-3 text-left transition-all duration-200 ease-smooth"
    :class="[
      selected
        ? 'border-clay-300 bg-clay-50 shadow-soft dark:border-clay-400/60 dark:bg-clay-700/20'
        : 'border-sand-300/70 bg-sand-50/70 hover:border-sand-400 hover:bg-sand-50 hover:shadow-soft dark:border-ink-600/60 dark:bg-ink-800/50 dark:hover:border-ink-500 dark:hover:bg-ink-800',
      prefs.hidden ? 'opacity-60' : '',
    ]"
    :draggable="draggable"
    :aria-current="selected ? 'true' : undefined"
  >
    
    <button
      type="button"
      class="absolute inset-0 z-0 rounded-xl2"
      :aria-label="`Select ${device.name}`"
      @click="emit('select', device.id)"
    />

    <div class="pointer-events-none relative z-10 flex gap-3">
      <div class="relative shrink-0">
        <img
          v-if="prefs.markerIcon === 'custom' && prefs.customIconUrl"
          :src="prefs.customIconUrl"
          alt=""
          class="h-12 w-12 rounded-xl object-contain"
        />
        <Vehicle3D
          v-else
          :type="vehicleType"
          :color="prefs.markerColor || '#B4643C'"
          :size="62"
          :heading="-34"
          :shadow="false"
        />
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0">
            <h3 class="flex items-center gap-1.5 truncate text-sm font-semibold text-ink-700 dark:text-sand-100">
              <AppIcon v-if="prefs.pinned" name="pin" :size="13" class="text-clay-400" />
              {{ device.name }}
            </h3>
            <p class="truncate text-xs text-ink-400 dark:text-sand-500">
              <span v-if="device.renamed">{{ device.providerName }} · </span>
              <span v-else-if="device.make || device.model">{{ device.make }} {{ device.model }}</span>
              <span v-else>{{ device.id }}</span>
            </p>
          </div>

          <span class="chip shrink-0" :class="statusClass">
            <span class="h-1.5 w-1.5 rounded-full" :style="{ background: status.dot }" />
            {{ status.label }}
          </span>
        </div>

        <dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
          <div class="flex items-center gap-1.5 text-ink-500 dark:text-sand-400">
            <AppIcon name="gauge" :size="13" class="text-ink-300 dark:text-sand-500" />
            <dt class="sr-only">Speed</dt>
            <dd class="font-medium tabular-nums">
              {{ formatSpeed(position.speed, position.speedUnit, speedUnit) }}
            </dd>
          </div>

          <div class="flex items-center gap-1.5 text-ink-500 dark:text-sand-400">
            <AppIcon name="clock" :size="13" class="text-ink-300 dark:text-sand-500" />
            <dt class="sr-only">Last update</dt>
            <dd :class="isCold ? 'text-ink-300 dark:text-sand-500' : ''">{{ updatedLabel }}</dd>
          </div>

          <div class="col-span-2 flex items-center gap-1.5 text-ink-400 dark:text-sand-500">
            <AppIcon name="pin" :size="13" class="text-ink-300 dark:text-sand-500" />
            <dt class="sr-only">Coordinates</dt>
            <dd class="truncate font-mono text-[11px] tabular-nums">
              <template v-if="position.valid">
                {{ formatCoords(position.lat, position.lng) }}
                <span v-if="device.driveStatus === 'driving'" class="ml-1 text-ink-300 dark:text-sand-500">
                  · {{ headingToCompass(position.heading) }}
                </span>
              </template>
              <template v-else>No GPS fix</template>
            </dd>
          </div>
        </dl>

        <p v-if="prefs.notes" class="mt-1.5 truncate text-xs italic text-ink-400 dark:text-sand-500">
          {{ truncate(prefs.notes, 64) }}
        </p>

        <div v-if="!device.online || prefs.hidden" class="mt-1.5 flex flex-wrap gap-1.5">
          <span v-if="!device.online" class="chip bg-sand-200 text-[11px] text-ink-400 dark:bg-ink-700 dark:text-sand-400">
            <AppIcon name="offline" :size="11" /> Offline
          </span>
          <span v-if="prefs.hidden" class="chip bg-sand-200 text-[11px] text-ink-400 dark:bg-ink-700 dark:text-sand-400">
            <AppIcon name="eye-off" :size="11" /> Hidden
          </span>
        </div>
      </div>
    </div>


    <div
      class="relative z-10 mt-2 flex items-center justify-end gap-0.5 opacity-0 transition-opacity duration-200
             focus-within:opacity-100 group-hover:opacity-100 md:-mb-1"
      :class="selected ? 'opacity-100' : ''"
    >
      <button
        v-if="draggable"
        type="button"
        class="btn-ghost cursor-grab px-1.5 py-1 active:cursor-grabbing"
        aria-label="Drag to reorder"
        title="Drag to reorder"
      >
        <AppIcon name="grip" :size="15" />
      </button>
      <button
        type="button"
        class="btn-ghost px-1.5 py-1"
        :aria-label="prefs.pinned ? `Unpin ${device.name}` : `Pin ${device.name}`"
        :title="prefs.pinned ? 'Unpin' : 'Pin to top'"
        @click.stop="emit('toggle-pin', device)"
      >
        <AppIcon name="pin" :size="15" :class="prefs.pinned ? 'text-clay-400' : ''" />
      </button>
      <button
        type="button"
        class="btn-ghost px-1.5 py-1"
        :aria-label="prefs.hidden ? `Show ${device.name}` : `Hide ${device.name}`"
        :title="prefs.hidden ? 'Show on dashboard' : 'Hide from dashboard'"
        @click.stop="emit('toggle-hidden', device)"
      >
        <AppIcon :name="prefs.hidden ? 'eye' : 'eye-off'" :size="15" />
      </button>
      <button
        type="button"
        class="btn-ghost px-1.5 py-1"
        :aria-label="`Customise ${device.name}`"
        title="Rename, recolour, upload icon"
        @click.stop="emit('edit', device)"
      >
        <AppIcon name="edit" :size="15" />
      </button>
    </div>
  </article>
</template>
