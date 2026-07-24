/**
 * API接口统一导出
 *
 * 所有API接口都在这里统一导出，方便使用
 */

// 知识库相关API
export {
  listKnowledgeBases,
  createKnowledgeBase,
  getKnowledgeBaseById,
  updateKnowledgeBase,
  deleteKnowledgeBase,
  copyKnowledgeBase,
  togglePinKnowledgeBase,
  uploadKnowledgeFile,
  createKnowledgeFromURL,
  listKnowledgeFiles,
  getKnowledgeDetails,
  delKnowledgeDetails,
  batchDeleteKnowledge,
  searchKnowledge,
  listKnowledgeTags,
  createKnowledgeBaseTag,
  deleteKnowledgeBaseTag
} from './knowledge-base'

// 聊天相关API
export {
  listSessions,
  createSession,
  getSessionById,
  updateSession,
  deleteSession,
  listMessages,
  sendMessage,
  streamMessage,
  stopGeneration
} from './chat'

// Agent相关API
export {
  listAgents,
  createAgent,
  getAgentById,
  updateAgent,
  deleteAgent,
  setDefaultAgent
} from './agent'

// 模型相关API
export {
  listProviders,
  updateProviderConfig,
  listModels,
  testModel,
  getModelConfig
} from './model'

// 认证相关API
export {
  login,
  logout,
  getCurrentUser,
  refreshToken,
  changePassword
} from './auth'
