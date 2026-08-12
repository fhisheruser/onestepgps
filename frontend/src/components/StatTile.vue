<script setup>
import AppIcon from './AppIcon.vue'

defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], required: true },
  hint: { type: String, default: '' },
  icon: { type: String, default: 'activity' },
  tone: { type: String, default: 'neutral' },
  loading: { type: Boolean, default: false },
})

const TONES = {
  neutral: 'text-ink-500 dark:text-sand-300',
  positive: 'text-sage-700 dark:text-sage-300',
  warning: 'text-amberish-700 dark:text-amberish-300',
  accent: 'text-clay-500 dark:text-clay-200',
}
</script>

<template>
  <div class="panel-flush flex items-center gap-3 rounded-2xl px-3.5 py-2.5">
    <span
      class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-sand-200/70 dark:bg-ink-700/70"
      :class="TONES[tone] || TONES.neutral"
    >
      <AppIcon :name="icon" :size="17" />
    </span>

    <div class="min-w-0">
      <p class="label leading-tight">{{ label }}</p>
      <p v-if="loading" class="skeleton mt-1 h-5 w-12" />
      <p v-else class="truncate text-lg font-semibold leading-tight text-ink-700 dark:text-sand-100">
        {{ value }}
        <span v-if="hint" class="ml-1 text-xs font-normal text-ink-400 dark:text-sand-500">{{ hint }}</span>
      </p>
    </div>
  </div>
</template>
