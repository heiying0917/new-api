package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 线上渠道 #154 回归：gpt-6 系列的测试请求经 OpenAI 适配器转换后若仍带 max_tokens，
// 上游会返回 400 "Use 'max_completion_tokens' instead"，渠道测试误报失败。
func TestChannelTestRequestTokenLimitField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		model     string
		wantLimit string
		dropLimit string
	}{
		{model: "gpt-6.1-terra", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-6.1-sol", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-6-astra", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-6-luna", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-6-sol", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-5.6-terra", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-5.5", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "o3-mini", wantLimit: "max_completion_tokens", dropLimit: "max_tokens"},
		{model: "gpt-4.1", wantLimit: "max_tokens", dropLimit: "max_completion_tokens"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			request, ok := buildTestRequest(tt.model, "", &model.Channel{}, false).(*dto.GeneralOpenAIRequest)
			require.True(t, ok)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeOpenAI,
					UpstreamModelName: tt.model,
				},
			}
			converted, err := (&openai.Adaptor{}).ConvertOpenAIRequest(c, info, request)
			require.NoError(t, err)
			body, err := common.Marshal(converted)
			require.NoError(t, err)

			require.Equal(t, int64(16), gjson.GetBytes(body, tt.wantLimit).Int(), string(body))
			require.False(t, gjson.GetBytes(body, tt.dropLimit).Exists(), string(body))
		})
	}
}
