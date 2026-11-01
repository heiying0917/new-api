package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
