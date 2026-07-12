package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

func RegisterMealCollectionRoutes(h *handlers.MealCollectionHandler) {
	staff := []string{models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist}

	// ── Sessions ──────────────────────────────────────────────────────────────
	http.HandleFunc("GET /api/v1/meal-sessions",
		withAuth(h.ListSessions))
	http.HandleFunc("GET /api/v1/meal-sessions/{id}",
		withAuth(h.GetSession))
	http.HandleFunc("POST /api/v1/meal-sessions",
		withAuthAndRole(h.CreateSession, staff...))
	http.HandleFunc("PUT /api/v1/meal-sessions/{id}",
		withAuthAndRole(h.UpdateSession, staff...))
	http.HandleFunc("PUT /api/v1/meal-sessions/{id}/status",
		withAuthAndRole(h.UpdateSessionStatus, staff...))
	http.HandleFunc("PUT /api/v1/meal-sessions/{id}/grace-period",
		withAuthAndRole(h.UpdateGracePeriod, staff...))
	// Delete is a higher bar — branch admin / manager only.
	http.HandleFunc("DELETE /api/v1/meal-sessions/{id}",
		withAuthAndRole(h.DeleteSession, models.RoleBranchAdmin, models.RoleManager))
	http.HandleFunc("GET /api/v1/meal-sessions/{id}/collections",
		withAuth(h.ListCollections))

	// ── Collect ───────────────────────────────────────────────────────────────
	// Serving a meal is front-line work — waiters do this in addition to desk staff.
	http.HandleFunc("POST /api/v1/meal-sessions/{id}/collect",
		withAuthAndRole(h.Collect, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist, models.RoleWaiter))

	// ── Cards ─────────────────────────────────────────────────────────────────
	http.HandleFunc("GET /api/v1/meal-cards",
		withAuth(h.ListCards))
	http.HandleFunc("POST /api/v1/meal-cards",
		withAuthAndRole(h.AssignCard, staff...))
	http.HandleFunc("PATCH /api/v1/meal-cards/{id}",
		withAuthAndRole(h.UpdateCard, staff...))
	http.HandleFunc("PUT /api/v1/meal-cards/{id}/replace",
		withAuthAndRole(h.ReplaceCard, staff...))
	http.HandleFunc("PUT /api/v1/meal-cards/{id}/void",
		withAuthAndRole(h.VoidCard, staff...))

	// ── Current stay (card dialog occupant picker) ──────────────────────────────
	http.HandleFunc("GET /api/v1/rooms/{id}/current-stay",
		withAuth(h.GetCurrentStay))
}
