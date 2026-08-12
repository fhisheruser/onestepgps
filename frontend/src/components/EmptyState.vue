<script setup>
import Vehicle3D from './Vehicle3D.vue'

defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' },
  actionLabel: { type: String, default: '' },
  vehicle: { type: String, default: 'van' },
  compact: { type: Boolean, default: false },
})

defineEmits(['action'])
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 px-6 text-center" :class="compact ? 'py-8' : 'py-14'">
    <Vehicle3D :type="vehicle" :size="compact ? 92 : 130" color="#D8C7AC" :heading="-28" spin />

    <div class="max-w-xs space-y-1.5">
      <h3 class="text-balance text-base font-semibold text-ink-700 dark:text-sand-100">{{ title }}</h3>
      <p v-if="description" class="text-balance text-sm leading-relaxed text-ink-400 dark:text-sand-500">
        {{ description }}
      </p>
    </div>

    <button v-if="actionLabel" type="button" class="btn-outline mt-1" @click="$emit('action')">
      {{ actionLabel }}
    </button>
  </div>
</template>
