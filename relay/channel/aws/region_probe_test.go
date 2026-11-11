package aws

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
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

// 期望值来自 Bedrock ListInferenceProfiles 实拉（2026-11-09，账号默认启用的 17 个区域）：
// Claude 4.5+ 在 us-*/eu-* 有 us./eu. 地理配置文件，APAC 全部区域没有 apac.（只有 global. 及部分 jp./au./in.），
// sa-east-1 只有 global.，ca-central-1 有 us./global.；apac. 只剩 Claude 3.x 老模型。
func TestResolveModelIDForRegion(t *testing.T) {
	cases := []struct {
		model, region, want string
	}{
		// 有地理跨区配置文件的区域：us./eu.（与原行为一致）
		{"claude-opus-4-6", "us-east-1", "us.anthropic.claude-opus-4-6-v1"},
		{"claude-opus-4-6", "eu-west-1", "eu.anthropic.claude-opus-4-6-v1"},
		{"claude-haiku-4-5-20251001", "us-west-1", "us.anthropic.claude-haiku-4-5-20251001-v1:0"},
		{"claude-haiku-4-5-20251001", "eu-north-1", "eu.anthropic.claude-haiku-4-5-20251001-v1:0"},
		// Claude 4.5+ 没有 apac. 配置文件（曾拼出 apac.xxx => 400 The provided model identifier is invalid），回退 global.
		{"claude-opus-4-6", "ap-northeast-1", "global.anthropic.claude-opus-4-6-v1"},
		{"claude-haiku-4-5-20251001", "ap-northeast-2", "global.anthropic.claude-haiku-4-5-20251001-v1:0"},
		{"claude-sonnet-4-6", "ap-southeast-1", "global.anthropic.claude-sonnet-4-6"},
		{"claude-sonnet-4-5-20250929", "ap-south-1", "global.anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{"claude-opus-4-5-20251101", "ap-southeast-2", "global.anthropic.claude-opus-4-5-20251101-v1:0"},
		// 默认启用但没有 us./eu./apac. 前缀映射的区域：回退 global.，不能用 base ID（按需调用会 400）
		{"claude-haiku-4-5-20251001", "sa-east-1", "global.anthropic.claude-haiku-4-5-20251001-v1:0"},
		{"claude-haiku-4-5-20251001", "ca-central-1", "global.anthropic.claude-haiku-4-5-20251001-v1:0"},
		// Claude 5 系列同样只能经推理配置文件调用：sonnet-5/opus-5 有 us./eu.，fable-5-1 只有 us.，其余回退 global.
		{"claude-sonnet-5", "us-east-1", "us.anthropic.claude-sonnet-5"},
		{"claude-opus-5", "eu-central-1", "eu.anthropic.claude-opus-5"},
		{"claude-opus-5", "ap-northeast-1", "global.anthropic.claude-opus-5"},
		{"claude-fable-5-1", "us-west-2", "us.anthropic.claude-fable-5-1"},
		{"claude-fable-5-1", "eu-west-1", "global.anthropic.claude-fable-5-1"},
		// Claude 3.x 老模型仍保留 apac.
		{"claude-3-5-sonnet-20241022", "ap-northeast-1", "apac.anthropic.claude-3-5-sonnet-20241022-v2:0"},
		// 该 region 前缀不在白名单且无 global 配置文件 => 返回 base ID（haiku-3-5 仅 us）
		{"claude-3-5-haiku-20241022", "eu-west-1", "anthropic.claude-3-5-haiku-20241022-v1:0"},
		{"claude-3-5-haiku-20241022", "us-east-1", "us.anthropic.claude-3-5-haiku-20241022-v1:0"},
		// 未知模型名直接透传
		{"some-unknown-model", "us-east-1", "some-unknown-model"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, ResolveModelIDForRegion(tc.model, tc.region, false), "%s @ %s", tc.model, tc.region)
	}

	// 强制 Global：与 relay 一致，无条件加 global.
	require.Equal(t, "global.anthropic.claude-opus-4-6-v1", ResolveModelIDForRegion("claude-opus-4-6", "us-east-1", true))
	require.Equal(t, "global.anthropic.claude-haiku-4-5-20251001-v1:0", ResolveModelIDForRegion("claude-haiku-4-5", "eu-west-1", true))
}

// AWS 账号默认启用的 17 个商用区域（EC2 DescribeRegions 实测）。
var awsDefaultEnabledRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2", "ca-central-1", "sa-east-1",
	"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-south-1", "ap-southeast-1", "ap-southeast-2",
}

