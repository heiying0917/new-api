package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// seedExclusionChannels 两档优先级：10 档有渠道 1、2，0 档有渠道 3。同一 group/model。
func seedExclusionChannels(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM abilities")
	})

	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"

	hi, lo := int64(10), int64(0)
	for _, ch := range []struct {
		id       int
		priority *int64
	}{{1, &hi}, {2, &hi}, {3, &lo}} {
		require.NoError(t, DB.Create(&Channel{Id: ch.id, Name: "c", Key: "k", Priority: ch.priority, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
		require.NoError(t, DB.Create(&Ability{Group: "g", Model: "m", ChannelId: ch.id, Enabled: true, Priority: ch.priority}).Error)
	}
}

func assertExclusionSemantics(t *testing.T) {
	t.Helper()
	// 排除 1：同档只剩 2，必须选 2（不降档）
	for i := 0; i < 20; i++ {
		ch, err := GetRandomSatisfiedChannelExcluding("g", "m", 0, map[int]bool{1: true})
		require.NoError(t, err)
		require.NotNil(t, ch)
		require.Equal(t, 2, ch.Id)
	}
	// 整个 10 档被排除：retry=0 自然降到 0 档
	ch, err := GetRandomSatisfiedChannelExcluding("g", "m", 0, map[int]bool{1: true, 2: true})
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, 3, ch.Id)

	// 全部排除：返回 nil 而不是错误
	ch, err = GetRandomSatisfiedChannelExcluding("g", "m", 0, map[int]bool{1: true, 2: true, 3: true})
	require.NoError(t, err)
	require.Nil(t, ch)
	ch, err = GetRandomSatisfiedChannelExcluding("g", "m", 1, map[int]bool{1: true, 2: true, 3: true})
	require.NoError(t, err)
	require.Nil(t, ch)

	// 不排除时行为不变：retry=1 落到 0 档
	ch, err = GetRandomSatisfiedChannelExcluding("g", "m", 1, nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, 3, ch.Id)
}

func TestGetRandomSatisfiedChannelExcluding_MemoryCache(t *testing.T) {
	seedExclusionChannels(t)
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = prev }()
	InitChannelCache()

	assertExclusionSemantics(t)
}

func TestGetRandomSatisfiedChannelExcluding_Database(t *testing.T) {
	seedExclusionChannels(t)
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	defer func() { common.MemoryCacheEnabled = prev }()

	assertExclusionSemantics(t)
}

func TestChannelValidateMaxConcurrency(t *testing.T) {
	ptr := func(v int) *int { return &v }
	require.NoError(t, (&Channel{}).ValidateMaxConcurrency(), "未提交放行")
	require.NoError(t, (&Channel{MaxConcurrency: ptr(0)}).ValidateMaxConcurrency())
	require.NoError(t, (&Channel{MaxConcurrency: ptr(MaxChannelConcurrencyLimit)}).ValidateMaxConcurrency())
	require.Error(t, (&Channel{MaxConcurrency: ptr(-1)}).ValidateMaxConcurrency())
	require.Error(t, (&Channel{MaxConcurrency: ptr(MaxChannelConcurrencyLimit + 1)}).ValidateMaxConcurrency())

	require.Equal(t, 0, (&Channel{}).GetMaxConcurrency())
	require.Equal(t, 5, (&Channel{MaxConcurrency: ptr(5)}).GetMaxConcurrency())
}

// 改回 0（不限）必须能落库：Update 走 Updates(struct)，非指针 0 会被跳过。
func TestChannelUpdate_MaxConcurrencyCanResetToZero(t *testing.T) {
	seedExclusionChannels(t)
	five, zero := 5, 0
	require.NoError(t, (&Channel{Id: 1, MaxConcurrency: &five}).Update())
	got, err := GetChannelById(1, true)
	require.NoError(t, err)
	require.Equal(t, 5, got.GetMaxConcurrency())

	require.NoError(t, (&Channel{Id: 1, MaxConcurrency: &zero}).Update())
	got, err = GetChannelById(1, true)
	require.NoError(t, err)
	require.Equal(t, 0, got.GetMaxConcurrency())

	// 未提交该字段的更新不应改动已有值
	require.NoError(t, (&Channel{Id: 1, MaxConcurrency: &five}).Update())
	name := "renamed"
	require.NoError(t, (&Channel{Id: 1, Name: name}).Update())
	got, err = GetChannelById(1, true)
	require.NoError(t, err)
	require.Equal(t, 5, got.GetMaxConcurrency())
}
