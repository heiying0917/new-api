package service

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/types"
)

// 「余额不足」先探测再禁用：Anthropic 的 "Your credit balance is too low" 不一定代表整个 key 没钱。
// 订阅（Max/Ultra/Team）附送的 API 额度不覆盖 Claude Code，被识别为 Claude Code 的请求（如带
// anthropic-beta: claude-code-*）会改扣购买余额，购买余额为 0 时同一个 key 上普通请求照常成功、
// 这类请求却报余额不足。一个这样的请求换渠道重试时会把同组 key 在几秒内全部禁光（2026-11-06 生产事故）。
// 因此命中该报错时不直接禁用，由调用方用一条普通请求探测同一个 key，探测也命中禁用规则才禁用。
// 换渠道重试的判定不变（IsChannelFatalError 仍为 true）。

const (
	creditBalanceTooLowMessage = "your credit balance is too low"
	// 同一渠道同一 key 两次探测的最小间隔：防并发请求/重试重复探测（每次探测会记一条「模型测试」日志），
	// 代价是 key 真没钱时最多晚这么久才被禁用（期间请求在该渠道失败后照常换渠道重试）。
	disableProbeCooldown = 5 * time.Minute
)

// NeedsDisableProbe 该渠道失效报错是否需要先用普通请求探测确认，再决定是否禁用。
func NeedsDisableProbe(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), creditBalanceTooLowMessage)
}

type disableProbeGateKey struct {
	channelId int
	usingKey  string
}

// DisableProbeGate 冷却期内同一渠道同一 key 只放行一次探测。
type DisableProbeGate struct {
	mu   sync.Mutex
	last map[disableProbeGateKey]time.Time
	now  func() time.Time
}

func NewDisableProbeGate(now func() time.Time) *DisableProbeGate {
	return &DisableProbeGate{last: map[disableProbeGateKey]time.Time{}, now: now}
}

// TryAcquire 是否由本次报错发起禁用前探测；冷却期内已探测过则返回 false（本次不禁用）。
func (g *DisableProbeGate) TryAcquire(channelId int, usingKey string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := disableProbeGateKey{channelId: channelId, usingKey: usingKey}
	now := g.now()
	if last, ok := g.last[k]; ok && now.Sub(last) < disableProbeCooldown {
		return false
	}
	g.last[k] = now
	return true
}
