<script setup>
import { computed, ref } from 'vue'
import { usePreferencesStore } from '@/stores/preferences'
import { useUiStore } from '@/stores/ui'
import AppIcon from './AppIcon.vue'
import Vehicle3D from './Vehicle3D.vue'

const props = defineProps({
  markerIcon: { type: String, default: 'car' },
  markerColor: { type: String, default: '#B4643C' },
  customIconUrl: { type: String, default: '' },
})

const emit = defineEmits(['update:markerIcon', 'update:markerColor', 'upload', 'remove-icon'])

const preferences = usePreferencesStore()
const ui = useUiStore()
const fileInput = ref(null)
const uploading = ref(false)

const ICONS = [
  { value: 'car', label: 'Car' },
  { value: 'van', label: 'Van' },
  { value: 'truck', label: 'Truck' },
  { value: 'pickup', label: 'Pickup' },
  { value: 'bus', label: 'Bus' },
  { value: 'pin', label: 'Pin' },
]

// A restrained palette: each swatch stays legible against both the light beige
// map and the dark one, which random user-picked hues do not guarantee.
const SWATCHES = [
  '#B4643C',
  '#9A5231',
  '#C9A227',
  '#6B7F6B',
  '#4C8C4A',
  '#3E7C8C',
  '#4A5C93',
  '#8C4A78',
  '#A8443A',
  '#5F4E39',
  '#877053',
  '#2E2823',
]

const acceptTypes = 'image/png,image/jpeg,image/webp,image/gif'
const maxKb = computed(() => preferences.maxIconKb)

function pickFile() {
  fileInput.value?.click()
}

async function onFileChange(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return

  if (file.size > preferences.runtimeConfig.maxIconBytes) {
    ui.notify(`That image is larger than ${maxKb.value} KB.`, { type: 'error' })
    return
  }

  uploading.value = true
  try {
    emit('upload', file)
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div>
      <p class="label mb-2">Marker colour</p>
      <div class="flex flex-wrap gap-1.5">
        <button
          v-for="swatch in SWATCHES"
          :key="swatch"
          type="button"
          class="h-7 w-7 rounded-lg border-2 transition-transform duration-150 hover:scale-110"
          :class="
            markerColor?.toLowerCase() === swatch.toLowerCase()
              ? 'border-ink-700 dark:border-sand-100'
              : 'border-transparent'
          "
          :style="{ background: swatch }"
          :aria-label="`Use colour ${swatch}`"
          :aria-pressed="markerColor?.toLowerCase() === swatch.toLowerCase()"
          @click="emit('update:markerColor', swatch)"
        />

        <label
          class="relative grid h-7 w-7 cursor-pointer place-items-center rounded-lg border border-dashed border-sand-400 text-ink-400 transition hover:border-clay-300 hover:text-clay-500 dark:border-ink-500 dark:text-sand-400"
          title="Pick any colour"
        >
          <AppIcon name="palette" :size="14" />
          <input
            type="color"
            class="absolute inset-0 cursor-pointer opacity-0"
            :value="markerColor"
            aria-label="Custom marker colour"
            @input="emit('update:markerColor', $event.target.value)"
          />
        </label>
      </div>
    </div>

    <div>
      <p class="label mb-2">Marker shape</p>
      <div class="grid grid-cols-3 gap-1.5">
        <button
          v-for="icon in ICONS"
          :key="icon.value"
          type="button"
          class="flex flex-col items-center gap-1 rounded-xl border px-2 py-1.5 text-[11px] font-medium transition-colors duration-200"
          :class="
            markerIcon === icon.value
              ? 'border-clay-300 bg-clay-50 text-clay-600 dark:border-clay-400/60 dark:bg-clay-700/25 dark:text-clay-100'
              : 'border-sand-300 text-ink-500 hover:border-sand-400 dark:border-ink-600 dark:text-sand-300'
          "
          :aria-pressed="markerIcon === icon.value"
          @click="emit('update:markerIcon', icon.value)"
        >
          <Vehicle3D :type="icon.value" :color="markerColor" :size="46" :heading="-30" :shadow="false" />
          {{ icon.label }}
        </button>
      </div>
    </div>

    <div>
      <p class="label mb-2">Custom image</p>
      <div class="flex items-center gap-2">
        <div
          v-if="customIconUrl"
          class="grid h-12 w-12 shrink-0 place-items-center rounded-xl border border-sand-300 bg-sand-100 dark:border-ink-600 dark:bg-ink-700"
        >
          <img :src="customIconUrl" alt="Current custom marker" class="max-h-10 max-w-10 object-contain" />
        </div>

        <div class="flex flex-1 flex-wrap gap-1.5">
          <button type="button" class="btn-outline text-xs" :disabled="uploading" @click="pickFile">
            <AppIcon name="upload" :size="14" />
            {{ customIconUrl ? 'Replace' : 'Upload' }}
          </button>
          <button
            v-if="customIconUrl"
            type="button"
            class="btn-ghost text-xs text-clay-500"
            @click="emit('remove-icon')"
          >
            <AppIcon name="trash" :size="14" />
            Remove
          </button>
        </div>
      </div>

      <p class="mt-1.5 text-[11px] leading-relaxed text-ink-400 dark:text-sand-500">
        PNG, JPEG, WebP or GIF up to {{ maxKb }} KB. Uploading switches this vehicle to the custom marker.
      </p>

      <input ref="fileInput" type="file" class="hidden" :accept="acceptTypes" @change="onFileChange" />
    </div>
  </div>
</template>
