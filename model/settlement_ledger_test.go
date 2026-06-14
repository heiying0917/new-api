package model

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSettlementLedger_AppendOnly 账本可追加、按时序返回、记录操作者。
func TestSettlementLedger_AppendOnly(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&SettlementLedger{}))
	require.NoError(t, DB.Exec("DELETE FROM settlement_ledgers").Error)

	RecordSettlementLedger(&SettlementLedger{SettlementId: 1, SupplierId: 7, Action: "create", OfficialUsd: 0.3, ComputedCNY: 0.65, OperatorId: 7})
	RecordSettlementLedger(&SettlementLedger{SettlementId: 1, SupplierId: 7, Action: "confirm", ActualAmount: 0.6, ActualCurrency: "CNY", OperatorId: 3, OperatorIsAdmin: true})

	rows, err := GetSettlementLedger(1)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "create", rows[0].Action)
	require.Equal(t, "confirm", rows[1].Action)
	require.Equal(t, 3, rows[1].OperatorId)
	require.True(t, rows[1].OperatorIsAdmin)
	require.InDelta(t, 0.6, rows[1].ActualAmount, 1e-9)
}

// TestSettlementSnapshotHash_Deterministic 哈希稳定且对篡改敏感。
func TestSettlementSnapshotHash_Deterministic(t *testing.T) {
	s := &Settlement{Id: 5, SupplierId: 7, Status: 2, OfficialUsd: 0.3, ComputedCNY: 0.65, ActualAmount: 0.6, ActualCurrency: "CNY", LogCount: 2}
	h1 := SettlementSnapshotHash(s)
	require.Equal(t, h1, SettlementSnapshotHash(s))
	require.Len(t, h1, 64)
	s.ActualAmount = 999 // 篡改实付额
	require.NotEqual(t, h1, SettlementSnapshotHash(s), "篡改金额后哈希必须变化")
}

// TestConfirmSettlement_NoDoublePayment 二次确认必须失败，且不覆盖已确认金额（条件原子 UPDATE）。
func TestConfirmSettlement_NoDoublePayment(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)
	require.NoError(t, ConfirmSettlement(s.Id, 0.6, "CNY", "转账", "first", 1000))
	// 第二次确认（不同金额）必须失败，且不得覆盖已确认金额/时间
	require.Error(t, ConfirmSettlement(s.Id, 999.0, "CNY", "x", "second", 2000))
	got, _ := GetSettlementById(s.Id)
	require.Equal(t, SettlementStatusSettled, got.Status)
	require.InDelta(t, 0.6, got.ActualAmount, 1e-9, "二次确认不得覆盖金额")
	require.Equal(t, int64(1000), got.SettledAt)
}

// TestConfirmSettlement_CannotConfirmCancelled 已撤销的账单不能被确认。
func TestConfirmSettlement_CannotConfirmCancelled(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, _ := CreateSettlement(7, "manual", 999)
	require.NoError(t, CancelSettlement(s.Id, 7, true))
	require.Error(t, ConfirmSettlement(s.Id, 0.6, "CNY", "x", "y", 1000), "已撤销不能确认")
}

// TestConfirmSettlement_ConcurrentOnlyOnePays 并发确认同一账单只成功一次，杜绝重复打款（TOCTOU）。
func TestConfirmSettlement_ConcurrentOnlyOnePays(t *testing.T) {
	setupSettlementTables(t)
	seedForSettlement(t)
	s, err := CreateSettlement(7, "manual", 999)
	require.NoError(t, err)

	const N = 8
	var wg sync.WaitGroup
	var okCount int32
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if err := ConfirmSettlement(s.Id, 0.6, "CNY", "转账", "ok", 1000); err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), okCount, "并发确认只能成功一次")
	got, _ := GetSettlementById(s.Id)
	require.Equal(t, SettlementStatusSettled, got.Status)
}
