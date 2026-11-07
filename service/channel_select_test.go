package service

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

type selectSeed struct {
	id       int
	priority int64
	limit    int
	weight   uint
	key      string // 为空用 "k-<id>"；含换行则建为多 key 渠道
	models   string // 为空用 "m"
}

// seedSelectChannels 建一个分组 g 下的渠道集合（priority 策略），并按 memoryCache 切换选路数据源。
func seedSelectChannels(t *testing.T, memoryCache bool, seeds ...selectSeed) {
	t.Helper()
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
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = memoryCache
	t.Cleanup(func() { common.MemoryCacheEnabled = prev })

	for _, s := range seeds {
		priority, limit, weight := s.priority, s.limit, s.weight
		key := s.key
		if key == "" {
			key = "k-" + strconv.Itoa(s.id)
		}
		models := s.models
		if models == "" {
			models = "m"
		}
		ch := model.Channel{Id: s.id, Name: "c" + strconv.Itoa(s.id), Key: key, Priority: &priority, MaxConcurrency: &limit, Weight: &weight, Models: models, Group: "g", Status: common.ChannelStatusEnabled}
		if n := len(splitNonEmptyLines(key)); n > 1 {
			ch.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: n, MultiKeyMode: constant.MultiKeyModeRandom}
		}
		require.NoError(t, model.DB.Create(&ch).Error)
		for _, m := range splitComma(models) {
			require.NoError(t, model.DB.Create(&model.Ability{Group: "g", Model: m, ChannelId: s.id, Enabled: true, Priority: &priority, Weight: weight}).Error)
		}
	}
	model.InitChannelCache()
	resetMemChannelInflight(t)
	resetChannelCooldowns(t)
}

func splitNonEmptyLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func splitComma(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// holdSlots 模拟在途请求：在渠道上占 n 个位，测试结束释放。
func holdSlots(t *testing.T, channelId int, n int) {
	t.Helper()
	ch, err := model.CacheGetChannel(channelId)
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		c := newConcurrencyTestContext()
		require.True(t, TryAcquireChannelSlot(c, ch))
		t.Cleanup(func() { ReleaseChannelSlot(c) })
	}
}

// pickOnce 以重试第 retry 次、失败过 failed 的状态选一次路，立即释放占位，返回选中的渠道 id（无则 0）。
func pickOnce(t *testing.T, modelName string, retry int, failed ...int) int {
	t.Helper()
	c := newConcurrencyTestContext()
	param := &RetryParam{Ctx: c, TokenGroup: "g", ModelName: modelName, Retry: common.GetPointer(retry)}
	for _, id := range failed {
		param.MarkChannelFailed(id)
	}
	ch, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	ReleaseChannelSlot(c)
	if ch == nil {
		return 0
	}
	return ch.Id
}

func forBothSelectPaths(t *testing.T, run func(t *testing.T, memoryCache bool)) {
	for _, memoryCache := range []bool{true, false} {
		t.Run("memoryCache="+strconv.FormatBool(memoryCache), func(t *testing.T) { run(t, memoryCache) })
	}
}

// 同档内优先选在飞最少的渠道（而不是随机）。
func TestSelect_SameTierPrefersLeastInflight(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache, selectSeed{id: 1, priority: 10}, selectSeed{id: 2, priority: 10}, selectSeed{id: 3, priority: 10})
		holdSlots(t, 1, 2)
		holdSlots(t, 2, 1)
		for i := 0; i < 20; i++ {
			require.Equal(t, 3, pickOnce(t, "m", 0))
		}
		holdSlots(t, 3, 2)
		for i := 0; i < 20; i++ {
			require.Equal(t, 2, pickOnce(t, "m", 0), "在飞 2/1/2：选 2")
		}
	})
}

// 有上限的渠道按占用比例比较；不限的渠道容量取本档最大上限。
func TestSelect_LoadRatioMixesLimitedAndUnlimited(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache,
			selectSeed{id: 1, priority: 10, limit: 10}, // 4/10 = 0.4
			selectSeed{id: 2, priority: 10, limit: 2},  // 1/2  = 0.5
			selectSeed{id: 3, priority: 10},            // 3/10 = 0.3（不限，容量按本档最大上限 10）
		)
		holdSlots(t, 1, 4)
		holdSlots(t, 2, 1)
		holdSlots(t, 3, 3)
		for i := 0; i < 20; i++ {
			require.Equal(t, 3, pickOnce(t, "m", 0))
		}
		holdSlots(t, 3, 2) // 5/10 = 0.5
		for i := 0; i < 20; i++ {
			require.Equal(t, 1, pickOnce(t, "m", 0), "0.4 < 0.5 = 0.5")
		}
	})
}

