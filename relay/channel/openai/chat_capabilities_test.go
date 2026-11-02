package openai

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 上游会对不支持的 max_tokens / temperature / top_p / logprobs 直接返回 400，
// 转换结果必须按模型能力表改写或剥离这些参数。
func TestConvertOpenAIRequestAppliesChatCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const sampling = `"temperature":0.2,"top_p":0.8,"logprobs":true,"top_logprobs":5`

	tests := []struct {
		name  string
		model string
		input string
		want  string
	}{
		{
			name:  "gpt-6-astra rewrites limit and drops sampling",
			model: "gpt-6-astra",
			input: `{"max_tokens":100,` + sampling + `}`,
			want:  `{"model":"gpt-6-astra","max_completion_tokens":100,"messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "gpt-6-sol default keeps sampling",
			model: "gpt-6-sol",
			input: `{"max_tokens":100,` + sampling + `}`,
			want:  `{"model":"gpt-6-sol","max_completion_tokens":100,` + sampling + `,"messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "gpt-6.1-terra effort suffix is stripped",
			model: "gpt-6.1-terra-high",
			input: `{"max_tokens":100,` + sampling + `}`,
			want:  `{"model":"gpt-6.1-terra","reasoning_effort":"high","max_completion_tokens":100,"messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "explicit max_completion_tokens wins",
			model: "gpt-6-luna",
			input: `{"max_tokens":100,"max_completion_tokens":50}`,
			want:  `{"model":"gpt-6-luna","max_tokens":100,"max_completion_tokens":50,"messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "gpt-5.6 keeps existing policy",
			model: "gpt-5.6-luna",
			input: `{"max_tokens":100,` + sampling + `}`,
			want:  `{"model":"gpt-5.6-luna","max_completion_tokens":100,"messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "o3 drops temperature only",
			model: "o3-mini-high",
			input: `{"max_tokens":100,` + sampling + `}`,
			want:  `{"model":"o3-mini","reasoning_effort":"high","max_completion_tokens":100,"top_p":0.8,"logprobs":true,"top_logprobs":5,"messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "gpt-4.1 unchanged",
			model: "gpt-4.1",
			input: `{"max_tokens":100,` + sampling + `}`,
			want:  `{"model":"gpt-4.1","max_tokens":100,` + sampling + `,"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
		{
			name:  "unknown future model unchanged",
			model: "gpt-7-high",
			input: `{"max_tokens":100}`,
			want:  `{"model":"gpt-7-high","max_tokens":100,"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model:    tt.model,
				Messages: []dto.Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}},
			}
			require.NoError(t, common.UnmarshalJsonStr(tt.input, request))

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeOpenAI,
					UpstreamModelName: tt.model,
				},
			}
			converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
			require.NoError(t, err)
			body, err := common.Marshal(converted)
			require.NoError(t, err)

			require.JSONEq(t, tt.want, string(body))
		})
	}
}
