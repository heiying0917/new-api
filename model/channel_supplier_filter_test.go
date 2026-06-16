package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// resetSupplierFilterTables 复用包级内存 DB，确保供应商名过滤测试涉及的表存在并清空。
func resetSupplierFilterTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Channel{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
}

func TestResolveSupplierIdsByName(t *testing.T) {
	resetSupplierFilterTables(t)

	// 两个供应商(role=5)，其一带 email beta@x.com
	require.NoError(t, DB.Create(&User{
		Id: 701, Username: "alpha_supplier", Role: common.RoleSupplierUser,
		Status: common.UserStatusEnabled, AffCode: "aff_701",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id: 702, Username: "beta_supplier", Email: "beta@x.com", Role: common.RoleSupplierUser,
		Status: common.UserStatusEnabled, AffCode: "aff_702",
	}).Error)
	// 一个普通用户(role=1)，username 也含 supplier，但不应被匹配
	require.NoError(t, DB.Create(&User{
		Id: 703, Username: "normaluser_supplier", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, AffCode: "aff_703",
	}).Error)

	// "supplier" 命中两个供应商，不含普通用户
	ids, err := ResolveSupplierIdsByName("supplier")
	require.NoError(t, err)
	require.ElementsMatch(t, []int{701, 702}, ids)

	// 无匹配返回空
	none, err := ResolveSupplierIdsByName("nomatch_xyz")
	require.NoError(t, err)
	require.Empty(t, none)

	// 空 keyword 返回 nil（不查库）
	empty, err := ResolveSupplierIdsByName("   ")
	require.NoError(t, err)
	require.Nil(t, empty)

	// 邮箱模糊匹配
	byEmail, err := ResolveSupplierIdsByName("beta@x.com")
	require.NoError(t, err)
	require.ElementsMatch(t, []int{702}, byEmail)
}

// TestSearchChannelsSupplierFilter 验证 model.SearchChannels 的 supplierIds 过滤只返回匹配渠道。
func TestSearchChannelsSupplierFilter(t *testing.T) {
	resetSupplierFilterTables(t)

	// 两个供应商各一条渠道
	require.NoError(t, DB.Create(&User{
		Id: 711, Username: "sup_one", Role: common.RoleSupplierUser,
		Status: common.UserStatusEnabled, AffCode: "aff_711",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id: 712, Username: "sup_two", Role: common.RoleSupplierUser,
		Status: common.UserStatusEnabled, AffCode: "aff_712",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 7111, Name: "chan_one", Key: "k7111", SupplierId: 711, Models: "gpt-4", Group: "default",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 7121, Name: "chan_two", Key: "k7121", SupplierId: 712, Models: "gpt-4", Group: "default",
	}).Error)

	// 不带 supplier 过滤（nil）→ 两条都命中（keyword=chan 匹配 name）
	all, err := SearchChannels("chan", "", "", false, nil)
	require.NoError(t, err)
	require.Len(t, all, 2)

	// 仅过滤供应商 711 → 只返回其渠道
	only, err := SearchChannels("chan", "", "", false, []int{711})
	require.NoError(t, err)
	require.Len(t, only, 1)
	require.Equal(t, 7111, only[0].Id)
	require.Equal(t, 711, only[0].SupplierId)
}
