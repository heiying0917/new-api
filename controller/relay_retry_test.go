package controller

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newRetryTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func awsRelayErr(msg string, status int) *types.NewAPIError {
	return types.NewOpenAIError(errors.New(msg), types.ErrorCodeAwsInvokeError, status)
}

// 渠道失效类报错不论状态码都要换渠道重试；请求本身的问题换渠道也一样失败，不重试。
func TestShouldRetry_ChannelFailuresRetryRegardlessOfStatus(t *testing.T) {
	cases := []struct {
		name  string
		err   *types.NewAPIError
		retry bool
	}{
		{"Bedrock 账号被封 400 Operation not allowed", awsRelayErr("ValidationException: Operation not allowed", 400), true},
		{"Bedrock 模型 ID 无效 400", awsRelayErr("ValidationException: The provided model identifier is invalid.", 400), true},
		{"Bedrock API key 失效 403", awsRelayErr("AccessDeniedException: Authentication failed: Please make sure your API Key is valid.", 403), true},
		{"限流 429", awsRelayErr("ThrottlingException: Too many requests, please wait before trying again.", 429), true},
		{"上游 503", awsRelayErr("ServiceUnavailableException: Bedrock is unable to process your request.", 503), true},
		{"请求问题：输入过长 400", awsRelayErr("ValidationException: Input is too long for requested model.", 400), false},
		{"网关超时 504 不重试", awsRelayErr("gateway timeout", 504), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.retry, shouldRetry(newRetryTestContext(), tc.err, 3))
		})
	}
}

func TestShouldRetry_ChannelFailureStillHonorsLimits(t *testing.T) {
	err := awsRelayErr("ValidationException: Operation not allowed", 400)

	require.False(t, shouldRetry(newRetryTestContext(), err, 0), "重试次数用完不再重试")

	c := newRetryTestContext()
	c.Set("specific_channel_id", 88)
	require.False(t, shouldRetry(c, err, 3), "指定渠道的请求不换渠道")
}
