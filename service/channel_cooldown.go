package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// 429 冷却：上游限流后，该渠道的该 key 对该模型冷却 RateLimitCooldownSeconds 秒，冷却期间选路与渠道内选 key 都绕开它。
// 粒度为 渠道 × key × 模型（AWS 配额按账号 × 区域 × 模型，多 key 渠道里一个区域被限流不影响其它区域）。
// 单 key 渠道：该 key 冷却即渠道冷却；多 key 渠道：启用中的 key 全部冷却才算渠道冷却。
//
// Redis 启用时跨实例共享：每个 渠道×模型 一个 ZSET，member 为 key 指纹，score 为冷却到期毫秒。
// Redis 未启用时退回进程内表。Redis 出错按「不冷却」放行：冷却只是优化，故障不能拖垮转发。

const (
	channelCooldownKeyPrefix    = "channel_cooldown:"
	channelCooldownRedisTimeout = 2 * time.Second
)

// cooldownNow 当前时间（测试可替换）。
var cooldownNow = time.Now

// KEYS[1] 渠道×模型 ZSET；ARGV: now_ms, expire_at_ms, member, ttl_ms。到期时间以最近一次标记为准。
var channelCooldownMarkScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
local ttl = redis.call('PTTL', KEYS[1])
if ttl < tonumber(ARGV[4]) then
	redis.call('PEXPIRE', KEYS[1], ARGV[4])
