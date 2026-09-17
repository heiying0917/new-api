package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func viewerTestStrp(s string) *string   { return &s }
func viewerTestF64p(f float64) *float64 { return &f }
func viewerTestInt64p(i int64) *int64   { return &i }
func viewerTestUintp(u uint) *uint      { return &u }

// fullySensitiveChannel 每个敏感字段都填上可辨识的值，任何一个泄露都会被 NotContains 抓到。
func fullySensitiveChannel() *model.Channel {
	return &model.Channel{
		Id: 42, Type: 14, Key: "sk-secret", Status: 1, Name: "claude-main",
		Weight: viewerTestUintp(7), Priority: viewerTestInt64p(9), CreatedTime: 1700000000, TestTime: 1700000100,
		ResponseTime: 350, BaseURL: viewerTestStrp("https://res.services.ai.azure.com/anthropic"),
		Other: "azure-other", Balance: 123.45, BalanceUpdatedTime: 1700000200,
		SupplierId: 5, SupplierName: "sup-a", CreatedBy: 5, CreatedByName: "sup-a",
		CostPrice: viewerTestF64p(6.8), Models: "claude-opus-5,claude-sonnet-5", Group: "default,azure-claude",
		UsedQuota: 500000, ModelMapping: viewerTestStrp(`{"a":"b"}`), StatusCodeMapping: viewerTestStrp(`{"429":"503"}`),
		OtherInfo: `{"status_reason":"key leaked sk-xxx"}`, Tag: viewerTestStrp("t1"),
		Setting: viewerTestStrp(`{"proxy":"socks5://x"}`), ParamOverride: viewerTestStrp(`{"temperature":0}`),
		HeaderOverride: viewerTestStrp(`{"Authorization":"Bearer leak"}`), Remark: viewerTestStrp("内部备注"),
		ChannelInfo:   model.ChannelInfo{IsMultiKey: true, MultiKeySize: 3, MultiKeyStatusList: map[int]int{0: 1, 1: 2}, MultiKeyPollingIndex: 1},
		OtherSettings: `{"azure_version":"2024-02-01"}`, OfficialUsd: 12.3, Receivable: 83.6,
	}
}

// TestViewerChannelView_Whitelist 白名单之外的任何键都不得出现在观察员响应里。
func TestViewerChannelView_Whitelist(t *testing.T) {
	view := viewerChannelView(fullySensitiveChannel())
	data, err := common.Marshal(view)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))

	allowed := map[string]bool{
		"id": true, "name": true, "type": true, "status": true, "models": true, "group": true, "tag": true,
		"response_time": true, "test_time": true, "created_time": true,
		"used_quota": true, "balance": true, "balance_updated_time": true, "channel_info": true,
	}
	for k := range got {
		require.True(t, allowed[k], "unexpected key %q leaked to viewer", k)
	}
	for k := range allowed {
		_, ok := got[k]
		require.True(t, ok, "whitelisted key %q missing", k)
	}
	require.EqualValues(t, 42, got["id"])
	require.Equal(t, "claude-main", got["name"])
	require.EqualValues(t, 123.45, got["balance"])
	require.EqualValues(t, 500000, got["used_quota"])
	require.Equal(t, "t1", got["tag"])

	ci, ok := got["channel_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, ci["is_multi_key"])
	require.EqualValues(t, 3, ci["multi_key_size"])
	require.Len(t, ci, 2, "channel_info must only expose is_multi_key and multi_key_size")

	s := string(data)
	for _, leak := range []string{"sk-secret", "azure.com", "6.8", "Bearer leak", "内部备注", "socks5", "sup-a", "status_reason", "83.6", "12.3", "temperature", "azure_version"} {
		require.NotContains(t, s, leak)
	}
}

// TestViewerChannelViews_SkipsNilAndOmitsEmptyTag nil 安全；无标签渠道不输出 tag 键。
func TestViewerChannelViews_SkipsNilAndOmitsEmptyTag(t *testing.T) {
	ch := fullySensitiveChannel()
	ch.Tag = nil
	views := viewerChannelViews([]*model.Channel{nil, ch})
	require.Len(t, views, 1)
	data, err := common.Marshal(views[0])
	require.NoError(t, err)
	require.NotContains(t, string(data), `"tag"`)
	require.Nil(t, viewerChannelView(nil))
}
