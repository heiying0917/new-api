package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/stretchr/testify/require"
)

// TestValidateRoleChange 编辑弹窗改角色的规则表（设计文档 §3.1）。
func TestValidateRoleChange(t *testing.T) {
	root, admin := common.RoleRootUser, common.RoleAdminUser
	cases := []struct {
		name                 string
		myRole, myId         int
		targetId, originRole int
		newRole              int
		wantRole             int
		wantErr              string
	}{
		{"缺省不改(0)保留原角色", root, 1, 2, common.RoleSupplierUser, 0, common.RoleSupplierUser, ""},
		{"角色不变直接放行", admin, 1, 2, common.RoleCommonUser, common.RoleCommonUser, common.RoleCommonUser, ""},
		{"root 设观察员", root, 1, 2, common.RoleSupplierUser, common.RoleViewerUser, common.RoleViewerUser, ""},
		{"root 设管理员", root, 1, 2, common.RoleCommonUser, admin, admin, ""},
		{"root 把管理员降为普通用户", root, 1, 2, admin, common.RoleCommonUser, common.RoleCommonUser, ""},
		{"admin 设供应商", admin, 1, 2, common.RoleCommonUser, common.RoleSupplierUser, common.RoleSupplierUser, ""},
		{"admin 不能设管理员", admin, 1, 2, common.RoleCommonUser, admin, 0, i18n.MsgUserCannotCreateHigherLevel},
		{"任何人不能设 root", root, 1, 2, common.RoleCommonUser, root, 0, i18n.MsgUserRoleInvalid},
		{"非法值 4", root, 1, 2, common.RoleCommonUser, 4, 0, i18n.MsgUserRoleInvalid},
		{"非法负数", root, 1, 2, common.RoleCommonUser, -1, 0, i18n.MsgUserRoleInvalid},
		{"不能改 root 的角色", root, 1, 2, root, admin, 0, i18n.MsgUserCannotChangeRootRole},
		{"不能改自己的角色", root, 1, 1, root, admin, 0, i18n.MsgUserCannotChangeOwnRole},
		{"admin 改自己也拒", admin, 5, 5, admin, common.RoleCommonUser, 0, i18n.MsgUserCannotChangeOwnRole},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateRoleChange(tc.myRole, tc.myId, tc.targetId, tc.originRole, tc.newRole)
			if tc.wantErr == "" {
				require.Nil(t, err)
				require.Equal(t, tc.wantRole, got)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tc.wantErr, err.key)
		})
	}
}