end
return 1
`)

var (
	memChannelCooldownMu sync.Mutex
	// "渠道id:模型" -> key 指纹 -> 到期毫秒
	memChannelCooldown = map[string]map[string]int64{}
)

func channelCooldownKey(channelId int, modelName string) string {
	return channelCooldownKeyPrefix + strconv.Itoa(channelId) + ":" + modelName
}

// cooldownKeyFingerprint key 指纹：不在 Redis / 日志里存明文 key。
func cooldownKeyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

func rateLimitCooldownEnabled() bool {
	return common.RateLimitCooldownSeconds > 0
}

// MarkRateLimitCooldown 上游 429 后让 渠道×key×模型 冷却 RateLimitCooldownSeconds 秒（0 表示不冷却）。
func MarkRateLimitCooldown(c *gin.Context, channelId int, key string, modelName string) {
	if !rateLimitCooldownEnabled() || channelId <= 0 {
		return
	}
	d := time.Duration(common.RateLimitCooldownSeconds) * time.Second
	markChannelCooldown(channelId, key, modelName, d)
	msg := fmt.Sprintf("channel #%d key %s model %s rate limited (429), cooling down %s", channelId, cooldownKeyFingerprint(key), modelName, d)
	if c != nil {
		logger.LogInfo(c, msg)
	} else {
		common.SysLog(msg)
	}
}

func markChannelCooldown(channelId int, key string, modelName string, d time.Duration) {
	now := cooldownNow()
	expireAt := now.Add(d).UnixMilli()
	fp := cooldownKeyFingerprint(key)
	if channelConcurrencyUseRedis() {
		ctx, cancel := context.WithTimeout(context.Background(), channelCooldownRedisTimeout)
		defer cancel()
		err := channelCooldownMarkScript.Run(ctx, common.RDB, []string{channelCooldownKey(channelId, modelName)},
			now.UnixMilli(), expireAt, fp, d.Milliseconds()).Err()
		if err != nil && !errors.Is(err, redis.Nil) {
			common.SysError(fmt.Sprintf("failed to mark channel #%d cooldown: %v", channelId, err))
		}
		return
	}
	memChannelCooldownMu.Lock()
	defer memChannelCooldownMu.Unlock()
	// 标记时顺带清掉全部已过期条目（429 不频繁，遍历开销可忽略），避免从此不再被读到的 渠道×模型 残留
	nowMs := now.UnixMilli()
	for k, entry := range memChannelCooldown {
		for fp, at := range entry {
			if at <= nowMs {
				delete(entry, fp)
			}
		}
		if len(entry) == 0 {
			delete(memChannelCooldown, k)
		}
	}
	k := channelCooldownKey(channelId, modelName)
	entry := memChannelCooldown[k]
	if entry == nil {
		entry = map[string]int64{}
		memChannelCooldown[k] = entry
	}
	entry[fp] = expireAt
}

// loadChannelCooldowns 读取各渠道对该模型冷却中的 key 指纹及到期毫秒（不在冷却的渠道不出现在结果中）。
func loadChannelCooldowns(channelIds []int, modelName string) map[int]map[string]int64 {
	result := make(map[int]map[string]int64)
	if !rateLimitCooldownEnabled() || len(channelIds) == 0 {
		return result
	}
	nowMs := cooldownNow().UnixMilli()
	if !channelConcurrencyUseRedis() {
		memChannelCooldownMu.Lock()
		defer memChannelCooldownMu.Unlock()
		for _, id := range channelIds {
			k := channelCooldownKey(id, modelName)
			entry := memChannelCooldown[k]
			for fp, expireAt := range entry {
				if expireAt <= nowMs {
					delete(entry, fp)
					continue
				}
				if result[id] == nil {
					result[id] = map[string]int64{}
				}
				result[id][fp] = expireAt
			}
			if len(entry) == 0 {
				delete(memChannelCooldown, k)
			}
		}
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), channelCooldownRedisTimeout)
	defer cancel()
	pipe := common.RDB.Pipeline()
	cmds := make(map[int]*redis.ZSliceCmd, len(channelIds))
	queueChannelCooldownReads(ctx, pipe, cmds, channelIds, modelName, nowMs)
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		common.SysError("failed to read channel cooldowns: " + err.Error())
		return result
	}
	collectChannelCooldowns(cmds, result)
	return result
}

func queueChannelCooldownReads(ctx context.Context, pipe redis.Pipeliner, cmds map[int]*redis.ZSliceCmd, channelIds []int, modelName string, nowMs int64) {
	minScore := "(" + strconv.FormatInt(nowMs, 10)
	for _, id := range channelIds {
		if _, dup := cmds[id]; !dup {
			cmds[id] = pipe.ZRangeByScoreWithScores(ctx, channelCooldownKey(id, modelName), &redis.ZRangeBy{Min: minScore, Max: "+inf"})
		}
	}
}

func collectChannelCooldowns(cmds map[int]*redis.ZSliceCmd, result map[int]map[string]int64) {
	for id, cmd := range cmds {
		zs, err := cmd.Result()
		if err != nil || len(zs) == 0 {
			continue
		}
		entry := make(map[string]int64, len(zs))
		for _, z := range zs {
			if fp, ok := z.Member.(string); ok {
				entry[fp] = int64(z.Score)
			}
		}
		result[id] = entry
	}
}

// channelCoolingUntil 渠道对该模型是否冷却（启用中的 key 全部冷却），以及最早有 key 解冻的时刻（毫秒）。
// 已禁用 key 的冷却记录不计入。DB 选路的候选不带 key 内容，只能按冷却数与启用 key 数比较兜底。
func channelCoolingUntil(ch *model.Channel, cooling map[string]int64) (bool, int64) {
	if len(cooling) == 0 {
		return false, 0
	}
	var earliest int64
	if ch.Key == "" {
		need := ch.EnabledKeyCount()
		if need < 1 {
			need = 1
		}
		if len(cooling) < need {
			return false, 0
		}
		for _, expireAt := range cooling {
			if earliest == 0 || expireAt < earliest {
				earliest = expireAt
			}
		}
		return true, earliest
	}
	enabled := ch.EnabledKeys()
	if len(enabled) == 0 {
		return false, 0
	}
	for _, key := range enabled {
		expireAt, ok := cooling[cooldownKeyFingerprint(key)]
		if !ok {
			return false, 0
		}
		if earliest == 0 || expireAt < earliest {
			earliest = expireAt
		}
	}
	return true, earliest
}

// IsChannelCooling 渠道对该模型是否处于 429 冷却。
func IsChannelCooling(channel *model.Channel, modelName string) bool {
	if channel == nil {
		return false
	}
	cooling, _ := channelCoolingUntil(channel, loadChannelCooldowns([]int{channel.Id}, modelName)[channel.Id])
	return cooling
}

// PickChannelKey 选本次请求使用的 key：多 key 渠道跳过对该模型冷却中的 key（全部冷却时照常选）。
func PickChannelKey(channel *model.Channel, modelName string) (string, int, *types.NewAPIError) {
	if !channel.ChannelInfo.IsMultiKey || !rateLimitCooldownEnabled() {
		return channel.GetNextEnabledKey()
	}
	cooling := loadChannelCooldowns([]int{channel.Id}, modelName)[channel.Id]
	if len(cooling) == 0 {
		return channel.GetNextEnabledKey()
	}
	return channel.GetNextEnabledKeyAvoiding(func(key string) bool {
		_, ok := cooling[cooldownKeyFingerprint(key)]
		return ok
	})
}
