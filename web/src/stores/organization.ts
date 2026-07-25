import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

/**
 * Organization store - stub implementation
 * 共享空间功能已移除，保留空实现以确保现有代码正常编译和运行
 */
export const useOrganizationStore = defineStore('organization', () => {
  const organizations = ref<any[]>([])
  const sharedKnowledgeBases = ref<any[]>([])
  const sharedAgents = ref<any[]>([])
  const resourceCounts = ref<any>(null)

  const clearState = () => {
    organizations.value = []
    sharedKnowledgeBases.value = []
    sharedAgents.value = []
    resourceCounts.value = null
  }

  const fetchOrganizations = async (_options?: { force?: boolean }) => {
    // 共享空间功能已移除
    return []
  }

  const fetchSharedKnowledgeBases = async (_options?: { force?: boolean }) => {
    // 共享空间功能已移除
    return []
  }

  const fetchSharedAgents = async (_options?: { force?: boolean }) => {
    // 共享空间功能已移除
    return []
  }

  return {
    organizations,
    sharedKnowledgeBases,
    sharedAgents,
    resourceCounts,
    clearState,
    fetchOrganizations,
    fetchSharedKnowledgeBases,
    fetchSharedAgents,
  }
})
