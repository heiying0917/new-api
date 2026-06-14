package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func setupSettlementTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Supplier{}, &Log{}, &Settlement{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)
}

func seedForSettlement(t *testing.T) {
	cp1, cp2 := 2.5, 2.0
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "a", Key: "k1", SupplierId: 7, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "b", Key: "k2", SupplierId: 7, CostPrice: &cp2, Models: "m", Group: "g"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 0.10, CostPriceSnapshot: cp1, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 2, OfficialUsd: 0.20, CostPriceSnapshot: cp2, SettlementId: 0, CreatedAt: 200}).Error)
}

func TestCreateSettlement_PacksLogs(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	require.Equal(t, SettlementStatusApplied, s.Status)
	require.InDelta(t, 0.30, s.OfficialUsd, 1e-9)
	require.InDelta(t, 0.10*2.5+0.20*2.0, s.ComputedCNY, 1e-9)
	require.Equal(t, int64(2), s.LogCount)
	require.Equal(t, int64(100), s.PeriodStart)
	require.Equal(t, int64(999), s.PeriodEnd)
	// logs tagged
	var cnt int64
	LOG_DB.Model(&Log{}).Where("settlement_id = ?", s.Id).Count(&cnt)
	require.Equal(t, int64(2), cnt)
	// second create → nothing to settle
	_, err = CreateSettlement(7, "manual", 1000)
	require.Error(t, err)
}

func TestCancelSettlement_ReleasesLogs(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	// non-owner cannot cancel
	require.Error(t, CancelSettlement(s.Id, 8, false))
	// owner cancels
	require.NoError(t, CancelSettlement(s.Id, 7, false))
	got, _ := GetSettlementById(s.Id)
	require.Equal(t, SettlementStatusCancelled, got.Status)
	var unsettled int64
	LOG_DB.Model(&Log{}).Where("settlement_id = 0 AND type = ?", LogTypeConsume).Count(&unsettled)
	require.Equal(t, int64(2), unsettled)
	// can settle again after release
	s2, err := CreateSettlement(7, "manual", 1001)
	require.NoError(t, err)
	require.InDelta(t, 0.30, s2.OfficialUsd, 1e-9)
}

func TestConfirmSettlement(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	require.Error(t, ConfirmSettlement(s.Id, 0.6, "EUR", "x", "y", 1000)) // bad currency
	require.NoError(t, ConfirmSettlement(s.Id, 0.6, "CNY", "转账", "ok", 1000))
	got, _ := GetSettlementById(s.Id)
	require.Equal(t, SettlementStatusSettled, got.Status)
	require.InDelta(t, 0.6, got.ActualAmount, 1e-9)
	require.Equal(t, int64(1000), got.SettledAt)
	// cannot confirm again
	require.Error(t, ConfirmSettlement(s.Id, 0.6, "CNY", "x", "y", 1001))
}

func TestListAndDetail(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, _ := CreateSettlement(7, "manual", 999)
	list, total, err := GetSettlementsBySupplier(7, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	applied, atotal, _ := ListSettlements(SettlementStatusApplied, nil, 0, 20)
	require.Equal(t, int64(1), atotal)
	require.Len(t, applied, 1)
	logs, ltotal, _ := GetSettlementLogs(s.Id, 0, 20)
	require.Equal(t, int64(2), ltotal)
	require.Len(t, logs, 2)
}

// TestSettlement_AntiCashOut 核心安全回归：消费时按 2.0 冻结成交价，结算前把渠道改到 10.0，
// 待结算与结算金额都必须仍按冻结价 2.0 累加（不被改价套现）。
func TestSettlement_AntiCashOut(t *testing.T) {
	setupSettlementTables(t)
	cp := 2.0
	require.NoError(t, DB.Create(&Channel{Id: 31, Name: "c", Key: "k", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	// 两条消费，成交价逐条冻结 = 2.0
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 31, OfficialUsd: 1.00, CostPriceSnapshot: 2.0, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 31, OfficialUsd: 0.50, CostPriceSnapshot: 2.0, SettlementId: 0, CreatedAt: 200}).Error)

	// 攻击：结算前把成本价抬高到 10.0
	hi := 10.0
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 31).Update("cost_price", &hi).Error)

	// 待结算应付仍按冻结价：1.5 × 2.0 = 3.0（不是 ×10=15）
	pending, err := GetSupplierPendingStat(7)
	require.NoError(t, err)
	require.InDelta(t, 1.5, pending.OfficialUsd, 1e-9)
	require.InDelta(t, 3.0, pending.PayableCNY, 1e-9, "改价后待结算仍按冻结价 2.0")

	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	require.InDelta(t, 1.5, s.OfficialUsd, 1e-9)
	require.InDelta(t, 3.0, s.ComputedCNY, 1e-9, "结算金额按冻结价 2.0 累加，免疫改价套现")

	// 明细 receivable 也按冻结价；有效单价 = 3.0/1.5 = 2.0
	rows, err := GetSettlementChannelBreakdown(s.Id)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.InDelta(t, 3.0, rows[0].Receivable, 1e-9)
	require.InDelta(t, 2.0, rows[0].CostPrice, 1e-9, "明细有效单价=冻结价")
}

// TestSettlement_PriceChangeMidPeriod 同一渠道、期间内改价：每条按各自冻结价累加。
func TestSettlement_PriceChangeMidPeriod(t *testing.T) {
	setupSettlementTables(t)
	cp := 2.0
	require.NoError(t, DB.Create(&Channel{Id: 32, Name: "c", Key: "k", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	// 先按 2.0 服务一条，后按 4.0 服务一条（模拟期间内调价）
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 32, OfficialUsd: 1.00, CostPriceSnapshot: 2.0, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 32, OfficialUsd: 1.00, CostPriceSnapshot: 4.0, SettlementId: 0, CreatedAt: 200}).Error)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	require.InDelta(t, 1.0*2.0+1.0*4.0, s.ComputedCNY, 1e-9, "逐条按各自冻结价：2+4=6")
}

// TestSettlement_DeletedChannelBreakdown 结算后删除渠道，明细 receivable 仍按冻结快照正确（不再归零）。
func TestSettlement_DeletedChannelBreakdown(t *testing.T) {
	setupSettlementTables(t)
	cp := 3.0
	require.NoError(t, DB.Create(&Channel{Id: 41, Name: "d", Key: "k", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 41, OfficialUsd: 2.00, CostPriceSnapshot: 3.0, SettlementId: 0, CreatedAt: 100}).Error)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	require.InDelta(t, 6.0, s.ComputedCNY, 1e-9)
	// 删除渠道后，明细 receivable 仍正确（snapshot 在日志上，不依赖渠道行）
	require.NoError(t, DB.Where("id = ?", 41).Delete(&Channel{}).Error)
	rows, err := GetSettlementChannelBreakdown(s.Id)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.InDelta(t, 6.0, rows[0].Receivable, 1e-9, "渠道删除后 receivable 仍按冻结价")
}
