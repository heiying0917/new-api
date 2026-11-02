package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeneralOpenAIRequestPreserveExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-4.1",
		"stream":false,
		"max_tokens":0,
		"max_completion_tokens":0,
		"top_p":0,
		"top_k":0,
		"n":0,
		"frequency_penalty":0,
		"presence_penalty":0,
		"seed":0,
		"logprobs":false,
		"top_logprobs":0,
		"dimensions":0,
		"return_images":false,
		"return_related_questions":false
	}`)

	var req GeneralOpenAIRequest
	err := common.Unmarshal(raw, &req)
	require.NoError(t, err)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "stream").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_completion_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_p").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_k").Exists())
	require.True(t, gjson.GetBytes(encoded, "n").Exists())
	require.True(t, gjson.GetBytes(encoded, "frequency_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "presence_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "seed").Exists())
	require.True(t, gjson.GetBytes(encoded, "logprobs").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_logprobs").Exists())
	require.True(t, gjson.GetBytes(encoded, "dimensions").Exists())
	require.True(t, gjson.GetBytes(encoded, "return_images").Exists())
	require.True(t, gjson.GetBytes(encoded, "return_related_questions").Exists())
}

func TestOpenAIResponsesRequestPreserveExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-4.1",
		"max_output_tokens":0,
		"max_tool_calls":0,
		"stream":false,
		"top_p":0
	}`)

	var req OpenAIResponsesRequest
	err := common.Unmarshal(raw, &req)
	require.NoError(t, err)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "max_output_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_tool_calls").Exists())
	require.True(t, gjson.GetBytes(encoded, "stream").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_p").Exists())
}

func TestGeneralOpenAIRequestGetSystemRoleName(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{name: "o1 uses developer", model: "o1", want: "developer"},
		{name: "o3 family uses developer", model: "o3-mini-high", want: "developer"},
		{name: "o4 family uses developer", model: "o4-mini", want: "developer"},
		{name: "o1 mini stays system", model: "o1-mini", want: "system"},
		{name: "o1 preview stays system", model: "o1-preview", want: "system"},
		{name: "gpt 5 uses developer", model: "gpt-5", want: "developer"},
		{name: "gpt 5.6 uses developer", model: "gpt-5.6-luna", want: "developer"},
		{name: "gpt 6 uses developer", model: "gpt-6-astra", want: "developer"},
		{name: "gpt 6 snapshot uses developer", model: "gpt-6-astra-2026-09-03", want: "developer"},
		{name: "gpt 6 sol uses developer", model: "gpt-6-sol", want: "developer"},
		{name: "gpt 6.1 terra uses developer", model: "gpt-6.1-terra", want: "developer"},
		{name: "unknown gpt 6 variant stays system", model: "gpt-6-astra-pro", want: "system"},
		{name: "invalid gpt 6 snapshot stays system", model: "gpt-6-astra-2026-99-03", want: "system"},
		{name: "unknown generation stays system", model: "gpt-7", want: "system"},
		{name: "gpt 4.1 stays system", model: "gpt-4.1-nano", want: "system"},
		{name: "omni is not o series", model: "omni-moderation-latest", want: "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := GeneralOpenAIRequest{Model: tt.model}

			require.Equal(t, tt.want, req.GetSystemRoleName())
		})
	}
}

func TestIsOpenAIGPT5Model(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "gpt-5", want: true},
		{model: "gpt-5-mini", want: true},
		{model: "gpt-5-chat-latest", want: true},
		{model: "gpt-5.6-luna", want: true},
		{model: "gpt-5.2-2025-12-11", want: true},
		{model: "gpt-6-astra", want: false},
		{model: "gpt-50", want: false},
		{model: "gpt-5custom", want: false},
		{model: "gpt-4.1", want: false},
		{model: "gpt-4o", want: false},
		{model: "gpt-oss-120b", want: false},
		{model: "o3-mini", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			require.Equal(t, tt.want, IsOpenAIGPT5Model(tt.model))
		})
	}
}

// 线上渠道 #154 的 gpt-6 系列模型因未被识别，max_tokens 原样透传被上游 400 拒绝；
// 能力表决定 max_completion_tokens 转换、developer 角色和采样参数是否保留。
func TestGetOpenAIChatCapabilities(t *testing.T) {
	sampling := OpenAIChatCapabilities{UseMaxCompletionTokens: true, UseDeveloperRole: true, SupportsTemperature: true, SupportsTopP: true, SupportsLogProbs: true}
	noSampling := OpenAIChatCapabilities{UseMaxCompletionTokens: true, UseDeveloperRole: true}
	unchanged := OpenAIChatCapabilities{SupportsTemperature: true, SupportsTopP: true, SupportsLogProbs: true}

	tests := []struct {
		name   string
		model  string
		effort string
		want   OpenAIChatCapabilities
	}{
		{name: "gpt-6-astra never samples", model: "gpt-6-astra", want: noSampling},
		{name: "gpt-6-astra snapshot", model: "gpt-6-astra-2026-09-03", want: noSampling},
		{name: "gpt-6.1-sol never samples", model: "gpt-6.1-sol", effort: "none", want: noSampling},
		{name: "gpt-6-sol default keeps sampling", model: "gpt-6-sol", want: sampling},
		{name: "gpt-6-luna effort drops sampling", model: "gpt-6-luna", effort: "high", want: noSampling},
		{name: "gpt-6.1-terra default drops sampling", model: "gpt-6.1-terra", want: noSampling},
		{name: "gpt-6.1-terra explicit none keeps sampling", model: "gpt-6.1-terra", effort: "none", want: sampling},
		{name: "gpt-5 drops sampling", model: "gpt-5", want: noSampling},
		{name: "gpt-5.6 drops sampling", model: "gpt-5.6-luna", want: noSampling},
		{name: "gpt-5.2 default none keeps sampling", model: "gpt-5.2", want: sampling},
		{name: "gpt-5.2 snapshot none keeps sampling", model: "gpt-5.2-2025-12-11", effort: "none", want: sampling},
		{name: "gpt-5.4 reasoning drops sampling", model: "gpt-5.4", effort: "high", want: noSampling},
		{name: "gpt-5.2 named variant drops sampling", model: "gpt-5.2-chat-latest", want: noSampling},
		{name: "o3 drops temperature only", model: "o3-mini", want: OpenAIChatCapabilities{UseMaxCompletionTokens: true, UseDeveloperRole: true, SupportsTopP: true, SupportsLogProbs: true}},
		{name: "o1-mini keeps system role", model: "o1-mini", want: OpenAIChatCapabilities{UseMaxCompletionTokens: true, SupportsTopP: true, SupportsLogProbs: true}},
		{name: "gpt-4.1 unchanged", model: "gpt-4.1", want: unchanged},
		{name: "unknown gpt-6 variant unchanged", model: "gpt-6-astra-pro", want: unchanged},
		{name: "future generation unchanged", model: "gpt-7", want: unchanged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GetOpenAIChatCapabilities(tt.model, tt.effort))
		})
	}
}
