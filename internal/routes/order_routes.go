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

	// Place orders — branch admin, manager, receptionist, waiter
	http.HandleFunc("POST /api/v1/orders",
		withAuthAndRole(h.PlaceOrder, models.RoleBranchAdmin, models.RoleManager, models.RoleWaiter))
	http.HandleFunc("POST /api/v1/orders/walk-in",
		withAuthAndRole(h.PlaceWalkInOrder, models.RoleBranchAdmin, models.RoleManager, models.RoleWaiter))

	// Kitchen status — kitchen staff own this
	http.HandleFunc("PATCH /api/v1/orders/{id}/kitchen-status",
		withAuthAndRole(h.UpdateKitchenStatus, models.RoleBranchAdmin, models.RoleManager, models.RoleKitchenStaff))

	// Bar status — bar staff own this
	http.HandleFunc("PATCH /api/v1/orders/{id}/bar-status",
		withAuthAndRole(h.UpdateBarStatus, models.RoleBranchAdmin, models.RoleManager, models.RoleBarStaff))

	// Add / remove items on an existing order — branch admin, manager, receptionist, waiter
	http.HandleFunc("POST /api/v1/orders/{id}/items",
		withAuthAndRole(h.AddItems, models.RoleBranchAdmin, models.RoleManager, models.RoleWaiter))
	http.HandleFunc("DELETE /api/v1/orders/{id}/items/{item_id}",
		withAuthAndRole(h.RemoveItem, models.RoleBranchAdmin, models.RoleManager, models.RoleWaiter))

	// Manually close all open orders for the org — branch admin, manager
	http.HandleFunc("PATCH /api/v1/orders/close-all",
		withAuthAndRole(h.CloseAllOrders, models.RoleBranchAdmin, models.RoleManager))
}
