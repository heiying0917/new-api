package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestBlankViewerLog 观察员看全站日志：隐藏消费者身份与平台成本，保留渠道/模型/花费。
func TestBlankViewerLog(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"model_ratio": 2.5, "group_ratio": 3.3, "admin_info": map[string]any{"use_channel": []string{"1"}},
		"stream_status": "ok", "frt": 120,
	})
	logs := []*model.Log{
		{Id: 1, UserId: 9, Username: "alice", TokenName: "tok", TokenId: 3, Ip: "1.2.3.4",
			ModelName: "claude-opus-5", ChannelId: 42, ChannelName: "claude-main", Quota: 1234,
			OfficialUsd: 0.5, CostPriceSnapshot: 6.8, Other: other},
		nil,
		{Id: 2, Username: "bob", Other: ""},
	}
	blankViewerLog(logs)
	l := logs[0]
	require.Equal(t, "", l.Username)
	require.Equal(t, "", l.TokenName)
	require.Equal(t, "", l.Ip)
	require.Equal(t, 0, l.UserId)
	require.Equal(t, 0, l.TokenId)
	require.Equal(t, float64(0), l.OfficialUsd)
	require.Equal(t, float64(0), l.CostPriceSnapshot)
	require.Equal(t, "claude-main", l.ChannelName)
	require.Equal(t, 42, l.ChannelId)
	require.Equal(t, 1234, l.Quota)
	m, _ := common.StrToMap(l.Other)
	_, hasAdmin := m["admin_info"]
	_, hasStream := m["stream_status"]
	require.False(t, hasAdmin)
	require.False(t, hasStream)
	require.EqualValues(t, 3.3, m["group_ratio"], "group ratio is public pricing for a customer, keep it")
	require.EqualValues(t, 120, m["frt"])
	require.Equal(t, "", logs[2].Username)
	require.Equal(t, "", logs[2].Other)
	require.NotPanics(t, func() { blankViewerLog(nil) })
}