// 负载并列时按权重随机：权重全 0 等概率；有权重时 0 权重渠道不参与并列抽签。
func TestSelect_TieBreaksByWeight(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10}, selectSeed{id: 2, priority: 10})
	seen := map[int]int{}
	for i := 0; i < 200; i++ {
		seen[pickOnce(t, "m", 0)]++
	}
	require.Greater(t, seen[1], 30, "权重全 0 时两个渠道都应被选到：%v", seen)
	require.Greater(t, seen[2], 30, "权重全 0 时两个渠道都应被选到：%v", seen)

	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10, weight: 0}, selectSeed{id: 2, priority: 10, weight: 5})
	for i := 0; i < 30; i++ {
		require.Equal(t, 2, pickOnce(t, "m", 0))
	}
}

// 重试先找同档其它渠道，同档没有了才降档（不再「第 N 次重试走第 N 档」）。
func TestSelect_RetryPrefersSameTierBeforeLowerTier(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache, selectSeed{id: 1, priority: 10}, selectSeed{id: 2, priority: 10}, selectSeed{id: 3, priority: 0})
		for retry := 1; retry <= 3; retry++ {
			require.Equal(t, 2, pickOnce(t, "m", retry, 1), "retry=%d：同档的 2 还没失败过", retry)
		}
		require.Equal(t, 3, pickOnce(t, "m", 2, 1, 2), "同档都失败过才降档")
		require.NotZero(t, pickOnce(t, "m", 3, 1, 2, 3), "全部失败过：退回照常选路，不报无可用渠道")
	})
}

// 429 冷却：冷却中的渠道不再接新请求；只影响该模型。
func TestSelect_SkipsCoolingChannel(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache,
			selectSeed{id: 1, priority: 10, models: "m,m2"},
			selectSeed{id: 2, priority: 10, models: "m,m2"},
			selectSeed{id: 3, priority: 10, models: "m,m2"},
		)
		markChannelCooldown(1, "k-1", "m", 5*time.Second)
		seenM2 := map[int]bool{}
		for i := 0; i < 60; i++ {
			require.NotEqual(t, 1, pickOnce(t, "m", 0), "渠道 1 对模型 m 在冷却")
			seenM2[pickOnce(t, "m2", 0)] = true
		}
		require.True(t, seenM2[1], "冷却只针对模型 m，模型 m2 仍会选到渠道 1")
		require.False(t, IsChannelCooling(mustCacheChannel(t, 2), "m"))
		require.True(t, IsChannelCooling(mustCacheChannel(t, 1), "m"))
	})
}

// 冷却优先于降档：同档未冷却的渠道都没了才降档；全部冷却时选最早解冻的（不报错）。
func TestSelect_AllCoolingPicksEarliestThaw(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache, selectSeed{id: 1, priority: 10}, selectSeed{id: 2, priority: 10}, selectSeed{id: 3, priority: 0})
		markChannelCooldown(1, "k-1", "m", 5*time.Second)
		markChannelCooldown(2, "k-2", "m", 5*time.Second)
		require.Equal(t, 3, pickOnce(t, "m", 0), "高档全部冷却：降到未冷却的低档")

		markChannelCooldown(3, "k-3", "m", 4*time.Second)
		markChannelCooldown(1, "k-1", "m", 2*time.Second) // 重新标记缩短：以最近一次为准
		require.Equal(t, 1, pickOnce(t, "m", 0), "全部冷却：先按档，再按最早解冻")
	})
}

// 冷却到期自动恢复。
func TestCooldown_Expires(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10})
	now := time.Now()
	setCooldownNow(t, func() time.Time { return now })
	markChannelCooldown(1, "k-1", "m", 5*time.Second)
	require.True(t, IsChannelCooling(mustCacheChannel(t, 1), "m"))
	now = now.Add(5*time.Second + time.Millisecond)
	require.False(t, IsChannelCooling(mustCacheChannel(t, 1), "m"))
}

// 冷却时长为 0 时关闭冷却。
func TestMarkRateLimitCooldown_DisabledWhenZero(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10})
	prev := common.RateLimitCooldownSeconds
	t.Cleanup(func() { common.RateLimitCooldownSeconds = prev })

	common.RateLimitCooldownSeconds = 0
	MarkRateLimitCooldown(nil, 1, "k-1", "m")
	require.False(t, IsChannelCooling(mustCacheChannel(t, 1), "m"))

	common.RateLimitCooldownSeconds = 5
	MarkRateLimitCooldown(nil, 1, "k-1", "m")
	require.True(t, IsChannelCooling(mustCacheChannel(t, 1), "m"))
}

