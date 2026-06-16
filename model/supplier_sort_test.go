package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// resetSupplierSortTables 复用包级内存 DB，确保排序测试涉及的表存在并清空。
func resetSupplierSortTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Supplier{}, &Channel{}, &Settlement{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
}

// seedSupplierWithPending 建 1 个 role=5 用户 + Supplier 记录 + 1 渠道 + 未结算日志，
// 使该供应商的待结算应付(PayableCNY) == payable（snapshot=1.0 → payable==official_usd）。
func seedSupplierWithPending(t *testing.T, supplierId int, payable float64) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       supplierId,
		Username: "sup_" + itoa(supplierId),
		Role:     common.RoleSupplierUser,
		Status:   common.UserStatusEnabled,
		AffCode:  randAff(supplierId),
	}).Error)
	_, err := CreateSupplierProfile(supplierId)
	require.NoError(t, err)

	channelId := supplierId*10 + 1
	cp := 1.0
	require.NoError(t, DB.Create(&Channel{
		Id: channelId, Name: "ch", Key: randAff(channelId), SupplierId: supplierId,
		CostPrice: &cp, Models: "m", Group: "g",
	}).Error)
	if payable > 0 {
		require.NoError(t, LOG_DB.Create(&Log{
			Type: LogTypeConsume, ChannelId: channelId,
			OfficialUsd: payable, CostPriceSnapshot: 1.0, SettlementId: 0,
		}).Error)
	}
}

func randAff(n int) string {
	return "aff_" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestGetAllSuppliersSortByPending(t *testing.T) {
	resetSupplierSortTables(t)
	// 造三个供应商,pending 分别 30/12/0
	seedSupplierWithPending(t, 901, 30)
	seedSupplierWithPending(t, 902, 12)
	seedSupplierWithPending(t, 903, 0)

	items, total, err := GetAllSuppliers(0, 100, "pending_cny", "desc")
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, items, 3)
	require.Equal(t, 901, items[0].UserId)
	require.Equal(t, 902, items[1].UserId)
	require.Equal(t, 903, items[2].UserId)
	require.InDelta(t, 30.0, items[0].PendingCNY, 1e-9)
	require.InDelta(t, 30.0, items[0].PendingUsd, 1e-9)

	itemsAsc, _, err := GetAllSuppliers(0, 100, "pending_cny", "asc")
	require.NoError(t, err)
	require.Equal(t, 903, itemsAsc[0].UserId)
	require.Equal(t, 902, itemsAsc[1].UserId)
	require.Equal(t, 901, itemsAsc[2].UserId)
}

func TestGetAllSuppliersSortByPendingUsd(t *testing.T) {
	resetSupplierSortTables(t)
	seedSupplierWithPending(t, 901, 7)
	seedSupplierWithPending(t, 902, 3)
	seedSupplierWithPending(t, 903, 11)

	items, _, err := GetAllSuppliers(0, 100, "pending_usd", "desc")
	require.NoError(t, err)
	require.Equal(t, 903, items[0].UserId) // 11
	require.Equal(t, 901, items[1].UserId) // 7
	require.Equal(t, 902, items[2].UserId) // 3
}

func TestGetAllSuppliersSortBySettled(t *testing.T) {
	resetSupplierSortTables(t)
	seedSupplierWithPending(t, 901, 0)
	seedSupplierWithPending(t, 902, 0)
	// 901 已结算 25 + 5 = 30; 902 已结算 10。
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, ComputedCNY: 25}).Error)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, ComputedCNY: 5}).Error)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 902, Status: SettlementStatusSettled, ComputedCNY: 10}).Error)

	items, _, err := GetAllSuppliers(0, 100, "settled_cny", "desc")
	require.NoError(t, err)
	require.Equal(t, 901, items[0].UserId)
	require.InDelta(t, 30.0, items[0].SettledCNY, 1e-9)
	require.Equal(t, 902, items[1].UserId)
	require.InDelta(t, 10.0, items[1].SettledCNY, 1e-9)
}

func TestGetAllSuppliersComputedSortPaginates(t *testing.T) {
	resetSupplierSortTables(t)
	seedSupplierWithPending(t, 901, 30)
	seedSupplierWithPending(t, 902, 20)
	seedSupplierWithPending(t, 903, 10)

	// 第二页（每页 1 条，偏移 1）→ 应取排序后的第 2 名 902。
	items, total, err := GetAllSuppliers(1, 1, "pending_cny", "desc")
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, items, 1)
	require.Equal(t, 902, items[0].UserId)
}

func TestGetAllSuppliersSortByPriority(t *testing.T) {
	resetSupplierSortTables(t)
	seedSupplierWithPending(t, 901, 0)
	seedSupplierWithPending(t, 902, 0)
	seedSupplierWithPending(t, 903, 0)
	require.NoError(t, UpdateSupplier(901, map[string]interface{}{"priority": 5}))
	require.NoError(t, UpdateSupplier(902, map[string]interface{}{"priority": 9}))
	require.NoError(t, UpdateSupplier(903, map[string]interface{}{"priority": 1}))

	items, _, err := GetAllSuppliers(0, 100, "priority", "desc")
	require.NoError(t, err)
	require.Equal(t, 902, items[0].UserId) // priority 9
	require.Equal(t, 901, items[1].UserId) // priority 5
	require.Equal(t, 903, items[2].UserId) // priority 1
}
