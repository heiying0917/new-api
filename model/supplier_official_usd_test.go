package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOfficialUsdDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	prevDB, prevLog := DB, LOG_DB
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB, LOG_DB = prevDB, prevLog
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestSumSupplierOfficialUsd 只汇总本人渠道、消费类型、时间窗内的 official_usd，与 quota 无关。
func TestSumSupplierOfficialUsd(t *testing.T) {
	setupOfficialUsdDB(t)
	rows := []Log{
		{Type: LogTypeConsume, ChannelId: 1, CreatedAt: 100, Quota: 99999, OfficialUsd: 1.5},
		{Type: LogTypeConsume, ChannelId: 1, CreatedAt: 200, Quota: 99999, OfficialUsd: 2.25},
		{Type: LogTypeConsume, ChannelId: 2, CreatedAt: 150, Quota: 99999, OfficialUsd: 100}, // 他人渠道
		{Type: LogTypeError, ChannelId: 1, CreatedAt: 150, Quota: 0, OfficialUsd: 50},        // 非消费
		{Type: LogTypeConsume, ChannelId: 1, CreatedAt: 999, Quota: 1, OfficialUsd: 7},       // 窗外
	}
	require.NoError(t, LOG_DB.Create(&rows).Error)

	got, err := SumSupplierOfficialUsd([]int{1}, 100, 300)
	require.NoError(t, err)
	require.InDelta(t, 3.75, got, 1e-9)

	all, err := SumSupplierOfficialUsd([]int{1}, 0, 0)
	require.NoError(t, err)
	require.InDelta(t, 10.75, all, 1e-9)

	none, err := SumSupplierOfficialUsd(nil, 0, 0)
	require.NoError(t, err)
	require.Equal(t, float64(0), none)
}
