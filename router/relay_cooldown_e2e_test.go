package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const e2eTokenKey = "e2ecooldowntokenkey0000000000000000000000000000"

var e2eRunSeq atomic.Int64

// fakeUpstream OpenAI 兼容假上游：前 okLimit 个请求成功，之后回 429（okLimit<0 表示永不限流）；每个请求耗时 delay。
type fakeUpstream struct {
	srv      *httptest.Server
	okLimit  int64
	hits     atomic.Int64
	limited  atomic.Int64
	inflight atomic.Int64
	maxInfl  atomic.Int64
}

func newFakeUpstream(t *testing.T, okLimit int64, delay time.Duration) *fakeUpstream {
	u := &fakeUpstream{okLimit: okLimit}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := u.hits.Add(1)
		cur := u.inflight.Add(1)
		defer u.inflight.Add(-1)
		for {
			m := u.maxInfl.Load()
			if cur <= m || u.maxInfl.CompareAndSwap(m, cur) {
				break
			}
		}
		time.Sleep(delay)
		w.Header().Set("Content-Type", "application/json")
		if u.okLimit >= 0 && n > u.okLimit {
			u.limited.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"Rate limit reached for requests","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func setupRelayE2E(t *testing.T, upstreams map[int]*fakeUpstream) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false

	type saved struct {
		redis, cache       bool
		retry, rlRetry, cd int
		batch              bool
	}
	prev := saved{common.RedisEnabled, common.MemoryCacheEnabled, common.RetryTimes, common.RateLimitRetryTimes, common.RateLimitCooldownSeconds, common.BatchUpdateEnabled}
	common.RedisEnabled, common.MemoryCacheEnabled = false, true
	// 冷却 30 秒：本测试校验的是「冷却期内绕开」，留足时序余量（-race / 慢机器下 20 个串行请求也在冷却期内）
	common.RetryTimes, common.RateLimitRetryTimes, common.RateLimitCooldownSeconds = 10, 5, 30
	common.BatchUpdateEnabled = false
	// 只写回有变化的开关：转发路径的异步协程（计费、日志）在测试结束时可能仍在读这些全局变量
	t.Cleanup(func() {
		if common.RedisEnabled != prev.redis {
			common.RedisEnabled = prev.redis
		}
		if common.MemoryCacheEnabled != prev.cache {
			common.MemoryCacheEnabled = prev.cache
		}
		if common.BatchUpdateEnabled != prev.batch {
			common.BatchUpdateEnabled = prev.batch
		}
		common.RetryTimes, common.RateLimitRetryTimes, common.RateLimitCooldownSeconds = prev.retry, prev.rlRetry, prev.cd
	})

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // 内存 SQLite 并发写会报 locked：串行化即可，本测试关注选路而非 DB 并发
	prevDB, prevLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = prevDB, prevLogDB
		_ = sqlDB.Close()
	})
	model.InitColNames()
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Token{}, &model.User{}, &model.Supplier{}, &model.Option{},
		&model.Ability{}, &model.Log{}, &model.QuotaData{}, &model.PerfMetric{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}))
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"

	savedRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { _ = ratio_setting.UpdateModelRatioByJSONString(savedRatios) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o-mini":0.075}`))

	require.NoError(t, db.Create(&model.User{Id: 1, Username: "e2e", Password: "x", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 1_000_000_000, Group: "default", AffCode: "e2e"}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: e2eTokenKey, Status: common.TokenStatusEnabled, Name: "e2e", ExpiredTime: -1, UnlimitedQuota: true}).Error)
	p0 := int64(0)
	for id, up := range upstreams {
		base := up.srv.URL
		require.NoError(t, db.Create(&model.Channel{Id: id, Name: fmt.Sprintf("c%d", id), Key: fmt.Sprintf("k%d", id), Type: constant.ChannelTypeOpenAI, BaseURL: &base, Priority: &p0, Models: "gpt-4o-mini", Group: "default", Status: common.ChannelStatusEnabled}).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-4o-mini", ChannelId: id, Enabled: true, Priority: &p0}).Error)
	}
	model.InitChannelCache()
	service.InitHttpClient()

	engine := gin.New()
	SetRelayRouter(engine)
	return engine
}

func sendChat(engine *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-"+e2eTokenKey)
	engine.ServeHTTP(w, req)
	return w
}

// 场景：50 个并发打到同档的 3 个渠道，其中渠道 61 的上游前 10 个请求成功、之后一律 429（模拟账号 × 模型限流）。
// 期望：同档按在飞数均摊；61 的 429 被换渠道重试接住，50 个请求全部成功；61 进入冷却，冷却期内不再收到请求。
func TestRelay_ConcurrentBurstSurvivesUpstream429WithCooldown(t *testing.T) {
	// 每次运行用新的渠道 id：冷却状态在进程内保留 5 秒，-count>1 连跑时不能沿用上一次的冷却
	base := int(e2eRunSeq.Add(1)) * 100
	limitedID, okID1, okID2 := base+61, base+62, base+63
	ups := map[int]*fakeUpstream{
		limitedID: newFakeUpstream(t, 10, 150*time.Millisecond),
		okID1:     newFakeUpstream(t, -1, 150*time.Millisecond),
		okID2:     newFakeUpstream(t, -1, 150*time.Millisecond),
	}
	engine := setupRelayE2E(t, ups)

	const burst = 50
	codes := make([]int, burst)
	bodies := make([]string, burst)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			w := sendChat(engine)
			codes[i], bodies[i] = w.Code, w.Body.String()
		}(i)
	}
	close(start)
	wg.Wait()

	for i, code := range codes {
		require.Equal(t, http.StatusOK, code, "请求 %d 失败：%s", i, bodies[i])
	}
	t.Logf("hits: 限流渠道=%d(429×%d) 渠道2=%d 渠道3=%d; 最大在飞: %d/%d/%d",
		ups[limitedID].hits.Load(), ups[limitedID].limited.Load(), ups[okID1].hits.Load(), ups[okID2].hits.Load(),
		ups[limitedID].maxInfl.Load(), ups[okID1].maxInfl.Load(), ups[okID2].maxInfl.Load())
	require.Greater(t, ups[limitedID].limited.Load(), int64(0), "场景要求限流渠道确实触发过 429，重试路径被走到")
	require.GreaterOrEqual(t, ups[okID1].hits.Load(), int64(12), "同档按在飞数均摊，不应堆在少数渠道")
	require.GreaterOrEqual(t, ups[okID2].hits.Load(), int64(12), "同档按在飞数均摊，不应堆在少数渠道")

	// 冷却期内新请求不再打到限流渠道
	before := ups[limitedID].hits.Load()
	for i := 0; i < 20; i++ {
		w := sendChat(engine)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	require.Equal(t, before, ups[limitedID].hits.Load(), "限流渠道冷却中，新请求必须绕开")
}
