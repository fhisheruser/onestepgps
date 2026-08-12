import { defineStore } from 'pinia'

const THEME_KEY = 'fleetview.theme'

/**
 * UI-only state: what is open, what is selected, which theme is applied and
 * any transient toasts. Nothing here is persisted server-side except the
 * theme, which the preferences store mirrors so it follows the user across
 * devices; localStorage is only used to avoid a flash before the API answers.
 */
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
        /* storage unavailable: keep the default */
      }
      this.applyTheme()

      // Follow the OS while the user has not made an explicit choice.
      window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', () => {
        if (this.theme === 'system') this.applyTheme()
      })
    },

    setTheme(theme) {
      this.theme = ['light', 'dark', 'system'].includes(theme) ? theme : 'system'
      try {
        window.localStorage.setItem(THEME_KEY, this.theme)
      } catch {
        /* ignore */
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

    /** Show a transient message. Errors stay longer and never auto-stack up. */
    notify(message, { type = 'info', timeout = 4000 } = {}) {
      const id = this.nextToastId++
      this.toasts.push({ id, message, type })

      if (timeout > 0) {
        window.setTimeout(() => this.dismiss(id), type === 'error' ? Math.max(timeout, 6000) : timeout)
      }
      // Keep the stack short so it never covers the map.
      if (this.toasts.length > 4) this.toasts.splice(0, this.toasts.length - 4)
      return id
    },

    dismiss(id) {
      const index = this.toasts.findIndex((toast) => toast.id === id)
      if (index !== -1) this.toasts.splice(index, 1)
    },
  },
})
