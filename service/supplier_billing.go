package service

import "github.com/QuantumNous/new-api/common"

// OfficialUsdFromQuota 从「已含分组倍率的最终额度」反推供应商官方价美元（不含分组倍率）。
// 所有计费路径（文本、音频/实时流、任务、MJ）的 quota 都满足 quota = officialUsd × groupRatio × QuotaPerUnit。
// 文本路径的官方价在 calculateTextQuotaSummary 内以 decimal 同源计算（避免整数额度取整误差）；
// 这里保留给按额度/按次计费、无 token 拆分的路径使用。实现见 common.OfficialUsdFromQuota。
func OfficialUsdFromQuota(quota int, groupRatio float64) float64 {
	return common.OfficialUsdFromQuota(quota, groupRatio)
}
