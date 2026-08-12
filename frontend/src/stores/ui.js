import { defineStore } from 'pinia'

const THEME_KEY = 'fleetview.theme'


export const useUiStore = defineStore('ui', {
  state: () => ({
    theme: 'system',
    sidebarOpen: false,
    settingsOpen: false,
    detailOpen: false,
    toasts: [],
    nextToastId: 1,
  }),

  getters: {
    resolvedTheme(state) {
      if (state.theme === 'system') {
        return typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches
          ? 'dark'
          : 'light'
      }
      return state.theme
    },
    isDark() {
      return this.resolvedTheme === 'dark'
    },
  },

  actions: {
    initTheme() {
      try {
        const stored = window.localStorage.getItem(THEME_KEY)
        if (stored === 'light' || stored === 'dark' || stored === 'system') {
          this.theme = stored
        }
      } catch {
       
      }
      this.applyTheme()

     
      window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', () => {
        if (this.theme === 'system') this.applyTheme()
      })
    },

    setTheme(theme) {
      this.theme = ['light', 'dark', 'system'].includes(theme) ? theme : 'system'
      try {
        window.localStorage.setItem(THEME_KEY, this.theme)
      } catch {
       
      }
      this.applyTheme()
    },

    applyTheme() {
      if (typeof document === 'undefined') return
      document.documentElement.classList.toggle('dark', this.isDark)
    },

    toggleSidebar(force) {
      this.sidebarOpen = force ?? !this.sidebarOpen
    },

    openSettings() {
      this.settingsOpen = true
    },

    closeSettings() {
      this.settingsOpen = false
    },

    openDetail() {
      this.detailOpen = true
    },

    closeDetail() {
      this.detailOpen = false
    },

    
    notify(message, { type = 'info', timeout = 4000 } = {}) {
      const id = this.nextToastId++
      this.toasts.push({ id, message, type })

      if (timeout > 0) {
        window.setTimeout(() => this.dismiss(id), type === 'error' ? Math.max(timeout, 6000) : timeout)
      }
     
      if (this.toasts.length > 4) this.toasts.splice(0, this.toasts.length - 4)
      return id
    },

    dismiss(id) {
      const index = this.toasts.findIndex((toast) => toast.id === id)
      if (index !== -1) this.toasts.splice(index, 1)
    },
  },
})
