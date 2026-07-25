package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type CorporateBookingRequestHandler struct {
	service interfaces.CorporateBookingRequestInterface
}

func NewCorporateBookingRequestHandler(service interfaces.CorporateBookingRequestInterface) *CorporateBookingRequestHandler {
	return &CorporateBookingRequestHandler{service: service}
}

// ─── Guest submission ─────────────────────────────────────────────────────────

// SubmitAccommodation handles POST /api/v1/guest/bookings/corporate
// The org_id is expected in the request body (from frontend), not as a query param.
func (h *CorporateBookingRequestHandler) SubmitAccommodation(w http.ResponseWriter, r *http.Request) {
	webUserID, _ := middleware.GetUserIDFromContext(r.Context())

	var req models.SubmitAccommodationRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate org_id from body
	if req.OrgID == uuid.Nil {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}

	result, err := h.service.SubmitAccommodation(r.Context(), req.OrgID, webUserID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, result)
}

// SubmitEvent handles POST /api/v1/guest/bookings/corporate-event.
// Accepts the standalone event envelope (Flow B) for corporate bookers.
func (h *CorporateBookingRequestHandler) SubmitEvent(w http.ResponseWriter, r *http.Request) {
	webUserID, _ := middleware.GetUserIDFromContext(r.Context())

	var req models.SubmitEventBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.OrgID == uuid.Nil {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}

	result, err := h.service.SubmitEventBooking(r.Context(), req.OrgID, webUserID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, result)
}

// SubmitMeal handles POST /api/v1/guest/bookings/corporate-meal.
// Accepts the standalone meal envelope (Flow B) for corporate bookers.
func (h *CorporateBookingRequestHandler) SubmitMeal(w http.ResponseWriter, r *http.Request) {
	webUserID, _ := middleware.GetUserIDFromContext(r.Context())

	var req models.SubmitMealBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.OrgID == uuid.Nil {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}

	result, err := h.service.SubmitMealBooking(r.Context(), req.OrgID, webUserID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, result)
}

// ─── Backoffice ───────────────────────────────────────────────────────────────

// List handles GET /api/v1/bookings/requests
func (h *CorporateBookingRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	p := utils.ParsePagination(r)
	bookingType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	requests, total, err := h.service.List(r.Context(), orgID, bookingType, status, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     requests,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
	})
}

// GetByID handles GET /api/v1/bookings/requests/{id}
func (h *CorporateBookingRequestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	req, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, req)
}

// Approve handles PUT /api/v1/bookings/requests/{id}/approve
func (h *CorporateBookingRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	if err := h.service.Approve(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Request approved"})
}

// Reject handles PUT /api/v1/bookings/requests/{id}/reject
func (h *CorporateBookingRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	if err := h.service.Reject(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Request rejected"})
}

// Cancel handles PUT /api/v1/bookings/requests/{id}/cancel
func (h *CorporateBookingRequestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	if err := h.service.Cancel(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Request cancelled"})
}
