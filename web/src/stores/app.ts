import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppStore = defineStore('app', () => {
  // 状态
  const sidebarCollapsed = ref(false)
  const currentTheme = ref<'light' | 'dark'>('light')
  const currentUser = ref<any>(null)
  const isLoading = ref(false)

  // 操作
  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setTheme(theme: 'light' | 'dark') {
    currentTheme.value = theme
    document.documentElement.setAttribute('data-theme', theme)
  }

  function setCurrentUser(user: any) {
    currentUser.value = user
  }

  function setLoading(loading: boolean) {
    isLoading.value = loading
  }

  return {
    sidebarCollapsed,
    currentTheme,
    currentUser,
    isLoading,
    toggleSidebar,
    setTheme,
    setCurrentUser,
    setLoading
  }
})
