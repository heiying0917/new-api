package model

import (
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

// SettledStats 供应商已结算金额（人民币）按时间窗汇总。
type SettledStats struct {
	Today float64 `json:"today"`
	Last7 float64 `json:"last7"`
	Total float64 `json:"total"`
}

// GetSupplierSettledStats 汇总某供应商所有已结算账单(status=settled)的 computed_cny。
// 三个窗口按 settled_at 划分：
//   - today: settled_at >= 今日零点 (now - now%86400)
//   - last7: settled_at >= now - 7*86400
//   - total: 全部
//
// now 为传入的 unix 秒。
func GetSupplierSettledStats(supplierId int, now int64) (SettledStats, error) {
	startOfToday := now - (now % 86400)
	last7Start := now - 7*86400

	var stats SettledStats

	sumWindow := func(extraWhere string, arg int64) (float64, error) {
		var sum float64
		q := DB.Model(&Settlement{}).
			Select("COALESCE(SUM(computed_cny), 0)").
			Where("supplier_id = ? AND status = ?", supplierId, SettlementStatusSettled)
		if extraWhere != "" {
			q = q.Where(extraWhere, arg)
		}
		err := q.Row().Scan(&sum)
		return sum, err
	}

	var err error
	if stats.Today, err = sumWindow("settled_at >= ?", startOfToday); err != nil {
		return SettledStats{}, err
	}
	if stats.Last7, err = sumWindow("settled_at >= ?", last7Start); err != nil {
		return SettledStats{}, err
	}
	if stats.Total, err = sumWindow("", 0); err != nil {
		return SettledStats{}, err
	}
	return stats, nil
}

// GetSupplierChannelStatusCounts 统计某供应商渠道按状态的数量。
// available = status==ChannelStatusEnabled；unavailable = status IN (手动禁用, 自动禁用)。
func GetSupplierChannelStatusCounts(supplierId int) (available int64, unavailable int64, err error) {
	if err = DB.Model(&Channel{}).
		Where("supplier_id = ? AND status = ?", supplierId, common.ChannelStatusEnabled).
		Count(&available).Error; err != nil {
		return 0, 0, err
	}
	if err = DB.Model(&Channel{}).
		Where("supplier_id = ? AND status IN ?", supplierId,
			[]int{common.ChannelStatusManuallyDisabled, common.ChannelStatusAutoDisabled}).
		Count(&unavailable).Error; err != nil {
		return 0, 0, err
	}
	return available, unavailable, nil
}

// GetSupplierTodayUsage 统计给定渠道集合今日的消费日志数与 token 总数。
// 条件：type=LogTypeConsume，channel_id IN channelIds，created_at >= 今日零点。
// requests=COUNT(*)，tokens=SUM(prompt_tokens+completion_tokens)。
// channelIds 为空时直接返回 (0,0,nil)，不执行 IN ()。
func GetSupplierTodayUsage(channelIds []int, now int64) (requests int64, tokens int64, err error) {
	if len(channelIds) == 0 {
		return 0, 0, nil
	}
	startOfToday := now - (now % 86400)

	if err = LOG_DB.Model(&Log{}).
		Where("type = ? AND channel_id IN ? AND created_at >= ?", LogTypeConsume, channelIds, startOfToday).
		Count(&requests).Error; err != nil {
		return 0, 0, err
	}

	if err = LOG_DB.Model(&Log{}).
		Select("COALESCE(SUM(prompt_tokens + completion_tokens), 0)").
		Where("type = ? AND channel_id IN ? AND created_at >= ?", LogTypeConsume, channelIds, startOfToday).
		Row().Scan(&tokens); err != nil {
		return 0, 0, err
	}
	return requests, tokens, nil
}

// GetUnsettledOfficialUsdByChannels 汇总给定渠道集合「未结算」的 official_usd（按渠道分组）。
// 条件：type=LogTypeConsume AND settlement_id=0 AND channel_id IN channelIds，GROUP BY channel_id。
// 返回 map[channelId]totalOfficialUsd，仅包含有未结算日志的渠道。
// channelIds 为空时直接返回空 map（非 nil），不执行 IN ()。
func GetUnsettledOfficialUsdByChannels(channelIds []int) (map[int]float64, error) {
	result := make(map[int]float64)
	if len(channelIds) == 0 {
		return result, nil
	}
	var rows []struct {
		ChannelId int
		Total     float64
	}
	if err := LOG_DB.Model(&Log{}).
		Select("channel_id, COALESCE(SUM(official_usd), 0) as total").
		Where("type = ? AND settlement_id = ? AND channel_id IN ?", LogTypeConsume, 0, channelIds).
		Group("channel_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.ChannelId] = r.Total
	}
	return result, nil
}

// GetUnsettledReceivableByChannels 返回每个渠道未结算消费日志的应收款（¥），
// 按「每条 official_usd × 冻结成交价 cost_price_snapshot」累加，与结算口径一致、免疫事后改价。
func GetUnsettledReceivableByChannels(channelIds []int) (map[int]float64, error) {
	result := make(map[int]float64)
	if len(channelIds) == 0 {
		return result, nil
	}
	var rows []struct {
		ChannelId  int
		Receivable float64
	}
	if err := LOG_DB.Model(&Log{}).
		Select("channel_id, COALESCE(SUM(official_usd * cost_price_snapshot), 0) as receivable").
		Where("type = ? AND settlement_id = ? AND channel_id IN ?", LogTypeConsume, 0, channelIds).
		Group("channel_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.ChannelId] = r.Receivable
	}
	return result, nil
}

// MarketBid 一条匿名报价。
type MarketBid struct {
	Price float64 `json:"price"`
	Mine  bool    `json:"mine"`
}

// MarketBidGroup 某 (type, group) 竞价梯队。
type MarketBidGroup struct {
	Type     int         `json:"type"`
	TypeName string      `json:"type_name"`
	Group    string      `json:"group"`
	Bids     []MarketBid `json:"bids"`    // ascending by price
	MyRank   int         `json:"my_rank"` // 1-based best rank among my bids; 0 if none
	MyBest   *float64    `json:"my_best"` // my lowest price; nil if none
	Total    int         `json:"total"`
}

// GetSupplierChannelIds 返回某供应商的全部渠道 id（任意状态）。
func GetSupplierChannelIds(supplierId int) ([]int, error) {
	var ids []int
	err := DB.Model(&Channel{}).
		Where("supplier_id = ?", supplierId).
		Pluck("id", &ids).Error
	return ids, err
}

// bidKey 用于按 (type, group) 分桶的内部 key。
type bidKey struct {
	Type  int
	Group string
}

// GetSupplierMarketBids 构建以 (type, group) 为键的匿名竞价梯队。
//
// 入选规则（INCLUSION）：仅包含供应商「实际参与」的竞争桶 —— 即供应商拥有至少一个
// 渠道（任意状态）其 (type, group) 命中该桶。
//
// 报价数据（bids）只取启用且 cost_price>0 的渠道（含其他供应商），按价格升序排列，
// mine=渠道 supplier_id==supplierId。MyRank/MyBest 仅由该供应商自己的报价计算。
//
// 返回结果按 (TypeName, Group) 排序。
func GetSupplierMarketBids(supplierId int) ([]MarketBidGroup, error) {
	// 1. 先确定供应商参与的竞争桶（拥有任意状态渠道的 (type, group) 集合）。
	type ownRow struct {
		Type  int
		Group string
	}
	var ownChannels []ownRow
	if err := DB.Model(&Channel{}).
		Select("type, "+commonGroupCol).
		Where("supplier_id = ?", supplierId).
		Scan(&ownChannels).Error; err != nil {
		return nil, err
	}
	included := map[bidKey]bool{}
	for _, oc := range ownChannels {
		ch := Channel{Group: oc.Group}
		for _, g := range ch.GetGroups() {
			if g == "" {
				continue
			}
			included[bidKey{Type: oc.Type, Group: g}] = true
		}
	}
	if len(included) == 0 {
		return []MarketBidGroup{}, nil
	}

	// 2. 加载所有启用且有正成本价的渠道（含所有供应商），用于构建报价梯队。
	type bidRow struct {
		SupplierId int
		Type       int
		Group      string
		CostPrice  *float64
	}
	var rows []bidRow
	if err := DB.Model(&Channel{}).
		Select("supplier_id, type, "+commonGroupCol+", cost_price").
		Where("status = ?", common.ChannelStatusEnabled).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	// 3. 分桶：仅保留供应商参与的桶。
	buckets := map[bidKey][]MarketBid{}
	for _, r := range rows {
		if r.CostPrice == nil || *r.CostPrice <= 0 {
			continue
		}
		ch := Channel{Group: r.Group}
		for _, g := range ch.GetGroups() {
			if g == "" {
				continue
			}
			key := bidKey{Type: r.Type, Group: g}
			if !included[key] {
				continue
			}
			buckets[key] = append(buckets[key], MarketBid{
				Price: *r.CostPrice,
				Mine:  r.SupplierId == supplierId,
			})
		}
	}

	// 4. 为每个参与桶产出一个 MarketBidGroup（即使该桶当前没有任何有效报价）。
	result := make([]MarketBidGroup, 0, len(included))
	for key := range included {
		bids := buckets[key]
		sort.Slice(bids, func(i, j int) bool {
			return bids[i].Price < bids[j].Price
		})

		var myBest *float64
		myRank := 0
		for idx, b := range bids {
			if b.Mine {
				myRank = idx + 1 // 1-based, first (lowest) mine is the best rank
				best := b.Price
				myBest = &best
				break
			}
		}

		result = append(result, MarketBidGroup{
			Type:     key.Type,
			TypeName: constant.GetChannelTypeName(key.Type),
			Group:    key.Group,
			Bids:     bids,
			MyRank:   myRank,
			MyBest:   myBest,
			Total:    len(bids),
		})
	}

	// 5. 按 (TypeName, Group) 排序。
	sort.Slice(result, func(i, j int) bool {
		if result[i].TypeName != result[j].TypeName {
			return result[i].TypeName < result[j].TypeName
		}
		return result[i].Group < result[j].Group
	})

	return result, nil
}

// SupplierDayPoint 供应商单日用量数据点。
type SupplierDayPoint struct {
	Day         int64   `json:"day" gorm:"column:day"`                   // bucket start (unix seconds)
	Requests    int64   `json:"requests" gorm:"column:requests"`         // COUNT(*)
	Tokens      int64   `json:"tokens" gorm:"column:tokens"`             // SUM(prompt_tokens+completion_tokens)
	OfficialUsd float64 `json:"official_usd" gorm:"column:official_usd"` // SUM(official_usd)
}

// GetSupplierUsageSeries 按天返回给定渠道集合的消费用量序列。
// 条件：type=LogTypeConsume，channel_id IN channelIds，created_at BETWEEN startTs AND endTs。
// 按跨库安全的「天桶」表达式分组，结果按 day 升序。
// requests=COUNT(*)，tokens=SUM(prompt_tokens+completion_tokens)，official_usd=SUM(official_usd)。
// channelIds 为空时直接返回空切片，不执行 IN ()。
func GetSupplierUsageSeries(channelIds []int, startTs, endTs int64) ([]SupplierDayPoint, error) {
	if len(channelIds) == 0 {
		return []SupplierDayPoint{}, nil
	}
	bucketExpr := rankingBucketExpr(86400)
	var rows []SupplierDayPoint
	err := LOG_DB.Model(&Log{}).
		Select(fmt.Sprintf("%s as day, COUNT(*) as requests, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, COALESCE(SUM(official_usd), 0) as official_usd", bucketExpr)).
		Where("type = ? AND channel_id IN ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, channelIds, startTs, endTs).
		Group(bucketExpr).
		Order("day ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []SupplierDayPoint{}
	}
	return rows, nil
}

// SupplierChannelRank 供应商单渠道用量排名条目。
type SupplierChannelRank struct {
	ChannelId   int     `json:"channel_id" gorm:"column:channel_id"`
	ChannelName string  `json:"channel_name" gorm:"-"`
	Requests    int64   `json:"requests" gorm:"column:requests"`
	Tokens      int64   `json:"tokens" gorm:"column:tokens"`
	OfficialUsd float64 `json:"official_usd" gorm:"column:official_usd"`
}

// GetSupplierChannelRanking 按渠道维度汇总给定渠道集合的消费用量排名。
// 条件同 GetSupplierUsageSeries（同一时间窗 + type + channel_id IN）。
// 按 channel_id 分组，结果按 official_usd DESC、requests DESC 排序。
// ChannelName 通过一次 channels 表查询回填（参考 GetAllLogs）。
// channelIds 为空时直接返回空切片。
func GetSupplierChannelRanking(channelIds []int, startTs, endTs int64) ([]SupplierChannelRank, error) {
	if len(channelIds) == 0 {
		return []SupplierChannelRank{}, nil
	}
	var rows []SupplierChannelRank
	err := LOG_DB.Model(&Log{}).
		Select("channel_id, COUNT(*) as requests, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, COALESCE(SUM(official_usd), 0) as official_usd").
		Where("type = ? AND channel_id IN ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, channelIds, startTs, endTs).
		Group("channel_id").
		Order("official_usd DESC, requests DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []SupplierChannelRank{}, nil
	}

	// 回填渠道名（一次查询）。
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ChannelId)
	}
	var channels []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err = DB.Table("channels").Select("id, name").Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	nameMap := make(map[int]string, len(channels))
	for _, ch := range channels {
		nameMap[ch.Id] = ch.Name
	}
	for i := range rows {
		rows[i].ChannelName = nameMap[rows[i].ChannelId]
	}
	return rows, nil
}

// GetAllSuppliersPendingStat 一次聚合所有供应商未结算消费,返回 per-supplier map 与全局合计。
// cross-DB safe, no JOIN:渠道→供应商映射(DB) + 日志按 channel_id 聚合(LOG_DB) → Go 折叠。
func GetAllSuppliersPendingStat() (map[int]SupplierPendingStat, SupplierPendingStat, error) {
	type chanRow struct {
		Id         int
		SupplierId int
	}
	var rows []chanRow
	if err := DB.Model(&Channel{}).Select("id, supplier_id").
		Where("supplier_id > 0").Scan(&rows).Error; err != nil {
		return nil, SupplierPendingStat{}, err
	}
	perSupplier := make(map[int]SupplierPendingStat)
	var global SupplierPendingStat
	if len(rows) == 0 {
		return perSupplier, global, nil
	}
	chanToSupplier := make(map[int]int, len(rows))
	channelIds := make([]int, 0, len(rows))
	for _, r := range rows {
		chanToSupplier[r.Id] = r.SupplierId
		channelIds = append(channelIds, r.Id)
	}
	type agg struct {
		ChannelId   int
		OfficialUsd float64
		PayableCNY  float64
		LogCount    int64
	}
	var aggs []agg
	if err := LOG_DB.Model(&Log{}).
		Select("channel_id AS channel_id, " +
			"COALESCE(SUM(official_usd),0) AS official_usd, " +
			"COALESCE(SUM(official_usd * cost_price_snapshot),0) AS payable_cny, " +
			"COUNT(*) AS log_count").
		Where("type = ? AND settlement_id = 0 AND channel_id IN ?", LogTypeConsume, channelIds).
		Group("channel_id").Scan(&aggs).Error; err != nil {
		return nil, SupplierPendingStat{}, err
	}
	for _, a := range aggs {
		sid := chanToSupplier[a.ChannelId]
		s := perSupplier[sid]
		s.OfficialUsd += a.OfficialUsd
		s.PayableCNY += a.PayableCNY
		s.LogCount += a.LogCount
		perSupplier[sid] = s
		global.OfficialUsd += a.OfficialUsd
		global.PayableCNY += a.PayableCNY
		global.LogCount += a.LogCount
	}
	return perSupplier, global, nil
}

// SettlementTotals 某状态结算单的合计。
type SettlementTotals struct {
	OfficialUsd float64 `json:"official_usd"`
	ComputedCNY float64 `json:"computed_cny"`
	ActualCNY   float64 `json:"actual_cny"`
	ActualUSD   float64 `json:"actual_usd"`
	Count       int64   `json:"count"`
}

// GetSettlementTotalsByStatus 汇总指定状态结算单;actual_* 仅对已结算有意义,按币种拆分。
func GetSettlementTotalsByStatus(status int) (SettlementTotals, error) {
	var t SettlementTotals
	var base struct {
		OfficialUsd float64
		ComputedCNY float64
		Count       int64
	}
	if err := DB.Model(&Settlement{}).
		Select("COALESCE(SUM(official_usd),0) AS official_usd, " +
			"COALESCE(SUM(computed_cny),0) AS computed_cny, " +
			"COUNT(*) AS count").
		Where("status = ?", status).Scan(&base).Error; err != nil {
		return t, err
	}
	t.OfficialUsd, t.ComputedCNY, t.Count = base.OfficialUsd, base.ComputedCNY, base.Count

	type curRow struct {
		ActualCurrency string
		Amount         float64
	}
	var cur []curRow
	if err := DB.Model(&Settlement{}).
		Select("actual_currency AS actual_currency, COALESCE(SUM(actual_amount),0) AS amount").
		Where("status = ?", status).Group("actual_currency").Scan(&cur).Error; err != nil {
		return t, err
	}
	for _, c := range cur {
		if c.ActualCurrency == "USD" {
			t.ActualUSD += c.Amount
		} else {
			t.ActualCNY += c.Amount // CNY 或空币种归入人民币
		}
	}
	return t, nil
}

// GetAllSuppliersSettledTotal per-supplier 已结算 computed_cny 合计。
func GetAllSuppliersSettledTotal() (map[int]float64, error) {
	type row struct {
		SupplierId int
		Total      float64
	}
	var rows []row
	if err := DB.Model(&Settlement{}).
		Select("supplier_id AS supplier_id, COALESCE(SUM(computed_cny),0) AS total").
		Where("status = ?", SettlementStatusSettled).
		Group("supplier_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[int]float64, len(rows))
	for _, r := range rows {
		m[r.SupplierId] = r.Total
	}
	return m, nil
}

// SupplierOverviewSummary 全局供应商概览的汇总指标。
type SupplierOverviewSummary struct {
	SupplierTotal      int64 `json:"supplier_total"`
	SupplierEnabled    int64 `json:"supplier_enabled"`
	ChannelTotal       int64 `json:"channel_total"`
	ChannelAvailable   int64 `json:"channel_available"`
	ChannelUnavailable int64 `json:"channel_unavailable"`
}

// SupplierTypeGroup 某官方 key 类型下某分组的最低成本价。
type SupplierTypeGroup struct {
	Group       string  `json:"group"`
	LowestPrice float64 `json:"lowest_price"`
}

// SupplierTypeStat 按官方 key 类型(= 渠道 type)聚合的供给统计。
type SupplierTypeStat struct {
	Type          int                 `json:"type"`
	TypeName      string              `json:"type_name"`
	SupplierCount int                 `json:"supplier_count"`
	ChannelCount  int                 `json:"channel_count"`
	Available     int                 `json:"available"`
	Unavailable   int                 `json:"unavailable"`
	LowestPrice   float64             `json:"lowest_price"`
	Groups        []SupplierTypeGroup `json:"groups"`
}

// SupplierOverview 全局供应商概览：汇总 + 分类型供给。
type SupplierOverview struct {
	Summary SupplierOverviewSummary `json:"summary"`
	ByType  []SupplierTypeStat      `json:"by_type"`
}

// GetSupplierOverview 全局供应商概览：供应商总数/启用数、渠道可用性、按官方 key 类型(渠道 type)的供给统计。
//
// 数据来源（cross-DB safe，无方言 SQL）：
//   - SupplierTotal：role=RoleSupplierUser 的用户数。
//   - SupplierEnabled：suppliers 表 enabled=true 的行数。
//   - 渠道：一次 Find(&channels) 取 supplier_id>0 的全部渠道（避开保留字 group），在 Go 中折叠。
//
// 「可用」定义与 GetGroupMarketPrices 一致：status==ChannelStatusEnabled；
// 最低价仅统计「启用且 cost_price>0」的渠道。
func GetSupplierOverview() (SupplierOverview, error) {
	var ov SupplierOverview

	if err := DB.Model(&User{}).
		Where("role = ?", common.RoleSupplierUser).
		Count(&ov.Summary.SupplierTotal).Error; err != nil {
		return SupplierOverview{}, err
	}
	if err := DB.Model(&Supplier{}).
		Where("enabled = ?", true).
		Count(&ov.Summary.SupplierEnabled).Error; err != nil {
		return SupplierOverview{}, err
	}

	// 取全部供应商渠道（任意状态）。用整行 Find 避开保留字 group，三库安全。
	var channels []Channel
	if err := DB.Where("supplier_id > 0").Find(&channels).Error; err != nil {
		return SupplierOverview{}, err
	}

	type typeAgg struct {
		channelCount int
		available    int
		unavailable  int
		suppliers    map[int]bool
		lowestPrice  float64                       // 该 type 下启用且 cost_price>0 的最低价；0 表示无
		groupPrices  map[string]float64            // group -> 最低价（启用且 cost_price>0）
	}
	byType := make(map[int]*typeAgg)

	for i := range channels {
		ch := &channels[i]
		agg := byType[ch.Type]
		if agg == nil {
			agg = &typeAgg{suppliers: map[int]bool{}, groupPrices: map[string]float64{}}
			byType[ch.Type] = agg
		}
		agg.channelCount++
		agg.suppliers[ch.SupplierId] = true

		enabled := ch.Status == common.ChannelStatusEnabled
		if enabled {
			agg.available++
		} else {
			agg.unavailable++
		}
		ov.Summary.ChannelTotal++
		if enabled {
			ov.Summary.ChannelAvailable++
		} else {
			ov.Summary.ChannelUnavailable++
		}

		// 最低价/分组价：仅启用且 cost_price>0。
		if enabled && ch.CostPrice != nil && *ch.CostPrice > 0 {
			price := *ch.CostPrice
			if agg.lowestPrice == 0 || price < agg.lowestPrice {
				agg.lowestPrice = price
			}
			for _, g := range ch.GetGroups() {
				if g == "" {
					continue
				}
				if cur, ok := agg.groupPrices[g]; !ok || price < cur {
					agg.groupPrices[g] = price
				}
			}
		}
	}

	ov.ByType = make([]SupplierTypeStat, 0, len(byType))
	for typ, agg := range byType {
		groups := make([]SupplierTypeGroup, 0, len(agg.groupPrices))
		for g, p := range agg.groupPrices {
			groups = append(groups, SupplierTypeGroup{Group: g, LowestPrice: p})
		}
		sort.Slice(groups, func(i, j int) bool {
			if groups[i].LowestPrice != groups[j].LowestPrice {
				return groups[i].LowestPrice < groups[j].LowestPrice
			}
			return groups[i].Group < groups[j].Group
		})
		ov.ByType = append(ov.ByType, SupplierTypeStat{
			Type:          typ,
			TypeName:      constant.GetChannelTypeName(typ),
			SupplierCount: len(agg.suppliers),
			ChannelCount:  agg.channelCount,
			Available:     agg.available,
			Unavailable:   agg.unavailable,
			LowestPrice:   agg.lowestPrice,
			Groups:        groups,
		})
	}
	// 按 ChannelCount 降序，稳定展示；并列时按 Type 升序保证确定性。
	sort.Slice(ov.ByType, func(i, j int) bool {
		if ov.ByType[i].ChannelCount != ov.ByType[j].ChannelCount {
			return ov.ByType[i].ChannelCount > ov.ByType[j].ChannelCount
		}
		return ov.ByType[i].Type < ov.ByType[j].Type
	})

	return ov, nil
}

// GetSupplierRealtimeStat 汇总给定渠道集合的实时统计。
// Rpm/Tpm 取最近 60 秒：rpm=COUNT(*)，tpm=SUM(prompt_tokens+completion_tokens)。
// Quota 取最近 24 小时：SUM(quota)。
// 仅统计 type=LogTypeConsume 且 channel_id IN channelIds 的日志。
// channelIds 为空时直接返回零值 Stat。
func GetSupplierRealtimeStat(channelIds []int) (Stat, error) {
	var stat Stat
	if len(channelIds) == 0 {
		return stat, nil
	}
	now := time.Now()
	last60 := now.Add(-60 * time.Second).Unix()
	last24h := now.Add(-24 * time.Hour).Unix()

	if err := LOG_DB.Model(&Log{}).
		Select("COUNT(*) as rpm, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tpm").
		Where("type = ? AND channel_id IN ? AND created_at >= ?", LogTypeConsume, channelIds, last60).
		Scan(&stat).Error; err != nil {
		return Stat{}, err
	}

	var quota int
	if err := LOG_DB.Model(&Log{}).
		Select("COALESCE(SUM(quota), 0)").
		Where("type = ? AND channel_id IN ? AND created_at >= ?", LogTypeConsume, channelIds, last24h).
		Row().Scan(&quota); err != nil {
		return Stat{}, err
	}
	stat.Quota = quota
	return stat, nil
}
