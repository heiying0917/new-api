package claude

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 移植自上游 new-api rc.47：thinking-binding-controls 的响应字段 input_transformations
// 在非流式（顶层）与流式（message_start.message）都必须原样透传，不能被结构体重组丢掉。
func TestClaudeResponsePassesInputTransformationsThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-sonnet-4-6"},
	}
	transformations := `"input_transformations":[{"type":"thinking_dropped","path":"messages.1.content.0","reason":"prefix_binding_mismatch"}]`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	body := `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"hi"}],` +
		`"stop_reason":"end_turn",` + transformations + `,"usage":{"input_tokens":5,"output_tokens":1}}`
	require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, nil, []byte(body)))
	require.Equal(t, body, w.Body.String())

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	event := `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],` +
		transformations + `,"usage":{"input_tokens":5,"output_tokens":1}}}`
	require.Nil(t, HandleStreamResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, event))
	require.Contains(t, w.Body.String(), "data: "+event+"\n")
}
