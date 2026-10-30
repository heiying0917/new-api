package service

import (
	"context"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func newConcurrencyTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func concurrencyTestChannel(id int, limit int) *model.Channel {
	return &model.Channel{Id: id, MaxConcurrency: &limit}
}

func resetMemChannelInflight(t *testing.T) {
	t.Helper()
	memChannelInflightMu.Lock()
	memChannelInflight = map[int]int{}
	memChannelInflightMu.Unlock()
}

func TestTryAcquireChannelSlot_MemoryEnforcesLimit(t *testing.T) {
	resetMemChannelInflight(t)
	ch := concurrencyTestChannel(101, 2)

	c1, c2, c3 := newConcurrencyTestContext(), newConcurrencyTestContext(), newConcurrencyTestContext()
	require.True(t, TryAcquireChannelSlot(c1, ch))
	require.True(t, TryAcquireChannelSlot(c2, ch))
	require.False(t, TryAcquireChannelSlot(c3, ch), "第 3 个在途请求超过上限 2")
	require.False(t, HoldsChannelSlot(c3, ch.Id))
	require.Equal(t, 2, GetChannelConcurrencyCounts([]int{ch.Id})[ch.Id])

	ReleaseChannelSlot(c1)
	require.Equal(t, 1, GetChannelConcurrencyCounts([]int{ch.Id})[ch.Id])
	require.True(t, TryAcquireChannelSlot(c3, ch), "释放后空出位置")

	ReleaseChannelSlot(c2)
	ReleaseChannelSlot(c3)
	require.Empty(t, GetChannelConcurrencyCounts([]int{ch.Id}))
}

func TestTryAcquireChannelSlot_UnlimitedStillCounted(t *testing.T) {
	resetMemChannelInflight(t)
	ch := concurrencyTestChannel(102, 0)
	ctxs := make([]*gin.Context, 0, 50)
	for i := 0; i < 50; i++ {
		c := newConcurrencyTestContext()
		require.True(t, TryAcquireChannelSlot(c, ch))
		ctxs = append(ctxs, c)
	}
	require.Equal(t, 50, GetChannelConcurrencyCounts([]int{ch.Id})[ch.Id], "不限并发也要计数，供列表展示")
	for _, c := range ctxs {
		ReleaseChannelSlot(c)
	}
	require.Empty(t, GetChannelConcurrencyCounts([]int{ch.Id}))
}

func TestReleaseChannelSlot_IdempotentAndSwitching(t *testing.T) {
	resetMemChannelInflight(t)
	a, b := concurrencyTestChannel(103, 1), concurrencyTestChannel(104, 1)
	c := newConcurrencyTestContext()

	ReleaseChannelSlot(c) // 未持有时为空操作
	require.True(t, TryAcquireChannelSlot(c, a))
	require.True(t, HoldsChannelSlot(c, a.Id))

	// 同一请求换渠道：自动放掉 a 的占位
	require.True(t, TryAcquireChannelSlot(c, b))
	require.False(t, HoldsChannelSlot(c, a.Id))
	require.True(t, HoldsChannelSlot(c, b.Id))
	counts := GetChannelConcurrencyCounts([]int{a.Id, b.Id})
	require.Equal(t, 0, counts[a.Id])
	require.Equal(t, 1, counts[b.Id])

	// 重试回到同一个满载上限为 1 的渠道：不能被自己占着的位挡住
	require.True(t, TryAcquireChannelSlot(c, b))
	require.Equal(t, 1, GetChannelConcurrencyCounts([]int{b.Id})[b.Id])

	ReleaseChannelSlot(c)
	ReleaseChannelSlot(c)
	require.Empty(t, GetChannelConcurrencyCounts([]int{a.Id, b.Id}), "重复释放不能把计数减成负数或误扣他人")
}

// seedConcurrencySelectChannels 10 档：渠道 1（上限 1）、2（上限 1）；0 档：渠道 3（不限）。
func seedConcurrencySelectChannels(t *testing.T, memoryCache bool) {
	t.Helper()
	model.InitColNames() // DB 选路路径的原生 SQL 依赖 group 列名转义
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Supplier{}))
	require.NoError(t, model.DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM abilities").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM abilities")
	})
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"

	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = memoryCache
	t.Cleanup(func() { common.MemoryCacheEnabled = prev })

	hi, lo := int64(10), int64(0)
	one, unlimited := 1, 0
	for _, ch := range []struct {
		id       int
		priority *int64
		limit    *int
	}{{1, &hi, &one}, {2, &hi, &one}, {3, &lo, &unlimited}} {
		require.NoError(t, model.DB.Create(&model.Channel{Id: ch.id, Name: "c" + strconv.Itoa(ch.id), Key: "k", Priority: ch.priority, MaxConcurrency: ch.limit, Models: "m", Group: "g", Status: common.ChannelStatusEnabled}).Error)
		require.NoError(t, model.DB.Create(&model.Ability{Group: "g", Model: "m", ChannelId: ch.id, Enabled: true, Priority: ch.priority}).Error)
	}
	model.InitChannelCache()
}

