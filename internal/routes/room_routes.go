package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

func RegisterRoomRoutes(h *handlers.RoomHandler) {
	// Read — staff only (org-scoped via JWT)
	http.HandleFunc("GET /api/v1/rooms",
		withAuth(h.List))

	http.HandleFunc("GET /api/v1/rooms/available",
		withAuth(h.ListAvailable))

	http.HandleFunc("GET /api/v1/rooms/status",
		withAuth(h.Status))

	http.HandleFunc("GET /api/v1/rooms/{id}",
		withAuth(h.GetByID))

	// Write — branch admin and manager only (admin has no branch to scope creation to)
	http.HandleFunc("POST /api/v1/rooms",
		withAuthAndRole(h.Create, models.RoleBranchAdmin, models.RoleManager))

	http.HandleFunc("PUT /api/v1/rooms/{id}",
		withAuthAndRole(h.Update, models.RoleBranchAdmin, models.RoleManager))

	http.HandleFunc("PUT /api/v1/rooms/{id}/images",
		withAuthAndRole(h.UpdateImages, models.RoleBranchAdmin, models.RoleManager))

	http.HandleFunc("PATCH /api/v1/rooms/{id}/availability",
		withAuthAndRole(h.SetAvailability, models.RoleBranchAdmin, models.RoleManager))

	http.HandleFunc("DELETE /api/v1/rooms/{id}",
		withAuthAndRole(h.Delete, models.RoleBranchAdmin, models.RoleManager))
}
