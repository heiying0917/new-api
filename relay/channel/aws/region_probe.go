package aws

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/dto"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/auth/bearer"
	"github.com/pkg/errors"
)

// awsProbePayload 是探测用的极小 Anthropic 负载（max_tokens=1）。
// 注意：探测固定使用 Claude/Anthropic 格式，因此测试模型必须是 Claude 模型。
const awsProbePayload = `{"anthropic_version":"bedrock-2023-05-31","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`

// BuildBedrockRuntimeClient 按密钥类型构造 bedrockruntime 客户端，供 relay 与区域测试接口共用。
// keyType 决定使用 Bearer Token (api_key) 还是静态 AK/SK 凭证。
func BuildBedrockRuntimeClient(keyType dto.AwsKeyType, accessKey, secretKey, apiKey, region string, httpClient *http.Client) (*bedrockruntime.Client, error) {
	if region == "" {
		return nil, errors.New("aws region is required")
	}
	switch keyType {
	case dto.AwsKeyTypeApiKey:
		if apiKey == "" {
			return nil, errors.New("aws api key is required")
		}
		return bedrockruntime.New(bedrockruntime.Options{
			Region:                  region,
			BearerAuthTokenProvider: bearer.StaticTokenProvider{Token: bearer.Token{Value: apiKey}},
			HTTPClient:              httpClient,
		}), nil
	case dto.AwsKeyTypeAKSK:
		if accessKey == "" || secretKey == "" {
			return nil, errors.New("aws access key and secret key are required")
		}
		return bedrockruntime.New(bedrockruntime.Options{
			Region:      region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
			HTTPClient:  httpClient,
		}), nil
	default:
		return nil, errors.New("invalid aws key type")
	}
}

// ResolveModelIDForRegion 把 OpenAI 侧模型名解析为该区域应使用的 Bedrock 模型 ID，
// 与真实 relay 流量一致：支持跨区的模型加 us./eu./apac. 前缀，否则用 base ID。
func ResolveModelIDForRegion(model, region string) string {
	awsModelId := getAwsModelID(model)
	regionPrefix := getAwsRegionPrefix(region)
	if awsModelCanCrossRegion(awsModelId, regionPrefix) {
		return awsModelCrossRegion(awsModelId, regionPrefix)
	}
	return awsModelId
}

// ProbeRegionAvailability 向指定区域发一个极小 InvokeModel 探测可用性，返回 HTTP 状态码与错误。
// 成功返回 200；失败时用 getAwsErrorStatusCode 提取状态码并连同 error 一并返回。
func ProbeRegionAvailability(ctx context.Context, client *bedrockruntime.Client, modelID string) (int, error) {
	_, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
		Body:        []byte(awsProbePayload),
	})
	if err != nil {
		return getAwsErrorStatusCode(err), err
	}
	return http.StatusOK, nil
}

// ClassifyRegionProbe 把探测得到的 HTTP 状态码映射为「区域是否可用」。
// 200 表示成功；429 限流说明凭证有效仅被限流，同样视为可用。
func ClassifyRegionProbe(statusCode int) bool {
	return statusCode == http.StatusOK || statusCode == http.StatusTooManyRequests
}

// AwsErrorCode 从 AWS SDK 错误中提取简短的 API 错误码（如 "AccessDeniedException"、
// "UnrecognizedClientException"、"ValidationException"），用于区域状态的精简展示。
// 非 AWS API 错误（如网络/超时）返回 ""，调用方可回退到完整错误串。
func AwsErrorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	return ""
}
