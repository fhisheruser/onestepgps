<script setup>
import { onMounted } from 'vue'
import { useUiStore } from '@/stores/ui'
import DashboardView from '@/views/DashboardView.vue'
import ToastStack from '@/components/ToastStack.vue'

const ui = useUiStore()
onMounted(() => ui.initTheme())
</script>

<template>
  <DashboardView />
  <ToastStack />
</template>
