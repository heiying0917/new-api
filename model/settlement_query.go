package model

// SupplierPendingStat 供应商待结算统计
type SupplierPendingStat struct {
	OfficialUsd float64 `json:"official_usd"` // 未结算官方价总额($)
	PayableCNY  float64 `json:"payable_cny"`  // 应付人民币 = Σ(各渠道 officialUsd × cost_price)
	LogCount    int64   `json:"log_count"`
}

// GetSupplierPendingStat 汇总某供应商所有渠道未结算(settlement_id=0)消费日志的官方价与应付金额。
// 使用两步查询（cross-DB safe, no JOIN）：
//  1. 从 channels 表取供应商的所有渠道（id + cost_price）
//  2. 对每个渠道从 LOG_DB 聚合未结算消费日志
func GetSupplierPendingStat(supplierId int) (SupplierPendingStat, error) {
	type channelCost struct {
		Id        int
		CostPrice *float64
	}

	var channels []channelCost
	if err := DB.Model(&Channel{}).
		Select("id, cost_price").
		Where("supplier_id = ?", supplierId).
		Scan(&channels).Error; err != nil {
		return SupplierPendingStat{}, err
	}

	var stat SupplierPendingStat
	for _, ch := range channels {
		var sumUsd float64
		var count int64
		row := LOG_DB.Model(&Log{}).
			Select("COALESCE(SUM(official_usd), 0)").
			Where("type = ? AND settlement_id = 0 AND channel_id = ?", LogTypeConsume, ch.Id).
			Row()
		if err := row.Scan(&sumUsd); err != nil {
			return SupplierPendingStat{}, err
		}

		if err := LOG_DB.Model(&Log{}).
			Where("type = ? AND settlement_id = 0 AND channel_id = ?", LogTypeConsume, ch.Id).
			Count(&count).Error; err != nil {
			return SupplierPendingStat{}, err
		}

		stat.OfficialUsd += sumUsd
		if ch.CostPrice != nil {
			stat.PayableCNY += sumUsd * *ch.CostPrice
		}
		stat.LogCount += count
	}

	return stat, nil
}
