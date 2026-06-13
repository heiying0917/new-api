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
