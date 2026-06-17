package model

import (
	"sort"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

// setupSupplierStatsTables resets the tables used by supplier overview aggregation tests.
func setupSupplierStatsTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Settlement{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM settlements").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
}

func ptrFloat(v float64) *float64 { return &v }

func TestGetSupplierSettledStats(t *testing.T) {
	setupSupplierStatsTables(t)
	const now int64 = 1_700_000_000
	startOfToday := now - (now % 86400)

	// Supplier 7 settled settlements across windows.
	require.NoError(t, DB.Create(&Settlement{SupplierId: 7, Status: SettlementStatusSettled, ComputedCNY: 10, SettledAt: startOfToday + 100}).Error) // today
	require.NoError(t, DB.Create(&Settlement{SupplierId: 7, Status: SettlementStatusSettled, ComputedCNY: 5, SettledAt: now - 3*86400}).Error)       // within 7d (not today)
	require.NoError(t, DB.Create(&Settlement{SupplierId: 7, Status: SettlementStatusSettled, ComputedCNY: 3, SettledAt: now - 30*86400}).Error)      // older
	// Applied (not settled) must be excluded.
	require.NoError(t, DB.Create(&Settlement{SupplierId: 7, Status: SettlementStatusApplied, ComputedCNY: 100, SettledAt: startOfToday + 200}).Error)
	// Other supplier must be excluded.
	require.NoError(t, DB.Create(&Settlement{SupplierId: 9, Status: SettlementStatusSettled, ComputedCNY: 99, SettledAt: startOfToday + 50}).Error)

	stats, err := GetSupplierSettledStats(7, now)
	require.NoError(t, err)
	require.InDelta(t, 10, stats.Today, 1e-9)
	require.InDelta(t, 15, stats.Last7, 1e-9) // today + within7
	require.InDelta(t, 18, stats.Total, 1e-9) // all settled
}

func TestGetSupplierChannelStatusCounts(t *testing.T) {
	setupSupplierStatsTables(t)
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "k1", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "b", Key: "k2", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "c", Key: "k3", SupplierId: 7, Status: common.ChannelStatusManuallyDisabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "d", Key: "k4", SupplierId: 7, Status: common.ChannelStatusAutoDisabled, Models: "m", Group: "g"}).Error)
	// Other supplier.
	require.NoError(t, DB.Create(&Channel{Name: "e", Key: "k5", SupplierId: 9, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)

	available, unavailable, err := GetSupplierChannelStatusCounts(7)
	require.NoError(t, err)
	require.Equal(t, int64(2), available)
	require.Equal(t, int64(2), unavailable)
}

func TestGetSupplierTodayUsage(t *testing.T) {
	setupSupplierStatsTables(t)
	const now int64 = 1_700_000_000
	startOfToday := now - (now % 86400)

	// Channel ids used in logs.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 10, CompletionTokens: 5, CreatedAt: startOfToday + 10}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 102, PromptTokens: 3, CompletionTokens: 7, CreatedAt: startOfToday + 20}).Error)
	// Yesterday (excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 100, CompletionTokens: 100, CreatedAt: startOfToday - 10}).Error)
	// Non-consume type (excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 101, PromptTokens: 1, CompletionTokens: 1, CreatedAt: startOfToday + 30}).Error)
	// Channel not in set (excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, PromptTokens: 50, CompletionTokens: 50, CreatedAt: startOfToday + 40}).Error)

	requests, tokens, err := GetSupplierTodayUsage([]int{101, 102}, now)
	require.NoError(t, err)
	require.Equal(t, int64(2), requests)
	require.Equal(t, int64(25), tokens) // 10+5 + 3+7

	// Empty channelIds short-circuits without running IN ().
	requests, tokens, err = GetSupplierTodayUsage(nil, now)
	require.NoError(t, err)
	require.Equal(t, int64(0), requests)
	require.Equal(t, int64(0), tokens)
}

func TestGetSupplierMarketBids(t *testing.T) {
	setupSupplierStatsTables(t)

	// Supplier 7 channels.
	// type=1, group "a,b": enabled, cost 2.0
	require.NoError(t, DB.Create(&Channel{Name: "s7-1", Key: "k1", SupplierId: 7, Type: 1, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(2.0), Models: "m", Group: "a,b"}).Error)
	// type=1, group "a": enabled, cost 1.5 (my best in (1,a))
	require.NoError(t, DB.Create(&Channel{Name: "s7-2", Key: "k2", SupplierId: 7, Type: 1, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(1.5), Models: "m", Group: "a"}).Error)
	// type=1, group "c": disabled => excluded from ladder, but bucket (1,c) inclusion comes from ownership (any status). However no enabled competitor either, so bucket (1,c) appears with only... nothing. We expect (1,c) excluded because no enabled bids at all? It IS owned (any status) so it is included with empty bids.
	require.NoError(t, DB.Create(&Channel{Name: "s7-3", Key: "k3", SupplierId: 7, Type: 1, Status: common.ChannelStatusManuallyDisabled, CostPrice: ptrFloat(0.5), Models: "m", Group: "c"}).Error)
	// type=2, group "x": enabled, nil cost => excluded from ladder bids, but ownership counts for inclusion of (2,x)
	require.NoError(t, DB.Create(&Channel{Name: "s7-4", Key: "k4", SupplierId: 7, Type: 2, Status: common.ChannelStatusEnabled, CostPrice: nil, Models: "m", Group: "x"}).Error)

	// Supplier 9 channels (competitors).
	// type=1, group "a": enabled, cost 1.0 (cheapest in (1,a))
	require.NoError(t, DB.Create(&Channel{Name: "s9-1", Key: "k5", SupplierId: 9, Type: 1, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(1.0), Models: "m", Group: "a"}).Error)
	// type=1, group "b": enabled, cost 3.0
	require.NoError(t, DB.Create(&Channel{Name: "s9-2", Key: "k6", SupplierId: 9, Type: 1, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(3.0), Models: "m", Group: "b"}).Error)
	// type=1, group "z": enabled, cost 1.0 — supplier 7 NOT in this bucket => must be excluded from result
	require.NoError(t, DB.Create(&Channel{Name: "s9-3", Key: "k7", SupplierId: 9, Type: 1, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(1.0), Models: "m", Group: "z"}).Error)
	// type=2, group "x": enabled, cost 4.0 — competitor in (2,x) which supplier 7 owns (nil cost)
	require.NoError(t, DB.Create(&Channel{Name: "s9-4", Key: "k8", SupplierId: 9, Type: 2, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(4.0), Models: "m", Group: "x"}).Error)
	// type=1, group "c": enabled, cost 0.9 — competitor in (1,c) which supplier 7 owns (disabled)
	require.NoError(t, DB.Create(&Channel{Name: "s9-5", Key: "k9", SupplierId: 9, Type: 1, Status: common.ChannelStatusEnabled, CostPrice: ptrFloat(0.9), Models: "m", Group: "c"}).Error)

	groups, err := GetSupplierMarketBids(7)
	require.NoError(t, err)

	// Build a lookup keyed by type|group for assertions.
	byKey := map[string]MarketBidGroup{}
	for _, g := range groups {
		byKey[keyOf(g.Type, g.Group)] = g
	}

	// (1,z) must be excluded — supplier 7 not in it.
	_, ok := byKey[keyOf(1, "z")]
	require.False(t, ok, "bucket (1,z) should be excluded; supplier not participating")

	// Buckets supplier 7 participates in: (1,a),(1,b),(1,c),(2,x)
	require.Contains(t, byKey, keyOf(1, "a"))
	require.Contains(t, byKey, keyOf(1, "b"))
	require.Contains(t, byKey, keyOf(1, "c"))
	require.Contains(t, byKey, keyOf(2, "x"))

	// (1,a): s7-1 (group "a,b", cost 2.0) + s7-2 (group "a", cost 1.5) + s9-1 (group "a", cost 1.0).
	// Ascending: 1.0 (s9), 1.5 (s7 mine), 2.0 (s7 mine). MyBest=1.5, MyRank=2.
	ga := byKey[keyOf(1, "a")]
	require.Equal(t, "OpenAI", ga.TypeName) // type 1 => OpenAI per constant.GetChannelTypeName
	require.Equal(t, constant.GetChannelTypeName(1), ga.TypeName)
	require.Len(t, ga.Bids, 3)
	require.InDelta(t, 1.0, ga.Bids[0].Price, 1e-9)
	require.False(t, ga.Bids[0].Mine)
	require.InDelta(t, 1.5, ga.Bids[1].Price, 1e-9)
	require.True(t, ga.Bids[1].Mine)
	require.InDelta(t, 2.0, ga.Bids[2].Price, 1e-9)
	require.True(t, ga.Bids[2].Mine)
	require.NotNil(t, ga.MyBest)
	require.InDelta(t, 1.5, *ga.MyBest, 1e-9)
	require.Equal(t, 2, ga.MyRank)
	require.Equal(t, 3, ga.Total)

	// (1,b): bids 2.0 (s7, mine), 3.0 (s9) ascending; MyBest=2.0, MyRank=1.
	gb := byKey[keyOf(1, "b")]
	require.Len(t, gb.Bids, 2)
	require.InDelta(t, 2.0, gb.Bids[0].Price, 1e-9)
	require.True(t, gb.Bids[0].Mine)
	require.InDelta(t, 3.0, gb.Bids[1].Price, 1e-9)
	require.False(t, gb.Bids[1].Mine)
	require.NotNil(t, gb.MyBest)
	require.InDelta(t, 2.0, *gb.MyBest, 1e-9)
	require.Equal(t, 1, gb.MyRank)

	// (1,c): supplier 7 owns a DISABLED channel (excluded from bids). Only competitor 0.9 appears.
	// MyBest=nil, MyRank=0.
	gc := byKey[keyOf(1, "c")]
	require.Len(t, gc.Bids, 1)
	require.InDelta(t, 0.9, gc.Bids[0].Price, 1e-9)
	require.False(t, gc.Bids[0].Mine)
	require.Nil(t, gc.MyBest)
	require.Equal(t, 0, gc.MyRank)

	// (2,x): supplier 7 owns enabled-but-nil-cost (excluded from bids). Only competitor 4.0 appears.
	gx := byKey[keyOf(2, "x")]
	require.Len(t, gx.Bids, 1)
	require.InDelta(t, 4.0, gx.Bids[0].Price, 1e-9)
	require.False(t, gx.Bids[0].Mine)
	require.Nil(t, gx.MyBest)
	require.Equal(t, 0, gx.MyRank)

	// Result is sorted by (TypeName, Group).
	require.True(t, sort.SliceIsSorted(groups, func(i, j int) bool {
		if groups[i].TypeName != groups[j].TypeName {
			return groups[i].TypeName < groups[j].TypeName
		}
		return groups[i].Group < groups[j].Group
	}), "groups must be sorted by (TypeName, Group)")
}

func keyOf(typ int, group string) string {
	return constant.GetChannelTypeName(typ) + "|" + group
}

func TestGetSupplierUsageSeries(t *testing.T) {
	setupSupplierStatsTables(t)

	// Anchor on day boundaries for deterministic bucketing.
	const day0 int64 = 1_700_000_000 - (1_700_000_000 % 86400) // a day start
	day1 := day0 + 86400
	day2 := day0 + 2*86400

	// Supplier channels: 101 and 102.
	// day0: two requests on ch101.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 10, CompletionTokens: 5, OfficialUsd: 1.0, CreatedAt: day0 + 10}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 20, CompletionTokens: 0, OfficialUsd: 2.0, CreatedAt: day0 + 20}).Error)
	// day1: one request on ch102.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 102, PromptTokens: 3, CompletionTokens: 7, OfficialUsd: 0.5, CreatedAt: day1 + 30}).Error)
	// day2: one request on ch101.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 1, CompletionTokens: 1, OfficialUsd: 0.1, CreatedAt: day2 + 40}).Error)

	// Non-consume type (excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 101, PromptTokens: 100, CompletionTokens: 100, OfficialUsd: 99, CreatedAt: day0 + 5}).Error)
	// Channel not in set (excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, PromptTokens: 50, CompletionTokens: 50, OfficialUsd: 9, CreatedAt: day0 + 6}).Error)
	// Before start (excluded by range).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 7, CompletionTokens: 7, OfficialUsd: 7, CreatedAt: day0 - 100}).Error)
	// After end (excluded by range).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, PromptTokens: 7, CompletionTokens: 7, OfficialUsd: 7, CreatedAt: day2 + 86400 + 100}).Error)

	series, err := GetSupplierUsageSeries([]int{101, 102}, day0, day2+86399)
	require.NoError(t, err)
	require.Len(t, series, 3)

	// Ascending by day.
	require.Equal(t, day0, series[0].Day)
	require.Equal(t, day1, series[1].Day)
	require.Equal(t, day2, series[2].Day)

	// day0: 2 requests, tokens 10+5+20+0=35, usd 3.0
	require.Equal(t, int64(2), series[0].Requests)
	require.Equal(t, int64(35), series[0].Tokens)
	require.InDelta(t, 3.0, series[0].OfficialUsd, 1e-9)

	// day1: 1 request, tokens 10, usd 0.5
	require.Equal(t, int64(1), series[1].Requests)
	require.Equal(t, int64(10), series[1].Tokens)
	require.InDelta(t, 0.5, series[1].OfficialUsd, 1e-9)

	// day2: 1 request, tokens 2, usd 0.1
	require.Equal(t, int64(1), series[2].Requests)
	require.Equal(t, int64(2), series[2].Tokens)
	require.InDelta(t, 0.1, series[2].OfficialUsd, 1e-9)

	// Empty channelIds short-circuits without running IN ().
	empty, err := GetSupplierUsageSeries(nil, day0, day2+86399)
	require.NoError(t, err)
	require.Len(t, empty, 0)
}

func TestGetSupplierChannelRanking(t *testing.T) {
	setupSupplierStatsTables(t)

	// Channels for name backfill.
	require.NoError(t, DB.Create(&Channel{Id: 201, Name: "alpha", Key: "k1", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 202, Name: "beta", Key: "k2", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 203, Name: "gamma", Key: "k3", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)

	const start int64 = 1_700_000_000
	const end int64 = 1_700_000_000 + 86400

	// ch201: 1 request, usd 5.0
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 201, PromptTokens: 10, CompletionTokens: 0, OfficialUsd: 5.0, CreatedAt: start + 10}).Error)
	// ch202: 2 requests, usd 3.0 total
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 202, PromptTokens: 1, CompletionTokens: 1, OfficialUsd: 1.5, CreatedAt: start + 20}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 202, PromptTokens: 1, CompletionTokens: 1, OfficialUsd: 1.5, CreatedAt: start + 21}).Error)
	// ch203: 3 requests, usd 3.0 total (same usd as ch202 but more requests => ranks above ch202)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 203, PromptTokens: 2, CompletionTokens: 0, OfficialUsd: 1.0, CreatedAt: start + 30}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 203, PromptTokens: 2, CompletionTokens: 0, OfficialUsd: 1.0, CreatedAt: start + 31}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 203, PromptTokens: 2, CompletionTokens: 0, OfficialUsd: 1.0, CreatedAt: start + 32}).Error)

	// Out of range / wrong type / wrong channel (excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 201, PromptTokens: 99, CompletionTokens: 99, OfficialUsd: 99, CreatedAt: end + 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 201, PromptTokens: 99, CompletionTokens: 99, OfficialUsd: 99, CreatedAt: start + 5}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, PromptTokens: 99, CompletionTokens: 99, OfficialUsd: 99, CreatedAt: start + 5}).Error)

	ranking, err := GetSupplierChannelRanking([]int{201, 202, 203}, start, end)
	require.NoError(t, err)
	require.Len(t, ranking, 3)

	// Order: usd DESC (201=5.0 first), then for tie 202==203==3.0, requests DESC (203 has 3 > 202 has 2).
	require.Equal(t, 201, ranking[0].ChannelId)
	require.Equal(t, "alpha", ranking[0].ChannelName)
	require.Equal(t, int64(1), ranking[0].Requests)
	require.Equal(t, int64(10), ranking[0].Tokens)
	require.InDelta(t, 5.0, ranking[0].OfficialUsd, 1e-9)

	require.Equal(t, 203, ranking[1].ChannelId)
	require.Equal(t, "gamma", ranking[1].ChannelName)
	require.Equal(t, int64(3), ranking[1].Requests)
	require.Equal(t, int64(6), ranking[1].Tokens)
	require.InDelta(t, 3.0, ranking[1].OfficialUsd, 1e-9)

	require.Equal(t, 202, ranking[2].ChannelId)
	require.Equal(t, "beta", ranking[2].ChannelName)
	require.Equal(t, int64(2), ranking[2].Requests)
	require.Equal(t, int64(4), ranking[2].Tokens)
	require.InDelta(t, 3.0, ranking[2].OfficialUsd, 1e-9)

	// Empty channelIds short-circuits.
	empty, err := GetSupplierChannelRanking(nil, start, end)
	require.NoError(t, err)
	require.Len(t, empty, 0)
}