// 多 key 渠道：部分 key 冷却仍可选，全部启用 key 冷却才算渠道冷却；渠道内选 key 跳过冷却的 key。
func TestSelect_MultiKeyChannelCooling(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache, selectSeed{id: 1, priority: 10, key: "ka\nkb\nkc"}, selectSeed{id: 2, priority: 10})
		holdSlots(t, 2, 1) // 让无冷却时选 1

		markChannelCooldown(1, "ka", "m", 5*time.Second)
		markChannelCooldown(1, "kb", "m", 5*time.Second)
		require.Equal(t, 1, pickOnce(t, "m", 0), "还有 kc 没冷却，渠道 1 仍可选")

		full, err := model.GetChannelById(1, true)
		require.NoError(t, err)
		for i := 0; i < 30; i++ {
			key, _, apiErr := PickChannelKey(full, "m")
			require.Nil(t, apiErr)
			require.Equal(t, "kc", key, "跳过冷却中的 ka、kb")
		}

		markChannelCooldown(1, "kc", "m", 5*time.Second)
		require.Equal(t, 2, pickOnce(t, "m", 0), "三个 key 都冷却：渠道 1 冷却")
		key, _, apiErr := PickChannelKey(full, "m")
		require.Nil(t, apiErr)
		require.Contains(t, []string{"ka", "kb", "kc"}, key, "全部冷却时照常选 key")
	})
}

// 并发已满的渠道照旧跳过；全满返回并发已满。
func TestSelect_SaturatedStillSkipped(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10, limit: 1}, selectSeed{id: 2, priority: 10, limit: 1})
	holdSlots(t, 1, 1)
	require.Equal(t, 2, pickOnce(t, "m", 0))
	holdSlots(t, 2, 1)
	c := newConcurrencyTestContext()
	ch, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: "g", ModelName: "m", Retry: common.GetPointer(0)})
	require.ErrorIs(t, err, ErrChannelConcurrencyLimited)
	require.Nil(t, ch)
}

func mustCacheChannel(t *testing.T, id int) *model.Channel {
	t.Helper()
	ch, err := model.CacheGetChannel(id)
	require.NoError(t, err)
	return ch
}

func resetChannelCooldowns(t *testing.T) {
	t.Helper()
	memChannelCooldownMu.Lock()
	memChannelCooldown = map[string]map[string]int64{}
	memChannelCooldownMu.Unlock()
	t.Cleanup(func() {
		memChannelCooldownMu.Lock()
		memChannelCooldown = map[string]map[string]int64{}
		memChannelCooldownMu.Unlock()
	})
}

func setCooldownNow(t *testing.T, now func() time.Time) {
	t.Helper()
	prev := cooldownNow
	cooldownNow = now
	t.Cleanup(func() { cooldownNow = prev })
}

// TestChannelCooldown_Redis 真 Redis 集成测试（同 CHANNEL_CONCURRENCY_TEST_REDIS 开关）：冷却跨实例可见、
// 选路与在飞数同一 pipeline 读取、到期恢复、键 TTL 不被较短的标记缩短。请勿指向生产库。
func TestChannelCooldown_Redis(t *testing.T) {
	url := os.Getenv("CHANNEL_CONCURRENCY_TEST_REDIS")
	if url == "" {
		t.Skip("set CHANNEL_CONCURRENCY_TEST_REDIS to run against a real Redis")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	require.NoError(t, rdb.Ping(ctx).Err())

	const modelName = "m-redis-cooldown"
	seedSelectChannels(t, true,
		selectSeed{id: 990101, priority: 10, models: modelName},
		selectSeed{id: 990102, priority: 10, models: modelName},
		selectSeed{id: 990103, priority: 10, models: modelName},
	)
	prevRDB, prevEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = rdb, true
	t.Cleanup(func() {
		common.RDB, common.RedisEnabled = prevRDB, prevEnabled
		_ = rdb.Close()
	})
	keys := []string{}
	for _, id := range []int{990101, 990102, 990103} {
		keys = append(keys, channelCooldownKey(id, modelName), channelConcurrencyKey(id))
	}
	require.NoError(t, rdb.Del(ctx, keys...).Err())
	t.Cleanup(func() { rdb.Del(context.Background(), keys...) })

	markChannelCooldown(990101, "k-990101", modelName, 5*time.Second)
	ttl, err := rdb.PTTL(ctx, channelCooldownKey(990101, modelName)).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, 4*time.Second)
	require.LessOrEqual(t, ttl, 5*time.Second)
	members, err := rdb.ZRange(ctx, channelCooldownKey(990101, modelName), 0, -1).Result()
	require.NoError(t, err)
	require.Equal(t, []string{cooldownKeyFingerprint("k-990101")}, members, "Redis 里只存 key 指纹，不存明文")

	// 本进程内存表为空：冷却状态来自 Redis，即其它实例也能看到
	require.True(t, IsChannelCooling(mustCacheChannel(t, 990101), modelName))
	require.False(t, IsChannelCooling(mustCacheChannel(t, 990101), "other-model"))

	// 选路：冷却的 990101 被绕开；990102 有在飞，选 990103（在飞与冷却同一 pipeline 读出）
	holdSlots(t, 990102, 1)
	for i := 0; i < 20; i++ {
		require.Equal(t, 990103, pickOnce(t, modelName, 0))
	}

	// 同一渠道再标一个更短的冷却：键 TTL 不被缩短
	markChannelCooldown(990101, "another-key", modelName, 300*time.Millisecond)
	ttl, err = rdb.PTTL(ctx, channelCooldownKey(990101, modelName)).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, 4*time.Second)

	// 到期恢复
	markChannelCooldown(990103, "k-990103", modelName, 300*time.Millisecond)
	require.True(t, IsChannelCooling(mustCacheChannel(t, 990103), modelName))
	time.Sleep(400 * time.Millisecond)
	require.False(t, IsChannelCooling(mustCacheChannel(t, 990103), modelName))
}

