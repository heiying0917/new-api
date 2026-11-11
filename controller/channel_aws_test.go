package controller

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/require"
)

// 区域测试默认用 claude-opus-4-6 探测：它在 17 个默认区域都有 us./eu./global. 配置文件，
// 且账号只开通部分 Claude 模型时它比 haiku-4-5 更具代表性（2026-11-11 排查：haiku-4-5 全区 404、opus-4-6 可用）。
func TestAwsTestDefaultModelIsOpus46(t *testing.T) {
	require.Equal(t, "claude-opus-4-6", awsTestDefaultModel)
}

// AWS API 错误：错误码单独返回（下拉框短展示），原因 = 「错误码: AWS 原始说明」，不含 RequestID 等 SDK 包装。
func TestSummarizeAwsProbeError_APIErrorKeepsReason(t *testing.T) {
	err := &smithy.OperationError{
		ServiceID:     "Bedrock Runtime",
		OperationName: "InvokeModel",
		Err: &types.ResourceNotFoundException{
			Message: aws.String("Model use case details have not been submitted for this account."),
		},
	}
	code, message := summarizeAwsProbeError(err)
	require.Equal(t, "ResourceNotFoundException", code)
	require.Equal(t, "ResourceNotFoundException: Model use case details have not been submitted for this account.", message)
}

// AWS 未给说明时只回错误码，不出现尾随的「: 」。
func TestSummarizeAwsProbeError_APIErrorWithoutMessage(t *testing.T) {
	code, message := summarizeAwsProbeError(&smithy.GenericAPIError{Code: "AccessDeniedException"})
	require.Equal(t, "AccessDeniedException", code)
	require.Equal(t, "AccessDeniedException", message)
}

// 非 AWS API 错误（网络 / 超时）：无错误码，回退为单行完整错误串。
func TestSummarizeAwsProbeError_NonAPIErrorIsSingleLine(t *testing.T) {
	code, message := summarizeAwsProbeError(errors.New("dial tcp 1.2.3.4:443:\n i/o timeout"))
	require.Equal(t, "", code)
	require.Equal(t, "dial tcp 1.2.3.4:443:  i/o timeout", message)
}

// 超长原因按字符截断并追加「...」，不能切坏多字节字符（否则 JSON 里出现 �）。
func TestSummarizeAwsProbeError_TruncatesByRune(t *testing.T) {
	long := strings.Repeat("区", awsTestMessageMaxLen+50)
	code, message := summarizeAwsProbeError(errors.New(long))
	require.Equal(t, "", code)
	require.True(t, utf8.ValidString(message))
	require.Equal(t, strings.Repeat("区", awsTestMessageMaxLen)+"...", message)
}

func TestSummarizeAwsProbeError_Nil(t *testing.T) {
	code, message := summarizeAwsProbeError(nil)
	require.Equal(t, "", code)
	require.Equal(t, "", message)
}
