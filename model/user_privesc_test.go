package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEdit_DoesNotEscalateRoleStatusQuota 锁定 User.Edit 的字段白名单：
// 即便传入的结构体携带越权的 role/status/quota（模拟管理员更新路径被注入），
// Edit 也只写 username/display_name/group/remark，绝不改动 role/status/quota。
func TestEdit_DoesNotEscalateRoleStatusQuota(t *testing.T) {
	seed := &User{
		Username: "victim_edit",
		Password: "x",
		Role:     common.RoleSupplierUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
		AffCode:  "vedit",
	}
	require.NoError(t, DB.Create(seed).Error)

	// 携带越权字段的“脏”结构体
	evil := &User{
		Id:          seed.Id,
		Username:    "victim_edit",
		DisplayName: "changed",
		Group:       "default",
		Role:        common.RoleRootUser,        // 想提权成超管
		Status:      common.UserStatusDisabled,  // 想顺手改状态
		Quota:       999999,                     // 想顺手加额度
	}
	require.NoError(t, evil.Edit(false))

	var got User
	require.NoError(t, DB.First(&got, seed.Id).Error)
	assert.Equal(t, common.RoleSupplierUser, got.Role, "Edit 不得修改 role")
	assert.Equal(t, common.UserStatusEnabled, got.Status, "Edit 不得修改 status")
	assert.Equal(t, 100, got.Quota, "Edit 不得修改 quota")
	assert.Equal(t, "changed", got.DisplayName, "Edit 应正常更新白名单字段")
}

// TestUpdate_CleanStructLeavesPrivilegeFieldsIntact 复刻 UpdateSelf 的实际机制：
// 控制器只构造仅含 Id/Username/Password/DisplayName 的 cleanUser 再调 Update；
// 由于 GORM Updates(struct) 跳过零值字段，role/status/quota 不会被触碰。
func TestUpdate_CleanStructLeavesPrivilegeFieldsIntact(t *testing.T) {
	seed := &User{
		Username: "victim_upd",
		Password: "x",
		Role:     common.RoleSupplierUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
		AffCode:  "vupd",
	}
	require.NoError(t, DB.Create(seed).Error)

	// UpdateSelf 中的 cleanUser：只设安全字段，越权字段保持零值（会被 Updates 跳过）
	clean := &User{
		Id:          seed.Id,
		Username:    "victim_upd",
		DisplayName: "self-renamed",
	}
	require.NoError(t, clean.Update(false))

	var got User
	require.NoError(t, DB.First(&got, seed.Id).Error)
	assert.Equal(t, common.RoleSupplierUser, got.Role, "cleanUser.Update 不得修改 role")
	assert.Equal(t, common.UserStatusEnabled, got.Status, "cleanUser.Update 不得修改 status")
	assert.Equal(t, 100, got.Quota, "cleanUser.Update 不得修改 quota")
	assert.Equal(t, "self-renamed", got.DisplayName, "白名单字段应正常更新")
}