func TestGetSupplierRealtimeStat(t *testing.T) {
	setupSupplierStatsTables(t)
	now := time.Now().Unix()

	// In-window log (last 60s) on a supplier channel.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 301, PromptTokens: 12, CompletionTokens: 8, Quota: 100, CreatedAt: now}).Error)
	// Old log (>60s ago) excluded from rpm/tpm, but within 24h => counts toward Quota.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 301, PromptTokens: 1000, CompletionTokens: 1000, Quota: 50, CreatedAt: now - 120}).Error)
	// Channel not owned (excluded entirely).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, PromptTokens: 5, CompletionTokens: 5, Quota: 999, CreatedAt: now}).Error)

	stat, err := GetSupplierRealtimeStat([]int{301})
	require.NoError(t, err)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 20, stat.Tpm) // 12+8 from the in-window log only
	require.Equal(t, 150, stat.Quota) // 100 + 50 within 24h

	// Empty channelIds => zero Stat.
	zero, err := GetSupplierRealtimeStat(nil)
	require.NoError(t, err)
	require.Equal(t, Stat{}, zero)
}

func TestGetUnsettledOfficialUsdByChannels(t *testing.T) {
	setupSupplierStatsTables(t)

	// Channel 101: two unsettled consume logs (settlement_id=0) => 1.0 + 2.5 = 3.5.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, OfficialUsd: 1.0, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, OfficialUsd: 2.5, SettlementId: 0, CreatedAt: 110}).Error)
	// Channel 101: a SETTLED log (settlement_id=99) => excluded.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, OfficialUsd: 50, SettlementId: 99, CreatedAt: 120}).Error)

	// Channel 102: one unsettled consume log => 4.0.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 102, OfficialUsd: 4.0, SettlementId: 0, CreatedAt: 130}).Error)
	// Channel 102: settled => excluded.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 102, OfficialUsd: 7.0, SettlementId: 5, CreatedAt: 140}).Error)

	// Non-consume type on a requested channel => excluded.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 101, OfficialUsd: 99, SettlementId: 0, CreatedAt: 150}).Error)

	// Unsettled log on a channel NOT requested => excluded from result.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, OfficialUsd: 8.0, SettlementId: 0, CreatedAt: 160}).Error)

	m, err := GetUnsettledOfficialUsdByChannels([]int{101, 102, 103})
	require.NoError(t, err)
	require.InDelta(t, 3.5, m[101], 1e-9)
	require.InDelta(t, 4.0, m[102], 1e-9)
	// Channel 103 has no logs => absent from map (or zero).
	_, ok := m[103]
	require.False(t, ok, "channel 103 has no unsettled logs => must be absent")
	// Channel 999 not requested => absent.
	_, ok = m[999]
	require.False(t, ok, "channel 999 was not requested => must be absent")

	// Empty input short-circuits => empty map, nil error, no IN ().
	empty, err := GetUnsettledOfficialUsdByChannels(nil)
	require.NoError(t, err)
	require.Len(t, empty, 0)
	require.NotNil(t, empty, "empty input should return a non-nil empty map")
}

