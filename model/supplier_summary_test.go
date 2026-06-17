package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetSupplierSummaryTables 复用包级内存 DB，确保 channels/settlements/logs 表存在并清空。
func resetSupplierSummaryTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Settlement{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
}

// seedSummaryChannel 建一条归属 supplierId 的渠道（cost_price 仅占位）。
func seedSummaryChannel(t *testing.T, supplierId, channelId int, costPrice float64) {
	t.Helper()
	cp := costPrice
	require.NoError(t, DB.Create(&Channel{
		Id: channelId, Name: "ch", Key: "k", SupplierId: supplierId,
		CostPrice: &cp, Models: "m", Group: "g",
	}).Error)
}

// seedSummaryLog 建一条消费日志（official_usd / cost_price_snapshot / settlement_id）。
func seedSummaryLog(t *testing.T, channelId int, officialUsd, snapshot float64, settlementId int) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		Type: LogTypeConsume, ChannelId: channelId,
		OfficialUsd: officialUsd, CostPriceSnapshot: snapshot, SettlementId: settlementId,
	}).Error)
}

func TestGetAllSuppliersPendingStat(t *testing.T) {
	resetSupplierSummaryTables(t)
	// 供应商 901 两条渠道、902 一条渠道
	seedSummaryChannel(t, 901, 7001, 2.0)
	seedSummaryChannel(t, 901, 7002, 2.0)
	seedSummaryChannel(t, 902, 7003, 3.0)
	// 未结算消费日志:official_usd, cost_price_snapshot, settlementId
	seedSummaryLog(t, 7001, 10.0, 2.0, 0)
	seedSummaryLog(t, 7002, 5.0, 2.0, 0)
	seedSummaryLog(t, 7003, 4.0, 3.0, 0)
	seedSummaryLog(t, 7003, 1.0, 3.0, 88) // 已结算(settlement_id!=0)→不计

	perSupplier, global, err := GetAllSuppliersPendingStat()
	require.NoError(t, err)

	require.InDelta(t, 15.0, perSupplier[901].OfficialUsd, 1e-9) // 10+5
	require.InDelta(t, 30.0, perSupplier[901].PayableCNY, 1e-9)  // 10*2 + 5*2
	require.Equal(t, int64(2), perSupplier[901].LogCount)

	require.InDelta(t, 4.0, perSupplier[902].OfficialUsd, 1e-9)
	require.InDelta(t, 12.0, perSupplier[902].PayableCNY, 1e-9) // 4*3
	require.Equal(t, int64(1), perSupplier[902].LogCount)

	// 全局 = 各供应商之和（已结算日志被排除）。
	require.InDelta(t, 19.0, global.OfficialUsd, 1e-9) // 15 + 4
	require.InDelta(t, 42.0, global.PayableCNY, 1e-9)  // 30 + 12
	require.Equal(t, int64(3), global.LogCount)
}

func TestGetSettlementTotalsByStatus(t *testing.T) {
	resetSupplierSummaryTables(t)
	// 已申请(1):official 20, computed 40
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusApplied, OfficialUsd: 20, ComputedCNY: 40}).Error)
	// 已结算(2):official 10, computed 25, 实付 30 CNY
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, OfficialUsd: 10, ComputedCNY: 25, ActualAmount: 30, ActualCurrency: "CNY"}).Error)
	// 已结算(2):official 5, computed 12, 实付 6 USD
	require.NoError(t, DB.Create(&Settlement{SupplierId: 902, Status: SettlementStatusSettled, OfficialUsd: 5, ComputedCNY: 12, ActualAmount: 6, ActualCurrency: "USD"}).Error)

	applied, err := GetSettlementTotalsByStatus(SettlementStatusApplied)
	require.NoError(t, err)
	require.InDelta(t, 20.0, applied.OfficialUsd, 1e-9)
	require.InDelta(t, 40.0, applied.ComputedCNY, 1e-9)
	require.Equal(t, int64(1), applied.Count)

	settled, err := GetSettlementTotalsByStatus(SettlementStatusSettled)
	require.NoError(t, err)
	require.InDelta(t, 15.0, settled.OfficialUsd, 1e-9) // 10 + 5
	require.InDelta(t, 37.0, settled.ComputedCNY, 1e-9) // 25 + 12
	require.InDelta(t, 30.0, settled.ActualCNY, 1e-9)   // CNY bucket
	require.InDelta(t, 6.0, settled.ActualUSD, 1e-9)    // USD bucket
	require.Equal(t, int64(2), settled.Count)
}

func TestGetSettlementTotalsByStatus_EmptyCurrencyGoesToCNY(t *testing.T) {
	resetSupplierSummaryTables(t)
	// 已结算但 actual_currency 为空 → 归入人民币。
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, OfficialUsd: 1, ComputedCNY: 2, ActualAmount: 9, ActualCurrency: ""}).Error)

	settled, err := GetSettlementTotalsByStatus(SettlementStatusSettled)
	require.NoError(t, err)
	require.InDelta(t, 9.0, settled.ActualCNY, 1e-9)
	require.InDelta(t, 0.0, settled.ActualUSD, 1e-9)
	require.Equal(t, int64(1), settled.Count)
}

func TestGetAllSuppliersSettledTotal(t *testing.T) {
	resetSupplierSummaryTables(t)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, ComputedCNY: 25}).Error)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, ComputedCNY: 5}).Error)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 902, Status: SettlementStatusApplied, ComputedCNY: 99}).Error) // 非已结算→不计

	m, err := GetAllSuppliersSettledTotal()
	require.NoError(t, err)
	require.InDelta(t, 30.0, m[901], 1e-9) // 25 + 5
	_, ok := m[902]
	require.False(t, ok, "902 仅有已申请单 → 不计入已结算合计")
}
