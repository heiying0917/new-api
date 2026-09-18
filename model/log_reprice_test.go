package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 供应商渠道成本价变更后，「未结算」日志的成交价必须跟随最新成本价；已打包进账单的保持冻结。
func seedRepriceFixture(t *testing.T) {
	t.Helper()
	truncateTables(t)
	DB.Exec("DELETE FROM logs")
	DB.Exec("DELETE FROM channels")
	p20, p25 := 2.0, 2.5
	require.NoError(t, DB.Create(&Channel{Id: 93001, Name: "a", Key: "k", SupplierId: 777, CostPrice: &p20, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 93002, Name: "b", Key: "k", SupplierId: 777, CostPrice: &p25, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 93003, Name: "plain", Key: "k", SupplierId: 0, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, LOG_DB.Create([]*Log{
		{Id: 1, UserId: 1, Type: LogTypeConsume, ChannelId: 93001, SettlementId: 0, OfficialUsd: 0.5, CostPriceSnapshot: 2.2, ModelName: "m", CreatedAt: 1001},
		{Id: 2, UserId: 1, Type: LogTypeConsume, ChannelId: 93001, SettlementId: 0, OfficialUsd: 0.25, CostPriceSnapshot: 2.2, ModelName: "m", CreatedAt: 1002},
		{Id: 3, UserId: 1, Type: LogTypeConsume, ChannelId: 93001, SettlementId: 7, OfficialUsd: 1.0, CostPriceSnapshot: 2.2, ModelName: "m", CreatedAt: 1003}, // 已打包，冻结
		{Id: 4, UserId: 1, Type: LogTypeConsume, ChannelId: 93002, SettlementId: 0, OfficialUsd: 0.4, CostPriceSnapshot: 2.5, ModelName: "m", CreatedAt: 1004},
		{Id: 5, UserId: 1, Type: LogTypeConsume, ChannelId: 93003, SettlementId: 0, OfficialUsd: 0.3, CostPriceSnapshot: 1.0, ModelName: "m", CreatedAt: 1005}, // 非供应商
	}).Error)
}

func snapshotsById(t *testing.T) map[int]float64 {
	t.Helper()
	var logs []Log
	require.NoError(t, LOG_DB.Order("id asc").Find(&logs).Error)
	m := make(map[int]float64, len(logs))
	for _, l := range logs {
		m[l.Id] = l.CostPriceSnapshot
	}
	return m
}

func TestRepriceUnsettledLogs_AppliesNewPriceOnlyToUnsettledOfChannel(t *testing.T) {
	seedRepriceFixture(t)

	res, err := RepriceUnsettledLogs(93001, 777, 2.0, false)
	require.NoError(t, err)
	require.Equal(t, 93001, res.ChannelId)
	require.Equal(t, 777, res.SupplierId)
	require.InDelta(t, 2.0, res.NewPrice, 1e-9)
	require.EqualValues(t, 2, res.Logs)
	require.InDelta(t, 0.75, res.OfficialUsd, 1e-9)
	require.InDelta(t, 0.75*2.2, res.OldCny, 1e-9)
	require.InDelta(t, 0.75*2.0, res.NewCny, 1e-9)
	require.InDelta(t, 0.75*2.0-0.75*2.2, res.DeltaCny, 1e-9)

	snap := snapshotsById(t)
	require.InDelta(t, 2.0, snap[1], 1e-9)
	require.InDelta(t, 2.0, snap[2], 1e-9)
	require.InDelta(t, 2.2, snap[3], 1e-9) // 已打包不动
	require.InDelta(t, 2.5, snap[4], 1e-9) // 别的渠道不动
	require.InDelta(t, 1.0, snap[5], 1e-9) // 非供应商不动
}

func TestRepriceUnsettledLogs_DryRunWritesNothing(t *testing.T) {
	seedRepriceFixture(t)

	res, err := RepriceUnsettledLogs(93001, 777, 2.0, true)
	require.NoError(t, err)
	require.EqualValues(t, 2, res.Logs)
	require.InDelta(t, 0.75*2.0-0.75*2.2, res.DeltaCny, 1e-9)

	snap := snapshotsById(t)
	require.InDelta(t, 2.2, snap[1], 1e-9)
	require.InDelta(t, 2.2, snap[2], 1e-9)
}

func TestRepriceUnsettledLogs_RejectsNonPositivePrice(t *testing.T) {
	seedRepriceFixture(t)
	_, err := RepriceUnsettledLogs(93001, 777, 0, false)
	require.Error(t, err)
	require.InDelta(t, 2.2, snapshotsById(t)[1], 1e-9)
}

// 全量对齐：所有供应商渠道的未结算日志按各渠道当前成本价重定价；非供应商渠道不参与。
func TestRepriceAllUnsettledLogsToCurrent_CoversEverySupplierChannel(t *testing.T) {
	seedRepriceFixture(t)

	dry, err := RepriceAllUnsettledLogsToCurrent(true)
	require.NoError(t, err)
	byCh := map[int]*UnsettledRepriceResult{}
	for _, r := range dry {
		byCh[r.ChannelId] = r
	}
	require.Contains(t, byCh, 93001)
	require.Contains(t, byCh, 93002)
	require.NotContains(t, byCh, 93003)
	require.EqualValues(t, 2, byCh[93001].Logs)
	require.InDelta(t, 0.75*2.0-0.75*2.2, byCh[93001].DeltaCny, 1e-9)
	require.EqualValues(t, 1, byCh[93002].Logs)
	require.InDelta(t, 0, byCh[93002].DeltaCny, 1e-9)
	require.InDelta(t, 2.2, snapshotsById(t)[1], 1e-9) // dry-run 未写

	_, err = RepriceAllUnsettledLogsToCurrent(false)
	require.NoError(t, err)
	snap := snapshotsById(t)
	require.InDelta(t, 2.0, snap[1], 1e-9)
	require.InDelta(t, 2.0, snap[2], 1e-9)
	require.InDelta(t, 2.2, snap[3], 1e-9)
	require.InDelta(t, 2.5, snap[4], 1e-9)
	require.InDelta(t, 1.0, snap[5], 1e-9)
}
