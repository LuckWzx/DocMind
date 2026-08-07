package memory

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// archiveMaxCharsPerMsg 归档时单条消息的最大字符数（防止降级后仍然超长）
const archiveMaxCharsPerMsg = 500

// degradeMessages 降级压缩：LLM 摘要不可用时，把旧消息归档为一条 system 消息，
// 再保留最近 preserveTurns 轮完整对话。归档是纯文本截断，质量低于 LLM 摘要，
// 但保证 Token 不超限且对话不中断。
func degradeMessages(messages []*schema.Message, preserveTurns int) []*schema.Message {
	if preserveTurns <= 0 {
		preserveTurns = DefaultPreserveTurns
	}
	if len(messages) <= 2 {
		return messages
	}

	systemMsgs, contextMsgs := splitSystemMsgs(messages)

	keepStart := tailKeepStart(contextMsgs, preserveTurns)
	if keepStart == 0 {
		return messages
	}

	archiveMsg := &schema.Message{
		Role: schema.System,
		Content: fmt.Sprintf("[Memory Summary - %d earlier messages archived (LLM summarization unavailable)]\n\n%s",
			keepStart, rawArchive(contextMsgs[:keepStart])),
	}

	result := make([]*schema.Message, 0, len(systemMsgs)+1+len(contextMsgs)-keepStart)
	result = append(result, systemMsgs...)
	result = append(result, archiveMsg)
	result = append(result, contextMsgs[keepStart:]...)
	return result
}

// rawArchive 将消息列表转为纯文本归档，单条消息截断到 archiveMaxCharsPerMsg
func rawArchive(messages []*schema.Message) string {
	var sb strings.Builder
	sb.WriteString("Raw conversation archive:\n\n")

	for _, msg := range messages {
		if msg == nil {
			continue
		}
		content := truncateString(msg.Content, archiveMaxCharsPerMsg)
		switch msg.Role {
		case schema.User:
			fmt.Fprintf(&sb, "- User: %s\n", content)
		case schema.Assistant:
			if len(msg.ToolCalls) > 0 {
				names := make([]string, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					names = append(names, tc.Function.Name)
				}
				fmt.Fprintf(&sb, "- Assistant [tools: %s]: %s\n", strings.Join(names, ","), content)
			} else {
				fmt.Fprintf(&sb, "- Assistant: %s\n", content)
			}
		case schema.Tool:
			fmt.Fprintf(&sb, "- Tool[%s]: %s\n", msg.ToolName, content)
		default:
			fmt.Fprintf(&sb, "- %s: %s\n", msg.Role, content)
		}
	}
	return sb.String()
}

// truncateString 按 rune 截断字符串，超出部分用省略号标记
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
