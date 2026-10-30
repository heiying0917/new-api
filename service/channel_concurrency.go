package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// 渠道并发上限：按渠道统计同一时刻在途的请求数（流式 / WebSocket 从开始到结束都算占用）。
// max_concurrency>0 的渠道满载时在选路阶段被跳过；不限并发的渠道同样计数，供渠道列表展示当前并发。
//
// Redis 启用时计数跨实例共享：每渠道一个 ZSET，member 为单次占位 ID，score 为租约到期的毫秒时间戳。
// 本进程持有的占位由后台协程定期续期，实例崩溃遗留的占位在租约到期后自动失效。
// Redis 未启用时退回进程内计数。Redis 出错时放行（fail-open）：计数故障不能拖垮转发。

// ErrChannelConcurrencyLimited 候选渠道因并发已满全部被跳过。
var ErrChannelConcurrencyLimited = errors.New("channel concurrency limit reached")

const (
	channelConcurrencyKeyPrefix     = "channel_concurrency:"
	channelConcurrencyLeaseTTL      = 60 * time.Second
	channelConcurrencyRenewInterval = 15 * time.Second
	channelConcurrencyRedisTimeout  = 2 * time.Second
)

// KEYS[1] 渠道 ZSET；ARGV: now_ms, expire_at_ms, limit(0=不限), member, key_ttl_ms。返回 1=占位成功，0=已满。
var channelConcurrencyAcquireScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local limit = tonumber(ARGV[3])
if limit > 0 and redis.call('ZCARD', KEYS[1]) >= limit then
	return 0
