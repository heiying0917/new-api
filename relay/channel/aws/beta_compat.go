package aws

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// bedrockBetaRenames Anthropic 官方 API 与 Bedrock 对同一 beta 的不同命名（键为小写的官方名）。
var bedrockBetaRenames = map[string]string{
	"advanced-tool-use-2025-11-20": "tool-search-tool-2025-10-19",
}

// normalizeBedrockBetas 写入 Bedrock 请求体前规范化 anthropic_beta：逐个 trim、去空，按 Bedrock 命名改名，
// 剔除 Claude 设置里配置的 Bedrock 不支持值，再去重（保留首次出现的顺序）。
// Claude Code 等客户端会带 Anthropic 官方 API 专用的 beta，Bedrock 遇到不认识的值会整单 400
// （ValidationException: Unexpected value(s) ... for the anthropic-beta header）。
func normalizeBedrockBetas(values []string) []string {
	unsupported := make(map[string]struct{})
	for _, v := range model_setting.GetClaudeSettings().BedrockUnsupportedBetas {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			unsupported[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if renamed, ok := bedrockBetaRenames[strings.ToLower(v)]; ok {
			v = renamed
		}
		key := strings.ToLower(v)
		if _, ok := unsupported[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

// bedrockBetaValues 取出 anthropic_beta 的各个值：支持字符串数组或逗号分隔字符串。
// 其它形态返回 ok=false，调用方原样保留、交给 Bedrock 校验。
func bedrockBetaValues(v any) (values []string, ok bool) {
	switch t := v.(type) {
	case string:
		return strings.Split(t, ","), true
	case []string:
		return t, true
	case []any:
		values = make([]string, 0, len(t))
		for _, item := range t {
			s, isString := item.(string)
			if !isString {
				return nil, false
			}
			values = append(values, s)
		}
		return values, true
	default:
		return nil, false
	}
}

// normalizeBedrockBetaJSON 规范化请求体中的 anthropic_beta 原始 JSON；清理后为空返回 nil（不写该字段）。
func normalizeBedrockBetaJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var v any
	if err := common.Unmarshal(raw, &v); err != nil {
		return raw, nil
	}
	values, ok := bedrockBetaValues(v)
	if !ok {
		return raw, nil
	}
	normalized := normalizeBedrockBetas(values)
	if len(normalized) == 0 {
		return nil, nil
	}
	return common.Marshal(normalized)
}
