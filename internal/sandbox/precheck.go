package sandbox

import (
	"fmt"
	"strings"
)

// dangerousTokens 静态预检黑名单（防君子不防小人，真正的边界在 guard 与进程级限制）
var dangerousTokens = []string{
	"socket", "requests", "urllib", "subprocess", "ctypes", "multiprocessing",
	"os.system", "os.popen", "os.fork", "os.exec",
	"eval(", "exec(", "compile(", "__import__",
}

// precheckCode 代码静态预检：长度上限 + 危险 token 初筛
func precheckCode(code string, maxLen int) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("代码为空")
	}
	if len(code) > maxLen {
		return fmt.Errorf("代码长度 %d 字节超过上限 %d 字节", len(code), maxLen)
	}
	lower := strings.ToLower(code)
	for _, t := range dangerousTokens {
		if strings.Contains(lower, t) {
			return fmt.Errorf("代码包含可疑内容 %q，已拒绝执行", t)
		}
	}
	return nil
}
