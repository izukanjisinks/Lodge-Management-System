package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin         = "admin"
	RoleBranchAdmin   = "branch_admin"
	RoleManager       = "manager"
	RoleReceptionist  = "receptionist"
	RoleKitchenStaff  = "kitchen_staff"
	RoleWaiter        = "waiter"
	RoleBarStaff      = "bar_staff"
)

type Role struct {
	RoleID      uuid.UUID `json:"role_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func GetPredefinedRoles() []Role {
	return []Role{
		{Name: RoleAdmin, Description: "Admin — full access including branch management"},
		{Name: RoleBranchAdmin, Description: "Branch admin — full access scoped to their assigned branch"},
		{Name: RoleManager, Description: "Oversees operations — approves bookings, views reports, manages rooms"},
		{Name: RoleReceptionist, Description: "Front-desk staff — handles bookings, clients, and invoices"},
		{Name: RoleKitchenStaff, Description: "Kitchen staff — views and updates meal orders and preparation status"},
		{Name: RoleWaiter, Description: "Waiter — takes and serves meal orders, manages table service"},
		{Name: RoleBarStaff, Description: "Bar staff — handles bar orders and updates bar order status"},
	}
}
