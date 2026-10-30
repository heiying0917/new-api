package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/go-redis/redis/v8"
)

// 「模型 ID 无效」禁用熔断：这类报错多数是渠道账号受限，但若是我方模型映射/区域配置有误，
// 所有渠道请求该模型都会报同样的错，直接禁用会在几秒内把渠道全部禁光。
// 因此同一模型在一个时间窗内最多因该类报错禁用 unavailableModelDisableLimit 个不同渠道，
// 超出后只换渠道重试、不再禁用，并打告警日志等人工排查。
//
// Redis 启用时计数跨实例共享（每模型一个 SET，首次写入时设过期作为时间窗）；
// 未启用或 Redis 出错时退回进程内计数。

const (
	unavailableModelDisableWindow       = 10 * time.Minute
	unavailableModelDisableLimit        = 3
	unavailableModelDisableKeyPrefix    = "unavailable_model_disable:"
	unavailableModelDisableRedisTimeout = 2 * time.Second
)

// KEYS[1] 模型 SET；ARGV: channel_id, limit, window_ms。返回 1=允许禁用，0=已熔断。
// 已记录过的渠道重复报错仍允许（禁用是幂等的），不额外占名额。
var unavailableModelDisableScript = redis.NewScript(`
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then
	return 1
end
if redis.call('SCARD', KEYS[1]) >= tonumber(ARGV[2]) then
	return 0
end
redis.call('SADD', KEYS[1], ARGV[1])
if redis.call('PTTL', KEYS[1]) < 0 then
	redis.call('PEXPIRE', KEYS[1], ARGV[3])
end
return 1
`)

type unavailableModelDisableEntry struct {
	windowStart time.Time
	channels    map[int]bool
}

type unavailableModelDisableBreaker struct {
	mu      sync.Mutex
	entries map[string]*unavailableModelDisableEntry
	now     func() time.Time
}

var defaultUnavailableModelDisableBreaker = &unavailableModelDisableBreaker{
	entries: map[string]*unavailableModelDisableEntry{},
	now:     time.Now,
}

// allowInMemory 进程内计数：窗口过期后重新开窗。
func (b *unavailableModelDisableBreaker) allowInMemory(modelName string, channelId int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	entry := b.entries[modelName]
	if entry == nil || now.Sub(entry.windowStart) >= unavailableModelDisableWindow {
		entry = &unavailableModelDisableEntry{windowStart: now, channels: map[int]bool{}}
		b.entries[modelName] = entry
	}
	if entry.channels[channelId] {
		return true
	}
	if len(entry.channels) >= unavailableModelDisableLimit {
		return false
	}
	entry.channels[channelId] = true
	return true
}

func redisAllowUnavailableModelDisable(modelName string, channelId int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), unavailableModelDisableRedisTimeout)
	defer cancel()
	res, err := unavailableModelDisableScript.Run(ctx, common.RDB,
		[]string{unavailableModelDisableKeyPrefix + modelName},
		strconv.Itoa(channelId),
		unavailableModelDisableLimit,
		unavailableModelDisableWindow.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// AllowUnavailableModelDisable 是否允许因「模型 ID 无效」禁用该渠道；熔断时记录告警并返回 false。
func AllowUnavailableModelDisable(modelName string, channelId int) bool {
	var allowed bool
	if common.RedisEnabled && common.RDB != nil {
		var err error
		allowed, err = redisAllowUnavailableModelDisable(modelName, channelId)
		if err != nil {
			common.SysError(fmt.Sprintf("unavailable model disable breaker redis error, fallback to in-memory: %v", err))
			allowed = defaultUnavailableModelDisableBreaker.allowInMemory(modelName, channelId)
		}
	} else {
		allowed = defaultUnavailableModelDisableBreaker.allowInMemory(modelName, channelId)
	}
	if !allowed {
		common.SysError(fmt.Sprintf("模型 %s 在 %v 内已因「模型 ID 无效」禁用 %d 个渠道，疑似模型映射或区域配置问题，熔断：不再禁用渠道 #%d，仅换渠道重试，请人工排查",
			modelName, unavailableModelDisableWindow, unavailableModelDisableLimit, channelId))
	}
	return allowed
}
