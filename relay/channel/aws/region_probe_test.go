package aws

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/require"
)

func TestBuildBedrockRuntimeClient(t *testing.T) {
	hc := &http.Client{}

	akskClient, err := BuildBedrockRuntimeClient(dto.AwsKeyTypeAKSK, "ak", "sk", "", "us-east-1", hc)
	require.NoError(t, err)
	require.NotNil(t, akskClient)
	require.Equal(t, "us-east-1", akskClient.Options().Region)

	apiKeyClient, err := BuildBedrockRuntimeClient(dto.AwsKeyTypeApiKey, "", "", "api-key", "eu-west-1", hc)
	require.NoError(t, err)
	require.NotNil(t, apiKeyClient)
	require.Equal(t, "eu-west-1", apiKeyClient.Options().Region)

	_, err = BuildBedrockRuntimeClient(dto.AwsKeyTypeAKSK, "", "", "", "us-east-1", hc)
	require.Error(t, err)

	_, err = BuildBedrockRuntimeClient(dto.AwsKeyTypeApiKey, "", "", "", "us-east-1", hc)
	require.Error(t, err)

	_, err = BuildBedrockRuntimeClient(dto.AwsKeyTypeAKSK, "ak", "sk", "", "", hc)
	require.Error(t, err)
}

func TestClassifyRegionProbe(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{http.StatusOK, true},              // 200 凭证可用
		{http.StatusTooManyRequests, true}, // 429 限流也算可用
		{http.StatusBadRequest, false},     // 400 模型不支持该区域
		{http.StatusForbidden, false},      // 403 未授予访问
		{http.StatusUnauthorized, false},   // 401 签名/凭证错误
		{http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		require.Equalf(t, tc.want, ClassifyRegionProbe(tc.code), "status %d", tc.code)
	}
}

func TestResolveModelIDForRegion(t *testing.T) {
	// 支持跨区的模型按 region 前缀加 us./eu./apac.
	require.Equal(t, "us.anthropic.claude-opus-4-6-v1", ResolveModelIDForRegion("claude-opus-4-6", "us-east-1"))
	require.Equal(t, "eu.anthropic.claude-opus-4-6-v1", ResolveModelIDForRegion("claude-opus-4-6", "eu-west-1"))
	require.Equal(t, "apac.anthropic.claude-opus-4-6-v1", ResolveModelIDForRegion("claude-opus-4-6", "ap-northeast-1"))

	// 该 region 前缀不在白名单 => 返回 base ID（haiku-3-5 仅 us）
	require.Equal(t, "anthropic.claude-3-5-haiku-20241022-v1:0", ResolveModelIDForRegion("claude-3-5-haiku-20241022", "eu-west-1"))
	require.Equal(t, "us.anthropic.claude-3-5-haiku-20241022-v1:0", ResolveModelIDForRegion("claude-3-5-haiku-20241022", "us-east-1"))

	// 未知模型名直接透传
	require.Equal(t, "some-unknown-model", ResolveModelIDForRegion("some-unknown-model", "us-east-1"))
}

func TestAwsErrorCode(t *testing.T) {
	// AWS API 错误 => 提取短错误码
	apiErr := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "no access"}
	require.Equal(t, "AccessDeniedException", AwsErrorCode(apiErr))

	// 被包裹的 AWS API 错误 => 仍能提取
	wrapped := fmt.Errorf("operation error Bedrock Runtime: %w", apiErr)
	require.Equal(t, "AccessDeniedException", AwsErrorCode(wrapped))

	// 非 AWS API 错误 => 空串（调用方回退完整错误串）
	require.Equal(t, "", AwsErrorCode(errors.New("dial tcp: i/o timeout")))
	require.Equal(t, "", AwsErrorCode(nil))
}
