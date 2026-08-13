import { get, post } from "../../utils/request";

// 会话短期记忆状态（后端 service.MemoryStatus）
export interface MemoryStatus {
  model_id: string;
  context_window: number;
  current_tokens: number;
  token_threshold: number;
  current_turns: number;
  total_turns: number;
  turns_threshold: number;
  compressed_count: number; // 已压缩进摘要的累计消息条数
  summary_type: string; // llm / raw / ''
  last_compressed_count: number; // 最近一次压缩并入摘要的消息条数（0 = 本次未实际压缩）
  attention: boolean;
}

// 查询会话短期记忆状态（未开启多轮时 data 为 null）
export async function getMemoryStatus(sessionId: string) {
  return get(`/api/v1/sessions/${sessionId}/memory/status`);
}

// 手动压缩会话短期记忆，返回压缩后状态
export async function compressMemory(sessionId: string) {
  return post(`/api/v1/sessions/${sessionId}/memory/compress`, {});
}
