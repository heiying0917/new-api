package controller

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/dto"
	awsrelay "github.com/QuantumNous/new-api/relay/channel/aws"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	awsTestDefaultModel      = "claude-haiku-4-5-20251001" // 未指定测试模型时的内置默认（便宜且 us/eu/apac 三区均跨区，探测更准）
	awsTestRegionConcurrency = 8                           // 区域探测并发上限
	awsTestRegionTimeout     = 10 * time.Second            // 单区域探测超时
	awsTestMessageMaxLen     = 200                         // 错误信息最大长度（截断，避免过长）
)

type awsTestRegionsRequest struct {
	AwsKeyType string   `json:"aws_key_type"` // "ak_sk" 或 "api_key"
	AccessKey  string   `json:"access_key"`
	SecretKey  string   `json:"secret_key"`
	ApiKey     string   `json:"api_key"`
	Regions    []string `json:"regions"`
	Model      string   `json:"model"` // 可空，空则用内置默认
}

type awsTestRegionResult struct {
	Region     string `json:"region"`
	Ok         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Message    string `json:"message"`
}

// summarizeAwsProbeError 把 AWS SDK 错误压成单行、限长的精简串（不含凭证）。
// 优先用 AWS 短错误码（如 AccessDeniedException），简洁且贴合区域状态展示；
// 非 AWS API 错误（网络/超时等）回退到截断的完整错误串。
func summarizeAwsProbeError(err error) string {
	if err == nil {
		return ""
	}
	if code := awsrelay.AwsErrorCode(err); code != "" {
		return code
	}
	msg := strings.TrimSpace(err.Error())
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > awsTestMessageMaxLen {
		msg = msg[:awsTestMessageMaxLen] + "..."
	}
	return msg
}

// TestAwsRegions 并发探测每个区域对给定凭证 + 测试模型的可用性。
// 安全：仅 AdminAuth（路由层保证）；绝不记录 access_key/secret_key/api_key；响应只回 region 维度状态。
func TestAwsRegions(c *gin.Context) {
	var req awsTestRegionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	keyType := dto.AwsKeyType(req.AwsKeyType)
	if keyType == "" {
		keyType = dto.AwsKeyTypeAKSK
	}

	switch keyType {
	case dto.AwsKeyTypeApiKey:
		if strings.TrimSpace(req.ApiKey) == "" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "请填写 API Key"})
			return
		}
	default:
		if strings.TrimSpace(req.AccessKey) == "" || strings.TrimSpace(req.SecretKey) == "" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "请填写 AccessKey 和 SecretAccessKey"})
			return
		}
	}

	if len(req.Regions) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请至少选择一个区域"})
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = awsTestDefaultModel
	}

	httpClient := service.GetHttpClient()
	results := make([]awsTestRegionResult, len(req.Regions))
	sem := make(chan struct{}, awsTestRegionConcurrency)
	var wg sync.WaitGroup

	for i, region := range req.Regions {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, region string) {
			defer wg.Done()
			defer func() { <-sem }()

			result := awsTestRegionResult{Region: region}

			client, err := awsrelay.BuildBedrockRuntimeClient(keyType, req.AccessKey, req.SecretKey, req.ApiKey, region, httpClient)
			if err != nil {
				result.Message = summarizeAwsProbeError(err)
				results[idx] = result
				return
			}

			modelID := awsrelay.ResolveModelIDForRegion(model, region)

			ctx, cancel := context.WithTimeout(context.Background(), awsTestRegionTimeout)
			defer cancel()

			start := time.Now()
			statusCode, probeErr := awsrelay.ProbeRegionAvailability(ctx, client, modelID)
			result.LatencyMs = time.Since(start).Milliseconds()
			result.StatusCode = statusCode
			result.Ok = awsrelay.ClassifyRegionProbe(statusCode)
			if !result.Ok && probeErr != nil {
				result.Message = summarizeAwsProbeError(probeErr)
			}
			results[idx] = result
		}(i, region)
	}
	wg.Wait()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    results,
	})
}
