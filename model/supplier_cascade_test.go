package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func setupCascadeTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
}

func TestCascadeSupplier(t *testing.T) {
	setupCascadeTables(t)
	require.NoError(t, DB.Create(&Supplier{UserId: 7, Enabled: true}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "a", Key: "k1", SupplierId: 7, Status: common.ChannelStatusEnabled, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "b", Key: "k2", SupplierId: 7, Status: common.ChannelStatusAutoDisabled, Models: "m", Group: "g"}).Error)

	// one enabled → supplier stays enabled
	require.NoError(t, CascadeSupplierBySupplierId(7))
	var s Supplier
	require.NoError(t, DB.First(&s, "user_id = ?", 7).Error)
	require.True(t, s.Enabled)

	// disable the remaining enabled channel → all disabled → supplier disabled
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 1).Update("status", common.ChannelStatusAutoDisabled).Error)
	require.NoError(t, CascadeSupplierBySupplierId(7))
	require.NoError(t, DB.First(&s, "user_id = ?", 7).Error)
	require.False(t, s.Enabled)

	// re-enable one channel → supplier re-enabled
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 1).Update("status", common.ChannelStatusEnabled).Error)
	require.NoError(t, CascadeSupplierBySupplierId(7))
	require.NoError(t, DB.First(&s, "user_id = ?", 7).Error)
	require.True(t, s.Enabled)

	// supplierId 0 → no-op, no error
	require.NoError(t, CascadeSupplierBySupplierId(0))
}
