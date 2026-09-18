package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 累计应收款 = Σ official_usd × cost_price_snapshot，覆盖已结算与未结算全部消费日志；其他渠道不计入。
func TestGetTotalReceivableByChannels_SumsAllConsumeLogs(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM logs")
	require.NoError(t, LOG_DB.Create([]*Log{
		{Id: 1, UserId: 1, Type: LogTypeConsume, ChannelId: 94001, SettlementId: 0, OfficialUsd: 1.0, CostPriceSnapshot: 2.0},
		{Id: 2, UserId: 1, Type: LogTypeConsume, ChannelId: 94001, SettlementId: 5, OfficialUsd: 2.0, CostPriceSnapshot: 2.2},
		{Id: 3, UserId: 1, Type: LogTypeConsume, ChannelId: 94002, SettlementId: 0, OfficialUsd: 5.0, CostPriceSnapshot: 3.0},
		{Id: 4, UserId: 1, Type: LogTypeTopup, ChannelId: 94001, SettlementId: 0, OfficialUsd: 9.0, CostPriceSnapshot: 9.0}, // 非消费日志不计
	}).Error)

	got, err := GetTotalReceivableByChannels([]int{94001})
	require.NoError(t, err)
	require.InDelta(t, 1.0*2.0+2.0*2.2, got[94001], 1e-9)
	require.NotContains(t, got, 94002)

	empty, err := GetTotalReceivableByChannels(nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Len(t, empty, 0)
}
