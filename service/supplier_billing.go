package service

import "github.com/QuantumNous/new-api/common"

// ComputeOfficialUsd 计算一次消费的官方价美元（不含分组折扣 groupRatio）。
// usePrice=true 时按固定价计费，官方价即 modelPrice；否则按倍率：
// officialQuota = (prompt + completion*completionRatio) * modelRatio，再 / QuotaPerUnit。
func ComputeOfficialUsd(promptTokens, completionTokens int, modelRatio, completionRatio, modelPrice float64, usePrice bool) float64 {
	if usePrice {
		if modelPrice < 0 {
			return 0
		}
		return modelPrice
	}
	officialQuota := (float64(promptTokens) + float64(completionTokens)*completionRatio) * modelRatio
	if officialQuota < 0 {
		officialQuota = 0
	}
	return officialQuota / common.QuotaPerUnit
}

// OfficialUsdFromQuota 从「已含分组倍率的最终额度」反推官方价美元（不含分组折扣）。
// 适用于音频 / 实时流(wss) / 任务(suno/视频/MJ) 等按额度或按次计费、无 token 拆分的路径：
// 这些路径的 quota 满足 quota = officialUsd × groupRatio × QuotaPerUnit，故反推 officialUsd = quota / (groupRatio × QuotaPerUnit)。
// 守卫非正额度与非正分组倍率，避免除零或负数。
func OfficialUsdFromQuota(quota int, groupRatio float64) float64 {
	if quota <= 0 || groupRatio <= 0 {
		return 0
	}
	return float64(quota) / (groupRatio * common.QuotaPerUnit)
}
