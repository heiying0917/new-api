package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestComputeOfficialUsd_Ratio(t *testing.T) {
	// (1000 + 1000*3)*2 = 8000 quota; usd = 8000/QuotaPerUnit
	usd := ComputeOfficialUsd(1000, 1000, 2, 3, 0, false)
	require.InDelta(t, 8000.0/common.QuotaPerUnit, usd, 1e-9)
}

func TestComputeOfficialUsd_Price(t *testing.T) {
	require.InDelta(t, 0.04, ComputeOfficialUsd(0, 0, 0, 0, 0.04, true), 1e-9)
}

func TestComputeOfficialUsd_Positive(t *testing.T) {
	require.Greater(t, ComputeOfficialUsd(100, 100, 1, 1, 0, false), 0.0)
}

// TestOfficialUsdFromQuota 验证从「已含分组倍率的最终额度」反推官方价美元（不含分组折扣）。
// 关系：quota = officialUsd × groupRatio × QuotaPerUnit，用于音频/实时流/任务/MJ 等无 token 拆分的路径。
func TestOfficialUsdFromQuota(t *testing.T) {
	// officialUsd=1, groupRatio=1 → quota = QuotaPerUnit
	require.InDelta(t, 1.0, OfficialUsdFromQuota(int(common.QuotaPerUnit), 1.0), 1e-9)

	// officialUsd=2, groupRatio=2 → quota = 2 × 2 × QuotaPerUnit
	require.InDelta(t, 2.0, OfficialUsdFromQuota(int(2*2*common.QuotaPerUnit), 2.0), 1e-9)

	// 分组折扣 0.5：officialUsd=4, groupRatio=0.5 → quota = 4 × 0.5 × QuotaPerUnit
	require.InDelta(t, 4.0, OfficialUsdFromQuota(int(4*0.5*common.QuotaPerUnit), 0.5), 1e-9)

	// 守卫：非正额度 / 非正分组倍率一律返回 0，避免除零或负数
	require.Equal(t, 0.0, OfficialUsdFromQuota(0, 1.0))
	require.Equal(t, 0.0, OfficialUsdFromQuota(100, 0))
	require.Equal(t, 0.0, OfficialUsdFromQuota(-100, 1.0))
}
