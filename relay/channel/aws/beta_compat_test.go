package aws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 生产 10-27~11-01 被 Bedrock 拒绝的值（tokenki CLS），外加 Bedrock 支持、必须原样保留的值。
const rejectedAndSupportedBetaHeader = "prompt-caching-scope-2026-01-05, redact-thinking-2026-02-12,token-efficient-tools-2026-03-28," +
	"oauth-2025-04-20,extended-cache-ttl-2025-04-11,prompt-caching-2024-07-31,messages-2023-12-15," +
	"interleaved-thinking-2025-05-14,context-management-2025-06-27,context-1m-2025-08-07,computer-use-2025-01-24," +
	"advanced-tool-use-2025-11-20,,interleaved-thinking-2025-05-14"

var expectedBedrockBetas = []any{
	"interleaved-thinking-2025-05-14", "context-management-2025-06-27", "context-1m-2025-08-07",
	"computer-use-2025-01-24", "tool-search-tool-2025-10-19",
}

func TestNormalizeBedrockBetas(t *testing.T) {
	got := normalizeBedrockBetas([]string{" interleaved-thinking-2025-05-14 ", "PROMPT-CACHING-SCOPE-2026-01-05", "", "advanced-tool-use-2025-11-20", "tool-search-tool-2025-10-19", "interleaved-thinking-2025-05-14"})
	require.Equal(t, []string{"interleaved-thinking-2025-05-14", "tool-search-tool-2025-10-19"}, got,
		"trim、大小写不敏感剔除、改名后与已有值去重")
	require.Empty(t, normalizeBedrockBetas([]string{"oauth-2025-04-20", " "}))
}

// 名单可在 Claude 设置里调整，修改即时生效。
func TestNormalizeBedrockBetas_UsesConfiguredList(t *testing.T) {
	settings := model_setting.GetClaudeSettings()
	saved := settings.BedrockUnsupportedBetas
	t.Cleanup(func() { settings.BedrockUnsupportedBetas = saved })

	settings.BedrockUnsupportedBetas = []string{"context-1m-2025-08-07"}
	require.Equal(t, []string{"oauth-2025-04-20"}, normalizeBedrockBetas([]string{"oauth-2025-04-20", "context-1m-2025-08-07"}))
}

func bedrockBodyFor(t *testing.T, info *relaycommon.RelayInfo, ctx *gin.Context, body string) map[string]any {
	t.Helper()
	adaptor := &Adaptor{}
	_, err := doAwsClientRequest(ctx, info, adaptor, bytes.NewBufferString(body))
	require.NoError(t, err)
	awsReq, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
	require.True(t, ok)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(awsReq.Body, &payload))
	return payload
}

func newBetaTestRequest(t *testing.T, body string, passThrough bool, headerOverride string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-5-sonnet-20240620",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "access-key|secret-key|us-east-1",
			UpstreamModelName: "claude-3-5-sonnet-20240620",
		},
	}
	info.ChannelSetting = dto.ChannelSettings{PassThroughBodyEnabled: passThrough}
	if headerOverride != "" {
		info.UseRuntimeHeadersOverride = true
		info.RuntimeHeadersOverride = map[string]any{"anthropic-beta": headerOverride}
	}
	return ctx, info
}

// 路径一：anthropic-beta 头（渠道 header override 之后）写入请求体前被规范化。
func TestDoAwsClientRequest_FiltersUnsupportedBetasFromHeader(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"hello"}],"max_tokens":128}`
	ctx, info := newBetaTestRequest(t, body, false, rejectedAndSupportedBetaHeader)
	payload := bedrockBodyFor(t, info, ctx, body)
	require.Equal(t, expectedBedrockBetas, payload["anthropic_beta"])

	// 全部是不支持的值：不写 anthropic_beta 字段
	ctx, info = newBetaTestRequest(t, body, false, "prompt-caching-scope-2026-01-05,redact-thinking-2026-02-12")
	_, exists := bedrockBodyFor(t, info, ctx, body)["anthropic_beta"]
	require.False(t, exists)
}

// 路径二：客户端请求体自带 anthropic_beta（无请求头）。
func TestDoAwsClientRequest_FiltersUnsupportedBetasFromBody(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"hello"}],"max_tokens":128,"anthropic_beta":["oauth-2025-04-20","advanced-tool-use-2025-11-20","context-1m-2025-08-07"]}`
	ctx, info := newBetaTestRequest(t, body, false, "")
	require.Equal(t, []any{"tool-search-tool-2025-10-19", "context-1m-2025-08-07"}, bedrockBodyFor(t, info, ctx, body)["anthropic_beta"])
}

// 路径三：透传请求体时同样剔除。
func TestDoAwsClientRequest_FiltersUnsupportedBetasInPassThrough(t *testing.T) {
	body := `{"model":"claude-3-5-sonnet-20240620","messages":[{"role":"user","content":"hello"}],"max_tokens":128,"anthropic_beta":["prompt-caching-scope-2026-01-05","interleaved-thinking-2025-05-14"]}`
	ctx, info := newBetaTestRequest(t, body, true, "")
	payload := bedrockBodyFor(t, info, ctx, body)
	require.Equal(t, []any{"interleaved-thinking-2025-05-14"}, payload["anthropic_beta"])
	_, hasModel := payload["model"]
	require.False(t, hasModel, "透传原有逻辑不变：去掉 model")

	body = `{"messages":[{"role":"user","content":"hello"}],"max_tokens":128,"anthropic_beta":["oauth-2025-04-20"]}`
	ctx, info = newBetaTestRequest(t, body, true, "")
	_, exists := bedrockBodyFor(t, info, ctx, body)["anthropic_beta"]
	require.False(t, exists, "透传时清理后为空则去掉字段")
}