end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1
`)

// channelSlot 一次并发占位。redisKey 非空表示 Redis 计数；inMemory 表示进程内计数；两者皆空表示 fail-open 未跟踪。
type channelSlot struct {
	channelId int
	redisKey  string
	member    string
	inMemory  bool
}

var (
	memChannelInflightMu sync.Mutex
	memChannelInflight   = map[int]int{}

	// heldRedisSlots 本进程持有的 Redis 占位：member -> key，供续期协程使用
	heldRedisSlots       sync.Map
	channelSlotRenewOnce sync.Once
)

func channelConcurrencyKey(channelId int) string {
	return channelConcurrencyKeyPrefix + strconv.Itoa(channelId)
}

func channelConcurrencyUseRedis() bool {
	return common.RedisEnabled && common.RDB != nil
}

// TryAcquireChannelSlot 为本次请求在 channel 上占一个并发位，满载返回 false。
// 先释放本请求已持有的占位：重试换渠道时不会自己占着自己的位，导致同一渠道被误判满载。
func TryAcquireChannelSlot(c *gin.Context, channel *model.Channel) bool {
	ReleaseChannelSlot(c)
	if channel == nil {
		return true
	}
	limit := channel.GetMaxConcurrency()
	slot := &channelSlot{channelId: channel.Id}
	if channelConcurrencyUseRedis() {
		key := channelConcurrencyKey(channel.Id)
		member := common.GetUUID()
		ok, err := redisAcquireChannelSlot(key, member, limit)
		if err != nil {
			logger.LogWarn(c, fmt.Sprintf("channel #%d concurrency acquire failed, fail-open: %v", channel.Id, err))
			// 脚本可能已在 Redis 执行、只是回包超时：尽力撤销，避免本请求放行后还留一个占着名额的占位直到租约到期
			go discardChannelSlotMember(common.RDB, key, member)
		} else if !ok {
			return false
		} else {
			slot.redisKey = key
			slot.member = member
			heldRedisSlots.Store(member, key)
			startChannelSlotRenewer()
		}
	} else {
		if !memAcquireChannelSlot(channel.Id, limit) {
			return false
		}
		slot.inMemory = true
	}
	common.SetContextKey(c, constant.ContextKeyChannelConcurrencySlot, slot)
	return true
}

// HoldsChannelSlot 本请求当前是否持有 channelId 的并发占位。
func HoldsChannelSlot(c *gin.Context, channelId int) bool {
	slot := heldChannelSlot(c)
	return slot != nil && slot.channelId == channelId
}

// ReleaseChannelSlot 释放本请求持有的并发占位；未持有时为空操作，可重复调用。
func ReleaseChannelSlot(c *gin.Context) {
	slot := heldChannelSlot(c)
	if slot == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyChannelConcurrencySlot, nil)
	switch {
	case slot.redisKey != "":
		heldRedisSlots.Delete(slot.member)
		ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
		defer cancel()
		if err := common.RDB.ZRem(ctx, slot.redisKey, slot.member).Err(); err != nil {
			// 释放失败：占位最多在租约到期（channelConcurrencyLeaseTTL）后自动失效
			logger.LogWarn(c, fmt.Sprintf("channel #%d concurrency release failed: %v", slot.channelId, err))
		}
	case slot.inMemory:
		memReleaseChannelSlot(slot.channelId)
	}
}

// ReleaseAllHeldChannelSlots 进程退出前释放本进程持有的全部 Redis 占位（优雅停机后调用）。
// 停机超时后仍未结束的长流 / WebSocket 随进程一起终止，它们的占位不必等租约到期。
func ReleaseAllHeldChannelSlots() {
	if !channelConcurrencyUseRedis() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
	defer cancel()
	pipe := common.RDB.Pipeline()
	n := 0
	heldRedisSlots.Range(func(member, key any) bool {
		pipe.ZRem(ctx, key.(string), member)
		heldRedisSlots.Delete(member)
		n++
		return true
	})
	if n == 0 {
		return
	}
	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to release channel concurrency slots on shutdown: " + err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("released %d channel concurrency slots on shutdown", n))
}

func discardChannelSlotMember(rdb *redis.Client, key string, member string) {
	ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
	defer cancel()
	_ = rdb.ZRem(ctx, key, member).Err()
}

func heldChannelSlot(c *gin.Context) *channelSlot {
	if c == nil {
		return nil
	}
	v, ok := common.GetContextKey(c, constant.ContextKeyChannelConcurrencySlot)
	if !ok {
		return nil
	}
	slot, _ := v.(*channelSlot)
	return slot
}

// GetChannelConcurrencyCounts 返回各渠道当前在途请求数（无在途的渠道不出现在结果中）。
func GetChannelConcurrencyCounts(channelIds []int) map[int]int {
	counts := make(map[int]int, len(channelIds))
	if len(channelIds) == 0 {
		return counts
	}
	if !channelConcurrencyUseRedis() {
		memChannelInflightMu.Lock()
		defer memChannelInflightMu.Unlock()
		for _, id := range channelIds {
			if n := memChannelInflight[id]; n > 0 {
				counts[id] = n
			}
		}
		return counts
	}
	ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
	defer cancel()
	// 只数未过期的占位：score(到期时间) > now
	minScore := "(" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	pipe := common.RDB.Pipeline()
	cmds := make(map[int]*redis.IntCmd, len(channelIds))
	for _, id := range channelIds {
		if _, dup := cmds[id]; !dup {
			cmds[id] = pipe.ZCount(ctx, channelConcurrencyKey(id), minScore, "+inf")
		}
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		common.SysError("failed to read channel concurrency counts: " + err.Error())
		return counts
	}
	for id, cmd := range cmds {
		if n, err := cmd.Result(); err == nil && n > 0 {
			counts[id] = int(n)
		}
	}
	return counts
}

func redisAcquireChannelSlot(key string, member string, limit int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
	defer cancel()
	now := time.Now()
	res, err := channelConcurrencyAcquireScript.Run(ctx, common.RDB, []string{key},
		now.UnixMilli(),
		now.Add(channelConcurrencyLeaseTTL).UnixMilli(),
		limit,
		member,
		channelConcurrencyLeaseTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func startChannelSlotRenewer() {
	channelSlotRenewOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(channelConcurrencyRenewInterval)
			defer ticker.Stop()
			for range ticker.C {
				renewHeldChannelSlots()
			}
		}()
	})
}

// renewHeldChannelSlots 给本进程持有的 Redis 占位续租约。用 ZADD XX：只更新仍存在的 member，
// 避免与释放（ZREM）竞态时把刚释放的占位重新加回去，留下一个要等租约到期才消失的幽灵占位。
func renewHeldChannelSlots() {
	if !channelConcurrencyUseRedis() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), channelConcurrencyRedisTimeout)
	defer cancel()
	expireAt := float64(time.Now().Add(channelConcurrencyLeaseTTL).UnixMilli())
	pipe := common.RDB.Pipeline()
	keys := make(map[string]struct{})
	heldRedisSlots.Range(func(member, key any) bool {
		k := key.(string)
		pipe.ZAddXX(ctx, k, &redis.Z{Score: expireAt, Member: member})
		keys[k] = struct{}{}
		return true
	})
	if len(keys) == 0 {
		return
	}
	for k := range keys {
		pipe.PExpire(ctx, k, channelConcurrencyLeaseTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to renew channel concurrency slots: " + err.Error())
	}
}

func memAcquireChannelSlot(channelId int, limit int) bool {
	memChannelInflightMu.Lock()
	defer memChannelInflightMu.Unlock()
	if limit > 0 && memChannelInflight[channelId] >= limit {
		return false
	}
	memChannelInflight[channelId]++
	return true
}

func memReleaseChannelSlot(channelId int) {
	memChannelInflightMu.Lock()
	defer memChannelInflightMu.Unlock()
	if memChannelInflight[channelId] > 1 {
		memChannelInflight[channelId]--
	} else {
		delete(memChannelInflight, channelId)
	}
}
