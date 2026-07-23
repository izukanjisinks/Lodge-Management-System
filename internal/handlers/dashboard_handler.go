package handlers

import (
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/services"
	"lodge-system/pkg/utils"
)

type DashboardHandler struct {
	service *services.DashboardService
}

func NewDashboardHandler(service *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Summary handles GET /api/v1/dashboard/summary — the 3 stat cards, loaded
// unconditionally regardless of which tab is selected.
func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := h.service.GetSummary(orgID, branchID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to load dashboard summary")
		return
	}
	utils.RespondJSON(w, http.StatusOK, summary)
}

// Bookings handles GET /api/v1/dashboard/bookings — the Bookings tab (default).
func (h *DashboardHandler) Bookings(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.service.GetBookings(orgID, branchID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to load bookings dashboard")
		return
	}
	utils.RespondJSON(w, http.StatusOK, data)
}

// Orders handles GET /api/v1/dashboard/orders — the Orders tab.
func (h *DashboardHandler) Orders(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.service.GetOrders(orgID, branchID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to load orders dashboard")
		return
	}
	utils.RespondJSON(w, http.StatusOK, data)
}

// Invoices handles GET /api/v1/dashboard/invoices — the Invoices tab.
func (h *DashboardHandler) Invoices(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.service.GetInvoices(orgID, branchID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to load invoices dashboard")
		return
	}
	utils.RespondJSON(w, http.StatusOK, data)
}
