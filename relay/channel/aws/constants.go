package aws

import "strings"

var awsModelIDMap = map[string]string{
	"claude-3-sonnet-20240229":   "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-opus-20240229":     "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-haiku-20240307":    "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3-5-sonnet-20240620": "anthropic.claude-3-5-sonnet-20240620-v1:0",
	"claude-3-5-sonnet-20241022": "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-5-haiku-20241022":  "anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-3-7-sonnet-20250219": "anthropic.claude-3-7-sonnet-20250219-v1:0",
	"claude-sonnet-4-20250514":   "anthropic.claude-sonnet-4-20250514-v1:0",
	"claude-opus-4-20250514":     "anthropic.claude-opus-4-20250514-v1:0",
	"claude-opus-4-1-20250805":   "anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-sonnet-4-5-20250929": "anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-6":          "anthropic.claude-sonnet-4-6",
	"claude-haiku-4-5-20251001":  "anthropic.claude-haiku-4-5-20251001-v1:0",
	"claude-opus-4-5-20251101":   "anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-6":            "anthropic.claude-opus-4-6-v1",
	"claude-opus-4-7":            "anthropic.claude-opus-4-7",
	"claude-opus-4-8":            "anthropic.claude-opus-4-8",
	"claude-haiku-4-5":           "anthropic.claude-haiku-4-5-20251001-v1:0", // 别名：Bedrock 无 anthropic.claude-haiku-4-5 这个 ID（400 model identifier is invalid）
	"claude-sonnet-5":            "anthropic.claude-sonnet-5",
	"claude-opus-5":              "anthropic.claude-opus-5",
	"claude-fable-5-1":           "anthropic.claude-fable-5-1",
	// Nova models
	"nova-micro-v1:0":   "amazon.nova-micro-v1:0",
	"nova-lite-v1:0":    "amazon.nova-lite-v1:0",
	"nova-pro-v1:0":     "amazon.nova-pro-v1:0",
	"nova-premier-v1:0": "amazon.nova-premier-v1:0",
	"nova-canvas-v1:0":  "amazon.nova-canvas-v1:0",
	"nova-reel-v1:0":    "amazon.nova-reel-v1:0",
	"nova-reel-v1:1":    "amazon.nova-reel-v1:1",
	"nova-sonic-v1:0":   "amazon.nova-sonic-v1:0",
}

var awsModelCanCrossRegionMap = map[string]map[string]bool{
	"anthropic.claude-3-sonnet-20240229-v1:0": {
		"us": true,
		"eu": true,
		"ap": true,
	},
	"anthropic.claude-3-opus-20240229-v1:0": {
		"us": true,
	},
	"anthropic.claude-3-haiku-20240307-v1:0": {
		"us": true,
		"eu": true,
		"ap": true,
	},
	"anthropic.claude-3-5-sonnet-20240620-v1:0": {
		"us": true,
		"eu": true,
		"ap": true,
	},
	"anthropic.claude-3-5-sonnet-20241022-v2:0": {
		"us": true,
		"ap": true,
	},
	"anthropic.claude-3-5-haiku-20241022-v1:0": {
		"us": true,
	},
	"anthropic.claude-3-7-sonnet-20250219-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-sonnet-4-20250514-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-20250514-v1:0": {
		"us": true,
	},
	"anthropic.claude-opus-4-1-20250805-v1:0": {
		"us": true,
	},
	// Claude 4.5 起没有 apac. 配置文件（APAC 区域只有 global. 及部分 jp./au./in.），ap 区域由
	// awsModelGlobalProfileModels 回退 global.；不能写 "ap": true，否则拼出不存在的 apac.xxx（400 model identifier is invalid）。
	"anthropic.claude-sonnet-4-5-20250929-v1:0": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-sonnet-4-6": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-5-20251101-v1:0": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-6-v1": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-7": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-8": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-haiku-4-5-20251001-v1:0": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-sonnet-5": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-opus-5": {
		"us": true,
		"eu": true,
	},
	"anthropic.claude-fable-5-1": {
		"us": true, // 无 eu. 配置文件
	},
	// Nova models - all support three major regions
	"amazon.nova-micro-v1:0": {
		"us":   true,
		"eu":   true,
		"apac": true,
	},
	"amazon.nova-lite-v1:0": {
		"us":   true,
		"eu":   true,
		"apac": true,
	},
	"amazon.nova-pro-v1:0": {
		"us":   true,
		"eu":   true,
		"apac": true,
	},
	"amazon.nova-premier-v1:0": {
		"us": true,
	},
	"amazon.nova-canvas-v1:0": {
		"us":   true,
		"eu":   true,
		"apac": true,
	},
	"amazon.nova-reel-v1:0": {
		"us":   true,
		"eu":   true,
		"apac": true,
	},
	"amazon.nova-reel-v1:1": {
		"us": true,
	},
	"amazon.nova-sonic-v1:0": {
		"us":   true,
		"eu":   true,
		"apac": true,
	},
}

var awsRegionCrossModelPrefixMap = map[string]string{
	"us": "us",
	"eu": "eu",
	"ap": "apac",
}

// awsModelGlobalProfileModels 只能经推理配置文件调用（ListFoundationModels 的 inferenceTypesSupported
// 只有 INFERENCE_PROFILE，base ID 按需调用必然 400），且所有默认启用区域都有 global. 配置文件的模型。
// 区域没有可用的地理跨区配置文件时（APAC 全部区域、sa-east-1、ca-central-1 等）回退 global.，而不是 base ID。
// 依据：2026-11-09 对 17 个默认区域实拉 ListInferenceProfiles。
var awsModelGlobalProfileModels = map[string]bool{
	"anthropic.claude-haiku-4-5-20251001-v1:0":  true,
	"anthropic.claude-sonnet-4-5-20250929-v1:0": true,
	"anthropic.claude-sonnet-4-6":               true,
	"anthropic.claude-opus-4-5-20251101-v1:0":   true,
	"anthropic.claude-opus-4-6-v1":              true,
	"anthropic.claude-opus-4-7":                 true,
	"anthropic.claude-opus-4-8":                 true,
	"anthropic.claude-sonnet-5":                 true,
	"anthropic.claude-opus-5":                   true,
	"anthropic.claude-fable-5-1":                true,
}

var ChannelName = "aws"

// 判断是否为Nova模型
func isNovaModel(modelId string) bool {
	return strings.Contains(modelId, "nova-")
}
