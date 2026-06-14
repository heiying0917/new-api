package common

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 登录防暴破：账号维度失败计数 + 渐进硬锁定 + 全局失败天花板。
//
// 设计要点：
//   - 所有计数按"归一化登录标识"（用户名/邮箱，小写去空白）进行，与客户端 IP
//     完全无关，因此对代理 IP 轮换 / X-Forwarded-For 伪造天然免疫（map 上界 = 用户量级）。
//   - 存在与否的账号都走同一计数路径，避免"锁定 vs 不锁定"成为账号枚举旁路。
//   - 双模：启用 Redis 时用 Redis（多节点一致），否则用进程内存。
//   - 硬锁定还会由 controller 持久化到 users 表列，作为权威兜底（重启 / Redis flush
//     后仍生效，且可被管理员解锁、被 UNLOCK_ALL_ON_START 清空）。

const (
	loginFailKeyPrefix   = "loginfail:"
	loginLockKeyPrefix   = "loginlock:"
	loginGlobalFailKey   = "loginfail:__global__"
	loginThrottleMaxKeys = 100000 // 进程内存模式 key 上限，防用户名轮换内存膨胀
)

// NormalizeLoginIdentifier 归一化登录标识，用于按账号计数。
func NormalizeLoginIdentifier(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type loginThrottleEntry struct {
	failCount   int
	windowStart int64 // 计数窗口起点（unix 秒）
	lockedUntil int64 // 硬锁定到期（unix 秒），0 = 未锁
}

var (
	loginThrottleMu    sync.Mutex
	loginThrottleStore = make(map[string]*loginThrottleEntry)

	loginGlobalMu          sync.Mutex
	loginGlobalCount       int
	loginGlobalWindowStart int64
)

// loginCaptchaThreshold / loginLockThreshold 计算生效阈值
// （管理员更严：阈值减半、时长加倍）。
func loginCaptchaThreshold(privileged bool) int {
	t := LoginCaptchaThreshold
	if privileged && AdminLoginStricter {
		t = (t + 1) / 2
	}
	if t < 1 {
		t = 1
	}
	return t
}

func loginLockThreshold(privileged bool) int {
	t := LoginLockThreshold
	if privileged && AdminLoginStricter {
		t = (t + 1) / 2
	}
	if t < 2 {
		t = 2
	}
	return t
}

// loginLockDuration 根据累计失败次数与锁定阈值计算本次硬锁定时长（秒），随档位递增。
func loginLockDuration(failCount, lockThreshold int, privileged bool) int64 {
	if failCount < lockThreshold {
		return 0
	}
	tier := failCount / lockThreshold // 1, 2, 3...
	var mult int64
	switch {
	case tier <= 1:
		mult = 1 // 基础（默认 15min）
	case tier == 2:
		mult = 4 // 默认 1h
	default:
		mult = 96 // 默认 24h
	}
	dur := LoginLockBaseDuration * mult
	if privileged && AdminLoginStricter {
		dur *= 2
	}
	return dur
}

// LoginLockStatus 返回该标识当前是否被硬锁定及剩余秒数。
func LoginLockStatus(identifier string) (locked bool, retryAfterSec int) {
	if !LoginThrottleEnable {
		return false, 0
	}
	id := NormalizeLoginIdentifier(identifier)
	if id == "" {
		return false, 0
	}
	now := time.Now().Unix()
	if RedisEnabled {
		val, err := RDB.Get(context.Background(), loginLockKeyPrefix+id).Result()
		if err != nil {
			return false, 0 // 未锁 / redis.Nil；权威态由 DB 列兜底
		}
		until, _ := strconv.ParseInt(val, 10, 64)
		if until > now {
			return true, int(until - now)
		}
		return false, 0
	}
	loginThrottleMu.Lock()
	defer loginThrottleMu.Unlock()
	if e := loginThrottleStore[id]; e != nil && e.lockedUntil > now {
		return true, int(e.lockedUntil - now)
	}
	return false, 0
}

// loginCurrentFailCount 读取当前失败计数（不自增）。
func loginCurrentFailCount(id string) int {
	if RedisEnabled {
		val, err := RDB.Get(context.Background(), loginFailKeyPrefix+id).Result()
		if err != nil {
			return 0
		}
		n, _ := strconv.Atoi(val)
		return n
	}
	loginThrottleMu.Lock()
	defer loginThrottleMu.Unlock()
	e := loginThrottleStore[id]
	if e == nil {
		return 0
	}
	now := time.Now().Unix()
	if now-e.windowStart > LoginFailWindow && now-e.lockedUntil > LoginFailWindow {
		return 0
	}
	return e.failCount
}

// LoginCaptchaRequired 当前标识失败次数是否已达到验证码阈值。
func LoginCaptchaRequired(identifier string, privileged bool) bool {
	if !LoginThrottleEnable {
		return false
	}
	id := NormalizeLoginIdentifier(identifier)
	if id == "" {
		return false
	}
	return loginCurrentFailCount(id) >= loginCaptchaThreshold(privileged)
}

// RecordLoginFailure 记录一次登录失败，返回更新后的失败次数、是否触发硬锁定、锁定剩余秒。
func RecordLoginFailure(identifier string, privileged bool) (failCount int, locked bool, retryAfterSec int) {
	if !LoginThrottleEnable {
		return 0, false, 0
	}
	id := NormalizeLoginIdentifier(identifier)
	if id == "" {
		return 0, false, 0
	}
	lockThr := loginLockThreshold(privileged)
	now := time.Now().Unix()

	if RedisEnabled {
		ctx := context.Background()
		failKey := loginFailKeyPrefix + id
		n, err := RDB.Incr(ctx, failKey).Result()
		if err != nil {
			return 0, false, 0 // fail-open，DB 列兜底
		}
		// 每次自增都续期，避免 INCR 后 crash 导致计数键永不过期、永久拉高该账号计数
		RDB.Expire(ctx, failKey, time.Duration(LoginFailWindow)*time.Second)
		failCount = int(n)
		if dur := loginLockDuration(failCount, lockThr, privileged); dur > 0 {
			until := now + dur
			RDB.Set(ctx, loginLockKeyPrefix+id, strconv.FormatInt(until, 10), time.Duration(dur)*time.Second)
			// 计数键续期到「锁定结束后再保留一个窗口」，使持续攻击的失败次数累加、档位递增
			RDB.Expire(ctx, failKey, time.Duration(dur+LoginFailWindow)*time.Second)
			return failCount, true, int(dur)
		}
		return failCount, false, 0
	}

	loginThrottleMu.Lock()
	defer loginThrottleMu.Unlock()
	if len(loginThrottleStore) > loginThrottleMaxKeys {
		loginEvictExpiredLocked(now)
	}
	e := loginThrottleStore[id]
	if e == nil || (now-e.windowStart > LoginFailWindow && now-e.lockedUntil > LoginFailWindow) {
		e = &loginThrottleEntry{windowStart: now}
		loginThrottleStore[id] = e
	}
	e.failCount++
	failCount = e.failCount
	if dur := loginLockDuration(failCount, lockThr, privileged); dur > 0 {
		e.lockedUntil = now + dur
		return failCount, true, int(dur)
	}
	return failCount, false, 0
}

// ResetLoginFailure 登录成功后清零该标识的失败计数与锁定。
func ResetLoginFailure(identifier string) {
	id := NormalizeLoginIdentifier(identifier)
	if id == "" {
		return
	}
	if RedisEnabled {
		ctx := context.Background()
		RDB.Del(ctx, loginFailKeyPrefix+id, loginLockKeyPrefix+id)
		return
	}
	loginThrottleMu.Lock()
	delete(loginThrottleStore, id)
	loginThrottleMu.Unlock()
}

// loginEvictExpiredLocked 清理过期条目（调用方须持锁）。
func loginEvictExpiredLocked(now int64) {
	for k, e := range loginThrottleStore {
		if now-e.windowStart > LoginFailWindow && now-e.lockedUntil > LoginFailWindow {
			delete(loginThrottleStore, k)
		}
	}
}

// RecordGlobalLoginFailure 记录一次全局登录失败（用于跨账号撞库 spray 天花板）。
func RecordGlobalLoginFailure() {
	if !LoginThrottleEnable {
		return
	}
	if RedisEnabled {
		ctx := context.Background()
		n, err := RDB.Incr(ctx, loginGlobalFailKey).Result()
		if err == nil && n == 1 {
			RDB.Expire(ctx, loginGlobalFailKey, time.Duration(LoginGlobalFailWindow)*time.Second)
		}
		return
	}
	now := time.Now().Unix()
	loginGlobalMu.Lock()
	if now-loginGlobalWindowStart > LoginGlobalFailWindow {
		loginGlobalWindowStart = now
		loginGlobalCount = 0
	}
	loginGlobalCount++
	loginGlobalMu.Unlock()
}

// IsGlobalLoginFlood 全局失败是否已超过阈值（此时应对所有登录强制验证码/降级）。
func IsGlobalLoginFlood() bool {
	if !LoginThrottleEnable {
		return false
	}
	if RedisEnabled {
		val, err := RDB.Get(context.Background(), loginGlobalFailKey).Result()
		if err != nil {
			return false
		}
		n, _ := strconv.Atoi(val)
		return n >= LoginGlobalFailMax
	}
	loginGlobalMu.Lock()
	defer loginGlobalMu.Unlock()
	if time.Now().Unix()-loginGlobalWindowStart > LoginGlobalFailWindow {
		return false
	}
	return loginGlobalCount >= LoginGlobalFailMax
}