func selectForTest(t *testing.T) (*gin.Context, *model.Channel, error) {
	t.Helper()
	c := newConcurrencyTestContext()
	ch, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: "g", ModelName: "m", Retry: common.GetPointer(0)})
	return c, ch, err
}

func runConcurrencyOverflowScenario(t *testing.T) {
	resetMemChannelInflight(t)

	// 前两个请求占满 10 档的两个渠道（各上限 1）
	c1, ch1, err := selectForTest(t)
	require.NoError(t, err)
	require.NotNil(t, ch1)
	c2, ch2, err := selectForTest(t)
	require.NoError(t, err)
	require.NotNil(t, ch2)
	require.ElementsMatch(t, []int{1, 2}, []int{ch1.Id, ch2.Id}, "同档还有空位时不能降档，也不能挤进已满渠道")
	require.True(t, HoldsChannelSlot(c1, ch1.Id))

	// 10 档全满：降到 0 档的不限渠道
	c3, ch3, err := selectForTest(t)
	require.NoError(t, err)
	require.NotNil(t, ch3)
	require.Equal(t, 3, ch3.Id)

	// 释放渠道 1 后又能选回 10 档
	freedId := ch1.Id
	ReleaseChannelSlot(c1)
	c4, ch4, err := selectForTest(t)
	require.NoError(t, err)
	require.Equal(t, freedId, ch4.Id)

	for _, c := range []*gin.Context{c2, c3, c4} {
		ReleaseChannelSlot(c)
	}
}

func TestCacheGetRandomSatisfiedChannel_OverflowsFullChannels_MemoryCache(t *testing.T) {
	seedConcurrencySelectChannels(t, true)
	runConcurrencyOverflowScenario(t)
}

func TestCacheGetRandomSatisfiedChannel_OverflowsFullChannels_Database(t *testing.T) {
	seedConcurrencySelectChannels(t, false)
	runConcurrencyOverflowScenario(t)
}

func TestCacheGetRandomSatisfiedChannel_AllFullReturnsLimited(t *testing.T) {
	seedConcurrencySelectChannels(t, true)
	resetMemChannelInflight(t)
	// 把不限渠道 3 改成上限 1，三个渠道全部可被占满
	one := 1
	require.NoError(t, (&model.Channel{Id: 3, MaxConcurrency: &one}).Update())
	model.InitChannelCache()

	held := make([]*gin.Context, 0, 3)
	for i := 0; i < 3; i++ {
		c, ch, err := selectForTest(t)
		require.NoError(t, err)
		require.NotNil(t, ch)
		held = append(held, c)
	}
	c, ch, err := selectForTest(t)
	require.ErrorIs(t, err, ErrChannelConcurrencyLimited)
	require.Nil(t, ch)
	require.False(t, HoldsChannelSlot(c, 1) || HoldsChannelSlot(c, 2) || HoldsChannelSlot(c, 3))

	// 分组下根本没有该模型：仍是「无可用渠道」(nil, nil)，不能误报为并发已满
	c5 := newConcurrencyTestContext()
	ch, _, err = CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c5, TokenGroup: "g", ModelName: "no-such-model", Retry: common.GetPointer(0)})
	require.NoError(t, err)
	require.Nil(t, ch)

	for _, h := range held {
		ReleaseChannelSlot(h)
	}
}

