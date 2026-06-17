package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSupplierPendingStat(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Supplier{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
	cp1, cp2 := 2.5, 2.0
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "a", Key: "k1", SupplierId: 7, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "b", Key: "k2", SupplierId: 7, CostPrice: &cp2, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 3, Name: "c", Key: "k3", SupplierId: 9, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	// consume logs: ch1 official 0.10 (×2.5=0.25), ch2 official 0.20 (×2.0=0.40); ch3 belongs to other supplier
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 0.10, CostPriceSnapshot: cp1, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 2, OfficialUsd: 0.20, CostPriceSnapshot: cp2, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 0.05, CostPriceSnapshot: cp1, SettlementId: 5}).Error) // already settled, excluded
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 3, OfficialUsd: 1.00, CostPriceSnapshot: cp1, SettlementId: 0}).Error) // other supplier

	stat, err := GetSupplierPendingStat(7)
	require.NoError(t, err)
	require.InDelta(t, 0.30, stat.OfficialUsd, 1e-9)     // 0.10+0.20
	require.InDelta(t, 0.25+0.40, stat.PayableCNY, 1e-9) // 0.10*2.5 + 0.20*2.0
	require.Equal(t, int64(2), stat.LogCount)
}

// resetSettlementTables 复用包级内存 DB，确保结算相关表存在并清空。
func resetSettlementTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Supplier{}, &Log{}, &Settlement{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
}

// item4+5：GetSettlementsBySupplier 支持按状态(0=全部)与时间段(created_at，0=不限)过滤。
func TestGetSettlementsBySupplier_StatusAndTimeFilter(t *testing.T) {
	resetSettlementTables(t)
	sup := 7
	// supplier 7 三笔：已结算(t=100)、已取消(t=200)、已申请(t=300)；外加他人一笔。
	require.NoError(t, DB.Create(&Settlement{Id: 1, SupplierId: sup, Status: SettlementStatusSettled, CreatedAt: 100}).Error)
	require.NoError(t, DB.Create(&Settlement{Id: 2, SupplierId: sup, Status: SettlementStatusCancelled, CreatedAt: 200}).Error)
	require.NoError(t, DB.Create(&Settlement{Id: 3, SupplierId: sup, Status: SettlementStatusApplied, CreatedAt: 300}).Error)
	require.NoError(t, DB.Create(&Settlement{Id: 4, SupplierId: 99, Status: SettlementStatusSettled, CreatedAt: 150}).Error)

	// 全部(status=0, 时间不限) → 仅 supplier 7 的 3 笔
	list, total, err := GetSettlementsBySupplier(sup, 0, 0, 0, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, list, 3)

	// 仅已结算 → id=1
	list, total, err = GetSettlementsBySupplier(sup, SettlementStatusSettled, 0, 0, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 1, list[0].Id)

	// 仅已取消 → 1 笔
	_, total, err = GetSettlementsBySupplier(sup, SettlementStatusCancelled, 0, 0, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// 时间段 [150,250] → 仅 id=2(t=200)
	list, total, err = GetSettlementsBySupplier(sup, 0, 150, 250, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 2, list[0].Id)

	// 状态+时间段组合：已申请 且 t>=250 → id=3
	list, total, err = GetSettlementsBySupplier(sup, SettlementStatusApplied, 250, 0, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 3, list[0].Id)
}

// TestGetSettlementChannelBreakdown 验证按渠道明细聚合正确：每渠道一行，
// requests/tokens/official_usd 正确，回填 channel_name + cost_price，
// receivable = official × cost，并按 official 降序排列。
func TestGetSettlementChannelBreakdown(t *testing.T) {
	resetSettlementTables(t)
	cp1, cp2 := 2.0, 3.0
	require.NoError(t, DB.Create(&Channel{Id: 11, Name: "chan-A", Key: "k1", SupplierId: 7, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 12, Name: "chan-B", Key: "k2", SupplierId: 7, CostPrice: &cp2, Models: "m", Group: "g"}).Error)

	// ch11: 2 条, official 0.10+0.20=0.30, tokens (3+7)+(11+13)=34
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 11, OfficialUsd: 0.10, CostPriceSnapshot: cp1, PromptTokens: 3, CompletionTokens: 7, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 11, OfficialUsd: 0.20, CostPriceSnapshot: cp1, PromptTokens: 11, CompletionTokens: 13, SettlementId: 0}).Error)
	// ch12: 1 条, official 0.50, tokens 5+5=10  (official 更高 → 排第一)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 12, OfficialUsd: 0.50, CostPriceSnapshot: cp2, PromptTokens: 5, CompletionTokens: 5, SettlementId: 0}).Error)

	s, err := CreateSettlement(7, "manual", 1000)
	require.NoError(t, err)

	rows, err := GetSettlementChannelBreakdown(s.Id)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// 排序 by official desc → ch12 (0.50) first, ch11 (0.30) second
	require.Equal(t, 12, rows[0].ChannelId)
	require.Equal(t, "chan-B", rows[0].ChannelName)
	require.Equal(t, int64(1), rows[0].Requests)
	require.Equal(t, int64(10), rows[0].Tokens)
	require.InDelta(t, 0.50, rows[0].OfficialUsd, 1e-9)
	require.InDelta(t, 3.0, rows[0].CostPrice, 1e-9)
	require.InDelta(t, 0.50*3.0, rows[0].Receivable, 1e-9)

	require.Equal(t, 11, rows[1].ChannelId)
	require.Equal(t, "chan-A", rows[1].ChannelName)
	require.Equal(t, int64(2), rows[1].Requests)
	require.Equal(t, int64(34), rows[1].Tokens)
	require.InDelta(t, 0.30, rows[1].OfficialUsd, 1e-9)
	require.InDelta(t, 2.0, rows[1].CostPrice, 1e-9)
	require.InDelta(t, 0.30*2.0, rows[1].Receivable, 1e-9)
}

