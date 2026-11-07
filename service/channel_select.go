package service

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
	// failedChannels 本次请求已失败过的渠道：重试选路时优先避开，免得随机又选回刚失败的渠道
	failedChannels map[int]bool
}

// MarkChannelFailed 记录本次请求在该渠道上失败，后续重试优先选其它渠道。
func (p *RetryParam) MarkChannelFailed(channelId int) {
	if p.failedChannels == nil {
		p.failedChannels = make(map[int]bool)
	}
	p.failedChannels[channelId] = true
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跨分组重试的 "auto" tokenGroup：
//
//   - Within a group, channels are picked by acquireSatisfiedChannel (tiers high→low, least in-flight first,
//     skipping channels that already failed for this request or are cooling down after a 429).
//     The request moves to the next group when the current one has no available channel, or when
//     cross-group retry is on and the retries spent in this group reach RetryTimes.
//     组内按 acquireSatisfiedChannel 选路（档位从高到低、在飞最少优先、跳过本请求失败过的与 429 冷却中的渠道）；
//     当前分组没有可用渠道，或开启跨分组重试且本组重试次数达到 RetryTimes 时，切到下一个分组。
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Uses ContextKeyAutoGroupRetryIndex to track the global Retry count when current group started.
//     使用 ContextKeyAutoGroupRetryIndex 跟踪当前分组开始时的全局重试次数。
//
//   - priorityRetry = Retry - startRetryIndex, represents the priority level within current group.
//     priorityRetry = Retry - startRetryIndex，表示当前分组内的优先级级别。
//
//   - When GetRandomSatisfiedChannel returns nil (priorities exhausted), moves to next group.
//     当 GetRandomSatisfiedChannel 返回 nil（优先级用完）时，切换到下一个分组。
//
// Example flow (2 groups, each with 2 priorities, RetryTimes=3):
// 示例流程（2个分组，每个有2个优先级，RetryTimes=3）：
//
//	Retry=0: GroupA, priority0 (startRetryIndex=0, priorityRetry=0)
//	         分组A, 优先级0
//
//	Retry=1: GroupA, priority1 (startRetryIndex=0, priorityRetry=1)
//	         分组A, 优先级1
//
//	Retry=2: GroupA exhausted → GroupB, priority0 (startRetryIndex=2, priorityRetry=0)
//	         分组A用完 → 分组B, 优先级0
//
//	Retry=3: GroupB, priority1 (startRetryIndex=2, priorityRetry=1)
//	         分组B, 优先级1
//
// The returned channel already holds a concurrency slot for this request (see TryAcquireChannelSlot).
// Channels at their concurrency limit are skipped; ErrChannelConcurrencyLimited is returned when no
// channel could be selected and at least one candidate was skipped for that reason.
// 返回的渠道已为本请求占好并发位；并发已满的渠道被跳过，若因此选不到任何渠道则返回 ErrChannelConcurrencyLimited。
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	var busy, sawBusy bool
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, busy, _ = acquireSatisfiedChannel(param.Ctx, autoGroup, param.ModelName, param.failedChannels)
			sawBusy = sawBusy || busy
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		channel, sawBusy, err = acquireSatisfiedChannel(param.Ctx, param.TokenGroup, param.ModelName, param.failedChannels)
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	if channel == nil && sawBusy {
		return nil, selectGroup, ErrChannelConcurrencyLimited
	}
	return channel, selectGroup, nil
}

// selectCandidate 一次选路中的候选渠道快照。
type selectCandidate struct {
	ch       *model.Channel
	inflight int
	limit    int   // 并发上限，0 = 不限
	cooling  bool  // 对该模型处于 429 冷却
	thawAt   int64 // 冷却中时最早有 key 解冻的毫秒时间
}

// acquireSatisfiedChannel 选路并为本请求占好并发位（首次选路与重试统一走这里）。
//
// 分三轮，前一轮选不出才进下一轮：
//  1. 排除本请求失败过的、429 冷却中的、并发已满的；
//  2. 排除失败过的、已满的（全部冷却时也不报错，按解冻早的优先）；
//  3. 只排除已满的（失败过的也可再用：分组只有一个渠道等情况的行为与原来一致）。
//
// 每轮内按有效优先级（竞价：成本价 → 渠道优先级）从高到低逐档；档内选负载最低的，并列按权重随机。
// 占位失败说明刚被别人占满，记为已满后继续本档。busy 表示有候选渠道因并发已满被跳过。
func acquireSatisfiedChannel(c *gin.Context, group string, modelName string, failed map[int]bool) (channel *model.Channel, busy bool, err error) {
	channels, err := model.GetSatisfiedChannelCandidates(group, modelName)
	if err != nil || len(channels) == 0 {
		return nil, false, err
	}
	tiers := buildSelectTiers(channels, modelName)
	saturated := make(map[int]bool)
	for _, tier := range tiers {
		for _, cand := range tier {
			if cand.limit > 0 && cand.inflight >= cand.limit {
				saturated[cand.ch.Id] = true
				busy = true
			}
		}
	}
	for pass := 0; pass < 3; pass++ {
		for _, tier := range tiers {
			for {
				cand := pickFromTier(tier, pass, failed, saturated)
				if cand == nil {
					break
				}
				if TryAcquireChannelSlot(c, cand.ch) {
					if cand.cooling {
						logger.LogDebug(c, "channel #%d is cooling down for model %s, but no other channel is available", cand.ch.Id, modelName)
					}
					if common.MemoryCacheEnabled {
						return cand.ch, false, nil
					}
					// DB 路径的候选查询不取 key：选中后读完整渠道（与原 DB 选路一致）
					full, err := model.GetChannelById(cand.ch.Id, true)
					if err != nil {
						ReleaseChannelSlot(c)
						return nil, false, err
					}
					return full, false, nil
				}
				saturated[cand.ch.Id] = true
				busy = true
			}
		}
	}
	return nil, busy, nil
}

