<template>
  <div class="user-menu" :class="{ 'user-menu--collapsed': uiStore.sidebarCollapsed }" ref="menuRef">
    <div class="user-button" data-guide="user-menu" @click="toggleMenu">
      <div class="user-avatar">
        <img v-if="userAvatar" :src="userAvatar" :alt="$t('common.avatar')" />
        <span v-else class="avatar-placeholder">{{ userInitial }}</span>
      </div>
      <template v-if="!uiStore.sidebarCollapsed">
        <div class="user-info">
          <div class="user-name">{{ userName }}</div>
          <div class="user-space">{{ activeWorkspaceName }}</div>
        </div>
        <t-icon :name="menuVisible ? 'chevron-up' : 'chevron-down'" class="dropdown-icon" />
      </template>
    </div>

    <Transition name="dropdown">
      <div v-if="menuVisible" class="user-dropdown" @click.stop>
        <div v-if="userName" class="dropdown-user-header">
          <div class="dropdown-user-avatar">
            <img v-if="userAvatar" :src="userAvatar" :alt="$t('common.avatar')" />
            <span v-else class="dropdown-user-avatar-placeholder">{{ userInitial }}</span>
          </div>
          <div class="dropdown-user-meta">
            <div class="dropdown-user-name-row">
              <span class="dropdown-user-name">{{ userName }}</span>
              <t-tooltip :content="$t('newUserGuide.reopen')" placement="top">
                <button
                  type="button"
                  class="dropdown-guide-btn"
                  :aria-label="$t('newUserGuide.reopen')"
                  @click.stop="reopenGuide"
                >
                  <t-icon name="help-circle" size="14px" />
                </button>
              </t-tooltip>
            </div>
            <div class="dropdown-user-subtitle">{{ activeWorkspaceName }}</div>
          </div>
        </div>

        <div class="menu-divider"></div>
        <div v-if="canSeeQuickNav('models')" class="menu-item" @click="handleQuickNav('models')">
          <t-icon name="control-platform" class="menu-icon" />
          <span>{{ $t('settings.modelManagement') }}</span>
        </div>
        <div v-if="canSeeQuickNav('websearch')" class="menu-item" @click="handleQuickNav('websearch')">
          <svg width="16" height="16" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg"
            class="menu-icon svg-icon">
            <circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.2" fill="none" />
            <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" stroke-width="1.2" fill="none" />
            <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" stroke-width="1.2" fill="none" />
            <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" stroke-width="1.2"
              stroke-linecap="round" />
            <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" stroke-width="1.2"
              stroke-linecap="round" />
          </svg>
          <span>{{ $t('settings.webSearchConfig') }}</span>
        </div>
        <div v-if="canSeeQuickNav('mcp')" class="menu-item" @click="handleQuickNav('mcp')">
          <t-icon name="tools" class="menu-icon" />
          <span>{{ $t('settings.mcpService') }}</span>
        </div>
        <div class="menu-divider"></div>
        <div class="menu-item" @click="handleSettings">
          <t-icon name="setting" class="menu-icon" />
          <span>{{ $t('general.allSettings') }}</span>
        </div>
        <div v-if="authStore.isSystemAdmin" class="menu-item" @click="handleSystemAdmin">
          <t-icon name="server" class="menu-icon" />
          <span>{{ $t('settings.system') }}</span>
        </div>
        <div class="menu-divider"></div>
        <div class="menu-item" :title="$t('common.githubStarTip')" @click="openGithub">
          <t-icon name="logo-github" class="menu-icon" />
          <span class="menu-text-with-icon">
            <span>{{ $t('common.github') }}</span>
            <t-icon name="star-filled" class="menu-github-star-icon" size="16px" aria-hidden="true" />
            <svg class="menu-external-icon" viewBox="0 0 16 16" aria-hidden="true">
              <path fill="currentColor"
                d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667a2.667 2.667 0 0 1-2.667-2.666V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334A1.333 1.333 0 0 0 4.667 13.333h6a1.333 1.333 0 0 0 1.333-1.333v-4A.667.667 0 0 1 12.667 8Zm2.666-6.667v4a.667.667 0 0 1-1.333 0V3.276l-5.195 5.195a.667.667 0 0 1-.943-.943l5.195-5.195h-2.057a.667.667 0 0 1 0-1.333h4a.667.667 0 0 1 .666.666Z" />
            </svg>
          </span>
        </div>
        <template v-if="!authStore.isLiteMode">
          <div class="menu-divider"></div>
          <div class="menu-item danger" @click="handleLogout">
            <t-icon name="logout" class="menu-icon" />
            <span>{{ $t('auth.logout') }}</span>
          </div>
        </template>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { getCurrentUser, logout as logoutApi, userInfoFromApi } from '@/api/auth'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { openNewUserGuide } from '@/config/contextualGuides'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const router = useRouter()
const uiStore = useUIStore()
const authStore = useAuthStore()

const QUICKNAV_MIN_ROLE: Record<string, 'viewer' | 'contributor' | 'admin' | 'owner'> = {
  models: 'viewer',
  websearch: 'admin',
  mcp: 'admin',
}

const canSeeQuickNav = (key: string): boolean => {
  if (authStore.canAccessAllTenants) return true
  return authStore.hasRole(QUICKNAV_MIN_ROLE[key] ?? 'viewer')
}

const menuRef = ref<HTMLElement>()
const menuVisible = ref(false)

