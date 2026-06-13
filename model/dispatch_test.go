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

// TestCacheUpdateChannel_PreservesSupplierFields verifies Fix A: CacheUpdateChannel must
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