// buildSelectTiers 读取候选渠道的在飞数与冷却状态，按有效优先级从高到低分档。
func buildSelectTiers(channels []*model.Channel, modelName string) [][]*selectCandidate {
	ids := make([]int, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.Id)
	}
	inflight, cooldowns := loadChannelSelectState(ids, modelName)
	strategy := model.GetDispatchStrategy()
	byTier := make(map[int64][]*selectCandidate)
	tierKeys := make([]int64, 0)
	for _, ch := range channels {
		cooling, thawAt := channelCoolingUntil(ch, cooldowns[ch.Id])
		cand := &selectCandidate{ch: ch, inflight: inflight[ch.Id], limit: ch.GetMaxConcurrency(), cooling: cooling, thawAt: thawAt}
		tier := model.DispatchEffectivePriority(ch, strategy)
		if _, ok := byTier[tier]; !ok {
			tierKeys = append(tierKeys, tier)
		}
		byTier[tier] = append(byTier[tier], cand)
	}
	sort.Slice(tierKeys, func(i, j int) bool { return tierKeys[i] > tierKeys[j] })
	tiers := make([][]*selectCandidate, 0, len(tierKeys))
	for _, k := range tierKeys {
		tiers = append(tiers, byTier[k])
	}
	return tiers
}

// pickFromTier 在一档内按本轮规则选出解冻最早（未冷却视为 0）、负载最低的渠道，并列按权重随机；无可选返回 nil。
// 负载 = 在飞数 ÷ 容量；容量 = 并发上限，不限并发的渠道取本档最大上限（本档都不限则为 1，即直接比在飞数）。
func pickFromTier(tier []*selectCandidate, pass int, failed map[int]bool, saturated map[int]bool) *selectCandidate {
	refCap := 1
	for _, cand := range tier {
		if cand.limit > refCap {
			refCap = cand.limit
		}
	}
	const eps = 1e-9
	var ties []*selectCandidate
	var bestThaw int64
	var bestLoad float64
	for _, cand := range tier {
		id := cand.ch.Id
		if saturated[id] {
			continue
		}
		if pass < 2 && failed[id] {
			continue
		}
		if pass == 0 && cand.cooling {
			continue
		}
		var thaw int64
		if cand.cooling {
			thaw = cand.thawAt
		}
		capacity := cand.limit
		if capacity <= 0 {
			capacity = refCap
		}
		load := float64(cand.inflight) / float64(capacity)
		switch {
		case ties == nil, thaw < bestThaw, thaw == bestThaw && load < bestLoad-eps:
			ties = []*selectCandidate{cand}
			bestThaw, bestLoad = thaw, load
		case thaw == bestThaw && load <= bestLoad+eps:
			ties = append(ties, cand)
		}
	}
	if len(ties) <= 1 {
		if len(ties) == 1 {
			return ties[0]
		}
		return nil
	}
	sumWeight := 0
	for _, cand := range ties {
		sumWeight += cand.ch.GetWeight()
	}
	if sumWeight <= 0 {
		return ties[rand.Intn(len(ties))]
	}
	r := rand.Intn(sumWeight)
	for _, cand := range ties {
		r -= cand.ch.GetWeight()
		if r < 0 {
			return cand
		}
	}
	return ties[len(ties)-1]
}

// loadChannelSelectState 一次读出候选渠道的在飞数与对该模型的冷却状态（Redis 下为同一个 pipeline）。
// 读失败按「在飞 0、不冷却」放行；能否占位最终以 TryAcquireChannelSlot 的原子结果为准。
func loadChannelSelectState(ids []int, modelName string) (map[int]int, map[int]map[string]int64) {
	if !channelConcurrencyUseRedis() {
		return GetChannelConcurrencyCounts(ids), loadChannelCooldowns(ids, modelName)
	}
	counts := make(map[int]int, len(ids))
	cooldowns := make(map[int]map[string]int64)
	ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
	defer cancel()
	pipe := common.RDB.Pipeline()
	countCmds := make(map[int]*redis.IntCmd, len(ids))
	minScore := "(" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	for _, id := range ids {
		if _, dup := countCmds[id]; !dup {
			countCmds[id] = pipe.ZCount(ctx, channelConcurrencyKey(id), minScore, "+inf")
		}
	}
	coolCmds := make(map[int]*redis.ZSliceCmd)
	if rateLimitCooldownEnabled() {
		queueChannelCooldownReads(ctx, pipe, coolCmds, ids, modelName, cooldownNow().UnixMilli())
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		common.SysError("failed to read channel select state: " + err.Error())
		return counts, cooldowns
	}
	for id, cmd := range countCmds {
		if n, err := cmd.Result(); err == nil && n > 0 {
			counts[id] = int(n)
		}
	}
	collectChannelCooldowns(coolCmds, cooldowns)
	return counts, cooldowns
}
