package common

import "testing"

func TestIsValidateRole_Supplier(t *testing.T) {
	if RoleSupplierUser != 5 {
		t.Fatalf("RoleSupplierUser 期望 5, 实际 %d", RoleSupplierUser)
	}
	if !IsValidateRole(RoleSupplierUser) {
		t.Fatalf("IsValidateRole(RoleSupplierUser) 应为 true")
	}
	for _, r := range []int{RoleGuestUser, RoleCommonUser, RoleAdminUser, RoleRootUser} {
		if !IsValidateRole(r) {
			t.Fatalf("IsValidateRole(%d) 应为 true", r)
		}
	}
	if IsValidateRole(7) {
		t.Fatalf("IsValidateRole(7) 应为 false")
	}
}

// TestIsValidateRole_Viewer 观察员(3)必须是合法角色，否则 authHelper 会拒绝其会话；
// 且必须落在普通用户(1)与供应商(5)之间，保证线性门槛下观察员过 UserAuth、不过 SupplierAuth。
func TestIsValidateRole_Viewer(t *testing.T) {
	if !IsValidateRole(RoleViewerUser) {
		t.Fatalf("RoleViewerUser must be valid")
	}
	if RoleViewerUser <= RoleCommonUser || RoleViewerUser >= RoleSupplierUser {
		t.Fatalf("RoleViewerUser must sit between common(1) and supplier(5), got %d", RoleViewerUser)
	}
	for _, bad := range []int{2, 4, 6, 9, 11, 99, 101, -1} {
		if IsValidateRole(bad) {
			t.Fatalf("role %d must be invalid", bad)
		}
	}
}