const userInfo = ref({
  username: t('common.defaultUser'),
  email: 'user@example.com',
  avatar: '',
})

const userName = computed(() => userInfo.value.username)
const userAvatar = computed(() => userInfo.value.avatar)
const userInitial = computed(() => userName.value.charAt(0).toUpperCase())
const activeWorkspaceName = computed(() => {
  return authStore.currentTenantName || authStore.tenant?.name || `${userName.value} 的空间`
})

const toggleMenu = () => {
  menuVisible.value = !menuVisible.value
}

const handleQuickNav = (section: string) => {
  menuVisible.value = false
  uiStore.openSettings()
  router.push('/platform/settings')

  setTimeout(() => {
    const event = new CustomEvent('settings-nav', { detail: { section } })
    window.dispatchEvent(event)
  }, 100)
}

const handleSettings = () => {
  menuVisible.value = false
  uiStore.openSettings()
  router.push('/platform/settings')
}

const handleSystemAdmin = () => {
  menuVisible.value = false
  uiStore.openSettings('system-global')
  router.push({ path: '/platform/settings', query: { section: 'system-global' } })
}

const reopenGuide = () => {
  menuVisible.value = false
  openNewUserGuide()
}

const openGithub = () => {
  menuVisible.value = false
  window.open('https://github.com/LuckWzx/DocMind', '_blank')
}

const handleLogout = async () => {
  menuVisible.value = false

  try {
    await logoutApi()
  } catch (error) {
    console.error('注销API调用失败:', error)
  }

  authStore.logout()
  MessagePlugin.success(t('auth.logout'))
  router.push('/login')
}

const loadUserInfo = async () => {
  try {
    const response = await getCurrentUser()
    if (response.success && response.data?.user) {
      const user = response.data.user
      userInfo.value = {
        username: user.username || t('common.info'),
        email: user.email || 'user@example.com',
        avatar: user.avatar || '',
      }
      authStore.setUser(userInfoFromApi(user))
      if (response.data.tenant) {
        authStore.setTenant({
          id: String(response.data.tenant.id),
          name: response.data.tenant.name,
          owner_id: user.id,
          created_at: response.data.tenant.created_at,
          updated_at: response.data.tenant.updated_at,
        })
      }
      if (Array.isArray(response.data.memberships)) {
        authStore.setMemberships(response.data.memberships)
      }
    }
  } catch (error) {
    console.error('Failed to load user info:', error)
  }
}

const handleClickOutside = (e: MouseEvent) => {
  const target = e.target as Node
  if (menuRef.value && menuRef.value.contains(target)) return
  menuVisible.value = false
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  loadUserInfo()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style lang="less" scoped>
.user-menu {
  position: relative;
  width: 100%;

  &--collapsed {
    .user-button {
      justify-content: center;
      padding: 6px 3px;
      gap: 0;
    }

    .user-dropdown {
      left: calc(100% + 8px);
      bottom: 0;
      right: auto;
      min-width: 260px;
    }
  }
}

.user-button {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 6px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  background: transparent;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &:active {
    transform: scale(0.98);
  }
}

.user-avatar,
.dropdown-user-avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  overflow: hidden;
  flex-shrink: 0;
  background: linear-gradient(135deg, var(--td-brand-color) 0%, var(--td-brand-color-active) 100%);
  display: flex;
  align-items: center;
  justify-content: center;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.avatar-placeholder,
.dropdown-user-avatar-placeholder {
  color: var(--td-text-color-anti);
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
}

.user-info {
  flex: 1;
  min-width: 0;
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-space {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dropdown-icon {
  font-size: 16px;
  color: var(--td-text-color-secondary);
  flex-shrink: 0;
}

.user-dropdown {
  position: absolute;
  bottom: 100%;
  left: -4px;
  right: -5px;
  margin-bottom: 6px;
  background: var(--td-bg-color-container);
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.12);
  border: 1px solid var(--td-component-stroke);
  overflow: hidden;
  z-index: 1000;
}

.dropdown-user-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 12px;
  min-width: 0;
}

.dropdown-user-avatar {
  margin-left: -4px;
}

.dropdown-user-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.dropdown-user-name-row {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
}

.dropdown-user-name {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.35;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dropdown-user-subtitle {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dropdown-guide-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  margin: 0;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-secondary);
  }
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
  color: var(--td-text-color-primary);

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &.danger {
    color: var(--td-error-color);

    &:hover {
      background: var(--td-error-color-light);
    }

    .menu-icon {
      color: var(--td-error-color);
    }
  }

  .menu-icon {
    font-size: 16px;
    color: var(--td-text-color-secondary);

    &.svg-icon {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
    }
  }

  .menu-text-with-icon {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 6px;
    color: inherit;
    min-width: 0;

    > span:first-of-type {
      display: inline-flex;
      align-items: center;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .menu-github-star-icon {
    flex-shrink: 0;
    color: var(--td-warning-color);
  }

  .menu-external-icon {
    width: 16px;
    height: 16px;
    color: var(--td-text-color-disabled);
    flex-shrink: 0;
    transition: color 0.2s ease;
    pointer-events: none;
  }

  &:hover .menu-external-icon {
    color: var(--td-brand-color);
  }
}

.menu-divider {
  height: 1px;
  background: var(--td-component-stroke);
  margin: 3px 0;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

.dropdown-enter-to,
.dropdown-leave-from {
  opacity: 1;
  transform: translateY(0);
}
</style>
