package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

func RegisterOrganizationSettingsRoutes(h *handlers.OrganizationSettingsHandler, emailTestHandler *handlers.EmailTestHandler) {
	http.HandleFunc("GET /api/v1/settings",
		withAuthAndRole(h.Get, models.RoleAdmin))

	http.HandleFunc("PUT /api/v1/settings",
		withAuthAndRole(h.Upsert, models.RoleAdmin, models.RoleBranchAdmin))

	http.HandleFunc("POST /api/v1/settings/test-email",
		withAuthAndRole(emailTestHandler.SendTest, models.RoleAdmin))
}
