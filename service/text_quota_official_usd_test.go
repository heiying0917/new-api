package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOfficialUsdTestCtx(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return ctx
}

func claudeRelayInfo(priceData types.PriceData) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatClaude,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-opus-5",
		PriceData:               priceData,
		StartTime:               time.Now(),
	}
}

func openaiRelayInfo(priceData types.PriceData) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
		OriginModelName:         "gpt-4o",
		PriceData:               priceData,
		StartTime:               time.Now(),
	}
}

// 生产真实日志（2026-09-17 渠道 22）：prompt 2 / completion 546 / cache_read 78919 / cache_write 578(5m)，
// model_ratio 2.5 / completion_ratio 5 / cache_ratio 0.1 / cache_creation_ratio 1.25。
func prodClaudeCacheUsage() *dto.Usage {
	return &dto.Usage{
		PromptTokens:     2,
		CompletionTokens: 546,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         78919,
			CachedCreationTokens: 578,
		},
		ClaudeCacheCreation5mTokens: 578,
	}
}

func prodClaudePriceData(groupRatio float64) types.PriceData {
	return types.PriceData{
		ModelRatio:           2.5,
		CompletionRatio:      5,
		CacheRatio:           0.1,
		CacheCreationRatio:   1.25,
		CacheCreation5mRatio: 1.25,
		CacheCreation1hRatio: 2,
		GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: groupRatio},
	}
}

// 官方价必须包含缓存读/写 token（Claude 口径下它们不在 prompt_tokens 里），且与分组倍率无关：
// (2 + 78919×0.1 + 578×1.25 + 546×5) × 2.5 = 28366 官方额度 → $0.056732。
func TestCalculateTextQuotaSummaryOfficialUsdIncludesClaudeCacheTokens(t *testing.T) {
	ctx := newOfficialUsdTestCtx(t)

	summary := calculateTextQuotaSummary(ctx, claudeRelayInfo(prodClaudePriceData(1)), prodClaudeCacheUsage())
	require.True(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, 28366, summary.Quota)
	require.InDelta(t, 28366.0/common.QuotaPerUnit, summary.OfficialUsd, 1e-9)

	// 分组倍率 3.3：quota 随之放大，官方价不变
	summary = calculateTextQuotaSummary(ctx, claudeRelayInfo(prodClaudePriceData(3.3)), prodClaudeCacheUsage())
	require.Equal(t, 93608, summary.Quota)
	require.InDelta(t, 28366.0/common.QuotaPerUnit, summary.OfficialUsd, 1e-9)
}

// OpenAI 口径 prompt_tokens 已含缓存命中 token：官方价须按 cache_ratio 折价，而不是全价。
// (600 + 400×0.5 + 100×2) × 1 = 1000 官方额度；quota 再乘分组倍率 2。
func TestCalculateTextQuotaSummaryOfficialUsdDiscountsOpenAICachedTokens(t *testing.T) {
	ctx := newOfficialUsdTestCtx(t)
	usage := &dto.Usage{
		PromptTokens:        1000,
		CompletionTokens:    100,
		PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 400},
	}
	priceData := types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
		CacheRatio:      0.5,
		GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 2},
	}

	summary := calculateTextQuotaSummary(ctx, openaiRelayInfo(priceData), usage)
	require.False(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, 2000, summary.Quota)
	require.InDelta(t, 1000.0/common.QuotaPerUnit, summary.OfficialUsd, 1e-9)
}

// 免费分组（倍率 0）用户额度为 0，但供应商真实服务了请求，官方价必须照算。
func TestCalculateTextQuotaSummaryOfficialUsdSurvivesZeroGroupRatio(t *testing.T) {
	ctx := newOfficialUsdTestCtx(t)
	usage := &dto.Usage{
		PromptTokens:        1000,
		CompletionTokens:    100,
		PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 400},
	}
	priceData := types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
		CacheRatio:      0.5,
		GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 0},
	}

	summary := calculateTextQuotaSummary(ctx, openaiRelayInfo(priceData), usage)
	require.Equal(t, 0, summary.Quota)
	require.InDelta(t, 1000.0/common.QuotaPerUnit, summary.OfficialUsd, 1e-9)
}

// 按次计费：官方价就是 model_price，不含分组倍率。
func TestCalculateTextQuotaSummaryOfficialUsdUsesModelPriceWhenFixedPrice(t *testing.T) {
	ctx := newOfficialUsdTestCtx(t)
	usage := &dto.Usage{PromptTokens: 10, CompletionTokens: 10}
	priceData := types.PriceData{
		UsePrice:       true,
		ModelPrice:     0.04,
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 2},
	}

	summary := calculateTextQuotaSummary(ctx, openaiRelayInfo(priceData), usage)
	require.Equal(t, 40000, summary.Quota)
	require.InDelta(t, 0.04, summary.OfficialUsd, 1e-9)
}

// 模型自身的附加倍率（思考/分辨率等）属于官方定价的一部分，官方价须一并计入。
func TestCalculateTextQuotaSummaryOfficialUsdAppliesOtherRatios(t *testing.T) {
	ctx := newOfficialUsdTestCtx(t)
	usage := &dto.Usage{PromptTokens: 1000}
	priceData := types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 1,
		GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 2},
	}
	priceData.AddOtherRatio("thinking", 2)

	summary := calculateTextQuotaSummary(ctx, openaiRelayInfo(priceData), usage)
	require.Equal(t, 4000, summary.Quota)
	require.InDelta(t, 2000.0/common.QuotaPerUnit, summary.OfficialUsd, 1e-9)
}

// 上游没有返回计费信息（total_tokens=0）：不扣费，也不应给供应商记官方价。
func TestCalculateTextQuotaSummaryOfficialUsdZeroWithoutUsage(t *testing.T) {
	ctx := newOfficialUsdTestCtx(t)

	summary := calculateTextQuotaSummary(ctx, claudeRelayInfo(prodClaudePriceData(1)), &dto.Usage{})
	require.Equal(t, 0, summary.Quota)
	require.Equal(t, 0.0, summary.OfficialUsd)
}

// 端到端：供应商渠道走 PostTextConsumeQuota 后，落库的 official_usd 必须含缓存 token（此前只记了 $0.01366）。
func TestPostTextConsumeQuotaRecordsOfficialUsdWithCacheTokensForSupplierChannel(t *testing.T) {
	truncate(t)
	common.MemoryCacheEnabled = false // CacheGetChannel 回退查 DB
	cp := 2.2
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: 92022, Name: "op5-1", Key: "k", SupplierId: 777, CostPrice: &cp,
		Status: 1, Models: "claude-opus-5", Group: "Claude官key",
	}).Error)

	ctx := newOfficialUsdTestCtx(t)
	relayInfo := claudeRelayInfo(prodClaudePriceData(1))
	relayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 92022}
	relayInfo.UserId = 1
	relayInfo.TokenId = 4
	relayInfo.UsingGroup = "Claude官key"
	relayInfo.IsStream = true

	PostTextConsumeQuota(ctx, relayInfo, prodClaudeCacheUsage(), nil)

	var l model.Log
	require.NoError(t, model.LOG_DB.Where("channel_id = ?", 92022).Order("id desc").First(&l).Error)
	require.Equal(t, 28366, l.Quota)
	require.InDelta(t, 28366.0/common.QuotaPerUnit, l.OfficialUsd, 1e-9)
	require.InDelta(t, 2.2, l.CostPriceSnapshot, 1e-9)
}
