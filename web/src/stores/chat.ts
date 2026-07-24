import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Session, Message } from '@/types/chat'
import {
  listSessions,
  createSession,
  getSessionById,
  deleteSession,
  listMessages,
  sendMessage,
  streamMessage
} from '@/api/chat'

export const useChatStore = defineStore('chat', () => {
  // 状态
  const sessions = ref<Session[]>([])
  const currentSession = ref<Session | null>(null)
  const messages = ref<Message[]>([])
  const isLoading = ref(false)
  const isStreaming = ref(false)
  const streamingContent = ref('')

  // 操作
  async function fetchSessions() {
    isLoading.value = true
    try {
      const result = await listSessions()
      sessions.value = result.items
    } catch (error) {
      console.error('获取会话列表失败:', error)
      throw error
    } finally {
      isLoading.value = false
    }
  }

  async function createNewSession(data?: {
    title?: string
    knowledge_base_ids?: string[]
    agent_id?: string
  }) {
    isLoading.value = true
    try {
      const newSession = await createSession(data)
      sessions.value.unshift(newSession)
      currentSession.value = newSession
      messages.value = []
      return newSession
    } catch (error) {
      console.error('创建会话失败:', error)
      throw error
    } finally {
      isLoading.value = false
    }
  }

  async function selectSession(id: string) {
    isLoading.value = true
    try {
      currentSession.value = await getSessionById(id)
      await fetchMessages(id)
    } catch (error) {
      console.error('获取会话详情失败:', error)
      throw error
    } finally {
      isLoading.value = false
    }
  }

  async function deleteExistingSession(id: string) {
    isLoading.value = true
    try {
      await deleteSession(id)
      sessions.value = sessions.value.filter(s => s.id !== id)
      if (currentSession.value?.id === id) {
        currentSession.value = null
        messages.value = []
      }
    } catch (error) {
      console.error('删除会话失败:', error)
      throw error
    } finally {
      isLoading.value = false
    }
  }

  async function fetchMessages(sessionId: string) {
    isLoading.value = true
    try {
      const result = await listMessages(sessionId)
      messages.value = result.items
    } catch (error) {
      console.error('获取消息列表失败:', error)
      throw error
    } finally {
      isLoading.value = false
    }
  }

  async function sendNewMessage(content: string) {
    if (!currentSession.value) return

    const userMessage: Message = {
      id: String(Date.now()),
      session_id: currentSession.value.id,
      role: 'user',
      content,
      created_at: new Date().toISOString()
    }
    messages.value.push(userMessage)

    isStreaming.value = true
    streamingContent.value = ''

    try {
      await streamMessage(
        {
          session_id: currentSession.value.id,
          content,
          knowledge_base_ids: currentSession.value.knowledge_base_ids
        },
        {
          onChunk: (chunk) => {
            streamingContent.value += chunk
          },
          onComplete: (assistantMessage) => {
            messages.value.push(assistantMessage)
            streamingContent.value = ''
            isStreaming.value = false
          },
          onError: (error) => {
            console.error('发送消息失败:', error)
            isStreaming.value = false
            streamingContent.value = ''
          }
        }
      )
    } catch (error) {
      console.error('发送消息失败:', error)
      isStreaming.value = false
      streamingContent.value = ''
    }
  }

  return {
    sessions,
    currentSession,
    messages,
    isLoading,
    isStreaming,
    streamingContent,
    fetchSessions,
    createNewSession,
    selectSession,
    deleteExistingSession,
    fetchMessages,
    sendNewMessage
  }
})
