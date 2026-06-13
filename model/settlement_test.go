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
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 0.10, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 2, OfficialUsd: 0.20, SettlementId: 0, CreatedAt: 200}).Error)
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
	applied, atotal, _ := ListSettlements(SettlementStatusApplied, 0, 20)
	require.Equal(t, int64(1), atotal)
	require.Len(t, applied, 1)
	logs, ltotal, _ := GetSettlementLogs(s.Id, 0, 20)
	require.Equal(t, int64(2), ltotal)
	require.Len(t, logs, 2)
}
