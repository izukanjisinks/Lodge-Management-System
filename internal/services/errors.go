package services

import (
	"errors"
	"strings"

	"github.com/lib/pq"
)

// formatConstraintError maps postgres constraint violations to human-readable messages.
// Handles both unique (23505) and not-null (23502) violation codes.
func formatConstraintError(err error) error {
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23502": // not_null_violation
			col := pqErr.Column
			if col == "branch_id" {
				return errors.New("branch is required")
			}
			if col == "org_id" {
				return errors.New("organisation is required")
			}
			if col != "" {
				return errors.New(col + " is required")
			}
			return errors.New("a required field is missing")
		case "23505": // unique_violation
			e := pqErr.Constraint
			switch {
			case strings.Contains(e, "uq_rooms_name_org"):
				return errors.New("a room with this name already exists")
			case strings.Contains(e, "uq_roles_name_org"):
				return errors.New("a role with this name already exists")
			case strings.Contains(e, "uq_individual_profiles_id_passport_number"):
				return errors.New("a client with this NRC/passport number already exists")
			case strings.Contains(e, "uq_cor_company_details_reg_number_org"):
				return errors.New("a company with this registration number already exists")
			case strings.Contains(e, "uq_orders_order_number_org"):
				return errors.New("an order with this number already exists")
			}
		}
	}
	// Fallback: string-match for callers that don't use pq.Error directly
	e := err.Error()
	switch {
	case strings.Contains(e, "uq_rooms_name_org"):
		return errors.New("a room with this name already exists")
	case strings.Contains(e, "uq_roles_name_org"):
		return errors.New("a role with this name already exists")
	case strings.Contains(e, "uq_individual_profiles_id_passport_number"):
		return errors.New("a client with this NRC/passport number already exists")
	case strings.Contains(e, "uq_cor_company_details_reg_number_org"):
		return errors.New("a company with this registration number already exists")
	case strings.Contains(e, "uq_orders_order_number_org"):
		return errors.New("an order with this number already exists")
	}
	return err
}
