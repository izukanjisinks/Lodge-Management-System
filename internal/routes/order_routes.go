package routes

import (
	"net/http"

	"lodge-system/internal/handlers"
	"lodge-system/internal/models"
)

func RegisterOrderRoutes(h *handlers.OrderHandler) {
	// Read — all authenticated staff
	http.HandleFunc("GET /api/v1/orders",
		withAuth(h.List))
	http.HandleFunc("GET /api/v1/orders/in-house-guests",
		withAuth(h.ListInHouseGuests))
	http.HandleFunc("GET /api/v1/orders/{id}",
		withAuth(h.GetByID))

	// Place orders — branch admin, manager, receptionist
	http.HandleFunc("POST /api/v1/orders",
		withAuthAndRole(h.PlaceOrder, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))
	http.HandleFunc("POST /api/v1/orders/walk-in",
		withAuthAndRole(h.PlaceWalkInOrder, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	// Kitchen status
	http.HandleFunc("PATCH /api/v1/orders/{id}/kitchen-status",
		withAuthAndRole(h.UpdateKitchenStatus, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	// Bar status
	http.HandleFunc("PATCH /api/v1/orders/{id}/bar-status",
		withAuthAndRole(h.UpdateBarStatus, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	// Add / remove items on an existing order
	http.HandleFunc("POST /api/v1/orders/{id}/items",
		withAuthAndRole(h.AddItems, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))
	http.HandleFunc("DELETE /api/v1/orders/{id}/items/{item_id}",
		withAuthAndRole(h.RemoveItem, models.RoleBranchAdmin, models.RoleManager, models.RoleReceptionist))

	// Manually close all open orders for the org — branch admin, manager
	http.HandleFunc("PATCH /api/v1/orders/close-all",
		withAuthAndRole(h.CloseAllOrders, models.RoleBranchAdmin, models.RoleManager))
}
