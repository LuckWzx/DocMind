package sandbox

import (
	"encoding/json"
	"strings"
	"time"
)

// resultMarker 协议标记：guard.py 输出的最后一行 = 标记 + JSON 事件数组
const resultMarker = "__SANDBOX_RESULT__"

// parseResult 解析沙箱 stdout：标记行之前为普通输出（Stdout），
// 标记行之后为事件 JSON 数组（text/table/chart/error）。
func parseResult(stdout, stderr string, dur time.Duration) *SandboxResult {
	res := &SandboxResult{Stdout: stdout, Stderr: stderr, Duration: dur}

	idx := strings.LastIndex(stdout, resultMarker)
	if idx < 0 {
		// 无协议输出（guard 未正常收尾）：stderr 即为错误信息
		if msg := strings.TrimSpace(stderr); msg != "" {
			res.Events = append(res.Events, SandboxEvent{Type: "error", Content: msg})
		}
		return res
	}

	res.Stdout = strings.TrimSpace(stdout[:idx])
	payload := strings.TrimSpace(stdout[idx+len(resultMarker):])
	var events []SandboxEvent
	if err := json.Unmarshal([]byte(payload), &events); err != nil {
		res.Events = append(res.Events, SandboxEvent{
			Type:    "error",
			Content: "沙箱结果协议解析失败: " + err.Error(),
		})
		return res
	}
	res.Events = events
	return res
}