func TestGetSupplierChannelIds(t *testing.T) {
	setupSupplierStatsTables(t)
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "k1", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "b", Key: "k2", SupplierId: 7, Status: common.ChannelStatusManuallyDisabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "c", Key: "k3", SupplierId: 9, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)

	ids, err := GetSupplierChannelIds(7)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	sort.Ints(ids)
	// ids belong to supplier 7 only (any status).
	for _, id := range ids {
		var ch Channel
		require.NoError(t, DB.First(&ch, id).Error)
		require.Equal(t, 7, ch.SupplierId)
	}
}

// V12：累计已跑金额($)——不带 settlement_id 过滤，统计渠道历史全部消费（含已结算）。
func TestGetTotalOfficialUsdByChannels(t *testing.T) {
	setupSupplierStatsTables(t)

	// ch101: 未结算 1.0 + 已结算(id=99) 2.0 => 累计 3.0。
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, OfficialUsd: 1.0, SettlementId: 0, CreatedAt: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 101, OfficialUsd: 2.0, SettlementId: 99, CreatedAt: 110}).Error)
	// ch102: 已结算 4.0 => 4.0（累计口径仍计入）。
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 102, OfficialUsd: 4.0, SettlementId: 5, CreatedAt: 120}).Error)
	// 非消费类型 => 排除。
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 101, OfficialUsd: 99, SettlementId: 0, CreatedAt: 130}).Error)
	// 未请求的渠道 => 排除。
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, OfficialUsd: 8.0, SettlementId: 0, CreatedAt: 140}).Error)

	m, err := GetTotalOfficialUsdByChannels([]int{101, 102, 103})
	require.NoError(t, err)
	require.InDelta(t, 3.0, m[101], 1e-9) // 含已结算
	require.InDelta(t, 4.0, m[102], 1e-9)
	_, ok := m[103]
	require.False(t, ok, "ch103 无日志 => 不在 map 中")
	_, ok = m[999]
	require.False(t, ok, "ch999 未请求 => 不在 map 中")

	empty, err := GetTotalOfficialUsdByChannels(nil)
	require.NoError(t, err)
	require.Len(t, empty, 0)
	require.NotNil(t, empty)
}

