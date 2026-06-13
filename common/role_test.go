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
