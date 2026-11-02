package aws

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// 按 claude_handler 的真实链路构造请求体：解析成 dto.ClaudeRequest → DeepCopy → Marshal，
// 再交给 doAwsClientRequest（formatRequest 会再解析成 AwsClaudeRequest）。
func gatewayClaudeBody(t *testing.T, clientBody string) string {
	t.Helper()
	var req dto.ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(clientBody), &req))
	copied, err := common.DeepCopy(&req)
	require.NoError(t, err)
	out, err := common.Marshal(copied)
	require.NoError(t, err)
	return string(out)
}

// 2026-11-02 tokensolo 验收 thinking-binding-controls：block_binding 在非透传路径被丢，
// 不带 beta 也返回 200（官方应回 400 Extra inputs are not permitted）。
func TestDoAwsClientRequest_KeepsThinkingBlockBinding(t *testing.T) {
	clientBody := `{"model":"claude-sonnet-4-6","max_tokens":512,` +
		`"thinking":{"type":"adaptive","display":"summarized","block_binding":{"prefix_mismatch_behavior":"drop_block"}},` +
		`"output_config":{"effort":"low"},"messages":[{"role":"user","content":"1+1=?"}]}`
	body := gatewayClaudeBody(t, clientBody)
	ctx, info := newBetaTestRequest(t, body, false, "thinking-binding-controls-2026-08-01")

	payload := bedrockBodyFor(t, info, ctx, body)
	require.Equal(t, map[string]any{
		"type":          "adaptive",
		"display":       "summarized",
		"block_binding": map[string]any{"prefix_mismatch_behavior": "drop_block"},
	}, payload["thinking"])
	require.Equal(t, map[string]any{"effort": "low"}, payload["output_config"])
	require.Equal(t, []any{"thinking-binding-controls-2026-08-01"}, payload["anthropic_beta"])
	require.Equal(t, "bedrock-2023-05-31", payload["anthropic_version"])
}

// 回归：原有 thinking 字段（budget_tokens 等）照常送达，没有未建模字段时不多出任何键。
func TestDoAwsClientRequest_ThinkingWithoutUnmodeledFieldsUnchanged(t *testing.T) {
	clientBody := `{"model":"claude-sonnet-4-5","max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":2048},"messages":[{"role":"user","content":"hi"}]}`
	body := gatewayClaudeBody(t, clientBody)
	ctx, info := newBetaTestRequest(t, body, false, "")

	payload := bedrockBodyFor(t, info, ctx, body)
	require.Equal(t, map[string]any{"type": "enabled", "budget_tokens": float64(2048)}, payload["thinking"])
}
