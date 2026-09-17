package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// TestUpdateUserRole 角色单列更新：只改 role，不碰 status/quota；拒绝游客/超管/非法值。
func TestUpdateUserRole(t *testing.T) {
	seed := &User{
		Username: "role_target",
		Password: "x",
		Role:     common.RoleSupplierUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
		AffCode:  "vrole",
	}
	require.NoError(t, DB.Create(seed).Error)

	require.NoError(t, UpdateUserRole(seed.Id, common.RoleViewerUser))
	var got User
	require.NoError(t, DB.First(&got, seed.Id).Error)
	require.Equal(t, common.RoleViewerUser, got.Role)
	require.Equal(t, common.UserStatusEnabled, got.Status)
	require.Equal(t, 100, got.Quota)

	for _, bad := range []int{common.RoleGuestUser, common.RoleRootUser, 4, -1} {
		require.Error(t, UpdateUserRole(seed.Id, bad), "role %d must be rejected", bad)
	}
	require.Error(t, UpdateUserRole(0, common.RoleCommonUser))
	require.NoError(t, DB.First(&got, seed.Id).Error)
	require.Equal(t, common.RoleViewerUser, got.Role, "rejected updates must not touch role")
}