// TestGetSettlementChannelBreakdown_Empty 无日志的结算 → 空切片，不报错。
func TestGetSettlementChannelBreakdown_Empty(t *testing.T) {
	resetSettlementTables(t)
	rows, err := GetSettlementChannelBreakdown(99999)
	require.NoError(t, err)
	require.Empty(t, rows)
}

// TestSettlementSnapshotRoundTrip 严格验证 req-3 核心保证：
// 申请结算清空当前计费 / 取消结算恢复。
// 全程真正调用 CreateSettlement + CancelSettlement（不手工置 settlement_id）。
func TestSettlementSnapshotRoundTrip(t *testing.T) {
	resetSettlementTables(t)
	cp1, cp2 := 2.0, 3.0
	require.NoError(t, DB.Create(&Channel{Id: 21, Name: "chan-A", Key: "k1", SupplierId: 7, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 22, Name: "chan-B", Key: "k2", SupplierId: 7, CostPrice: &cp2, Models: "m", Group: "g"}).Error)

	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 21, OfficialUsd: 0.10, CostPriceSnapshot: cp1, PromptTokens: 3, CompletionTokens: 7, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 21, OfficialUsd: 0.20, CostPriceSnapshot: cp1, PromptTokens: 1, CompletionTokens: 2, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 22, OfficialUsd: 0.50, CostPriceSnapshot: cp2, PromptTokens: 5, CompletionTokens: 5, SettlementId: 0}).Error)

	// --- BEFORE: 待结算非零 ---
	before, err := GetSupplierPendingStat(7)
	require.NoError(t, err)
	wantOfficial := 0.10 + 0.20 + 0.50            // 0.80
	wantPayable := (0.10+0.20)*2.0 + 0.50*3.0     // 0.6 + 1.5 = 2.1
	require.InDelta(t, wantOfficial, before.OfficialUsd, 1e-9)
	require.InDelta(t, wantPayable, before.PayableCNY, 1e-9)
	require.Equal(t, int64(3), before.LogCount)
	require.Greater(t, before.OfficialUsd, 0.0)
	require.Greater(t, before.PayableCNY, 0.0)

	// --- APPLY: 申请结算后待结算清零，明细反映这些日志 ---
	s, err := CreateSettlement(7, "manual", 1000)
	require.NoError(t, err)

	afterApply, err := GetSupplierPendingStat(7)
	require.NoError(t, err)
	require.InDelta(t, 0.0, afterApply.OfficialUsd, 1e-9, "申请后官方价应清零")
	require.InDelta(t, 0.0, afterApply.PayableCNY, 1e-9, "申请后应付应清零")
	require.Equal(t, int64(0), afterApply.LogCount, "申请后未结算条数应为0")

	rows, err := GetSettlementChannelBreakdown(s.Id)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	var sumOfficial, sumReceivable float64
	var sumReq int64
	for _, r := range rows {
		sumOfficial += r.OfficialUsd
		sumReceivable += r.Receivable
		sumReq += r.Requests
	}
	require.InDelta(t, wantOfficial, sumOfficial, 1e-9, "明细官方价之和应等于申请前待结算")
	require.InDelta(t, wantPayable, sumReceivable, 1e-9, "明细应收之和应等于申请前应付")
	require.Equal(t, int64(3), sumReq)

	// --- CANCEL: 取消结算后待结算恢复，明细清空 ---
	require.NoError(t, CancelSettlement(s.Id, 7, true))

	afterCancel, err := GetSupplierPendingStat(7)
	require.NoError(t, err)
	require.InDelta(t, before.OfficialUsd, afterCancel.OfficialUsd, 1e-9, "取消后官方价应恢复")
	require.InDelta(t, before.PayableCNY, afterCancel.PayableCNY, 1e-9, "取消后应付应恢复")
	require.Equal(t, before.LogCount, afterCancel.LogCount, "取消后未结算条数应恢复")

	emptyRows, err := GetSettlementChannelBreakdown(s.Id)
	require.NoError(t, err)
	require.Empty(t, emptyRows, "取消后该结算明细应为空（日志已释放）")
}