// 选中的渠道带完整字段（含 key）：DB 路径的候选查询不取 key，选中后必须补全，否则转发拿不到 key。
func TestSelect_ReturnsChannelWithKey(t *testing.T) {
	forBothSelectPaths(t, func(t *testing.T, memoryCache bool) {
		seedSelectChannels(t, memoryCache, selectSeed{id: 1, priority: 10, key: "secret-key-1"})
		c := newConcurrencyTestContext()
		ch, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: "g", ModelName: "m", Retry: common.GetPointer(0)})
		require.NoError(t, err)
		require.NotNil(t, ch)
		require.Equal(t, "secret-key-1", ch.Key)
		require.True(t, HoldsChannelSlot(c, 1))
		ReleaseChannelSlot(c)
	})
}

// 多 key 渠道判断冷却时读 key 状态表，必须与自动禁用 / 启用 key（UpdateChannelStatus）同锁，
// 否则内存缓存下的并发读写 map 会让进程直接崩溃（fatal error，recover 不住）。用 -race 跑可稳定复现。
func TestChannelCooling_ConcurrentWithKeyStatusUpdate(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10, key: "ka\nkb\nkc"})
	markChannelCooldown(1, "ka", "m", 5*time.Second)
	ch := mustCacheChannel(t, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			status := common.ChannelStatusAutoDisabled
			if i%2 == 1 {
				status = common.ChannelStatusEnabled
			}
			model.UpdateChannelStatus(1, "kb", status, "test")
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
			IsChannelCooling(ch, "m")
			pickOnce(t, "m", 0)
		}
	}
}

// 冷却记录属于已禁用的 key 时不计入：ka、kc 冷却后 kc 被禁用，kb 仍可用，渠道不算冷却。
func TestChannelCooling_IgnoresDisabledKeys(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10, key: "ka\nkb\nkc"})
	markChannelCooldown(1, "ka", "m", 5*time.Second)
	markChannelCooldown(1, "kc", "m", 5*time.Second)
	model.UpdateChannelStatus(1, "kc", common.ChannelStatusAutoDisabled, "test")
	require.False(t, IsChannelCooling(mustCacheChannel(t, 1), "m"), "启用中的 kb 没冷却")

	markChannelCooldown(1, "kb", "m", 5*time.Second)
	require.True(t, IsChannelCooling(mustCacheChannel(t, 1), "m"), "启用中的 ka、kb 都冷却")
}

// 进程内冷却表：标记时顺带清掉已过期的条目，不会因从此不再读到的 渠道×模型 无限增长。
func TestMemCooldown_PrunesExpiredOnMark(t *testing.T) {
	resetChannelCooldowns(t)
	prevRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prevRedis })
	now := time.Now()
	setCooldownNow(t, func() time.Time { return now })

	markChannelCooldown(1, "k", "gpt-4-gizmo-a", 5*time.Second)
	now = now.Add(6 * time.Second)
	markChannelCooldown(2, "k", "gpt-4-gizmo-b", 5*time.Second)

	memChannelCooldownMu.Lock()
	defer memChannelCooldownMu.Unlock()
	_, stale := memChannelCooldown[channelCooldownKey(1, "gpt-4-gizmo-a")]
	require.False(t, stale, "过期条目应在下一次标记时被清理")
	require.Len(t, memChannelCooldown, 1)
}
