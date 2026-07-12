package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID               uuid.UUID  `json:"user_id"`
	OrgID                *uuid.UUID `json:"org_id,omitempty"`
	OrgName              string     `json:"org_name,omitempty"`
	OrgLogoURL           string     `json:"org_logo_url,omitempty"`
	BranchID             *uuid.UUID `json:"branch_id,omitempty"`
	BranchName           string     `json:"branch_name,omitempty"`
	FullName             string     `json:"full_name"`
	Email                string     `json:"email"`
	Password             string     `json:"-"`
	RoleID               *uuid.UUID `json:"role_id,omitempty"`
	Role                 *Role      `json:"role,omitempty"`
	// RoleName is accepted on create/update requests to resolve the role by name.
	// It is never persisted directly; the service resolves it to RoleID.
	RoleName             string     `json:"role_name,omitempty"`
	IsActive             bool       `json:"is_active"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	ChangePassword       bool       `json:"change_password"`
	PasswordChangedAt    *time.Time `json:"password_changed_at,omitempty"`
	PasswordExpiresAt    *time.Time `json:"password_expires_at,omitempty"`
	FailedLoginAttempts  int        `json:"-"` // Never expose in JSON
	IsLocked             bool       `json:"is_locked"`
	LockedUntil          *time.Time `json:"locked_until,omitempty"`
	LastLoginAt          *time.Time `json:"last_login,omitempty"`
}

// HasPermission checks role-based access using lodge role names.
// The permission string follows the format "resource:action".
func (u *User) HasPermission(permission string) bool {
	if u.Role == nil {
		return false
	}
	switch u.Role.Name {
	case RoleAdmin, RoleBranchAdmin:
		return true
	case RoleManager:
		allowed := map[string]bool{
			"rooms:read":     true,
			"rooms:write":    true,
			"bookings:read":  true,
			"bookings:write": true,
			"bookings:approve": true,
			"invoices:read":  true,
			"invoices:write": true,
			"clients:read":   true,
			"clients:write":  true,
			"reports:read":   true,
			"cleaning:read":  true,
			"cleaning:write": true,
		}
		return allowed[permission]
	case RoleReceptionist:
		allowed := map[string]bool{
			"rooms:read":     true,
			"bookings:read":  true,
			"bookings:write": true,
			"invoices:read":  true,
			"clients:read":   true,
			"clients:write":  true,
		}
		return allowed[permission]
	case RoleKitchenStaff:
		allowed := map[string]bool{
			"orders:read":  true,
			"orders:write": true,
			"menu:read":    true,
		}
		return allowed[permission]
	case RoleWaiter:
		allowed := map[string]bool{
			"orders:read":  true,
			"orders:write": true,
			"menu:read":    true,
		}
		return allowed[permission]
	case RoleBarStaff:
		allowed := map[string]bool{
			"orders:read":  true,
			"orders:write": true,
			"menu:read":    true,
		}
		return allowed[permission]
	}
	return false
}
