<script setup>
import { useUiStore } from '@/stores/ui'
import AppIcon from './AppIcon.vue'

const ui = useUiStore()

const TONES = {
  success: { icon: 'check', classes: 'border-sage-300 bg-sage-100 text-sage-700 dark:bg-sage-700/40 dark:text-sage-100' },
  error: { icon: 'alert', classes: 'border-clay-300 bg-clay-100 text-clay-600 dark:bg-clay-700/50 dark:text-clay-100' },
  info: { icon: 'info', classes: 'border-sand-300 bg-sand-50 text-ink-600 dark:border-ink-600 dark:bg-ink-800 dark:text-sand-200' },
}

const toneFor = (type) => TONES[type] || TONES.info
</script>

<template>
  <div
    class="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex flex-col items-center gap-2 px-4 sm:bottom-6"
    role="status"
    aria-live="polite"
  >
    <TransitionGroup
      enter-active-class="transition duration-300 ease-smooth"
      enter-from-class="translate-y-3 opacity-0"
      leave-active-class="transition duration-200 ease-smooth"
      leave-to-class="translate-y-2 opacity-0"
    >
      <div
        v-for="toast in ui.toasts"
        :key="toast.id"
        class="pointer-events-auto flex w-full max-w-md items-start gap-2.5 rounded-xl2 border px-3.5 py-2.5 text-sm shadow-lifted"
        :class="toneFor(toast.type).classes"
      >
        <AppIcon :name="toneFor(toast.type).icon" :size="17" class="mt-0.5" />
        <p class="flex-1 leading-snug">{{ toast.message }}</p>
        <button
          type="button"
          class="rounded-lg p-0.5 opacity-60 transition hover:opacity-100"
          aria-label="Dismiss notification"
          @click="ui.dismiss(toast.id)"
        >
          <AppIcon name="close" :size="15" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
