package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config 沙箱运行配置
type Config struct {
	PythonBin  string        // Python 解释器（Windows: python/py，Linux: python3）
	Timeout    time.Duration // 单次执行超时（默认 30s）
	MaxOutput  int           // 输出大小上限字节（默认 1MB）
	MaxCodeLen int           // 代码长度上限字节（默认 20KB）
}

// PythonSandbox 纯进程级 Python 沙箱（薄壳）：
//   - 一次性子进程：每次 Execute 全新 python 进程，无状态残留
//   - 独立临时工作目录：数据文件挂载点，执行后整体删除
//   - 超时控制：context.WithTimeout 到期杀进程（guard 禁 subprocess → 无子进程残留）
//   - 环境净化：白名单继承环境变量，不携带 HOME / API Key 等敏感信息
type PythonSandbox struct {
	cfg Config
}

// NewPythonSandbox 创建沙箱并验证 Python 解释器可用（不可用时返回错误，调用方决定降级）
func NewPythonSandbox(cfg Config) (*PythonSandbox, error) {
	if cfg.PythonBin == "" {
		cfg.PythonBin = defaultPythonBin()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxOutput <= 0 {
		cfg.MaxOutput = 1 << 20
	}
	if cfg.MaxCodeLen <= 0 {
		cfg.MaxCodeLen = 20 << 10
	}
	if _, err := exec.LookPath(cfg.PythonBin); err != nil {
		return nil, fmt.Errorf("找不到 Python 解释器 %q: %w（请配置 sandbox.python_bin 或安装 Python）", cfg.PythonBin, err)
	}
	return &PythonSandbox{cfg: cfg}, nil
}

// defaultPythonBin 按操作系统选择默认解释器命令
func defaultPythonBin() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// Execute 执行代码（无数据文件）
func (s *PythonSandbox) Execute(ctx context.Context, code string) (*SandboxResult, error) {
	return s.ExecuteWithFiles(ctx, code, nil)
}

// ExecuteWithFiles 执行代码，files 为挂载到工作目录的数据文件（文件名 → 源路径）
func (s *PythonSandbox) ExecuteWithFiles(ctx context.Context, code string, files map[string]string) (*SandboxResult, error) {
	// 1. 静态预检（第一道防线）
	if err := precheckCode(code, s.cfg.MaxCodeLen); err != nil {
		return nil, err
	}

	// 2. 独立临时工作目录（数据文件挂载点，执行后整体删除）
	workDir, err := os.MkdirTemp("", "docmind-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("创建沙箱工作目录失败: %w", err)
	}
	defer os.RemoveAll(workDir)

	// 3. 写入安全壳
	guardPath := filepath.Join(workDir, "guard.py")
	if err := os.WriteFile(guardPath, []byte(guardSource), 0o600); err != nil {
		return nil, fmt.Errorf("写入沙箱安全壳失败: %w", err)
	}

	// 4. 复制数据文件到工作目录（Python 侧用文件名相对路径读取）
	for name, src := range files {
		if err := copyFile(filepath.Join(workDir, filepath.Base(name)), src); err != nil {
			return nil, fmt.Errorf("挂载数据文件 %q 失败: %w", name, err)
		}
	}

	// 5. 带超时启动子进程
	//    -I：隔离模式（忽略 PYTHONPATH / 用户 site-packages）；-B：不写 .pyc
	//    guard.py 从 stdin 读取用户代码并 exec
	execCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, s.cfg.PythonBin, "-I", "-B", "guard.py")
	cmd.Dir = workDir
	cmd.Env = sandboxEnv(workDir)
	cmd.Stdin = strings.NewReader(code)

	var stdout, stderr limitedBuffer
	stdout.max = s.cfg.MaxOutput
	stderr.max = s.cfg.MaxOutput / 4
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	dur := time.Since(start)

	// 6. 超时判定（CommandContext 到期自动 Kill 进程）
	if execCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("代码执行超时（%s），已终止", s.cfg.Timeout)
	}

	// 7. 解析结果协议
	result := parseResult(stdout.String(), stderr.String(), dur)
	if runErr != nil && len(result.Events) == 0 {
		// 进程级失败（guard 本身崩溃等）：stderr 带回溯
		result.Events = append(result.Events, SandboxEvent{
			Type:    "error",
			Content: fmt.Sprintf("沙箱进程异常退出: %v\n%s", runErr, strings.TrimSpace(stderr.String())),
		})
	}
	return result, nil
}

// sandboxEnv 环境白名单：只继承运行必需的变量，不携带敏感信息。
// HOME/USERPROFILE 指向沙箱工作目录：matplotlib 导入需确定 home（字体缓存），
// 指向临时目录可写且用完即焚，不暴露真实用户路径（缓存文件也落在 open 白名单内）。
// MPLBACKEND=Agg 强制 matplotlib 无 GUI 后端（沙箱无显示环境）。
func sandboxEnv(workDir string) []string {
	keep := map[string]bool{
		"PATH": true, "TEMP": true, "TMP": true,
		"SYSTEMROOT": true, "SystemRoot": true, "WINDIR": true, "LANG": true,
	}
	env := make([]string, 0, len(keep)+3)
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok && keep[k] {
			env = append(env, kv)
		}
	}
	return append(env,
		"MPLBACKEND=Agg",
		"HOME="+workDir,        // Linux/macOS expanduser
		"USERPROFILE="+workDir, // Windows ntpath.expanduser 优先 USERPROFILE
	)
}

// copyFile 复制文件（数据挂载用）
func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// limitedBuffer 受限输出缓冲：超过上限后丢弃并标记截断（防止脚本刷爆内存）
type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
			b.truncated = true
		} else {
			b.buf.Write(p)
		}
	} else {
		b.truncated = true
	}
	// 返回 len(p)：即使截断也报告全部消费，避免 exec 收到 ErrShortWrite 提前关闭管道
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	out := b.buf.String()
	if b.truncated {
		out += "\n[输出超过大小限制，已截断]"
	}
	return out
}
