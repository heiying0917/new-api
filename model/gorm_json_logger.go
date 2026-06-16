package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// JSONGormLogger 实现 gorm.io/gorm/logger.Interface，把 GORM 的 SQL / 错误日志
// 输出为单行 JSON（type=sql），消除上游默认 logger 的 ANSI 颜色 + 多行格式，
// 与应用日志统一走 common.WriteJSONLog，便于腾讯云 CLS 采集与按字段聚合。
type JSONGormLogger struct {
	LogLevel                  gormlogger.LogLevel
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

// NewJSONGormLogger 构造 GORM JSON logger。
// 默认：LogLevel=Warn（与 GORM 默认行为对齐：只打 warn/error，不打每条 SQL）
//       SlowThreshold=200ms（慢查询阈值）
//       IgnoreRecordNotFoundError=true（避免 gorm.ErrRecordNotFound 刷屏，这是业务正常情况）
func NewJSONGormLogger() gormlogger.Interface {
	return &JSONGormLogger{
		LogLevel:                  gormlogger.Warn,
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
	}
}

// LogMode 切换日志级别。GORM 在 db.Debug() 时会调用此方法把 level 提升为 Info。
func (l *JSONGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *JSONGormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.LogLevel < gormlogger.Info {
		return
	}
	l.write(ctx, "info", fmt.Sprintf(msg, data...), 0, 0)
}

func (l *JSONGormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.LogLevel < gormlogger.Warn {
		return
	}
	l.write(ctx, "warn", fmt.Sprintf(msg, data...), 0, 0)
}

func (l *JSONGormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.LogLevel < gormlogger.Error {
		return
	}
	l.write(ctx, "error", fmt.Sprintf(msg, data...), 0, 0)
}

// Trace 每次 SQL 执行后被调用，承载 SQL / 行数 / 耗时 / 错误。
// 策略：
//   - 错误（非 ErrRecordNotFound）→ error 级别 + sql/rows/elapsed 字段
//   - 慢查询（>SlowThreshold）→ warn 级别 + sql/rows/elapsed 字段
//   - 正常 SQL → 仅在 LogLevel >= Info 时输出 info 级别（生产默认 Warn 不打）
func (l *JSONGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	elapsedMs := float64(elapsed.Microseconds()) / 1000.0
	sql, rows := fc()

	switch {
	case err != nil && l.LogLevel >= gormlogger.Error && !(l.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound)):
		l.writeSQL(ctx, "error", err.Error(), sql, rows, elapsedMs)
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormlogger.Warn:
		l.writeSQL(ctx, "warn", fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold), sql, rows, elapsedMs)
	case l.LogLevel >= gormlogger.Info:
		l.writeSQL(ctx, "info", "sql", sql, rows, elapsedMs)
	}
}

func (l *JSONGormLogger) write(ctx context.Context, level, msg string, _ int64, _ float64) {
	requestID := ""
	if ctx != nil {
		if v := ctx.Value(common.RequestIdKey); v != nil {
			requestID = fmt.Sprintf("%v", v)
		}
	}
	writer := gin.DefaultErrorWriter
	if level == "info" {
		writer = gin.DefaultWriter
	}
	common.LogWriterMu.RLock()
	common.WriteJSONLog(writer, common.LogEntry{
		Level:     level,
		RequestID: requestID,
		Type:      "sql",
		Msg:       msg,
	})
	common.LogWriterMu.RUnlock()
}

func (l *JSONGormLogger) writeSQL(ctx context.Context, level, msg, sql string, rows int64, elapsedMs float64) {
	requestID := ""
	if ctx != nil {
		if v := ctx.Value(common.RequestIdKey); v != nil {
			requestID = fmt.Sprintf("%v", v)
		}
	}
	writer := gin.DefaultErrorWriter
	if level == "info" {
		writer = gin.DefaultWriter
	}
	common.LogWriterMu.RLock()
	common.WriteJSONLog(writer, common.LogEntry{
		Level:        level,
		RequestID:    requestID,
		Type:         "sql",
		Msg:          msg,
		SQL:          sql,
		RowsAffected: rows,
		ElapsedMs:    elapsedMs,
	})
	common.LogWriterMu.RUnlock()
}
