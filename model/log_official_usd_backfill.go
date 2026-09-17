package model

import (
	"math"

	"github.com/QuantumNous/new-api/common"
)

// OfficialUsdBackfillSupplierDelta 单个供应商在本批回填中的差额汇总。
type OfficialUsdBackfillSupplierDelta struct {
	SupplierId int     `json:"supplier_id"`
	Scanned    int64   `json:"scanned"`
	Updated    int64   `json:"updated"`
	OldUsd     float64 `json:"old_usd"`
	NewUsd     float64 `json:"new_usd"`
	DeltaUsd   float64 `json:"delta_usd"`
	DeltaCny   float64 `json:"delta_cny"` // Σ(new−old)×cost_price_snapshot，即应付款差额
}

// OfficialUsdBackfillResult 一批回填的结果；按 id 游标分页，Done=false 时用 NextAfterId 续传。
type OfficialUsdBackfillResult struct {
	DryRun      bool                                      `json:"dry_run"`
	AfterId     int                                       `json:"after_id"`
	Limit       int                                       `json:"limit"`
	Scanned     int64                                     `json:"scanned"`
	Updated     int64                                     `json:"updated"` // dry_run 时为「将更新」条数
	Unchanged   int64                                     `json:"unchanged"`
	Skipped     int64                                     `json:"skipped"` // other 无有效 group_ratio，无法反推
	NextAfterId int                                       `json:"next_after_id"`
	Done        bool                                      `json:"done"`
	PerSupplier map[int]*OfficialUsdBackfillSupplierDelta `json:"per_supplier"`
}

const officialUsdBackfillEpsilon = 1e-9

// BackfillUnsettledOfficialUsd 重算供应商渠道「未结算」消费日志的 official_usd。
// 背景：文本路径曾漏算 Claude 缓存读写 token（2026-09-18 WORKLOG），历史日志官方价偏低约 3/4。
// 口径：official = quota ÷ group_ratio ÷ QuotaPerUnit，group_ratio 取自该条日志 other（成交时倍率），
// 与修复后的全部计费路径同源，天然包含缓存/图像/音频/tool 附加费/附加倍率。
// 只动 type=consume 且 settlement_id=0 的供应商渠道日志；已打包进账单的不动（账单金额已冻结，需先撤销）。
// 按 id 升序游标分页，每批 limit 条；每条 UPDATE 再带 settlement_id=0 条件，避免与并发发起结算打架。
// cross-DB safe：不做 JSON 函数与 JOIN，other 在 Go 侧解析。
func BackfillUnsettledOfficialUsd(afterId int, limit int, dryRun bool) (*OfficialUsdBackfillResult, error) {
	if limit <= 0 {
		limit = 1000
	}
	if afterId < 0 {
		afterId = 0
	}
	res := &OfficialUsdBackfillResult{
		DryRun:      dryRun,
		AfterId:     afterId,
		Limit:       limit,
		NextAfterId: afterId,
		PerSupplier: make(map[int]*OfficialUsdBackfillSupplierDelta),
	}

	type chanRow struct {
		Id         int
		SupplierId int
	}
	var chans []chanRow
	if err := DB.Model(&Channel{}).Select("id, supplier_id").
		Where("supplier_id > 0").Scan(&chans).Error; err != nil {
		return nil, err
	}
	if len(chans) == 0 {
		res.Done = true
		return res, nil
	}
	chanToSupplier := make(map[int]int, len(chans))
	channelIds := make([]int, 0, len(chans))
	for _, ch := range chans {
		chanToSupplier[ch.Id] = ch.SupplierId
		channelIds = append(channelIds, ch.Id)
	}

	type logRow struct {
		Id                int
		ChannelId         int
		Quota             int
		Other             string
		OfficialUsd       float64
		CostPriceSnapshot float64
	}
	var rows []logRow
	if err := LOG_DB.Model(&Log{}).
		Select("id, channel_id, quota, other, official_usd, cost_price_snapshot").
		Where("type = ? AND settlement_id = 0 AND channel_id IN ? AND id > ?", LogTypeConsume, channelIds, afterId).
		Order("id asc").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	res.Done = len(rows) < limit

	for _, r := range rows {
		res.Scanned++
		res.NextAfterId = r.Id
		supplierId := chanToSupplier[r.ChannelId]
		sup := res.PerSupplier[supplierId]
		if sup == nil {
			sup = &OfficialUsdBackfillSupplierDelta{SupplierId: supplierId}
			res.PerSupplier[supplierId] = sup
		}
		sup.Scanned++
		sup.OldUsd += r.OfficialUsd

		gr, ok := logGroupRatio(r.Other)
		if !ok {
			res.Skipped++
			sup.NewUsd += r.OfficialUsd
			continue
		}
		newUsd := common.OfficialUsdFromQuota(r.Quota, gr)
		sup.NewUsd += newUsd
		if math.Abs(newUsd-r.OfficialUsd) < officialUsdBackfillEpsilon {
			res.Unchanged++
			continue
		}
		res.Updated++
		sup.Updated++
		sup.DeltaUsd += newUsd - r.OfficialUsd
		sup.DeltaCny += (newUsd - r.OfficialUsd) * r.CostPriceSnapshot
		if dryRun {
			continue
		}
		if err := LOG_DB.Model(&Log{}).
			Where("id = ? AND settlement_id = 0", r.Id).
			Update("official_usd", newUsd).Error; err != nil {
			return nil, err
		}
	}
	return res, nil
}

// logGroupRatio 从日志 other JSON 取成交时的分组倍率；缺失 / 非法 JSON / 非正 / 非有限值返回 false。
func logGroupRatio(other string) (float64, bool) {
	if other == "" {
		return 0, false
	}
	var m map[string]interface{}
	if err := common.UnmarshalJsonStr(other, &m); err != nil {
		return 0, false
	}
	v, ok := m["group_ratio"]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	if !ok || !(f > 0) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}
