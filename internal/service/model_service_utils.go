package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"strings"
)

// ---------- JSON 响应解析 ----------

// extractErrorMessage 从 OpenAI 兼容错误响应中提取可读消息。
func extractErrorMessage(body map[string]interface{}, status int) string {
	if body == nil {
		return fmt.Sprintf("请求失败，状态码 %d", status)
	}
	if errValue, ok := body["error"]; ok {
		switch typed := errValue.(type) {
		case string:
			if typed != "" {
				return typed
			}
		case map[string]interface{}:
			if msg := stringValue(typed["message"]); msg != "" {
				return msg
			}
			if msg := stringValue(typed["code"]); msg != "" {
				return msg
			}
		}
	}
	if msg := stringValue(body["message"]); msg != "" {
		return msg
	}
	if raw := stringValue(body["raw"]); raw != "" {
		return raw
	}
	return fmt.Sprintf("请求失败，状态码 %d", status)
}

// extractEmbeddingDimension 从向量响应中提取维度。
func extractEmbeddingDimension(body map[string]interface{}) int {
	vector := extractEmbeddingVector(body)
	return len(vector)
}

// extractEmbeddingVector 从 OpenAI 兼容 embedding 响应中提取向量（data[0].embedding 或 embeddings[0]）。
func extractEmbeddingVector(body map[string]interface{}) []float64 {
	dataArray, ok := body["data"].([]interface{})
	if ok && len(dataArray) > 0 {
		if itemMap, ok := dataArray[0].(map[string]interface{}); ok {
			if embArray, ok := itemMap["embedding"].([]interface{}); ok {
				result := make([]float64, 0, len(embArray))
				for _, value := range embArray {
					result = append(result, float64Value(value))
				}
				return result
			}
		}
	}
	embeddingsArray, ok := body["embeddings"].([]interface{})
	if ok && len(embeddingsArray) > 0 {
		if first, ok := embeddingsArray[0].([]interface{}); ok {
			result := make([]float64, 0, len(first))
			for _, value := range first {
				result = append(result, float64Value(value))
			}
			return result
		}
	}
	return nil
}

// extractChatAnswer 从 chat/completions 响应或 Ollama chat 响应中提取回复文本。
func extractChatAnswer(body map[string]interface{}) string {
	choices, ok := body["choices"].([]interface{})
	if ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := first["message"].(map[string]interface{}); ok {
				return stringValue(message["content"])
			}
		}
	}
	if message, ok := body["message"].(map[string]interface{}); ok {
		return stringValue(message["content"])
	}
	return ""
}

// extractReasoning 从 chat 响应中提取推理过程文本（如 DeepSeek R1 的 reasoning_content）。
func extractReasoning(body map[string]interface{}) string {
	choices, ok := body["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}
	first, ok := choices[0].(map[string]interface{})
	if !ok {
		return ""
	}
	message, ok := first["message"].(map[string]interface{})
	if !ok {
		return ""
	}
	return stringValue(message["reasoning_content"])
}

// extractResultsCount 从 rerank 响应中提取结果数量。
func extractResultsCount(body map[string]interface{}) int {
	if results, ok := body["results"].([]interface{}); ok {
		return len(results)
	}
	if data, ok := body["data"].([]interface{}); ok {
		return len(data)
	}
	return 0
}

// applyThinkingControl 根据 extraConfig 中的 thinking_control 策略向 payload 注入 thinking 参数。
func applyThinkingControl(payload map[string]interface{}, extraConfig map[string]string, options map[string]interface{}) {
	thinkingRaw, exists := options["thinking"]
	if !exists {
		return
	}
	thinking, _ := thinkingRaw.(bool)
	switch strings.TrimSpace(extraConfig["thinking_control"]) {
	case "enable_thinking":
		payload["enable_thinking"] = thinking
	case "thinking_type":
		if thinking {
			payload["thinking_type"] = "enabled"
		} else {
			payload["thinking_type"] = "disabled"
		}
	case "chat_template_kwargs":
		payload["chat_template_kwargs"] = map[string]interface{}{
			"thinking": thinking,
		}
	}
}

// ---------- 文件与字节工具 ----------

// buildTestWAV 生成一个极短的 WAV 头字节数组，用于 ASR 连通性测试。
func buildTestWAV() []byte {
	return []byte{
		0x52, 0x49, 0x46, 0x46, 0x24, 0x08, 0x00, 0x00, 0x57, 0x41, 0x56, 0x45,
		0x66, 0x6d, 0x74, 0x20, 0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
		0x40, 0x1f, 0x00, 0x00, 0x80, 0x3e, 0x00, 0x00, 0x02, 0x00, 0x10, 0x00,
		0x64, 0x61, 0x74, 0x61, 0x00, 0x08, 0x00, 0x00,
	}
}

// fileHeaderToDataURL 将上传文件读取为 base64 Data URL（用于 Vision 模型）。
func fileHeaderToDataURL(header *multipart.FileHeader) (string, error) {
	data, err := fileHeaderBytes(header)
	if err != nil {
		return "", err
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// fileHeaderBytes 读取上传文件的全部字节。
func fileHeaderBytes(header *multipart.FileHeader) ([]byte, error) {
	if header == nil {
		return nil, fmt.Errorf("文件不能为空")
	}
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// fileName 安全提取上传文件的文件名。
func fileName(header *multipart.FileHeader) string {
	if header == nil {
		return ""
	}
	return path.Base(header.Filename)
}

// ---------- 通用工具 ----------

// deduplicateStrings 字符串切片去重（保持原始顺序）。
func deduplicateStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// stringValue 从 interface{} 安全提取字符串值。
func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

// int64Value 从 interface{} 安全提取 int64 值，支持 float64/int64/int/json.Number。
func int64Value(value interface{}) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

// float64Value 从 interface{} 安全提取 float64 值，支持 float64/float32/int/int64/json.Number。
func float64Value(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		return 0
	}
}

// errString 将 error 转为字符串，nil 返回空串。
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// boolMessage 根据布尔值返回对应的成功/失败文案。
func boolMessage(ok bool, successMsg, failMsg string) string {
	if ok {
		return successMsg
	}
	return failMsg
}
