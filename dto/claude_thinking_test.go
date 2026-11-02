package dto

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// thinking 里没建模的子字段（如 thinking-binding-controls 的 block_binding）必须原样转发上游，
// 否则官方 beta 能力在网关被静默关掉，而且不带 beta 时上游该回的 400 也变成了 200。
func TestThinkingPreservesUnmodeledFields(t *testing.T) {
	body := `{"model":"claude-sonnet-4-6","max_tokens":512,"thinking":{"type":"adaptive","block_binding":{"prefix_mismatch_behavior":"drop_block"},"future_field":{"a":1}},"messages":[{"role":"user","content":"hi"}]}`
	var req ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(body), &req))
	// block_binding 与上游 new-api 一致建模为显式字段，其余未知子字段进 Extra
	require.JSONEq(t, `{"prefix_mismatch_behavior":"drop_block"}`, string(req.Thinking.BlockBinding))
	require.Equal(t, map[string]json.RawMessage{"future_field": json.RawMessage(`{"a":1}`)}, req.Thinking.Extra)

	out, err := common.Marshal(req)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"adaptive","block_binding":{"prefix_mismatch_behavior":"drop_block"},"future_field":{"a":1}}`, gjson.GetBytes(out, "thinking").Raw)
}

// 没有未建模字段时序列化结果与修复前逐字节一致，Extra 保持 nil（不影响 DeepEqual / 字面量构造）。
func TestThinkingWithoutUnmodeledFieldsMarshalsUnchanged(t *testing.T) {
	raw := `{"type":"enabled","budget_tokens":2048,"display":"summarized"}`
	var thinking Thinking
	require.NoError(t, common.Unmarshal([]byte(raw), &thinking))
	require.Nil(t, thinking.Extra)

	out, err := common.Marshal(thinking)
	require.NoError(t, err)
	require.Equal(t, raw, string(out))
}

// thinking 传错类型时，返回给客户端的 400 文案仍指向 dto.Thinking。
func TestThinkingTypeErrorStillNamesThinking(t *testing.T) {
	var req ClaudeRequest
	err := common.Unmarshal([]byte(`{"thinking":"high"}`), &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dto.Thinking")
}

// 网关把 thinking 改写成 adaptive 时，要带上客户端的 block_binding 和其他未建模子字段，且与原请求互不影响。
func TestNewAdaptiveThinkingFromCarriesUnmodeledFields(t *testing.T) {
	var prev Thinking
	require.NoError(t, common.Unmarshal([]byte(`{"type":"enabled","budget_tokens":2048,"block_binding":{"prefix_mismatch_behavior":"error"},"future_field":1}`), &prev))

	got := NewAdaptiveThinkingFrom(&prev)
	require.Equal(t, "adaptive", got.Type)
	require.Nil(t, got.BudgetTokens)
	out, err := common.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"adaptive","block_binding":{"prefix_mismatch_behavior":"error"},"future_field":1}`, string(out))

	got.BlockBinding[2] = 'X'
	got.Extra["future_field"][0] = '2'
	require.JSONEq(t, `{"prefix_mismatch_behavior":"error"}`, string(prev.BlockBinding))
	require.Equal(t, `1`, string(prev.Extra["future_field"]))

	require.Equal(t, &Thinking{Type: "adaptive"}, NewAdaptiveThinkingFrom(nil))
}
