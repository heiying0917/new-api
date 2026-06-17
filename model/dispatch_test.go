package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestDispatchEffectivePriority_BackwardCompat(t *testing.T) {
	p := int64(5)
	ch := &Channel{Priority: &p, SupplierPriority: 0}
	require.Equal(t, int64(5), dispatchEffectivePriority(ch, "priority"))
}

func TestDispatchEffectivePriority_SupplierDominates(t *testing.T) {
	p1, p2 := int64(100), int64(1)
	a := &Channel{Priority: &p1, SupplierPriority: 1}
	b := &Channel{Priority: &p2, SupplierPriority: 2}
	require.Greater(t, dispatchEffectivePriority(b, "priority"), dispatchEffectivePriority(a, "priority"))
}

func TestDispatchEffectivePriority_Bidding(t *testing.T) {
	c1, c2 := 2.5, 2.0
	a := &Channel{CostPrice: &c1}
	b := &Channel{CostPrice: &c2}
	require.Greater(t, dispatchEffectivePriority(b, "bidding"), dispatchEffectivePriority(a, "bidding"))
}

func TestDispatchEffectivePriority_BiddingNilCostLast(t *testing.T) {
	c := 2.0
	withCost := &Channel{CostPrice: &c}
	noCost := &Channel{CostPrice: nil}
	require.Greater(t, dispatchEffectivePriority(withCost, "bidding"), dispatchEffectivePriority(noCost, "bidding"))
}

func TestGetRandomSatisfiedChannel_SupplierPriorityTiers(t *testing.T) {
	// migrate + clean channels/abilities/suppliers
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)

	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = prev }()

	// ensure OptionMap is initialised (needed by GetDispatchStrategy)
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"

	// strategy = priority (default). Two suppliers: high(prio 2) and low(prio 1).
	require.NoError(t, DB.Create(&Supplier{UserId: 1, Priority: 2, Enabled: true}).Error)
	require.NoError(t, DB.Create(&Supplier{UserId: 2, Priority: 1, Enabled: true}).Error)
	p0 := int64(0)
	// channel 10 owned by high-priority supplier 1; channel 20 by supplier 2. same group/model, same channel priority.
	require.NoError(t, DB.Create(&Channel{Id: 10, Name: "hi", Key: "k1", SupplierId: 1, Priority: &p0, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 20, Name: "lo", Key: "k2", SupplierId: 2, Priority: &p0, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "g", Model: "m", ChannelId: 10, Enabled: true, Priority: &p0}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "g", Model: "m", ChannelId: 20, Enabled: true, Priority: &p0}).Error)

	InitChannelCache()

	// retry 0 → highest tier → supplier 1's channel (10)
	ch, err := GetRandomSatisfiedChannel("g", "m", 0)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, 10, ch.Id)

	// disabled supplier's channels excluded: disable supplier 1 → only channel 20 remains
	require.NoError(t, DB.Model(&Supplier{}).Where("user_id = ?", 1).Update("enabled", false).Error)
	InitChannelCache()
	ch2, err := GetRandomSatisfiedChannel("g", "m", 0)
	require.NoError(t, err)
	require.NotNil(t, ch2)
	require.Equal(t, 20, ch2.Id)
}

