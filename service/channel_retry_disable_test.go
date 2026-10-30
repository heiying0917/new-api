package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

// 生产上 AWS Bedrock 渠道实际出现过的报错原文（RequestID 已脱敏）
const (
	bedrockApiKeyInvalidMsg  = "InvokeModelWithResponseStream: operation error Bedrock Runtime: InvokeModelWithResponseStream, https response error StatusCode: 403, RequestID: x, AccessDeniedException: Authentication failed: Please make sure your API Key is valid."
	bedrockSecurityTokenMsg  = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 403, RequestID: x, AccessDeniedException: Model access is denied due to The security token included in the request is invalid.. Your AWS Marketplace subscription for this model cannot be completed at this time."
	bedrockTokenExpiredMsg   = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 403, RequestID: x, ExpiredTokenException: The security token included in the request is expired"
	bedrockAccountBlockedMsg = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 400, RequestID: x, ValidationException: Operation not allowed"
	bedrockModelInvalidMsg   = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 400, RequestID: x, ValidationException: The provided model identifier is invalid."
	bedrockThrottlingMsg     = "InvokeModelWithResponseStream: operation error Bedrock Runtime: InvokeModelWithResponseStream, https response error StatusCode: 429, RequestID: x, ThrottlingException: Too many tokens, please wait before trying again."
	bedrockInputTooLongMsg   = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 400, RequestID: x, ValidationException: Input is too long for requested model."
	bedrockAdaptiveThinkMsg  = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 400, RequestID: x, ValidationException: adaptive thinking is not supported on this model"
	bedrockServiceUnavailMsg = "InvokeModel: operation error Bedrock Runtime: InvokeModel, https response error StatusCode: 503, RequestID: x, ServiceUnavailableException: Bedrock is unable to process your request."
)

func bedrockErr(msg string, status int, opts ...types.NewAPIErrorOptions) *types.NewAPIError {
	return types.NewOpenAIError(errors.New(msg), types.ErrorCodeAwsInvokeError, status, opts...)
}

func TestIsChannelFatalError_BedrockErrors(t *testing.T) {
	cases := []struct {
		name             string
		err              *types.NewAPIError
		fatal            bool
		modelUnavailable bool
	}{
		{"api key 失效", bedrockErr(bedrockApiKeyInvalidMsg, 403), true, false},
		{"security token 无效", bedrockErr(bedrockSecurityTokenMsg, 403), true, false},
		{"security token 过期", bedrockErr(bedrockTokenExpiredMsg, 403), true, false},
		{"账号被封 400", bedrockErr(bedrockAccountBlockedMsg, 400), true, false},
		{"任意 401", bedrockErr("unauthorized", 401), true, false},
		{"模型 ID 无效：不按关键词直接禁用，单独走熔断", bedrockErr(bedrockModelInvalidMsg, 400), false, true},
		{"限流 429 不是渠道失效", bedrockErr(bedrockThrottlingMsg, 429), false, false},
		{"上游临时故障 503", bedrockErr(bedrockServiceUnavailMsg, 503), false, false},
		{"请求本身问题：输入过长", bedrockErr(bedrockInputTooLongMsg, 400), false, false},
		{"请求本身问题：模型不支持参数", bedrockErr(bedrockAdaptiveThinkMsg, 400), false, false},
		{"客户端已断开（skip retry）即使文本像 key 失效也不算", bedrockErr(bedrockApiKeyInvalidMsg, 403, types.ErrOptionWithSkipRetry()), false, false},
		{"nil", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.fatal, IsChannelFatalError(tc.err))
			require.Equal(t, tc.modelUnavailable, IsModelUnavailableError(tc.err))
		})
	}
}

func TestShouldDisableChannel_RespectsGlobalSwitch(t *testing.T) {
	prev := common.AutomaticDisableChannelEnabled
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = prev })

	err := bedrockErr(bedrockApiKeyInvalidMsg, 403)
	common.AutomaticDisableChannelEnabled = false
	require.False(t, ShouldDisableChannel(err), "总开关关闭时不禁用")
	require.True(t, IsChannelFatalError(err), "但仍判定为渠道失效，供重试使用")

	common.AutomaticDisableChannelEnabled = true
	require.True(t, ShouldDisableChannel(err))
	require.False(t, ShouldDisableChannel(bedrockErr(bedrockThrottlingMsg, 429)))
	require.False(t, ShouldDisableChannel(bedrockErr(bedrockModelInvalidMsg, 400)), "模型 ID 无效不走直接禁用")
}