// 只能经推理配置文件调用（ListFoundationModels: inferenceTypesSupported=[INFERENCE_PROFILE]）的模型，
// 在任何默认区域都不能解析成 base ID（按需调用必然 400），也不能拼出不存在的 apac.。
func TestInferenceProfileOnlyModelsNeverResolveToBaseID(t *testing.T) {
	for model := range awsModelGlobalProfileModels {
		for _, region := range awsDefaultEnabledRegions {
			got := ResolveModelIDForRegion(model, region, false)
			require.NotEqual(t, model, got, "%s @ %s resolved to base ID", model, region)
			require.Falsef(t, strings.HasPrefix(got, "apac."), "%s @ %s resolved to non-existent apac. profile %s", model, region, got)
		}
	}
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

// 区域测试要把 AWS 给出的原因展示出来：只有错误码（如 ResourceNotFoundException）看不出是
// 「账号未提交 Anthropic 用例表单」还是「模型下线 / Legacy」。
func TestAwsErrorMessage(t *testing.T) {
	const useCase = "Model use case details have not been submitted for this account."
	sdkErr := &smithy.OperationError{
		ServiceID:     "Bedrock Runtime",
		OperationName: "InvokeModel",
		Err:           &types.ResourceNotFoundException{Message: aws.String(useCase)},
	}
	require.Equal(t, useCase, AwsErrorMessage(sdkErr))

	generic := fmt.Errorf("wrap: %w", &smithy.GenericAPIError{Code: "UnrecognizedClientException", Message: "The security token included in the request is invalid."})
	require.Equal(t, "The security token included in the request is invalid.", AwsErrorMessage(generic))

	require.Equal(t, "", AwsErrorMessage(errors.New("dial tcp: i/o timeout")))
	require.Equal(t, "", AwsErrorMessage(nil))
}

// Bedrock 客户端不在同一渠道内重试：限流/故障立即返回，由上层换渠道重试（SDK 默认 3 次退避重试会拖慢切换、放大上游压力）。
func TestBuildBedrockRuntimeClient_DisablesSDKRetries(t *testing.T) {
	hc := &http.Client{}
	for _, keyType := range []dto.AwsKeyType{dto.AwsKeyTypeAKSK, dto.AwsKeyTypeApiKey} {
		cli, err := BuildBedrockRuntimeClient(keyType, "ak", "sk", "api-key", "us-east-1", hc)
		require.NoError(t, err)
		require.Equal(t, 1, cli.Options().Retryer.MaxAttempts(), "key type %s", keyType)
	}
}

// 别名 claude-haiku-4-5 必须解析到 Bedrock 真实存在的 ID（曾映射为不存在的 anthropic.claude-haiku-4-5，
// 渠道测试默认用模型列表首个模型时必然 400 model identifier is invalid），且跨区前缀与正式名称一致。
func TestHaikuAliasResolvesToDatedBedrockID(t *testing.T) {
	require.Equal(t, "anthropic.claude-haiku-4-5-20251001-v1:0", getAwsModelID("claude-haiku-4-5"))
	for _, region := range []string{"us-east-1", "eu-west-1", "ap-northeast-1"} {
		require.Equal(t, ResolveModelIDForRegion("claude-haiku-4-5-20251001", region, false), ResolveModelIDForRegion("claude-haiku-4-5", region, false), region)
	}
}
