package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func seedOfficialUsdBackfillFixture(t *testing.T) {
	t.Helper()
	truncateTables(t)
	DB.Exec("DELETE FROM logs")
	DB.Exec("DELETE FROM channels")
	common.MemoryCacheEnabled = false
	cp := 2.2
	require.NoError(t, DB.Create(&Channel{Id: 91001, Name: "sup", Key: "k", SupplierId: 777, CostPrice: &cp, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 91002, Name: "plain", Key: "k", SupplierId: 0, Status: 1, Models: "m", Group: "g"}).Error)
}

func backfillLog(id, channelId, settlementId, quota int, other string, official, snapshot float64) *Log {
	return &Log{
		Id: id, UserId: 1, Type: LogTypeConsume, ChannelId: channelId, SettlementId: settlementId,
		Quota: quota, Other: other, OfficialUsd: official, CostPriceSnapshot: snapshot,
		ModelName: "claude-opus-5", CreatedAt: 1000 + int64(id),
	}
}

// 回填只动「供应商渠道 + 未结算」的消费日志，按 quota ÷ group_ratio ÷ QuotaPerUnit 重算 official_usd
// （与修复后的文本路径同源，天然包含缓存 token / 附加倍率）；已打包进账单、非供应商渠道、
// other 里无 group_ratio 的一律不动。
func TestBackfillUnsettledOfficialUsd_RecomputesFromQuotaAndGroupRatio(t *testing.T) {
	seedOfficialUsdBackfillFixture(t)
	require.NoError(t, LOG_DB.Create([]*Log{
		backfillLog(1, 91001, 0, 65389, `{"group_ratio":1,"model_ratio":2.5}`, 0.004785, 2.2), // 漏算缓存的真实日志 → 0.130778
		backfillLog(2, 91001, 7, 65389, `{"group_ratio":1}`, 0.004785, 2.2),                   // 已打包进账单，不动
		backfillLog(3, 91002, 0, 65389, `{"group_ratio":1}`, 0, 0),                            // 非供应商渠道，不扫
		backfillLog(4, 91001, 0, 3300, `{"model_ratio":1}`, 0.001, 2.2),                       // 无 group_ratio，跳过
		backfillLog(5, 91001, 0, 3300, `{"group_ratio":3.3}`, 0.002, 2.2),                     // 已正确，不变
	}).Error)

	res, err := BackfillUnsettledOfficialUsd(0, 100, false)
	require.NoError(t, err)
	require.False(t, res.DryRun)
	require.True(t, res.Done)
	require.EqualValues(t, 3, res.Scanned)
	require.EqualValues(t, 1, res.Updated)
	require.EqualValues(t, 1, res.Skipped)
	require.EqualValues(t, 1, res.Unchanged)
	require.Equal(t, 5, res.NextAfterId)

	sup := res.PerSupplier[777]
	require.NotNil(t, sup)
	require.EqualValues(t, 1, sup.Updated)
	require.InDelta(t, 0.130778-0.004785, sup.DeltaUsd, 1e-9)
	require.InDelta(t, (0.130778-0.004785)*2.2, sup.DeltaCny, 1e-9)

	var got []Log
	require.NoError(t, LOG_DB.Order("id asc").Find(&got).Error)
	require.Len(t, got, 5)
	require.InDelta(t, 0.130778, got[0].OfficialUsd, 1e-9)
	require.InDelta(t, 0.004785, got[1].OfficialUsd, 1e-9)
	require.InDelta(t, 0.0, got[2].OfficialUsd, 1e-9)
	require.InDelta(t, 0.001, got[3].OfficialUsd, 1e-9)
	require.InDelta(t, 0.002, got[4].OfficialUsd, 1e-9)
}

// dry_run 只算差额不落库。
func TestBackfillUnsettledOfficialUsd_DryRunWritesNothing(t *testing.T) {
	seedOfficialUsdBackfillFixture(t)
	require.NoError(t, LOG_DB.Create(backfillLog(1, 91001, 0, 65389, `{"group_ratio":1}`, 0.004785, 2.2)).Error)

	res, err := BackfillUnsettledOfficialUsd(0, 100, true)
	require.NoError(t, err)
	require.True(t, res.DryRun)
	require.EqualValues(t, 1, res.Updated)
	require.InDelta(t, 0.130778-0.004785, res.PerSupplier[777].DeltaUsd, 1e-9)

	var l Log
	require.NoError(t, LOG_DB.First(&l, 1).Error)
	require.InDelta(t, 0.004785, l.OfficialUsd, 1e-9)
}

// 按 id 游标分页：limit 内未扫完 → Done=false 并给出 next_after_id，续传直到 Done=true。
func TestBackfillUnsettledOfficialUsd_PaginatesByIdCursor(t *testing.T) {
	seedOfficialUsdBackfillFixture(t)
	require.NoError(t, LOG_DB.Create([]*Log{
		backfillLog(1, 91001, 0, 65389, `{"group_ratio":1}`, 0.004785, 2.2),
		backfillLog(2, 91001, 0, 65389, `{"group_ratio":1}`, 0.004785, 2.2),
	}).Error)

	res1, err := BackfillUnsettledOfficialUsd(0, 1, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, res1.Scanned)
	require.False(t, res1.Done)
	require.Equal(t, 1, res1.NextAfterId)

	res2, err := BackfillUnsettledOfficialUsd(res1.NextAfterId, 1, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, res2.Scanned)
	require.Equal(t, 2, res2.NextAfterId)

	res3, err := BackfillUnsettledOfficialUsd(res2.NextAfterId, 1, false)
	require.NoError(t, err)
	require.EqualValues(t, 0, res3.Scanned)
	require.True(t, res3.Done)

	var got []Log
	require.NoError(t, LOG_DB.Order("id asc").Find(&got).Error)
	require.Len(t, got, 2)
	require.InDelta(t, 0.130778, got[0].OfficialUsd, 1e-9)
	require.InDelta(t, 0.130778, got[1].OfficialUsd, 1e-9)
}

// 没有任何供应商渠道时直接 Done，不得生成 IN () 空列表 SQL。
func TestBackfillUnsettledOfficialUsd_NoSupplierChannels(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM logs")
	DB.Exec("DELETE FROM channels")
	require.NoError(t, LOG_DB.Create(backfillLog(1, 91002, 0, 65389, `{"group_ratio":1}`, 0, 0)).Error)

	res, err := BackfillUnsettledOfficialUsd(0, 100, false)
	require.NoError(t, err)
	require.True(t, res.Done)
	require.EqualValues(t, 0, res.Scanned)
}
