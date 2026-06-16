package middleware

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const RouteTagKey = "route_tag"

func RouteTag(tag string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(RouteTagKey, tag)
		c.Next()
	}
}

// SetUpLogger 注册 HTTP 访问日志中间件，以统一 JSON 结构（type=access）输出。
// 与应用日志 / 系统日志共用 common.WriteJSONLog 的字段与时间格式，便于 CLS 采集与
// 按 status_code / path / user_id / error_reason 等维度聚合查询。
//
// 选择自定义实现而非 gin.LoggerWithFormatter 的原因：
//  1. gin Logger 输出文本格式，无法满足 JSON 结构化日志需求
//  2. 直接拿 c.GetXxx 比 param.Keys 更安全（避免与 handler 内后台 goroutine 并发写产生 data race）
func SetUpLogger(server *gin.Engine) {
	server.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		requestID := c.GetString(common.RequestIdKey)
		tag := c.GetString(RouteTagKey)
		if tag == "" {
			tag = "web"
		}

		status := c.Writer.Status()
		level := "info"
		switch {
		case status >= 500:
			level = "error"
		case status >= 400:
			level = "warn"
		}

		// RequestURI() = Path + "?" + RawQuery，与旧 gin LogFormatter 的 param.Path 行为一致
		uri := c.Request.URL.RequestURI()
		entry := common.LogEntry{
			Level:     level,
			RequestID: requestID,
			Type:      "access",
			Status:    status,
			LatencyMs: float64(latency.Microseconds()) / 1000.0,
			ClientIP:  c.ClientIP(),
			Method:    c.Request.Method,
			Path:      uri,
			RouteTag:  tag,
			Msg:       fmt.Sprintf("%s %s", c.Request.Method, uri),
			UserID:    c.GetInt("id"),
		}
		// TODO: 后续在 relay 错误路径埋点 ContextKeyRelayErrorReason 后，
		// 这里可补 entry.ErrorReason 字段，让 CLS 能按 error_reason 维度聚合 429/401 上游错误
		common.LogWriterMu.RLock()
		common.WriteJSONLog(gin.DefaultWriter, entry)
		common.LogWriterMu.RUnlock()
	})
}
