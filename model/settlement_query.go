package model

// SupplierPendingStat 供应商待结算统计
type SupplierPendingStat struct {
	OfficialUsd float64 `json:"official_usd"` // 未结算官方价总额($)
	PayableCNY  float64 `json:"payable_cny"`  // 应付人民币 = Σ(每条 officialUsd × 冻结成交价 cost_price_snapshot)
	LogCount    int64   `json:"log_count"`
}

// GetSupplierPendingStat 汇总某供应商所有渠道未结算(settlement_id=0)消费日志的官方价与应付金额。
// 应付按「每条日志冻结的成交价 cost_price_snapshot」累加，与结算口径一致、免疫事后改价。
// 两步（cross-DB safe, no JOIN）：取供应商渠道 id → 在 LOG_DB 一次聚合。
func GetSupplierPendingStat(supplierId int) (SupplierPendingStat, error) {
	var channelIds []int
	if err := DB.Model(&Channel{}).
		Where("supplier_id = ?", supplierId).
		Pluck("id", &channelIds).Error; err != nil {
		return SupplierPendingStat{}, err
	}
	var stat SupplierPendingStat
	if len(channelIds) == 0 {
		return stat, nil
	}
	var agg struct {
		OfficialUsd float64
		PayableCNY  float64
		LogCount    int64
	}
	if err := LOG_DB.Model(&Log{}).
		Select("COALESCE(SUM(official_usd),0) AS official_usd, " +
			"COALESCE(SUM(official_usd * cost_price_snapshot),0) AS payable_cny, " +
			"COUNT(*) AS log_count").
		Where("type = ? AND settlement_id = 0 AND channel_id IN ?", LogTypeConsume, channelIds).
		Scan(&agg).Error; err != nil {
		return SupplierPendingStat{}, err
	}
	stat.OfficialUsd = agg.OfficialUsd
	stat.PayableCNY = agg.PayableCNY
	stat.LogCount = agg.LogCount
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
// Receivable = Σ(每条 official_usd × 冻结成交价 cost_price_snapshot)，与结算总额口径一致、免疫改价。
// CostPrice 显示为该渠道本单的实际加权单价（receivable/official_usd），与逐条快照自洽。
// 按 official_usd 降序排列；无日志 → 空切片。仅回填 channel_name（不再活取现价）。
func GetSettlementChannelBreakdown(settlementId int) ([]SettlementChannelRow, error) {
	var rows []SettlementChannelRow
	if err := LOG_DB.Model(&Log{}).
		Select("channel_id AS channel_id, "+
			"COUNT(*) AS requests, "+
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, "+
			"COALESCE(SUM(official_usd), 0) AS official_usd, "+
			"COALESCE(SUM(official_usd * cost_price_snapshot), 0) AS receivable").
		Where("type = ? AND settlement_id = ?", LogTypeConsume, settlementId).
		Group("channel_id").
		Order("official_usd desc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []SettlementChannelRow{}, nil
	}

	// 回填渠道名（一次查询）
	channelIds := make([]int, 0, len(rows))
	for _, r := range rows {
		channelIds = append(channelIds, r.ChannelId)
	}
	type channelMeta struct {
		Id   int
		Name string
	}
	var metas []channelMeta
	if err := DB.Model(&Channel{}).
		Select("id, name").
		Where("id IN ?", channelIds).
		Scan(&metas).Error; err != nil {
		return nil, err
	}
	nameById := make(map[int]string, len(metas))
	for _, m := range metas {
		nameById[m.Id] = m.Name
	}

	for i := range rows {
		rows[i].ChannelName = nameById[rows[i].ChannelId]
		// 实际加权单价（与冻结快照一致）：official × 本列 = receivable
		if rows[i].OfficialUsd > 0 {
			rows[i].CostPrice = rows[i].Receivable / rows[i].OfficialUsd
		}
	}

	return rows, nil
}
