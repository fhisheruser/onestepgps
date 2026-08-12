<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { timeAgo } from '@/utils/format'
import AppIcon from './AppIcon.vue'



const fleet = useFleetStore()


const now = ref(Date.now())
const timer = window.setInterval(() => {
  now.value = Date.now()
}, 1000)
onBeforeUnmount(() => window.clearInterval(timer))

const state = computed(() => {
  if (fleet.meta.stale) {
    return {
      icon: 'alert',
      label: 'Cached data',
      detail: fleet.meta.error || 'The GPS provider is unreachable — showing the last known positions.',
      classes: 'bg-amberish-100 text-amberish-700 dark:bg-amberish-700/25 dark:text-amberish-300',
      pulse: false,
    }
  }
  if (fleet.transport === 'realtime') {
    return {
      icon: 'activity',
      label: 'Live',
      detail: 'Streaming over WebSocket',
      classes: 'bg-sage-100 text-sage-700 dark:bg-sage-700/30 dark:text-sage-300',
      pulse: true,
    }
  }
  if (fleet.error) {
    return {
      icon: 'offline',
      label: 'Offline',
      detail: fleet.error,
      classes: 'bg-clay-100 text-clay-600 dark:bg-clay-700/30 dark:text-clay-200',
      pulse: false,
    }
  }
  return {
    icon: 'refresh',
    label: 'Polling',
    detail: 'Refreshing on an interval',
    classes: 'bg-sand-200 text-ink-500 dark:bg-ink-700 dark:text-sand-300',
    pulse: false,
  }
})

const updatedLabel = computed(() => timeAgo(fleet.meta.fetchedAt, now.value))
</script>

<template>
  <div class="flex items-center gap-2" :title="state.detail">
    <span class="chip relative" :class="state.classes">
      <span v-if="state.pulse" class="relative flex h-2 w-2">
        <span class="absolute inline-flex h-full w-full animate-pulse-ring rounded-full bg-current" />
        <span class="relative inline-flex h-2 w-2 rounded-full bg-current" />
      </span>
      <AppIcon v-else :name="state.icon" :size="13" />
      {{ state.label }}
    </span>

    <span class="hidden text-xs text-ink-400 dark:text-sand-500 sm:inline">
      updated {{ updatedLabel }}
    </span>
  </div>
</template>
