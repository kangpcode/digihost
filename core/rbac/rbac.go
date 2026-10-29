// DigiPanel Role-Based Access Control (RBAC) Engine (core/rbac/rbac.go)
// Fine-grained permission model for Admin, Reseller, User, and Staff roles.
// Zero emojis, strictly production grade.

package rbac

import (
	"errors"
	"fmt"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleReseller Role = "reseller"
	RoleUser     Role = "user"
	RoleStaff    Role = "staff"
)

var (
	ErrUnauthorizedRole = errors.New("unauthorized role")
	ErrPermissionDenied = errors.New("permission denied for required capability")
)

var rolePermissions = map[Role]map[string]bool{
	RoleAdmin: {
		"*":                       true, // Superuser wildcard
		"system.fleet.read":       true,
		"system.fleet.write":      true,
		"system.modules.manage":   true,
		"system.telemetry.view":   true,
		"accounts.create":         true,
		"accounts.read":           true,
		"accounts.suspend":        true,
		"accounts.terminate":      true,
		"accounts.modify_package": true,
		"dns.read":                true,
		"dns.write":               true,
		"dns.delete":              true,
		"dns.sec":                 true,
		"files.read":              true,
		"files.write":             true,
		"runtimes.view":           true,
		"runtimes.manage":         true,
		"ssl.read":                true,
		"ssl.issue":               true,
		"ssl.renew":               true,
		"ssl.install":             true,
		"mail.read":               true,
		"mail.write":              true,
		"mail.dkim":               true,
		"mail.antispam":           true,
		"billing.ledger.read":     true,
		"billing.invoices.read":   true,
		"billing.invoices.write":  true,
		"audit.logs.read":         true,
	},
	RoleReseller: {
		"system.telemetry.view":   true,
		"accounts.create":         true,
		"accounts.read":           true,
		"accounts.suspend":        true,
		"accounts.modify_package": true,
		"dns.read":                true,
		"dns.write":               true,
		"ssl.read":                true,
		"ssl.issue":               true,
		"ssl.renew":               true,
		"mail.read":               true,
		"mail.write":              true,
		"files.read":              true,
		"files.write":             true,
		"runtimes.view":           true,
		"billing.invoices.read":   true,
		"billing.invoices.write":  true,
	},
	RoleUser: {
		"system.telemetry.view": true,
		"accounts.read":         true,
		"dns.read":              true,
		"dns.write":             true,
		"ssl.read":              true,
		"ssl.issue":             true,
		"ssl.renew":             true,
		"mail.read":             true,
		"mail.write":            true,
		"files.read":            true,
		"files.write":           true,
		"runtimes.view":         true,
		"runtimes.manage":       true,
		"billing.invoices.read": true,
	},
	RoleStaff: {
		"system.telemetry.view": true,
		"system.fleet.read":     true,
		"accounts.read":         true,
		"dns.read":              true,
		"ssl.read":              true,
		"mail.read":             true,
		"runtimes.view":         true,
		"billing.invoices.read": true,
		"audit.logs.read":       true,
	},
}

// HasPermission checks whether a given role holds the requested permission
func HasPermission(role Role, permission string) bool {
	perms, exists := rolePermissions[role]
	if !exists {
		return false
	}
	if perms["*"] {
		return true
	}
	return perms[permission]
}

// ValidateAccess returns an error if the role lacks the required permission
func ValidateAccess(role Role, requiredPermission string) error {
	if !HasPermission(role, requiredPermission) {
		return fmt.Errorf("%w: role '%s' lacks '%s'", ErrPermissionDenied, role, requiredPermission)
	}
	return nil
}

// GetRolePermissions returns all permission keys granted to a role
func GetRolePermissions(role Role) []string {
	perms, exists := rolePermissions[role]
	if !exists {
		return nil
	}
	res := make([]string, 0, len(perms))
	for p := range perms {
		res = append(res, p)
	}
	return res
}
