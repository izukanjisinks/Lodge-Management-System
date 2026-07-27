package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

func RegisterBranchRoutes(h *handlers.BranchHandler) {
	http.HandleFunc("GET /api/v1/branches",
		withAuthAndRole(h.List, models.RoleAdmin, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	http.HandleFunc("GET /api/v1/branches/{id}",
		withAuthAndRole(h.GetByID, models.RoleAdmin, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	http.HandleFunc("POST /api/v1/branches",
		withAuthAndRole(h.Create, models.RoleAdmin))

	http.HandleFunc("PUT /api/v1/branches/{id}",
		withAuthAndRole(h.Update, models.RoleAdmin, models.RoleBranchAdmin, models.RoleManager))

	http.HandleFunc("POST /api/v1/branches/{id}/printer/test",
		withAuthAndRole(h.TestPrint, models.RoleAdmin, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	http.HandleFunc("GET /api/v1/branches/{id}/printer/test-job",
		withAuthAndRole(h.TestPrintJob, models.RoleAdmin, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	http.HandleFunc("DELETE /api/v1/branches/{id}",
		withAuthAndRole(h.Delete, models.RoleAdmin))
}
