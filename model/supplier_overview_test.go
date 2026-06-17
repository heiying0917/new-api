package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// resetSupplierOverviewTables 复用包级内存 DB，确保概览测试涉及的表存在并清空。
func resetSupplierOverviewTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Supplier{}, &Channel{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
}

// seedOverviewSupplierUser 建一个 role=supplier 用户 + Supplier 记录（enabled 可控）。
func seedOverviewSupplierUser(t *testing.T, supplierId int, enabled bool) {
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
	require.NoError(t, UpdateSupplier(supplierId, map[string]interface{}{"enabled": enabled}))
}

// seedOverviewChannel 建一条供应商渠道。
func seedOverviewChannel(t *testing.T, channelId, typ, supplierId, status int, group string, cost float64) {
	t.Helper()
	cp := cost
	require.NoError(t, DB.Create(&Channel{
		Id: channelId, Name: "ch_" + itoa(channelId), Key: randAff(channelId),
		Type: typ, SupplierId: supplierId, Status: status,
		CostPrice: &cp, Models: "m", Group: group,
	}).Error)
}

func TestGetSupplierOverview(t *testing.T) {
	resetSupplierOverviewTables(t)

	// 两个供应商用户（901 enabled，902 disabled）。
	seedOverviewSupplierUser(t, 901, true)
	seedOverviewSupplierUser(t, 902, false)

	// Type 14 (Anthropic), group claude-official:
	//   supplier 901: 一个 enabled cost 2.0，一个 disabled cost 2.2
	//   supplier 902: 一个 enabled cost 2.5
	seedOverviewChannel(t, 1, 14, 901, common.ChannelStatusEnabled, "claude-official", 2.0)
	seedOverviewChannel(t, 2, 14, 901, common.ChannelStatusManuallyDisabled, "claude-official", 2.2)
	seedOverviewChannel(t, 3, 14, 902, common.ChannelStatusEnabled, "claude-official", 2.5)

	// Type 1 (OpenAI), group gpt-official:
	//   supplier 901: 一个 enabled cost 1.8
	seedOverviewChannel(t, 4, 1, 901, common.ChannelStatusEnabled, "gpt-official", 1.8)

	ov, err := GetSupplierOverview()
	require.NoError(t, err)

	// 汇总。
	require.Equal(t, int64(2), ov.Summary.SupplierTotal)
	require.Equal(t, int64(1), ov.Summary.SupplierEnabled)
	require.Equal(t, int64(4), ov.Summary.ChannelTotal)
	require.Equal(t, int64(3), ov.Summary.ChannelAvailable)
	require.Equal(t, int64(1), ov.Summary.ChannelUnavailable)

	// 仅含有供应商渠道的 type（14 + 1），按 ChannelCount 降序：14(3) 在前，1(1) 在后。
	require.Len(t, ov.ByType, 2)

	byType := map[int]SupplierTypeStat{}
	for _, ts := range ov.ByType {
		byType[ts.Type] = ts
	}

	// Type 14。
	t14, ok := byType[14]
	require.True(t, ok)
	require.Equal(t, 2, t14.SupplierCount)
	require.Equal(t, 3, t14.ChannelCount)
	require.Equal(t, 2, t14.Available)
	require.Equal(t, 1, t14.Unavailable)
	require.InDelta(t, 2.0, t14.LowestPrice, 1e-9) // 最低价仅取启用渠道：min(2.0, 2.5)=2.0
	require.Len(t, t14.Groups, 1)
	require.Equal(t, "claude-official", t14.Groups[0].Group)
	require.InDelta(t, 2.0, t14.Groups[0].LowestPrice, 1e-9)

	// Type 1。
	t1, ok := byType[1]
	require.True(t, ok)
	require.Equal(t, 1, t1.SupplierCount)
	require.Equal(t, 1, t1.ChannelCount)
	require.Equal(t, 1, t1.Available)
	require.Equal(t, 0, t1.Unavailable)
	require.InDelta(t, 1.8, t1.LowestPrice, 1e-9)
	require.Len(t, t1.Groups, 1)
	require.Equal(t, "gpt-official", t1.Groups[0].Group)

	// ByType 排序：14 (ChannelCount=3) 应排在 1 (ChannelCount=1) 之前。
	require.Equal(t, 14, ov.ByType[0].Type)
	require.Equal(t, 1, ov.ByType[1].Type)
}
