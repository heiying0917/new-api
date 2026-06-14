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

// SettlementChannelRow 结算单内单个渠道的聚合明细。
type SettlementChannelRow struct {
	ChannelId   int     `json:"channel_id"`
	ChannelName string  `json:"channel_name"`
	Requests    int64   `json:"requests"`
	Tokens      int64   `json:"tokens"`
	OfficialUsd float64 `json:"official_usd"`
	CostPrice   float64 `json:"cost_price"`
	Receivable  float64 `json:"receivable"` // official_usd × cost_price
}

// GetSettlementChannelBreakdown 按渠道聚合某结算单捕获的消费日志。
// 两步查询（cross-DB safe, no JOIN）：
//  1. 从 LOG_DB 按 channel_id 聚合 settlement_id=settlementId 的日志：
//     requests=COUNT(*), tokens=SUM(prompt+completion), official=SUM(official_usd)；
//  2. 从 DB 的 channels 表回填 channel_name + cost_price（一次查询）。
//
// Receivable = official_usd × cost_price（cost_price 为 nil 视为 0）。
// 按 official_usd 降序排列；无日志 → 空切片。
func GetSettlementChannelBreakdown(settlementId int) ([]SettlementChannelRow, error) {
	var rows []SettlementChannelRow
	if err := LOG_DB.Model(&Log{}).
		Select("channel_id AS channel_id, "+
			"COUNT(*) AS requests, "+
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, "+
			"COALESCE(SUM(official_usd), 0) AS official_usd").
		Where("type = ? AND settlement_id = ?", LogTypeConsume, settlementId).
		Group("channel_id").
		Order("official_usd desc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []SettlementChannelRow{}, nil
	}

	// 回填渠道名与成本价（一次查询）
	channelIds := make([]int, 0, len(rows))
	for _, r := range rows {
		channelIds = append(channelIds, r.ChannelId)
	}
	type channelMeta struct {
		Id        int
		Name      string
		CostPrice *float64
	}
	var metas []channelMeta
	if err := DB.Model(&Channel{}).
		Select("id, name, cost_price").
		Where("id IN ?", channelIds).
		Scan(&metas).Error; err != nil {
		return nil, err
	}
	metaById := make(map[int]channelMeta, len(metas))
	for _, m := range metas {
		metaById[m.Id] = m
	}

	for i := range rows {
		m, ok := metaById[rows[i].ChannelId]
		if !ok {
			continue
		}
		rows[i].ChannelName = m.Name
		if m.CostPrice != nil {
			rows[i].CostPrice = *m.CostPrice
		}
		rows[i].Receivable = rows[i].OfficialUsd * rows[i].CostPrice
	}

	return rows, nil
}
