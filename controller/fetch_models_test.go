package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

// 回归：新建渠道获取模型列表时，Anthropic 必须用 x-api-key + anthropic-version 头，
// 而不是 Authorization: Bearer（旧 fetchModelsByParams 一律用 Bearer 导致 401）。
func TestFetchModelsByParams_AnthropicUsesApiKeyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotAuthorization, gotApiKey, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotApiKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		// 模拟 Anthropic：只认 x-api-key，不认 Bearer。
		if gotApiKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"Invalid bearer token"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-test-model"}]}`))
	}))
	defer srv.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fetchModelsByParams(c, srv.URL, constant.ChannelTypeAnthropic, "sk-ant-test")

	var resp struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	if err := common.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("expected success, got failure: %s (body=%s)", resp.Message, w.Body.String())
	}
	if gotApiKey != "sk-ant-test" {
		t.Errorf("expected x-api-key header to carry the key, got %q", gotApiKey)
	}
	if gotVersion == "" {
		t.Errorf("expected anthropic-version header to be set")
	}
	if gotAuthorization != "" {
		t.Errorf("expected no Authorization Bearer header for Anthropic, got %q", gotAuthorization)
	}
	if len(resp.Data) != 1 || resp.Data[0] != "claude-test-model" {
		t.Errorf("expected [claude-test-model], got %v", resp.Data)
	}
}

// 回归：AWS Bedrock 没有 /v1/models 在线端点，应回退到适配器内置静态模型列表，
// 保证「获取模型列表」不空手而归。
func TestFetchModelsByParams_AwsFallsBackToStaticList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// base 留空：AWS 默认 base 为空、且无 /v1/models → 在线探测必失败 → 走静态兜底。
	fetchModelsByParams(c, "", constant.ChannelTypeAws, "ak|sk|us-east-1")

	var resp struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	if err := common.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("expected success via static fallback, got failure: %s", resp.Message)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected non-empty AWS static model list, got empty")
	}
}

// 回归：标准 OpenAI 式 /v1/models（自定义 URL）应正常解析，使用 Authorization: Bearer。
func TestFetchModelsByParams_OpenAICompatibleCustomURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotAuthorization string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"},{"id":"gpt-test-2"}]}`))
	}))
	defer srv.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fetchModelsByParams(c, srv.URL, constant.ChannelTypeOpenAI, "sk-openai-test")

	var resp struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
	}
	if err := common.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("expected success, body=%s", w.Body.String())
	}
	if gotAuthorization != "Bearer sk-openai-test" {
		t.Errorf("expected Authorization: Bearer header, got %q", gotAuthorization)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 models, got %v", resp.Data)
	}
}
