package service

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

// 生产上 Anthropic 官 key 实际返回的报错原文（2026-11-06，渠道 161、163–168）
const anthropicCreditBalanceTooLowMsg = "Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits."

func anthropicCreditErr() *types.NewAPIError {
	return types.WithOpenAIError(types.OpenAIError{Message: anthropicCreditBalanceTooLowMsg, Type: "invalid_request_error"}, 400)
}

// 「credit balance too low」在 Anthropic 是按请求计费分类判定的（订阅附送额度不覆盖 Claude Code 请求），
// 同一个 key 上普通请求照常成功，因此不能凭一次报错禁用，要先用普通请求探测；其它渠道失效报错仍直接禁用。
func TestNeedsDisableProbe(t *testing.T) {
	cases := []struct {
		name  string
		err   *types.NewAPIError
		probe bool
	}{
		{"Anthropic credit balance too low", anthropicCreditErr(), true},
		{"大小写不敏感", types.NewOpenAIError(errors.New("YOUR CREDIT BALANCE IS TOO LOW"), types.ErrorCodeBadResponseStatusCode, 400), true},
		{"Bedrock API key 失效：直接禁用", bedrockErr(bedrockApiKeyInvalidMsg, 403), false},
		{"任意 401：直接禁用", bedrockErr("unauthorized", 401), false},
		{"账号被封：直接禁用", bedrockErr(bedrockAccountBlockedMsg, 400), false},
		{"非渠道失效报错", bedrockErr(bedrockThrottlingMsg, 429), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.probe, NeedsDisableProbe(tc.err))
		})
	}
}

// 一个坏请求重试时会在同一渠道上反复报错，多个并发请求也会同时报错：同一渠道同一 key 冷却期内只探测一次。
func TestDisableProbeGate(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	g := NewDisableProbeGate(func() time.Time { return now })

	require.True(t, g.TryAcquire(161, "sk-a"), "首次报错发起探测")
	require.False(t, g.TryAcquire(161, "sk-a"), "冷却期内同渠道同 key 不重复探测")
	require.True(t, g.TryAcquire(161, "sk-b"), "多 key 渠道按 key 独立探测")
	require.True(t, g.TryAcquire(163, "sk-a"), "按渠道独立探测")

	now = now.Add(disableProbeCooldown - time.Second)
	require.False(t, g.TryAcquire(161, "sk-a"), "冷却期未满")

	now = now.Add(time.Second)
	require.True(t, g.TryAcquire(161, "sk-a"), "冷却期满后可再次探测（key 后来真没钱时能被禁用）")
}