// TestChannelConcurrency_Redis 真 Redis 集成测试：CHANNEL_CONCURRENCY_TEST_REDIS=redis://host:6379/15 时运行。
// 会清空所用 DB 中 channel_concurrency:* 的测试键，请勿指向生产库。
func TestChannelConcurrency_Redis(t *testing.T) {
	url := os.Getenv("CHANNEL_CONCURRENCY_TEST_REDIS")
	if url == "" {
		t.Skip("set CHANNEL_CONCURRENCY_TEST_REDIS to run against a real Redis")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	require.NoError(t, rdb.Ping(ctx).Err())

	prevRDB, prevEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = rdb, true
	t.Cleanup(func() {
		common.RDB, common.RedisEnabled = prevRDB, prevEnabled
		_ = rdb.Close()
	})

	const channelId = 990001
	key := channelConcurrencyKey(channelId)
	require.NoError(t, rdb.Del(ctx, key).Err())
	t.Cleanup(func() { rdb.Del(context.Background(), key) })

	ch := concurrencyTestChannel(channelId, 2)
	c1, c2, c3 := newConcurrencyTestContext(), newConcurrencyTestContext(), newConcurrencyTestContext()
	require.True(t, TryAcquireChannelSlot(c1, ch))
	require.True(t, TryAcquireChannelSlot(c2, ch))
	require.False(t, TryAcquireChannelSlot(c3, ch))
	require.Equal(t, 2, GetChannelConcurrencyCounts([]int{channelId})[channelId])

	ttl, err := rdb.PTTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0), "key 必须带过期时间，防止永久残留")

	// 崩溃遗留的过期占位：不计数，且下一次占位时被清理
	expired := float64(time.Now().Add(-time.Second).UnixMilli())
	require.NoError(t, rdb.ZAdd(ctx, key, &redis.Z{Score: expired, Member: "crashed-pod-slot"}).Err())
	require.Equal(t, 2, GetChannelConcurrencyCounts([]int{channelId})[channelId])
	ReleaseChannelSlot(c1)
	require.True(t, TryAcquireChannelSlot(c3, ch), "过期占位不能占着名额")
	_, err = rdb.ZScore(ctx, key, "crashed-pod-slot").Result()
	require.ErrorIs(t, err, redis.Nil, "占位脚本应清掉过期 member")

	// 续期：延长仍持有的占位；已释放的不会被续期复活
	slot2 := heldChannelSlot(c2)
	require.NotNil(t, slot2)
	before, err := rdb.ZScore(ctx, key, slot2.member).Result()
	require.NoError(t, err)
	ReleaseChannelSlot(c3)
	heldRedisSlots.Store("released-member", key) // 模拟续期快照与释放竞态：member 已不在 ZSET 中
	time.Sleep(5 * time.Millisecond)
	renewHeldChannelSlots()
	heldRedisSlots.Delete("released-member")
	after, err := rdb.ZScore(ctx, key, slot2.member).Result()
	require.NoError(t, err)
	require.Greater(t, after, before)
	_, err = rdb.ZScore(ctx, key, "released-member").Result()
	require.ErrorIs(t, err, redis.Nil, "ZADD XX 不能复活已释放的占位")

	ReleaseChannelSlot(c2)
	require.Empty(t, GetChannelConcurrencyCounts([]int{channelId}))

	// Redis 不可用：fail-open 放行
	broken := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 100 * time.Millisecond})
	common.RDB = broken
	cf := newConcurrencyTestContext()
	require.True(t, TryAcquireChannelSlot(cf, ch))
	v, ok := common.GetContextKey(cf, constant.ContextKeyChannelConcurrencySlot)
	require.True(t, ok)
	require.Empty(t, v.(*channelSlot).redisKey, "fail-open 的占位不跟踪，释放时无需访问 Redis")
	ReleaseChannelSlot(cf)
	_ = broken.Close()
	common.RDB = rdb
}

// 并发争抢下上限必须严格成立：N 个请求同时占位，恰好 limit 个成功（Redis 用同一 env 开关，未设置时测内存计数）。
func TestTryAcquireChannelSlot_AtomicUnderContention(t *testing.T) {
	if url := os.Getenv("CHANNEL_CONCURRENCY_TEST_REDIS"); url != "" {
		opt, err := redis.ParseURL(url)
		require.NoError(t, err)
		rdb := redis.NewClient(opt)
		prevRDB, prevEnabled := common.RDB, common.RedisEnabled
		common.RDB, common.RedisEnabled = rdb, true
		t.Cleanup(func() {
			rdb.Del(context.Background(), channelConcurrencyKey(990002))
			common.RDB, common.RedisEnabled = prevRDB, prevEnabled
			_ = rdb.Close()
		})
		require.NoError(t, rdb.Del(context.Background(), channelConcurrencyKey(990002)).Err())
	} else {
		resetMemChannelInflight(t)
	}

	const limit, contenders = 5, 64
	ch := concurrencyTestChannel(990002, limit)
	ctxs := make([]*gin.Context, contenders)
	results := make(chan bool, contenders)
	start := make(chan struct{})
	for i := 0; i < contenders; i++ {
		ctxs[i] = newConcurrencyTestContext()
		go func(c *gin.Context) {
			<-start
			results <- TryAcquireChannelSlot(c, ch)
		}(ctxs[i])
	}
	close(start)
	won := 0
	for i := 0; i < contenders; i++ {
		if <-results {
			won++
		}
	}
	require.Equal(t, limit, won)
	require.Equal(t, limit, GetChannelConcurrencyCounts([]int{ch.Id})[ch.Id])
	for _, c := range ctxs {
		ReleaseChannelSlot(c)
	}
	require.Empty(t, GetChannelConcurrencyCounts([]int{ch.Id}))
}

