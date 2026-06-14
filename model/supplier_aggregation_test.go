package model

import (
	"sort"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// ─── req5/6b: GetUsernamesByIds ────────────────────────────────────────────────

func TestGetUsernamesByIds(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	u1 := &User{Username: "alice", Password: "pwd12345", AffCode: "aff_alice", Role: common.RoleSupplierUser}
	u2 := &User{Username: "bob", Password: "pwd12345", AffCode: "aff_bob", Role: common.RoleSupplierUser}
	require.NoError(t, DB.Create(u1).Error)
	require.NoError(t, DB.Create(u2).Error)

	m, err := GetUsernamesByIds([]int{u1.Id, u2.Id, 999999})
	require.NoError(t, err)
	require.Equal(t, "alice", m[u1.Id])
	require.Equal(t, "bob", m[u2.Id])
	_, ok := m[999999]
	require.False(t, ok, "non-existent id must be absent")

	// Empty input short-circuits => non-nil empty map, no IN ().
	empty, err := GetUsernamesByIds(nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Len(t, empty, 0)
}

// ─── req6a: querySuppliers PendingCNY / SettledCNY ─────────────────────────────

func TestQuerySuppliers_PendingAndSettled(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}, &Supplier{}, &Channel{}, &Settlement{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	// Supplier user.
	u := &User{Username: "sup_agg", Password: "pwd12345", Role: common.RoleSupplierUser}
	require.NoError(t, DB.Create(u).Error)
	require.NoError(t, DB.Create(&Supplier{UserId: u.Id, Enabled: true, SettlementMode: "manual", SettlementCycle: "month"}).Error)

	// Channels for the supplier with cost prices.
	cp1, cp2 := 2.5, 2.0
	require.NoError(t, DB.Create(&Channel{Id: 5001, Name: "a", Key: "k1", SupplierId: u.Id, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 5002, Name: "b", Key: "k2", SupplierId: u.Id, CostPrice: &cp2, Models: "m", Group: "g"}).Error)

	// Unsettled consume logs => contribute to PendingCNY.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 5001, OfficialUsd: 0.10, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 5002, OfficialUsd: 0.20, SettlementId: 0, CreatedAt: 200}).Error)

	// Settled settlements => contribute to SettledCNY (sum of computed_cny on settled).
	require.NoError(t, DB.Create(&Settlement{SupplierId: u.Id, Status: SettlementStatusSettled, ComputedCNY: 12.5, SettledAt: time.Now().Unix()}).Error)
	require.NoError(t, DB.Create(&Settlement{SupplierId: u.Id, Status: SettlementStatusSettled, ComputedCNY: 7.5, SettledAt: time.Now().Unix() - 100000}).Error)
	// Applied (not settled) excluded.
	require.NoError(t, DB.Create(&Settlement{SupplierId: u.Id, Status: SettlementStatusApplied, ComputedCNY: 100}).Error)

	items, total, err := GetAllSuppliers(0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)

	it := items[0]
	require.Equal(t, u.Id, it.UserId)

	// Cross-check against the canonical aggregation functions.
	pending, err := GetSupplierPendingStat(u.Id)
	require.NoError(t, err)
	require.InDelta(t, pending.PayableCNY, it.PendingCNY, 1e-9)
	require.InDelta(t, 0.10*2.5+0.20*2.0, it.PendingCNY, 1e-9)

	settled, err := GetSupplierSettledStats(u.Id, time.Now().Unix())
	require.NoError(t, err)
	require.InDelta(t, settled.Total, it.SettledCNY, 1e-9)
	require.InDelta(t, 20.0, it.SettledCNY, 1e-9)
}

// ─── req6b: ListSettlements supplierIds + status filter ────────────────────────

func TestListSettlements_SupplierFilter(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Settlement{}))
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)

	// Supplier 7: one applied, one settled.
	require.NoError(t, DB.Create(&Settlement{SupplierId: 7, Status: SettlementStatusApplied, ComputedCNY: 1}).Error)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 7, Status: SettlementStatusSettled, ComputedCNY: 2}).Error)
	// Supplier 9: one applied.
	require.NoError(t, DB.Create(&Settlement{SupplierId: 9, Status: SettlementStatusApplied, ComputedCNY: 3}).Error)

	// single-supplier set filter only.
	list, total, err := ListSettlements(0, []int{7}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	for _, s := range list {
		require.Equal(t, 7, s.SupplierId)
	}

	// supplierIds={9} only.
	list9, total9, err := ListSettlements(0, []int{9}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total9)
	require.Len(t, list9, 1)
	require.Equal(t, 9, list9[0].SupplierId)

	// multi-supplier set => IN (7,9) returns all 3.
	multi, mtotal, err := ListSettlements(0, []int{7, 9}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(3), mtotal)
	require.Len(t, multi, 3)

	// status filter still works (no supplier filter): applied across all suppliers.
	applied, atotal, err := ListSettlements(SettlementStatusApplied, nil, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), atotal)
	require.Len(t, applied, 2)
	for _, s := range applied {
		require.Equal(t, SettlementStatusApplied, s.Status)
	}

	// status + supplierIds combined: supplier 7 settled only.
	combined, ctotal, err := ListSettlements(SettlementStatusSettled, []int{7}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), ctotal)
	require.Len(t, combined, 1)
	require.Equal(t, 7, combined[0].SupplierId)
	require.Equal(t, SettlementStatusSettled, combined[0].Status)

	// nil/empty supplierIds => no supplier filter => all 3.
	all, alltotal, err := ListSettlements(0, nil, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(3), alltotal)
	require.Len(t, all, 3)
	empty, etotal, err := ListSettlements(0, []int{}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(3), etotal)
	require.Len(t, empty, 3)
	supplierIds := make([]int, 0, len(all))
	for _, s := range all {
		supplierIds = append(supplierIds, s.SupplierId)
	}
	sort.Ints(supplierIds)
	require.Equal(t, []int{7, 7, 9}, supplierIds)
}

