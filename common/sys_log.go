package common

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LogWriterMu protects concurrent access to gin.DefaultWriter/gin.DefaultErrorWriter
// during log file rotation. Acquire RLock when reading/writing through the writers,
// acquire Lock when swapping writers and closing old files.
var LogWriterMu sync.RWMutex

// SysLog 系统级 info 日志（无 context / 无 request_id）。
// 输出统一 NDJSON 走 gin.DefaultWriter，便于 CLS 按 service:tokenki 字段过滤。
func SysLog(s string) {
	LogWriterMu.RLock()
	WriteJSONLog(gin.DefaultWriter, LogEntry{Level: "info", Msg: s})
	LogWriterMu.RUnlock()
}

// SysError 系统级 error 日志。输出统一 NDJSON 走 gin.DefaultErrorWriter。
func SysError(s string) {
	LogWriterMu.RLock()
	WriteJSONLog(gin.DefaultErrorWriter, LogEntry{Level: "error", Msg: s})
	LogWriterMu.RUnlock()
}

// FatalLog 输出 fatal 级别 NDJSON 后 os.Exit(1)。用于启动失败等不可恢复错误。
func FatalLog(v ...any) {
	LogWriterMu.RLock()
	WriteJSONLog(gin.DefaultErrorWriter, LogEntry{Level: "fatal", Msg: fmt.Sprint(v...)})
	LogWriterMu.RUnlock()
	os.Exit(1)
}

// LogStartupSuccess 启动成功提示。保留 ANSI 彩色文本格式——这是 console 友好输出而非
// 结构化日志，CLS 全文索引仍能采集到。不走 WriteJSONLog 是为了保持 vite/rsbuild 风格的
// 启动横幅观感。
func LogStartupSuccess(startTime time.Time, port string) {
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Get network IPs
	networkIps := GetNetworkIps()

	LogWriterMu.RLock()
	defer LogWriterMu.RUnlock()

	fmt.Fprintf(gin.DefaultWriter, "\n")
	fmt.Fprintf(gin.DefaultWriter, "  \033[32m%s %s\033[0m  ready in %d ms\n", SystemName, Version, durationMs)
	fmt.Fprintf(gin.DefaultWriter, "\n")

	if !IsRunningInContainer() {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mLocal:\033[0m   http://localhost:%s/\n", port)
	}

	for _, ip := range networkIps {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mNetwork:\033[0m http://%s:%s/\n", ip, port)
	}

	fmt.Fprintf(gin.DefaultWriter, "\n")
}