// 首次撞满后一次性排除所有已满渠道：组内大量渠道满载时不逐个试占（每次试占在 Redis 下是一次往返）。
func TestExcludeSaturatedChannels_MarksAllFullChannelsAtOnce(t *testing.T) {
	for _, memoryCache := range []bool{true, false} {
		seedConcurrencySelectChannels(t, memoryCache)
		resetMemChannelInflight(t)
		one := 1
		h1, h2 := newConcurrencyTestContext(), newConcurrencyTestContext()
		require.True(t, TryAcquireChannelSlot(h1, &model.Channel{Id: 1, MaxConcurrency: &one}))
		require.True(t, TryAcquireChannelSlot(h2, &model.Channel{Id: 2, MaxConcurrency: &one}))

		excluded := map[int]bool{}
		excludeSaturatedChannels("g", "m", excluded)
		require.Equal(t, map[int]bool{1: true, 2: true}, excluded, "memoryCache=%v：已满的 1、2 一次排除，不限的 3 保留", memoryCache)

		ReleaseChannelSlot(h1)
		excluded = map[int]bool{}
		excludeSaturatedChannels("g", "m", excluded)
		require.Equal(t, map[int]bool{2: true}, excluded, "memoryCache=%v：释放后的渠道不再被排除", memoryCache)
		ReleaseChannelSlot(h2)
	}
}

// auto 分组：当前分组渠道全满时溢出到下一个 auto 分组；所有分组都满才返回并发已满。
func TestCacheGetRandomSatisfiedChannel_AutoGroupOverflow(t *testing.T) {
	model.InitColNames()
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Supplier{}))
	require.NoError(t, model.DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM abilities").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM abilities")
	})
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"
	prevCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = prevCache })
	prevAuto := setting.AutoGroups2JsonString()
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))
	t.Cleanup(func() { _ = setting.UpdateAutoGroupsByJsonString(prevAuto) })

	p0, one := int64(0), 1
	for _, ch := range []struct {
		id    int
		group string
	}{{11, "default"}, {12, "vip"}} {
		require.NoError(t, model.DB.Create(&model.Channel{Id: ch.id, Name: "c", Key: "k", Priority: &p0, MaxConcurrency: &one, Models: "m", Group: ch.group, Status: common.ChannelStatusEnabled}).Error)
		require.NoError(t, model.DB.Create(&model.Ability{Group: ch.group, Model: "m", ChannelId: ch.id, Enabled: true, Priority: &p0}).Error)
	}
	model.InitChannelCache()
	resetMemChannelInflight(t)

	selectAuto := func() (*gin.Context, *model.Channel, string, error) {
		c := newConcurrencyTestContext()
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		ch, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "m", Retry: common.GetPointer(0)})
		return c, ch, group, err
	}

	c1, ch1, g1, err := selectAuto()
	require.NoError(t, err)
	require.Equal(t, 11, ch1.Id)
	require.Equal(t, "default", g1)

	c2, ch2, g2, err := selectAuto()
	require.NoError(t, err)
	require.Equal(t, 12, ch2.Id, "default 分组满载后溢出到 vip")
	require.Equal(t, "vip", g2)

	_, ch3, _, err := selectAuto()
	require.ErrorIs(t, err, ErrChannelConcurrencyLimited)
	require.Nil(t, ch3)

	ReleaseChannelSlot(c1)
	ReleaseChannelSlot(c2)
}

// 优雅停机：进程退出前归还本进程持有的全部 Redis 占位，不必等租约到期（真 Redis，同一 env 开关）。
func TestReleaseAllHeldChannelSlots_Redis(t *testing.T) {
	url := os.Getenv("CHANNEL_CONCURRENCY_TEST_REDIS")
	if url == "" {
		t.Skip("set CHANNEL_CONCURRENCY_TEST_REDIS to run against a real Redis")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	prevRDB, prevEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = rdb, true
	ids := []int{990003, 990004}
	t.Cleanup(func() {
		for _, id := range ids {
			rdb.Del(context.Background(), channelConcurrencyKey(id))
		}
		common.RDB, common.RedisEnabled = prevRDB, prevEnabled
		_ = rdb.Close()
	})

	ctxs := []*gin.Context{newConcurrencyTestContext(), newConcurrencyTestContext(), newConcurrencyTestContext()}
	require.True(t, TryAcquireChannelSlot(ctxs[0], concurrencyTestChannel(ids[0], 0)))
	require.True(t, TryAcquireChannelSlot(ctxs[1], concurrencyTestChannel(ids[0], 0)))
	require.True(t, TryAcquireChannelSlot(ctxs[2], concurrencyTestChannel(ids[1], 0)))
	require.Len(t, GetChannelConcurrencyCounts(ids), 2)

	ReleaseAllHeldChannelSlots()
	require.Empty(t, GetChannelConcurrencyCounts(ids))
	for _, c := range ctxs {
		ReleaseChannelSlot(c) // 停机后请求收尾再释放：幂等，不出错
	}
	require.Empty(t, GetChannelConcurrencyCounts(ids))
}