// ─── admin settlement list: keyword fuzzy-match by supplier name/email ──────────

func TestGetSupplierIdsByKeyword(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	sup1 := &User{Username: "acme_supplier", Email: "ops@acme.io", Password: "pwd12345", AffCode: "aff_acme", Role: common.RoleSupplierUser}
	sup2 := &User{Username: "globex_vendor", Email: "billing@globex.com", Password: "pwd12345", AffCode: "aff_globex", Role: common.RoleSupplierUser}
	// A non-supplier whose username/email also contains "acme" must be excluded.
	normal := &User{Username: "acme_customer", Email: "user@acme.io", Password: "pwd12345", AffCode: "aff_cust", Role: common.RoleCommonUser}
	require.NoError(t, DB.Create(sup1).Error)
	require.NoError(t, DB.Create(sup2).Error)
	require.NoError(t, DB.Create(normal).Error)

	// Match by username substring.
	ids, err := GetSupplierIdsByKeyword("acme")
	require.NoError(t, err)
	require.Equal(t, []int{sup1.Id}, ids, "username match excludes the non-supplier with same substring")

	// Match by email substring.
	ids2, err := GetSupplierIdsByKeyword("globex.com")
	require.NoError(t, err)
	require.Equal(t, []int{sup2.Id}, ids2)

	// No match => empty (non-nil) slice.
	none, err := GetSupplierIdsByKeyword("nonexistent_zzz")
	require.NoError(t, err)
	require.Len(t, none, 0)

	// Blank / whitespace keyword => nil (no-filter sentinel).
	blank, err := GetSupplierIdsByKeyword("   ")
	require.NoError(t, err)
	require.Nil(t, blank)
	empty, err := GetSupplierIdsByKeyword("")
	require.NoError(t, err)
	require.Nil(t, empty)
}
