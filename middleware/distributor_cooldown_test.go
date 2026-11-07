package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupDistributorCooldownDB 分组 default、模型 claude-cooldown-test 下的同档渠道（不限并发），内存缓存调度。
func setupDistributorCooldownDB(t *testing.T, channels ...model.Channel) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	prevRedis, prevCache := common.RedisEnabled, common.MemoryCacheEnabled
	common.RedisEnabled, common.MemoryCacheEnabled = false, true
	t.Cleanup(func() { common.RedisEnabled, common.MemoryCacheEnabled = prevRedis, prevCache })

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	prevDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	model.InitColNames()
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Supplier{}))
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["DispatchStrategy"] = "priority"
	p0 := int64(0)
	for _, ch := range channels {
		ch.Priority = &p0
		ch.Type = constant.ChannelTypeAnthropic
		ch.Models = cooldownTestModel
		ch.Group = "default"
		ch.Status = common.ChannelStatusEnabled
		require.NoError(t, db.Create(&ch).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: cooldownTestModel, ChannelId: ch.Id, Enabled: true, Priority: &p0}).Error)
	}
	model.InitChannelCache()
	service.ClearChannelAffinityCacheAll()
	t.Cleanup(func() { service.ClearChannelAffinityCacheAll() })
}

const cooldownTestModel = "claude-cooldown-test"

// newCooldownTestRouter 记录每次请求实际落到的渠道与 key。
func newCooldownTestRouter(picked chan<- [2]string) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	}, Distribute())
	router.POST("/v1/messages", func(c *gin.Context) {
		picked <- [2]string{fmt.Sprint(common.GetContextKeyInt(c, constant.ContextKeyChannelId)), common.GetContextKeyString(c, constant.ContextKeyChannelKey)}
		c.Status(http.StatusOK)
	})
	return router
}

func sendClaudeCodeRequest(router *gin.Engine, userID string) int {
	w := httptest.NewRecorder()
	body := fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":%q}}`, cooldownTestModel, userID)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w.Code
}

// 亲和首选渠道 429 冷却中：本次走正常选路换到别的渠道（冷却是暂时的，与并发满载同样处理）。
func TestDistribute_AffinityChannelCoolingFallsBack(t *testing.T) {
	setupDistributorCooldownDB(t, model.Channel{Id: 51, Name: "c51", Key: "k51"}, model.Channel{Id: 52, Name: "c52", Key: "k52"})
	picked := make(chan [2]string, 64)
	router := newCooldownTestRouter(picked)

	require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, "session-a"))
	first := (<-picked)[0]
	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, "session-a"))
		require.Equal(t, first, (<-picked)[0], "亲和生效：同一会话固定落在首个渠道")
	}

	firstID := 51
	key := "k51"
	if first == "52" {
		firstID, key = 52, "k52"
	}
	service.MarkRateLimitCooldown(nil, firstID, key, cooldownTestModel)
	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, "session-a"))
		require.NotEqual(t, first, (<-picked)[0], "亲和渠道冷却中，必须换渠道")
	}
}

// 多 key 渠道：选 key 时跳过对该模型冷却中的 key。
func TestDistribute_MultiKeyChannelSkipsCoolingKey(t *testing.T) {
	setupDistributorCooldownDB(t, model.Channel{
		Id: 53, Name: "c53", Key: "ka\nkb\nkc",
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 3, MultiKeyMode: constant.MultiKeyModeRandom},
	})
	picked := make(chan [2]string, 64)
	router := newCooldownTestRouter(picked)

	service.MarkRateLimitCooldown(nil, 53, "ka", cooldownTestModel)
	service.MarkRateLimitCooldown(nil, 53, "kc", cooldownTestModel)
	for i := 0; i < 20; i++ {
		require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, fmt.Sprintf("s-%d-%d", i, time.Now().UnixNano())))
		require.Equal(t, [2]string{"53", "kb"}, <-picked)
	}
}
