package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSupplierPendingStat(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Supplier{}, &Log{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
	cp1, cp2 := 2.5, 2.0
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "a", Key: "k1", SupplierId: 7, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "b", Key: "k2", SupplierId: 7, CostPrice: &cp2, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 3, Name: "c", Key: "k3", SupplierId: 9, CostPrice: &cp1, Models: "m", Group: "g"}).Error)
	// consume logs: ch1 official 0.10 (×2.5=0.25), ch2 official 0.20 (×2.0=0.40); ch3 belongs to other supplier
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 0.10, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 2, OfficialUsd: 0.20, SettlementId: 0}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 1, OfficialUsd: 0.05, SettlementId: 5}).Error) // already settled, excluded
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ChannelId: 3, OfficialUsd: 1.00, SettlementId: 0}).Error) // other supplier

	stat, err := GetSupplierPendingStat(7)
	require.NoError(t, err)
	require.InDelta(t, 0.30, stat.OfficialUsd, 1e-9)     // 0.10+0.20
	require.InDelta(t, 0.25+0.40, stat.PayableCNY, 1e-9) // 0.10*2.5 + 0.20*2.0
	require.Equal(t, int64(2), stat.LogCount)
}