// V12：每供应商渠道计数（上架=全部状态，启用=status=enabled），admin 渠道(supplier_id=0)排除。
func TestGetAllSuppliersChannelCounts(t *testing.T) {
	setupSupplierStatsTables(t)
	// supplier 7: 2 enabled + 2 disabled。
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "c1", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "b", Key: "c2", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "c", Key: "c3", SupplierId: 7, Status: common.ChannelStatusManuallyDisabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "d", Key: "c4", SupplierId: 7, Status: common.ChannelStatusAutoDisabled, Models: "m", Group: "g"}).Error)
	// supplier 9: 1 enabled。
	require.NoError(t, DB.Create(&Channel{Name: "e", Key: "c5", SupplierId: 9, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	// admin 渠道(supplier_id=0)排除。
	require.NoError(t, DB.Create(&Channel{Name: "f", Key: "c6", SupplierId: 0, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)

	m, err := GetAllSuppliersChannelCounts()
	require.NoError(t, err)
	require.Equal(t, 4, m[7].Total)
	require.Equal(t, 2, m[7].Enabled)
	require.Equal(t, 1, m[9].Total)
	require.Equal(t, 1, m[9].Enabled)
	_, ok := m[0]
	require.False(t, ok, "admin 渠道(supplier_id=0)不计入")
}

// V12：供应商列表项回填渠道总数/启用数。
func TestFillSupplierStatsChannelCounts(t *testing.T) {
	setupSupplierStatsTables(t)
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "d1", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "b", Key: "d2", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "c", Key: "d3", SupplierId: 7, Status: common.ChannelStatusManuallyDisabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "e", Key: "d4", SupplierId: 9, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)

	items := []*SupplierListItem{{UserId: 7}, {UserId: 9}, {UserId: 11}}
	require.NoError(t, fillSupplierStats(items))

	require.Equal(t, 3, items[0].ChannelTotal)
	require.Equal(t, 2, items[0].ChannelEnabled)
	require.Equal(t, 1, items[1].ChannelTotal)
	require.Equal(t, 1, items[1].ChannelEnabled)
	require.Equal(t, 0, items[2].ChannelTotal)
	require.Equal(t, 0, items[2].ChannelEnabled)
}
