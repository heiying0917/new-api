package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestGetSupplierLogs verifies channel-scoped log listing: IN filter, pagination,
// model/type filters, ChannelName backfill, and empty short-circuit.
func TestGetSupplierLogs(t *testing.T) {
	setupSupplierStatsTables(t)

	// Channels for name backfill: c1, c2 owned by supplier A; c9 not owned.
	require.NoError(t, DB.Create(&Channel{Id: 401, Name: "chan-one", Key: "k1", SupplierId: 7, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 402, Name: "chan-two", Key: "k2", SupplierId: 7, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 409, Name: "chan-nine", Key: "k9", SupplierId: 9, Status: 1, Models: "m", Group: "g"}).Error)

	const base int64 = 1_700_000_000

	// Owned-channel consume logs (varied created_at, models, with consumer identity set).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 401, ModelName: "gpt-4", PromptTokens: 10, CompletionTokens: 5, Quota: 100, Username: "alice", TokenName: "tok-a", CreatedAt: base + 10}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 401, ModelName: "gpt-3.5", PromptTokens: 1, CompletionTokens: 1, Quota: 20, Username: "bob", TokenName: "tok-b", CreatedAt: base + 20}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 402, ModelName: "gpt-4", PromptTokens: 3, CompletionTokens: 7, Quota: 30, Username: "carol", TokenName: "tok-c", CreatedAt: base + 30}).Error)
	// A non-consume (type=3) log on an owned channel — included only when logType=0/all.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 402, ModelName: "gpt-4", PromptTokens: 0, CompletionTokens: 0, Quota: 0, Username: "dave", CreatedAt: base + 40}).Error)
	// Log on a channel NOT owned by supplier A (must be excluded).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 409, ModelName: "gpt-4", PromptTokens: 99, CompletionTokens: 99, Quota: 999, Username: "evil", CreatedAt: base + 50}).Error)

	channelIds := []int{401, 402}

	// 1. All types (logType=0): 4 owned logs total, none from c9.
	logs, total, err := GetSupplierLogs(channelIds, 0, 0, 0, "", 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, logs, 4)
	for _, l := range logs {
		require.Contains(t, []int{401, 402}, l.ChannelId, "must not leak non-owned channel logs")
	}

	// 2. Consume-only (logType=2): excludes the type=3 log => 3.
	logs, total, err = GetSupplierLogs(channelIds, LogTypeConsume, 0, 0, "", 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, logs, 3)
	for _, l := range logs {
		require.Equal(t, LogTypeConsume, l.Type)
	}

	// 3. Model filter: gpt-4 consume logs => 2 (c401 + c402).
	logs, total, err = GetSupplierLogs(channelIds, LogTypeConsume, 0, 0, "gpt-4", 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, logs, 2)
	for _, l := range logs {
		require.Equal(t, "gpt-4", l.ModelName)
	}

	// 4. ChannelName backfilled.
	logs, _, err = GetSupplierLogs([]int{401}, LogTypeConsume, 0, 0, "", 0, 100)
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	for _, l := range logs {
		require.Equal(t, "chan-one", l.ChannelName)
	}

	// 5. Pagination: 4 all-type logs, page size 2.
	page1, total, err := GetSupplierLogs(channelIds, 0, 0, 0, "", 0, 2)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, page1, 2)
	page2, _, err := GetSupplierLogs(channelIds, 0, 0, 0, "", 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 2)
	// Pages are disjoint.
	require.NotEqual(t, page1[0].Id, page2[0].Id)
	require.NotEqual(t, page1[1].Id, page2[1].Id)

	// 6. Time-range filter: only base+20..base+30 (consume) => 2.
	logs, total, err = GetSupplierLogs(channelIds, LogTypeConsume, base+20, base+30, "", 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, logs, 2)

	// 7. Empty channelIds short-circuits without running IN ().
	empty, total, err := GetSupplierLogs(nil, 0, 0, 0, "", 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.Len(t, empty, 0)

	// 8. GetSupplierLogs returns the REAL Username (controller is responsible for blanking).
	logs, _, err = GetSupplierLogs([]int{401}, LogTypeConsume, base+10, base+10, "", 0, 100)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "alice", logs[0].Username)
	require.Equal(t, "tok-a", logs[0].TokenName)
}

// TestSumSupplierStat verifies range-bounded quota sum and last-60s rpm/tpm.
func TestSumSupplierStat(t *testing.T) {
	setupSupplierStatsTables(t)
	now := time.Now().Unix()

	// Recent log (last 60s) on owned channel.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 501, PromptTokens: 12, CompletionTokens: 8, Quota: 100, CreatedAt: now}).Error)
	// Old log (>60s ago) excluded from rpm/tpm.
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 501, PromptTokens: 1000, CompletionTokens: 1000, Quota: 50, CreatedAt: now - 120}).Error)
	// Non-owned channel (excluded entirely).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 999, PromptTokens: 5, CompletionTokens: 5, Quota: 999, CreatedAt: now}).Error)
	// Non-consume on owned channel (excluded from quota sum).
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume + 1, ChannelId: 501, PromptTokens: 5, CompletionTokens: 5, Quota: 7, CreatedAt: now}).Error)

	// Unbounded range (0,0): quota = 100 + 50 = 150; rpm=1, tpm=20 (last 60s only).
	stat, err := SumSupplierStat([]int{501}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, 150, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 20, stat.Tpm)

	// Range-bounded: only [now-1, now+1] => quota=100 (excludes the now-120 log).
	stat, err = SumSupplierStat([]int{501}, now-1, now+1)
	require.NoError(t, err)
	require.Equal(t, 100, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 20, stat.Tpm)

	// Empty channelIds => zero Stat.
	zero, err := SumSupplierStat(nil, 0, 0)
	require.NoError(t, err)
	require.Equal(t, Stat{}, zero)
}
