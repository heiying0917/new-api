package aws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoAwsClientRequest_ForceGlobalPrefix(t *testing.T) {
	cases := []struct {
		name        string
		forceGlobal bool
		region      string
		wantModelID string
	}{
		{"global on => global. prefix", true, "us-east-1", "global.anthropic.claude-opus-4-6-v1"},
		{"global off => region prefix", false, "us-east-1", "us.anthropic.claude-opus-4-6-v1"},
		{"global off eu => eu prefix", false, "eu-west-1", "eu.anthropic.claude-opus-4-6-v1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

			info := &relaycommon.RelayInfo{
				IsStream: false,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiKey:            "access-key|secret-key|" + tc.region,
					UpstreamModelName: "claude-opus-4-6",
					ChannelOtherSettings: dto.ChannelOtherSettings{
						AwsForceGlobal: tc.forceGlobal,
					},
				},
			}

			requestBody := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}],"max_tokens":1}`)
			adaptor := &Adaptor{}

			_, err := doAwsClientRequest(ctx, info, adaptor, requestBody)
			require.NoError(t, err)

			awsReq, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
			require.True(t, ok)
			require.Equal(t, tc.wantModelID, *awsReq.ModelId)
		})
	}
}
