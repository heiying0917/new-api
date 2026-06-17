package model

import (
	"errors"
	"sync"
)

const (
	SettlementStatusApplied   = 1
	SettlementStatusSettled   = 2
	SettlementStatusCancelled = 3
)

// settlementCreateLocks 每供应商一把进程内互斥锁，串行化同一供应商的结算发起，
// 与 DB 层 settlement_id=0 去重一起，杜绝并发重复打包。（多节点见方案 P3 的 Redis 锁）
var settlementCreateLocks sync.Map

func lockSupplierSettlement(supplierId int) func() {
	m, _ := settlementCreateLocks.LoadOrStore(supplierId, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

type Settlement struct {
	Id             int     `json:"id"`
	SupplierId     int     `json:"supplier_id" gorm:"index"`
	Status         int     `json:"status" gorm:"index;default:1"`
	PeriodStart    int64   `json:"period_start"`
	PeriodEnd      int64   `json:"period_end"`
	OfficialUsd    float64 `json:"official_usd"`
	ComputedCNY    float64 `json:"computed_cny"`
	ActualAmount   float64 `json:"actual_amount"`
	ActualCurrency string  `json:"actual_currency"`
	SettleMethod   string  `json:"settle_method"`
	Remark         string  `json:"remark"`
	Source         string  `json:"source" gorm:"default:'manual'"`
	LogCount       int64   `json:"log_count"`
	CreatedAt      int64   `json:"created_at" gorm:"autoCreateTime"`
	SettledAt      int64   `json:"settled_at"`

	// transient: 供应商用户名（admin 列表回填，不入库）
	SupplierName string `json:"supplier_name" gorm:"-"`
}

// CreateSettlement 把供应商所有未结算消费日志原子打包成一张待审核账单。
// 注意：账单建在 DB，日志打包用 LOG_DB（同库时等价；异库为补偿式，count==0 时删除账单）。
func CreateSettlement(supplierId int, source string, now int64) (*Settlement, error) {
	if supplierId <= 0 {
		return nil, errors.New("invalid supplier id")
	}
	// 串行化同一供应商的并发发起，防重复打包
	defer lockSupplierSettlement(supplierId)()
	// 1. 取供应商渠道 id（成交价已逐条冻结在日志的 cost_price_snapshot，结算不再活取现价）
	var channels []*Channel
	if err := DB.Where("supplier_id = ?", supplierId).Find(&channels).Error; err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, errors.New("supplier has no channels")
	}
	channelIds := make([]int, 0, len(channels))
	for _, ch := range channels {
		channelIds = append(channelIds, ch.Id)
	}
	// 2. 建账单占位
	if source == "" {
		source = "manual"
	}
	s := &Settlement{SupplierId: supplierId, Status: SettlementStatusApplied, Source: source, PeriodEnd: now}
	if err := DB.Create(s).Error; err != nil {
		return nil, err
	}
	// 3. 原子打包未结算日志
	res := LOG_DB.Model(&Log{}).
		Where("type = ? AND settlement_id = 0 AND channel_id IN ?", LogTypeConsume, channelIds).
		Update("settlement_id", s.Id)
	if res.Error != nil {
		DB.Delete(&Settlement{}, s.Id)
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		DB.Delete(&Settlement{}, s.Id)
		return nil, errors.New("no consumption to settle")
	}
	// 4. 统计：总 officialUsd / count / minCreatedAt；按渠道算 computedCNY
	var officialUsd float64
	var logCount int64
	var minCreated int64
	{
		LOG_DB.Model(&Log{}).Where("settlement_id = ?", s.Id).
			Select("COALESCE(SUM(official_usd),0)").Scan(&officialUsd)
		LOG_DB.Model(&Log{}).Where("settlement_id = ?", s.Id).
			Count(&logCount)
		LOG_DB.Model(&Log{}).Where("settlement_id = ?", s.Id).
			Select("COALESCE(MIN(created_at),0)").Scan(&minCreated)
	}
	// 应收款 = Σ(每条 official_usd × 该条冻结的成交价 cost_price_snapshot)，
	// 按条累加：免疫供应商事后改价套现，渠道被删也不影响。
	var computedCNY float64
	LOG_DB.Model(&Log{}).Where("settlement_id = ?", s.Id).
		Select("COALESCE(SUM(official_usd * cost_price_snapshot),0)").Scan(&computedCNY)
	// 5. 回填
	s.OfficialUsd = officialUsd
	s.ComputedCNY = computedCNY
	s.LogCount = logCount
	s.PeriodStart = minCreated
	if err := DB.Save(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// CancelSettlement 撤销待审核账单(status=1→3)，释放其日志(settlement_id 归0)。
// 状态转移用条件原子 UPDATE，防与确认/重复撤销竞态。
func CancelSettlement(settlementId int, supplierId int, operatorIsAdmin bool) error {
	var s Settlement
	if err := DB.First(&s, settlementId).Error; err != nil {
		return err
	}
	if !operatorIsAdmin && s.SupplierId != supplierId {
		return errors.New("forbidden: not your settlement")
	}
	// 原子：仅当仍为 applied 才置为 cancelled（RowsAffected==1 才算本次成功）
	res := DB.Model(&Settlement{}).
		Where("id = ? AND status = ?", settlementId, SettlementStatusApplied).
		Update("status", SettlementStatusCancelled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("only applied settlement can be cancelled")
	}
	// 释放日志（settlement_id 归 0）
	if err := LOG_DB.Model(&Log{}).Where("settlement_id = ?", settlementId).Update("settlement_id", 0).Error; err != nil {
		return err
	}
	return nil
}

// ConfirmSettlement 超管确认结算(status=1→2)。
// 用条件原子 UPDATE + RowsAffected 检查，杜绝两个超管并发确认导致的重复打款(TOCTOU)。
func ConfirmSettlement(settlementId int, actualAmount float64, currency, method, remark string, now int64) error {
	if currency != "CNY" && currency != "USD" {
		return errors.New("currency must be CNY or USD")
	}
	res := DB.Model(&Settlement{}).
		Where("id = ? AND status = ?", settlementId, SettlementStatusApplied).
		Updates(map[string]interface{}{
			"status":          SettlementStatusSettled,
			"actual_amount":   actualAmount,
			"actual_currency": currency,
			"settle_method":   method,
			"remark":          remark,
			"settled_at":      now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		var cnt int64
		DB.Model(&Settlement{}).Where("id = ?", settlementId).Count(&cnt)
		if cnt == 0 {
			return errors.New("settlement not found")
		}
		return errors.New("结算状态已变更（可能已确认或撤销），请刷新重试")
	}
	return nil
}

// GetSettlementsBySupplier 分页列出某供应商的账单，可选按状态(status!=0)与申请时间段
// (created_at，startTs/endTs>0 时生效)过滤。
func GetSettlementsBySupplier(supplierId, status int, startTs, endTs int64, startIdx, num int) ([]*Settlement, int64, error) {
	var list []*Settlement
	var total int64
	q := DB.Model(&Settlement{}).Where("supplier_id = ?", supplierId)
	if status != 0 {
		q = q.Where("status = ?", status)
	}
	if startTs > 0 {
		q = q.Where("created_at >= ?", startTs)
	}
	if endTs > 0 {
		q = q.Where("created_at <= ?", endTs)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id desc").Limit(num).Offset(startIdx).Find(&list).Error
	return list, total, err
}

// ListSettlements 分页列出账单，可选按状态(status!=0)与供应商集合(supplierIds 非空)过滤。
// supplierIds 为 nil 或空 → 不按供应商过滤（"全部"）；非空 → WHERE supplier_id IN (?)。
func ListSettlements(status int, supplierIds []int, startIdx, num int) ([]*Settlement, int64, error) {
	var list []*Settlement
	var total int64
	q := DB.Model(&Settlement{})
	if status != 0 {
		q = q.Where("status = ?", status)
	}
	if len(supplierIds) > 0 {
		q = q.Where("supplier_id IN ?", supplierIds)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id desc").Limit(num).Offset(startIdx).Find(&list).Error
	return list, total, err
}

func GetSettlementById(id int) (*Settlement, error) {
	var s Settlement
	err := DB.First(&s, id).Error
	return &s, err
}

func GetSettlementLogs(settlementId, startIdx, num int) ([]*Log, int64, error) {
	var logs []*Log
	var total int64
	q := LOG_DB.Model(&Log{}).Where("settlement_id = ?", settlementId)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, total, err
}
