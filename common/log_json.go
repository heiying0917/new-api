package common

import (
	stdjson "encoding/json" // 仅在主 Marshal 失败时用作兜底，不影响正常路径
	"fmt"
	"io"
	"os"
	"time"
)

// LogServiceName 标记日志来源服务，与 K8s namespace / 腾讯云 CLS 主题 service 字段对齐。
// 可通过环境变量 LOG_SERVICE_NAME 覆盖；未设置时默认 "tokenki"。
var LogServiceName = func() string {
	if v := os.Getenv("LOG_SERVICE_NAME"); v != "" {
		return v
	}
	return "tokenki"
}()

// logTimeFormat 为统一的 RFC3339 带毫秒与时区格式，例如 2026-06-16T18:30:00.123+08:00。
// 带时区可让 CLS 解析时间字段无歧义；依赖容器 TZ 环境变量（已在 configmap 设 Asia/Shanghai）。
const logTimeFormat = "2006-01-02T15:04:05.000Z07:00"

// LogEntry 是统一的结构化日志条目。
// 三个日志出口（应用日志 logger.logHelper、系统日志 SysLog/SysError、Gin access log 中间件、
// GORM SQL logger）共用此结构与编码逻辑，保证 stdout 输出的字段名、时间格式完全一致，
// 便于腾讯云 CLS 采集与按字段检索/聚合。
type LogEntry struct {
	Ts        string `json:"ts"`
	Level     string `json:"level"`
	Service   string `json:"service"`
	RequestID string `json:"request_id,omitempty"`
	Msg       string `json:"msg"`
	Stack     string `json:"stack,omitempty"`

	// Type 区分日志类别：app（默认，业务/系统日志）/ access（HTTP 访问日志）/ sql（GORM）
	Type string `json:"type,omitempty"`

	// 以下为 access 日志专用字段
	Status    int     `json:"status,omitempty"`
	LatencyMs float64 `json:"latency_ms,omitempty"`
	ClientIP  string  `json:"client_ip,omitempty"`
	Method    string  `json:"method,omitempty"`
	Path      string  `json:"path,omitempty"`
	RouteTag  string  `json:"route_tag,omitempty"`

	// 业务归因字段：access 日志带 user_id；relay 错误日志带 channel_id/model/upstream_status/error_reason
	UserID    int    `json:"user_id,omitempty"`
	ChannelID int    `json:"channel_id,omitempty"`
	Model     string `json:"model,omitempty"`
	// UpstreamStatus 用指针以区分 "未设置" 与 "上游无 HTTP 响应（值为 0）" 两种情况
	// 网络级错误（TCP 超时/连接拒绝）时 StatusCode=0 也应出现在日志里
	UpstreamStatus *int   `json:"upstream_status,omitempty"`
	ErrorReason    string `json:"error_reason,omitempty"`

	// 以下为 GORM sql 日志专用字段
	SQL          string  `json:"sql,omitempty"`
	RowsAffected int64   `json:"rows_affected,omitempty"`
	ElapsedMs    float64 `json:"elapsed_ms,omitempty"`
}

// WriteJSONLog 将日志条目编码为单行 NDJSON 写入 writer，并自动填充 ts/service/level。
// 编码失败时降级为最小 JSON，保证日志不丢、且仍是合法单行 JSON。
//
// 写入语义：JSON + 换行符一次性 Write，避免并发场景两次 Write 之间被其他 goroutine
// 的日志切入，导致 stdout 出现损坏的 NDJSON 行（CLS 解析会失败）。
func WriteJSONLog(writer io.Writer, entry LogEntry) {
	entry.Ts = time.Now().Format(logTimeFormat)
	entry.Service = LogServiceName
	if entry.Level == "" {
		entry.Level = "info"
	}
	data, err := Marshal(entry)
	if err != nil {
		// 主 Marshal 失败时用标准库重试，保留完整字段
		if data, err = stdjson.Marshal(entry); err != nil {
			// 双重失败时输出最小合法 JSON，至少保证日志不丢
			fmt.Fprintf(writer, "{\"ts\":%q,\"level\":%q,\"service\":%q,\"msg\":%q}\n",
				entry.Ts, entry.Level, LogServiceName, entry.Msg)
			return
		}
	}
	_, _ = writer.Write(append(data, '\n'))
}

// NormalizeLogLevel 将旧 logger 的内部 level 常量（INFO/WARN/ERR/DEBUG/SYS/FATAL）
// 映射为统一的小写级别，与 CLS 通用约定对齐。
func NormalizeLogLevel(level string) string {
	switch level {
	case "INFO", "info":
		return "info"
	case "WARN", "warn":
		return "warn"
	case "ERR", "ERROR", "error":
		return "error"
	case "DEBUG", "debug":
		return "debug"
	case "SYS", "sys":
		return "info"
	case "FATAL", "fatal":
		return "fatal"
	default:
		return "info"
	}
}
