package model

import (
	"errors"
	"math"
)

// UnsettledRepriceResult 单个渠道一次重定价的结果（dry-run 时为「将发生」的变化）。
type UnsettledRepriceResult struct {
	ChannelId   int     `json:"channel_id"`
	SupplierId  int     `json:"supplier_id"`
	NewPrice    float64 `json:"new_price"`
	Logs        int64   `json:"logs"`         // 受影响的未结算日志条数
	OfficialUsd float64 `json:"official_usd"` // 这些日志的官方价合计（美元）
	OldCny      float64 `json:"old_cny"`      // 重定价前 Σ official_usd × cost_price_snapshot
	NewCny      float64 `json:"new_cny"`      // 重定价后 Σ official_usd × newPrice
	DeltaCny    float64 `json:"delta_cny"`    // NewCny − OldCny，即应付款差额
}

// RepriceUnsettledLogs 把某渠道「未结算」消费日志的成交价 cost_price_snapshot 同步为 newPrice。
//
// 口径（2026-09-18 起）：未结算日志始终按渠道**当前**成本价计价——供应商/管理员在结算前改价，
// 待结算金额随之变化；日志一旦打包进账单（settlement_id≠0）即冻结，之后改价不再影响该账单。
// 只动 type=consume AND settlement_id=0 AND channel_id=? 的行；dryRun 只算差额不写库。
// 调用方负责把差额记入资金账本（action=reprice），保证改价有迹可循。
func RepriceUnsettledLogs(channelId, supplierId int, newPrice float64, dryRun bool) (*UnsettledRepriceResult, error) {
	if channelId <= 0 {
		return nil, errors.New("invalid channel id")
	}
	if !(newPrice > 0) || math.IsInf(newPrice, 0) {
		return nil, errors.New("cost price must be > 0")
	}
	res := &UnsettledRepriceResult{ChannelId: channelId, SupplierId: supplierId, NewPrice: newPrice}
	var agg struct {
		Logs        int64
		OfficialUsd float64
		OldCny      float64
	}
	if err := LOG_DB.Model(&Log{}).
		Select("COUNT(*) AS logs, COALESCE(SUM(official_usd),0) AS official_usd, COALESCE(SUM(official_usd * cost_price_snapshot),0) AS old_cny").
		Where("type = ? AND settlement_id = 0 AND channel_id = ?", LogTypeConsume, channelId).
		Scan(&agg).Error; err != nil {
		return nil, err
	}
	res.Logs = agg.Logs
	res.OfficialUsd = agg.OfficialUsd
	res.OldCny = agg.OldCny
	res.NewCny = agg.OfficialUsd * newPrice
	res.DeltaCny = res.NewCny - res.OldCny
	if dryRun || res.Logs == 0 {
		return res, nil
	}
	if err := LOG_DB.Model(&Log{}).
		Where("type = ? AND settlement_id = 0 AND channel_id = ? AND cost_price_snapshot <> ?", LogTypeConsume, channelId, newPrice).
		Update("cost_price_snapshot", newPrice).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// RepriceAllUnsettledLogsToCurrent 把所有供应商渠道的未结算日志按各渠道当前成本价重定价。
// 用于一次性对齐历史数据（例如改价 hook 上线前已发生的改价）；非供应商渠道、成本价缺失的渠道跳过，
// 没有未结算日志的渠道不出现在结果里。
func RepriceAllUnsettledLogsToCurrent(dryRun bool) ([]*UnsettledRepriceResult, error) {
	var chans []*Channel
	if err := DB.Select("id, supplier_id, cost_price").
		Where("supplier_id > 0").Order("id asc").Find(&chans).Error; err != nil {
		return nil, err
	}
	out := make([]*UnsettledRepriceResult, 0, len(chans))
	for _, ch := range chans {
		if ch.CostPrice == nil || !(*ch.CostPrice > 0) {
			continue
		}
		r, err := RepriceUnsettledLogs(ch.Id, ch.SupplierId, *ch.CostPrice, dryRun)
		if err != nil {
			return nil, err
		}
		if r.Logs == 0 {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