func TestUnavailableModelDisableBreaker_InMemory(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	b := &unavailableModelDisableBreaker{
		entries: map[string]*unavailableModelDisableEntry{},
		now:     func() time.Time { return now },
	}

	for id := 1; id <= unavailableModelDisableLimit; id++ {
		require.True(t, b.allowInMemory("claude-x", id), "窗口内前 %d 个渠道允许禁用", unavailableModelDisableLimit)
	}
	require.False(t, b.allowInMemory("claude-x", 99), "超出上限熔断")
	require.True(t, b.allowInMemory("claude-x", 1), "已记录渠道重复报错仍允许（禁用幂等，不占名额）")
	require.True(t, b.allowInMemory("claude-y", 99), "按模型独立计数")

	now = now.Add(unavailableModelDisableWindow)
	require.True(t, b.allowInMemory("claude-x", 99), "窗口过期后重新计数")
}

// TestAllowUnavailableModelDisable_Redis 真 Redis：CHANNEL_CONCURRENCY_TEST_REDIS=redis://host:6379/15 时运行，请勿指向生产库。
func TestAllowUnavailableModelDisable_Redis(t *testing.T) {
	url := os.Getenv("CHANNEL_CONCURRENCY_TEST_REDIS")
	if url == "" {
		t.Skip("set CHANNEL_CONCURRENCY_TEST_REDIS to run against a real Redis")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })
	prevRDB, prevEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = rdb, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = prevRDB, prevEnabled })

	modelName := "breaker-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	key := unavailableModelDisableKeyPrefix + modelName
	t.Cleanup(func() { rdb.Del(context.Background(), key) })

	for id := 1; id <= unavailableModelDisableLimit; id++ {
		require.True(t, AllowUnavailableModelDisable(modelName, id))
	}
	require.False(t, AllowUnavailableModelDisable(modelName, 99))
	require.True(t, AllowUnavailableModelDisable(modelName, 2))

	ttl, err := rdb.PTTL(context.Background(), key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0), "计数键必须带过期，作为时间窗")
	require.LessOrEqual(t, ttl, unavailableModelDisableWindow)
}

func selectWithFailedForTest(t *testing.T, retry int, failed ...int) *model.Channel {
	t.Helper()
	c := newConcurrencyTestContext()
	param := &RetryParam{Ctx: c, TokenGroup: "g", ModelName: "m", Retry: common.GetPointer(retry)}
	for _, id := range failed {
		param.MarkChannelFailed(id)
	}
	ch, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	ReleaseChannelSlot(c)
	return ch
}

// 种子渠道：1、2 在 10 档（上限 1），3 在 0 档（不限）
func runSkipFailedChannelsScenario(t *testing.T) {
	resetMemChannelInflight(t)

	for i := 0; i < 30; i++ {
		ch := selectWithFailedForTest(t, 0, 1)
		require.NotNil(t, ch)
		require.Equal(t, 2, ch.Id, "同档还有没失败过的渠道时，不能再选回刚失败的渠道")
	}

	ch := selectWithFailedForTest(t, 0, 1, 2)
	require.NotNil(t, ch)
	require.Equal(t, 3, ch.Id, "高档渠道都失败过，降到下一档")

	ch = selectWithFailedForTest(t, 1, 1, 2, 3)
	require.NotNil(t, ch, "所有渠道都失败过：退回原规则照常选路，不报无可用渠道")
}

func TestCacheGetRandomSatisfiedChannel_SkipsFailedChannels_MemoryCache(t *testing.T) {
	seedConcurrencySelectChannels(t, true)
	runSkipFailedChannelsScenario(t)
}

func TestCacheGetRandomSatisfiedChannel_SkipsFailedChannels_Database(t *testing.T) {
	seedConcurrencySelectChannels(t, false)
	runSkipFailedChannelsScenario(t)
}
