package services

import (
	"errors"
	"strings"
)

// formatConstraintError maps postgres unique constraint names to human-readable messages.
func formatConstraintError(err error) error {
	e := err.Error()
	switch {
	// rooms
	case strings.Contains(e, "uq_rooms_name_org"):
		return errors.New("a room with this name already exists")
	// roles
	case strings.Contains(e, "uq_roles_name_org"):
		return errors.New("a role with this name already exists")
	// individual profiles
	case strings.Contains(e, "uq_individual_profiles_id_passport_number"):
		return errors.New("a client with this NRC/passport number already exists")
	// corporate companies (cor_company_details)
	case strings.Contains(e, "uq_cor_company_details_reg_number_org"):
		return errors.New("a company with this registration number already exists")
	// orders
	case strings.Contains(e, "uq_orders_order_number_org"):
		return errors.New("an order with this number already exists")
	default:
		return err
	}
}
