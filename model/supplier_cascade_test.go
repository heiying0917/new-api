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

// 管理员手动停用的供应商不参与级联恢复：渠道恢复可用也保持停用；只有因无可用渠道被自动停用的才会恢复。
func TestCascadeSupplier_AdminDisabledStaysDisabled(t *testing.T) {
	setupCascadeTables(t)
	require.NoError(t, DB.Create(&Supplier{UserId: 7, Enabled: true}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "a", Key: "k1", SupplierId: 7, Status: common.ChannelStatusAutoDisabled, Models: "m", Group: "g"}).Error)

	// 全部渠道不可用 → 自动停用并打标
	require.NoError(t, CascadeSupplierBySupplierId(7))
	s, err := GetSupplierByUserId(7)
	require.NoError(t, err)
	require.False(t, s.Enabled)
	require.True(t, s.AutoDisabled)

	// 管理员明确停用 → 清除自动标记；之后渠道恢复也不能把供应商启用
	require.NoError(t, UpdateSupplier(7, map[string]interface{}{"enabled": false}))
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 1).Update("status", common.ChannelStatusEnabled).Error)
	require.NoError(t, CascadeSupplierBySupplierId(7))
	s, err = GetSupplierByUserId(7)
	require.NoError(t, err)
	require.False(t, s.Enabled, "管理员停用的供应商不被级联启用")
	require.False(t, s.AutoDisabled)

	// 管理员停用期间渠道全部失效：仍是管理员停用，不改记为自动停用
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 1).Update("status", common.ChannelStatusAutoDisabled).Error)
	require.NoError(t, CascadeSupplierBySupplierId(7))
	s, err = GetSupplierByUserId(7)
	require.NoError(t, err)
	require.False(t, s.AutoDisabled)

	// 管理员重新启用后恢复正常级联
	require.NoError(t, UpdateSupplier(7, map[string]interface{}{"enabled": true}))
	require.NoError(t, CascadeSupplierBySupplierId(7))
	s, err = GetSupplierByUserId(7)
	require.NoError(t, err)
	require.False(t, s.Enabled)
	require.True(t, s.AutoDisabled)
}
