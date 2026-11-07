package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

// 后台保存「限流重试次数」后立即作用于转发重试。
func TestUpdateOptionMap_RateLimitRetryTimes(t *testing.T) {
	prev := common.RateLimitRetryTimes
	t.Cleanup(func() { common.RateLimitRetryTimes = prev })
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}

	require.NoError(t, updateOptionMap("RateLimitRetryTimes", "2"))
	require.Equal(t, 2, common.RateLimitRetryTimes)
	require.Equal(t, "2", common.OptionMap["RateLimitRetryTimes"])

	require.NoError(t, updateOptionMap("RateLimitRetryTimes", "0"))
	require.Equal(t, 0, common.RateLimitRetryTimes, "0 表示限流不换渠道")
}

// 后台保存「限流冷却秒数」后立即作用于 429 冷却；默认 5 秒。
func TestUpdateOptionMap_RateLimitCooldownSeconds(t *testing.T) {
	prev := common.RateLimitCooldownSeconds
	t.Cleanup(func() { common.RateLimitCooldownSeconds = prev })
	require.Equal(t, 5, prev, "默认冷却 5 秒")
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}

	require.NoError(t, updateOptionMap("RateLimitCooldownSeconds", "8"))
	require.Equal(t, 8, common.RateLimitCooldownSeconds)
	require.Equal(t, "8", common.OptionMap["RateLimitCooldownSeconds"])

	require.NoError(t, updateOptionMap("RateLimitCooldownSeconds", "0"))
	require.Equal(t, 0, common.RateLimitCooldownSeconds, "0 表示关闭冷却")
}

// 后台「Claude 设置 → Bedrock 不支持的 beta」保存后写入 Claude 配置，AWS 渠道即时按新名单过滤。
func TestUpdateOptionMap_ClaudeBedrockUnsupportedBetas(t *testing.T) {
	settings := model_setting.GetClaudeSettings()
	saved := settings.BedrockUnsupportedBetas
	t.Cleanup(func() { settings.BedrockUnsupportedBetas = saved })
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}

	require.NoError(t, updateOptionMap("claude.bedrock_unsupported_betas", `["oauth-2025-04-20","new-beta-2026-12-01"]`))
	require.Equal(t, []string{"oauth-2025-04-20", "new-beta-2026-12-01"}, model_setting.GetClaudeSettings().BedrockUnsupportedBetas)
}
