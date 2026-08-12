<script setup>
import { computed } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import { useUiStore } from '@/stores/ui'
import { useFleetSync } from '@/composables/useFleetSync'
import AppHeader from '@/components/AppHeader.vue'
import SearchFilterBar from '@/components/SearchFilterBar.vue'
import DeviceList from '@/components/DeviceList.vue'
import FleetMap from '@/components/FleetMap.vue'
import DeviceDetailPanel from '@/components/DeviceDetailPanel.vue'
import SettingsPanel from '@/components/SettingsPanel.vue'
import AppIcon from '@/components/AppIcon.vue'

const fleet = useFleetStore()
const ui = useUiStore()
const { refreshNow } = useFleetSync()

const banner = computed(() => {
  if (fleet.meta.stale) {
    return {
      tone: 'bg-amberish-100 text-amberish-700 dark:bg-amberish-700/25 dark:text-amberish-100',
      icon: 'alert',
      text: fleet.meta.error
        ? `Showing cached positions — ${fleet.meta.error}`
        : 'Showing cached positions — the GPS provider is not responding.',
    }
  }
  if (fleet.error) {
    return {
      tone: 'bg-clay-100 text-clay-600 dark:bg-clay-700/30 dark:text-clay-100',
      icon: 'offline',
      text: fleet.error,
    }
  }
  return null
})

function openDetailFor(device) {
  fleet.selectDevice(device.id)
}
</script>

<template>
  <div class="flex h-full flex-col overflow-hidden">
    <AppHeader :refreshing="fleet.loading" @refresh="refreshNow" />

    <p
      v-if="banner"
      class="flex items-center justify-center gap-2 px-4 py-1.5 text-xs font-medium"
      :class="banner.tone"
      role="status"
    >
      <AppIcon :name="banner.icon" :size="14" />
      {{ banner.text }}
    </p>

    <div class="relative flex min-h-0 flex-1">
     
      <aside
        class="absolute inset-y-0 left-0 z-20 flex w-[min(22rem,88vw)] flex-col border-r border-sand-300/80
               bg-sand-100/95 backdrop-blur-md transition-transform duration-300 ease-smooth
               dark:border-ink-700/70 dark:bg-ink-900/95
               lg:static lg:w-96 lg:translate-x-0 lg:bg-sand-100/60 lg:dark:bg-ink-900/50"
        :class="ui.sidebarOpen ? 'translate-x-0 shadow-lifted' : '-translate-x-full'"
        aria-label="Vehicle list"
      >
        <div class="border-b border-sand-300/70 p-3 dark:border-ink-700/60">
          <SearchFilterBar />
        </div>
        <DeviceList class="min-h-0 flex-1" @edit="openDetailFor" />
      </aside>

    
      <button
        v-if="ui.sidebarOpen"
        type="button"
        class="absolute inset-0 z-10 bg-ink-900/25 backdrop-blur-[1px] lg:hidden"
        aria-label="Close vehicle list"
        @click="ui.toggleSidebar(false)"
      />

      <main class="relative min-w-0 flex-1">
        <FleetMap />
        <DeviceDetailPanel />
        <SettingsPanel />
      </main>
    </div>
  </div>
</template>
