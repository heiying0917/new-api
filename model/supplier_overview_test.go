package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// resetSupplierOverviewTables 复用包级内存 DB，确保概览测试涉及的表存在并清空。
// 含 Log：GetSupplierOverview 会按渠道聚合累计已跑金额($)（V12），需 logs 表存在。
func resetSupplierOverviewTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Supplier{}, &Channel{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
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

// V11 item3：每个 type 需返回该类目下的供应商名单（user_id + name），按 user_id 升序、最多 5 条。
func TestGetSupplierOverviewPerTypeSupplierBriefs(t *testing.T) {
	resetSupplierOverviewTables(t)

	seedOverviewSupplierUser(t, 901, true)
	seedOverviewSupplierUser(t, 902, true)

	// type 14: 供应商 901、902；type 1: 仅供应商 901。
	seedOverviewChannel(t, 1, 14, 901, common.ChannelStatusEnabled, "g", 2.0)
	seedOverviewChannel(t, 2, 14, 902, common.ChannelStatusEnabled, "g", 2.5)
	seedOverviewChannel(t, 3, 1, 901, common.ChannelStatusEnabled, "g", 1.8)

	ov, err := GetSupplierOverview()
	require.NoError(t, err)

	byType := map[int]SupplierTypeStat{}
	for _, ts := range ov.ByType {
		byType[ts.Type] = ts
	}

	// type 14：两个供应商，按 user_id 升序。
	t14 := byType[14]
	require.Len(t, t14.Suppliers, 2)
	require.Equal(t, 901, t14.Suppliers[0].UserId)
	require.Equal(t, "sup_901", t14.Suppliers[0].Name)
	require.Equal(t, 902, t14.Suppliers[1].UserId)
	require.Equal(t, "sup_902", t14.Suppliers[1].Name)

	// type 1：仅一个供应商。
	t1 := byType[1]
	require.Len(t, t1.Suppliers, 1)
	require.Equal(t, 901, t1.Suppliers[0].UserId)
	require.Equal(t, "sup_901", t1.Suppliers[0].Name)
}

// V11 item3：供应商名单最多 5 条，但 SupplierCount 仍是真实总数。
func TestGetSupplierOverviewSupplierListCappedAt5(t *testing.T) {
	resetSupplierOverviewTables(t)

	// 6 个供应商，各一条 type=1 启用渠道。
	for i := 0; i < 6; i++ {
		id := 920 + i
		seedOverviewSupplierUser(t, id, true)
		seedOverviewChannel(t, 700+i, 1, id, common.ChannelStatusEnabled, "g", float64(i+1))
	}

	ov, err := GetSupplierOverview()
	require.NoError(t, err)

	byType := map[int]SupplierTypeStat{}
	for _, ts := range ov.ByType {
		byType[ts.Type] = ts
	}

	t1 := byType[1]
	require.Equal(t, 6, t1.SupplierCount) // 总数不受截断影响
	require.Len(t, t1.Suppliers, 5)       // 列表截断到 5
}

// V12：每个 type 返回该类目下的渠道明细(channels)——每行=一个渠道，含供应商名/分组/
// 成本价(¥)/累计已跑金额($)，按成本价升序、未定价(<=0)沉底；详情展示全部不截断。
func TestGetSupplierOverviewPerTypeChannelsV12(t *testing.T) {
	resetSupplierOverviewTables(t)

	seedOverviewSupplierUser(t, 901, true)
	seedOverviewSupplierUser(t, 902, true)

	// type 14:
	//   ch1 sup901 group "claude" cost 2.0
	//   ch2 sup902 group "claude" cost 1.5  (最便宜 -> 第一)
	//   ch3 sup901 group "vip"    cost 0    (未定价 -> 沉底)
	seedOverviewChannel(t, 1, 14, 901, common.ChannelStatusEnabled, "claude", 2.0)
	seedOverviewChannel(t, 2, 14, 902, common.ChannelStatusEnabled, "claude", 1.5)
	seedOverviewChannel(t, 3, 14, 901, common.ChannelStatusEnabled, "vip", 0)

	// 累计已跑金额($)：ch1=6+4(含已结算)=10，ch2=3，ch3=1。
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 6, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 4, SettlementId: 99, CreatedAt: 110}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 2, OfficialUsd: 3, SettlementId: 0, CreatedAt: 120}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 3, OfficialUsd: 1, SettlementId: 0, CreatedAt: 130}).Error)

	ov, err := GetSupplierOverview()
	require.NoError(t, err)
	byType := map[int]SupplierTypeStat{}
	for _, ts := range ov.ByType {
		byType[ts.Type] = ts
	}

	t14 := byType[14]
	require.Len(t, t14.Channels, 3)

	// 排序：ch2(1.5) -> ch1(2.0) -> ch3(未定价沉底)。
	require.Equal(t, 2, t14.Channels[0].ChannelId)
	require.Equal(t, 902, t14.Channels[0].SupplierId)
	require.Equal(t, "sup_902", t14.Channels[0].SupplierName)
	require.Equal(t, "claude", t14.Channels[0].Group)
	require.InDelta(t, 1.5, t14.Channels[0].CostPrice, 1e-9)
	require.InDelta(t, 3.0, t14.Channels[0].OfficialUsd, 1e-9)

	require.Equal(t, 1, t14.Channels[1].ChannelId)
	require.InDelta(t, 2.0, t14.Channels[1].CostPrice, 1e-9)
	require.InDelta(t, 10.0, t14.Channels[1].OfficialUsd, 1e-9) // 6+4 累计含已结算

	require.Equal(t, 3, t14.Channels[2].ChannelId) // 未定价沉底
	require.InDelta(t, 0.0, t14.Channels[2].CostPrice, 1e-9)
	require.Equal(t, "vip", t14.Channels[2].Group)
	require.InDelta(t, 1.0, t14.Channels[2].OfficialUsd, 1e-9)
}
