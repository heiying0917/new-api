package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestBlankConsumerIdentity verifies suppliers never see platform-consumer identity.
func TestBlankConsumerIdentity(t *testing.T) {
	logs := []*model.Log{
		{Username: "alice", TokenName: "tok-a", ModelName: "gpt-4", ChannelId: 1},
		{Username: "bob", TokenName: "tok-b", ModelName: "gpt-3.5", ChannelId: 2},
	}
	blankConsumerIdentity(logs)
	for _, l := range logs {
		require.Equal(t, "", l.Username, "username must be blanked")
		require.Equal(t, "", l.TokenName, "token name must be blanked")
	}
	// Non-identity fields preserved.
	require.Equal(t, "gpt-4", logs[0].ModelName)
	require.Equal(t, 1, logs[0].ChannelId)
	require.Equal(t, "gpt-3.5", logs[1].ModelName)
	require.Equal(t, 2, logs[1].ChannelId)

	// nil-safe.
	require.NotPanics(t, func() { blankConsumerIdentity(nil) })
}

// TestBlankSellingPrice 供应商不得看到平台售价：quota 归零、other 中的分组倍率等加价字段剔除，官方价参数保留。
func TestBlankSellingPrice(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"model_ratio": 2.5, "completion_ratio": 5, "model_price": 0, "cache_ratio": 0.1,
		"group_ratio": 3.3, "user_group_ratio": 2.0, "billing_mode": "tiered_expr", "matched_tier": "t2",
		"billing_preference": "wallet", "billing_source": "subscription", "wallet_quota_deducted": 100,
		"subscription_id": 1, "subscription_plan_title": "Pro", "admin_info": map[string]any{"x": 1},
		"stream_status": "ok", "frt": 88, "upstream_model_name": "claude-opus-5", "is_model_mapped": true,
	})
	logs := []*model.Log{{Quota: 3300, OfficialUsd: 0.1, CostPriceSnapshot: 6.8, Other: other}, nil, {Quota: 5, Other: ""}}
	blankSellingPrice(logs)
	require.Equal(t, 0, logs[0].Quota)
	require.Equal(t, 0.1, logs[0].OfficialUsd, "official price is the supplier's settlement basis, keep it")
	require.Equal(t, 6.8, logs[0].CostPriceSnapshot)
	m, _ := common.StrToMap(logs[0].Other)
	for _, gone := range []string{"group_ratio", "user_group_ratio", "billing_mode", "matched_tier", "billing_preference",
		"billing_source", "wallet_quota_deducted", "subscription_id", "subscription_plan_title", "admin_info", "stream_status"} {
		_, ok := m[gone]
		require.False(t, ok, "%s must be removed", gone)
	}
	for _, kept := range []string{"model_ratio", "completion_ratio", "model_price", "cache_ratio", "frt", "upstream_model_name", "is_model_mapped"} {
		_, ok := m[kept]
		require.True(t, ok, "%s must be kept", kept)
	}
	require.Equal(t, 0, logs[2].Quota)
	require.Equal(t, "", logs[2].Other)
	require.NotPanics(t, func() { blankSellingPrice(nil) })
}
