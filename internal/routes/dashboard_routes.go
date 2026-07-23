package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

func RegisterDashboardRoutes(h *handlers.DashboardHandler) {
	staff := []string{models.RoleAdmin, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist}

	// Always-loaded summary (top 3 stat cards).
	http.HandleFunc("GET /api/v1/dashboard/summary",
		withAuthAndRole(h.Summary, staff...))

	// Per-tab panels — fetched only when their tab is selected.
	http.HandleFunc("GET /api/v1/dashboard/bookings",
		withAuthAndRole(h.Bookings, staff...))
	http.HandleFunc("GET /api/v1/dashboard/orders",
		withAuthAndRole(h.Orders, staff...))
	http.HandleFunc("GET /api/v1/dashboard/invoices",
		withAuthAndRole(h.Invoices, staff...))
}
