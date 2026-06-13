package model

import (
	"errors"
)

const (
	SettlementStatusApplied   = 1
	SettlementStatusSettled   = 2
	SettlementStatusCancelled = 3
)

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
}

// CreateSettlement 把供应商所有未结算消费日志原子打包成一张待审核账单。
// 注意：账单建在 DB，日志打包用 LOG_DB（同库时等价；异库为补偿式，count==0 时删除账单）。
func CreateSettlement(supplierId int, source string, now int64) (*Settlement, error) {
	if supplierId <= 0 {
		return nil, errors.New("invalid supplier id")
	}
	// 1. 取供应商渠道 id + cost_price
	var channels []*Channel
	if err := DB.Where("supplier_id = ?", supplierId).Find(&channels).Error; err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, errors.New("supplier has no channels")
	}
	channelIds := make([]int, 0, len(channels))
	costById := map[int]float64{}
	for _, ch := range channels {
		channelIds = append(channelIds, ch.Id)
		cp := 0.0
		if ch.CostPrice != nil {
			cp = *ch.CostPrice
		}
		costById[ch.Id] = cp
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
	var computedCNY float64
	for _, chId := range channelIds {
		var sum float64
		LOG_DB.Model(&Log{}).Where("settlement_id = ? AND channel_id = ?", s.Id, chId).
			Select("COALESCE(SUM(official_usd),0)").Scan(&sum)
		computedCNY += sum * costById[chId]
	}
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
func CancelSettlement(settlementId int, supplierId int, operatorIsAdmin bool) error {
	var s Settlement
	if err := DB.First(&s, settlementId).Error; err != nil {
		return err
	}
	if !operatorIsAdmin && s.SupplierId != supplierId {
		return errors.New("forbidden: not your settlement")
	}
	if s.Status != SettlementStatusApplied {
		return errors.New("only applied settlement can be cancelled")
	}
	if err := LOG_DB.Model(&Log{}).Where("settlement_id = ?", settlementId).Update("settlement_id", 0).Error; err != nil {
		return err
	}
	return DB.Model(&Settlement{}).Where("id = ?", settlementId).Update("status", SettlementStatusCancelled).Error
}

// ConfirmSettlement 超管确认结算(status=1→2)。
func ConfirmSettlement(settlementId int, actualAmount float64, currency, method, remark string, now int64) error {
	var s Settlement
	if err := DB.First(&s, settlementId).Error; err != nil {
		return err
	}
	if s.Status != SettlementStatusApplied {
		return errors.New("only applied settlement can be confirmed")
	}
	if currency != "CNY" && currency != "USD" {
		return errors.New("currency must be CNY or USD")
	}
	return DB.Model(&Settlement{}).Where("id = ?", settlementId).Updates(map[string]interface{}{
		"status":          SettlementStatusSettled,
		"actual_amount":   actualAmount,
		"actual_currency": currency,
		"settle_method":   method,
		"remark":          remark,
		"settled_at":      now,
	}).Error
}

func GetSettlementsBySupplier(supplierId, startIdx, num int) ([]*Settlement, int64, error) {
	var list []*Settlement
	var total int64
	q := DB.Model(&Settlement{}).Where("supplier_id = ?", supplierId)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id desc").Limit(num).Offset(startIdx).Find(&list).Error
	return list, total, err
}

func ListSettlements(status, startIdx, num int) ([]*Settlement, int64, error) {
	var list []*Settlement
	var total int64
	q := DB.Model(&Settlement{})
	if status != 0 {
		q = q.Where("status = ?", status)
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
