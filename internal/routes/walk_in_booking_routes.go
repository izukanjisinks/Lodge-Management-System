package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

// RegisterWalkInBookingRoutes wires staff-authed, materialise-immediately meal and
// event booking creation. These mirror the website's envelope shape but confirm on
// submit (no approval workflow) — used when a guest books in person at the desk.
func RegisterWalkInBookingRoutes(h *handlers.WalkInBookingHandler) {
	http.HandleFunc("POST /api/v1/bookings/meal",
		withAuthAndRole(h.SubmitMeal, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))
	http.HandleFunc("POST /api/v1/bookings/event",
		withAuthAndRole(h.SubmitEvent, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))
	http.HandleFunc("POST /api/v1/bookings/accommodation",
		withAuthAndRole(h.SubmitAccommodation, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))
}
