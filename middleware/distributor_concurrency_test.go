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

// setupDistributorConcurrencyDB 渠道 41（分组 default，模型 m，并发上限 1），内存缓存调度。
func setupDistributorConcurrencyDB(t *testing.T) {
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

	p0, one := int64(0), 1
	require.NoError(t, db.Create(&model.Channel{Id: 41, Name: "c41", Key: "k", Type: constant.ChannelTypeOpenAI, Priority: &p0, MaxConcurrency: &one, Models: "m", Group: "default", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "m", ChannelId: 41, Enabled: true, Priority: &p0}).Error)
	model.InitChannelCache()
}

// 并发占位贯穿整个请求：处理中持有，满载的第二个请求得到 429 channel_concurrency_limited，处理结束后名额归还。
func TestDistribute_ConcurrencyLimitReturns429AndReleasesAfterRequest(t *testing.T) {
	setupDistributorConcurrencyDB(t)

	entered := make(chan struct{})
	finish := make(chan struct{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	}, Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		require.True(t, service.HoldsChannelSlot(c, 41), "处理请求期间必须持有渠道占位")
		if c.GetHeader("X-Block") == "1" {
			entered <- struct{}{}
			<-finish
		}
		c.Status(http.StatusOK)
	})
	send := func(block bool) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[]}`))
		req.Header.Set("Content-Type", "application/json")
		if block {
			req.Header.Set("X-Block", "1")
		}
		router.ServeHTTP(w, req)
		return w
	}

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- send(true) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first request never reached the handler")
	}
	require.Equal(t, 1, service.GetChannelConcurrencyCounts([]int{41})[41])

	w := send(false)
	require.Equal(t, http.StatusTooManyRequests, w.Code, "resp=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"code":"channel_concurrency_limited"`)
	require.NotContains(t, w.Body.String(), "distributor.channel_concurrency_limited", "i18n 文案必须能解析，不能把 key 原样返回")

	close(finish)
	require.Equal(t, http.StatusOK, (<-firstDone).Code)
	require.Empty(t, service.GetChannelConcurrencyCounts([]int{41}), "请求结束后必须归还名额")

	require.Equal(t, http.StatusOK, send(false).Code, "名额归还后可再次处理")
	require.Empty(t, service.GetChannelConcurrencyCounts([]int{41}))
}
