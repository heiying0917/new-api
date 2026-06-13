package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// setupSupplierTables 确保 users/suppliers 表存在并清空（复用包级 TestMain 的内存 DB）
func setupSupplierTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
}

func TestUserPhonePersisted(t *testing.T) {
	setupSupplierTables(t)
	u := &User{Username: "sup_phone", Password: "pwd12345", Phone: "13800000000", Role: common.RoleSupplierUser}
	require.NoError(t, DB.Create(u).Error)

	var got User
	require.NoError(t, DB.First(&got, "username = ?", "sup_phone").Error)
	require.Equal(t, "13800000000", got.Phone)
}

func TestSupplierCreateAndFetch(t *testing.T) {
	setupSupplierTables(t)
	s := &Supplier{
		UserId:          42,
		Priority:        3,
		Enabled:         true,
		SettlementMode:  "manual",
		SettlementCycle: "month",
		Remark:          "首批入驻",
	}
	require.NoError(t, DB.Create(s).Error)

	got, err := GetSupplierByUserId(42)
	require.NoError(t, err)
	require.Equal(t, 3, got.Priority)
	require.True(t, got.Enabled)
	require.Equal(t, "manual", got.SettlementMode)
}

func seedSupplierUser(t *testing.T, id int, username, email, phone string) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: id, Username: username, Email: email, Phone: phone,
		Role: common.RoleSupplierUser, Status: common.UserStatusEnabled,
		AffCode: fmt.Sprintf("aff%d", id),
	}).Error)
	_, err := CreateSupplierProfile(id)
	require.NoError(t, err)
}

func TestCreateSupplierProfile_Defaults(t *testing.T) {
	setupSupplierTables(t)
	require.NoError(t, DB.Create(&User{Id: 1, Username: "s1", Role: common.RoleSupplierUser}).Error)
	s, err := CreateSupplierProfile(1)
	require.NoError(t, err)
	require.Equal(t, "manual", s.SettlementMode)
	require.Equal(t, "month", s.SettlementCycle)
	require.True(t, s.Enabled)
	again, err := CreateSupplierProfile(1)
	require.NoError(t, err)
	require.Equal(t, 1, again.UserId)
}

func TestGetAllSuppliers_MergesProfile(t *testing.T) {
	setupSupplierTables(t)
	seedSupplierUser(t, 1, "alice", "a@x.com", "13800000001")
	seedSupplierUser(t, 2, "bob", "b@x.com", "13800000002")
	require.NoError(t, DB.Create(&User{Id: 3, Username: "normal", Role: common.RoleCommonUser, AffCode: "aff3"}).Error)
	require.NoError(t, UpdateSupplier(2, map[string]interface{}{"priority": 9, "remark": "VIP"}))

	items, total, err := GetAllSuppliers(0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	byId := map[int]*SupplierListItem{}
	for _, it := range items {
		byId[it.UserId] = it
	}
	require.Equal(t, "alice", byId[1].Username)
	require.Equal(t, "13800000001", byId[1].Phone)
	require.Equal(t, 9, byId[2].Priority)
	require.Equal(t, "VIP", byId[2].Remark)
}

func TestSearchSuppliers_ByKeyword(t *testing.T) {
	setupSupplierTables(t)
	seedSupplierUser(t, 1, "alice", "a@x.com", "13800000001")
	seedSupplierUser(t, 2, "bob", "b@x.com", "13800000002")

	items, total, err := SearchSuppliers("alice", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "alice", items[0].Username)
}
