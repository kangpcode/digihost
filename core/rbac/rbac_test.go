// DigiPanel RBAC Unit Tests (core/rbac/rbac_test.go)
// Verifies fine-grained role capabilities and access validation.

package rbac

import (
	"testing"
)

func TestAdminPermissions(t *testing.T) {
	adminPermissions := []string{
		"system.fleet.write",
		"system.modules.manage",
		"accounts.create",
		"accounts.terminate",
		"audit.logs.read",
		"any.arbitrary.permission", // Due to wildcard
	}

	for _, perm := range adminPermissions {
		if !HasPermission(RoleAdmin, perm) {
			t.Errorf("Expected Admin to have permission '%s'", perm)
		}
		if err := ValidateAccess(RoleAdmin, perm); err != nil {
			t.Errorf("Expected ValidateAccess for Admin on '%s' to succeed: %v", perm, err)
		}
	}
}

func TestResellerPermissions(t *testing.T) {
	allowed := []string{
		"accounts.create",
		"accounts.read",
		"dns.write",
		"billing.invoices.write",
	}

	for _, p := range allowed {
		if !HasPermission(RoleReseller, p) {
			t.Errorf("Expected Reseller to have permission '%s'", p)
		}
	}

	forbidden := []string{
		"system.fleet.write",
		"system.modules.manage",
		"audit.logs.read",
	}

	for _, p := range forbidden {
		if HasPermission(RoleReseller, p) {
			t.Errorf("Security failure: Reseller should not have permission '%s'", p)
		}
		if err := ValidateAccess(RoleReseller, p); err == nil {
			t.Errorf("Expected ValidateAccess to fail for Reseller on '%s'", p)
		}
	}
}

func TestUserPermissions(t *testing.T) {
	allowed := []string{
		"dns.read",
		"dns.write",
		"files.write",
		"runtimes.manage",
	}

	for _, p := range allowed {
		if !HasPermission(RoleUser, p) {
			t.Errorf("Expected User to have permission '%s'", p)
		}
	}

	forbidden := []string{
		"accounts.create",
		"system.fleet.read",
		"billing.invoices.write",
		"audit.logs.read",
	}

	for _, p := range forbidden {
		if HasPermission(RoleUser, p) {
			t.Errorf("Security failure: User should not have permission '%s'", p)
		}
	}
}