// TestGetRandomSatisfiedChannel_BiddingCheapestFirst：bidding 策略下，同分组+模型按成本价低者优先调度，
// 且供应商优先级被忽略（贵渠道即便供应商优先级高，也排在便宜渠道之后）；无成本价渠道兜底排最后。
func TestGetRandomSatisfiedChannel_BiddingCheapestFirst(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)

	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = prev }()

	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	prevStrategy := common.OptionMap["DispatchStrategy"]
	common.OptionMap["DispatchStrategy"] = "bidding"
	defer func() { common.OptionMap["DispatchStrategy"] = prevStrategy }()

	// 贵渠道 10 属高优先级供应商(prio 9)；便宜渠道 20 属普通供应商(prio 0)；无价渠道 30(admin)。
	require.NoError(t, DB.Create(&Supplier{UserId: 1, Priority: 9, Enabled: true}).Error)
	require.NoError(t, DB.Create(&Supplier{UserId: 2, Priority: 0, Enabled: true}).Error)
	p0 := int64(0)
	expensive := 3.0
	cheap := 1.5
	require.NoError(t, DB.Create(&Channel{Id: 10, Name: "expensive", Key: "k1", SupplierId: 1, Priority: &p0, CostPrice: &expensive, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 20, Name: "cheap", Key: "k2", SupplierId: 2, Priority: &p0, CostPrice: &cheap, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 30, Name: "nocost", Key: "k3", SupplierId: 0, Priority: &p0, CostPrice: nil, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "g", Model: "m", ChannelId: 10, Enabled: true, Priority: &p0}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "g", Model: "m", ChannelId: 20, Enabled: true, Priority: &p0}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "g", Model: "m", ChannelId: 30, Enabled: true, Priority: &p0}).Error)

	InitChannelCache()

	// retry 0 → 最便宜 → channel 20（尽管 channel 10 的供应商优先级 9 更高，bidding 下被忽略）。
	ch, err := GetRandomSatisfiedChannel("g", "m", 0)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, 20, ch.Id, "bidding: 最便宜(¥1.5)的渠道必须最先被选，供应商优先级被忽略")

	// retry 1 → 次便宜 → channel 10（¥3.0）。
	ch1, err := GetRandomSatisfiedChannel("g", "m", 1)
	require.NoError(t, err)
	require.NotNil(t, ch1)
	require.Equal(t, 10, ch1.Id, "bidding: 第二梯队是次便宜的有价渠道")

	// retry 2 → 无成本价渠道兜底 → channel 30。
	ch2, err := GetRandomSatisfiedChannel("g", "m", 2)
	require.NoError(t, err)
	require.NotNil(t, ch2)
	require.Equal(t, 30, ch2.Id, "bidding: 无成本价渠道作为最后兜底")
}

// repopulate transient supplier fields (SupplierPriority, SupplierEnabled) from the DB
// so the cached channel retains the correct tier after an incremental update.
func TestCacheUpdateChannel_PreservesSupplierFields(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)

	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = prev }()

	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"

	// Create a supplier with priority=3 and a channel owned by it.
	require.NoError(t, DB.Create(&Supplier{UserId: 3, Priority: 3, Enabled: true}).Error)
	p := int64(0)
	require.NoError(t, DB.Create(&Channel{Id: 50, Name: "x", Key: "k", SupplierId: 3, Priority: &p, Models: "m2", Group: "g2", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "g2", Model: "m2", ChannelId: 50, Enabled: true, Priority: &p}).Error)

	InitChannelCache()

	// Confirm InitChannelCache populated the transient fields correctly.
	cached, err := CacheGetChannel(50)
	require.NoError(t, err)
	require.Equal(t, 3, cached.SupplierPriority, "InitChannelCache should set SupplierPriority=3")
	require.True(t, cached.SupplierEnabled, "InitChannelCache should set SupplierEnabled=true")

	// Simulate an incremental update: fetch a fresh channel object from DB (transient fields are zero).
	fresh, err := GetChannelById(50, true)
	require.NoError(t, err)
	require.Equal(t, 0, fresh.SupplierPriority, "freshly loaded channel has zero SupplierPriority (transient field not persisted)")

	// CacheUpdateChannel must re-enrich from supplier before storing.
	CacheUpdateChannel(fresh)

	updated, err := CacheGetChannel(50)
	require.NoError(t, err)
	require.Equal(t, 3, updated.SupplierPriority, "CacheUpdateChannel must restore SupplierPriority from supplier DB")
	require.True(t, updated.SupplierEnabled, "CacheUpdateChannel must restore SupplierEnabled from supplier DB")
}

