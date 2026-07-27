package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type IndividualBookingRequestHandler struct {
	service interfaces.IndividualBookingRequestInterface
}

func NewIndividualBookingRequestHandler(service interfaces.IndividualBookingRequestInterface) *IndividualBookingRequestHandler {
	return &IndividualBookingRequestHandler{service: service}
}

// ─── Web user / guest submission ──────────────────────────────────────────────

// SubmitAccommodation handles POST /api/v1/guest/bookings/accommodation.
// Accepts the unified envelope from the frontend (booking_context=individual).
// Auth is web_users (withWebUserAuth) — URL path says "guest" to match the frontend.
func (h *IndividualBookingRequestHandler) SubmitAccommodation(w http.ResponseWriter, r *http.Request) {
	guestID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.SubmitIndividualBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OrgID == uuid.Nil {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}

	result, err := h.service.Submit(r.Context(), guestID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, result)
}

// SubmitEvent handles POST /api/v1/guest/bookings/event.
// Accepts the standalone event envelope (Flow B, booking_context=individual).
func (h *IndividualBookingRequestHandler) SubmitEvent(w http.ResponseWriter, r *http.Request) {
	guestID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.SubmitEventBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OrgID == uuid.Nil {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}

	result, err := h.service.SubmitEvent(r.Context(), guestID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, result)
}

// SubmitMeal handles POST /api/v1/guest/bookings/meal.
// Accepts the standalone meal envelope (Flow B, booking_context=individual).
func (h *IndividualBookingRequestHandler) SubmitMeal(w http.ResponseWriter, r *http.Request) {
	guestID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.SubmitMealBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OrgID == uuid.Nil {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}

	result, err := h.service.SubmitMeal(r.Context(), guestID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, result)
}

// ─── Web user submission ──────────────────────────────────────────────────────

// Submit handles POST /api/v1/web/bookings
func (h *IndividualBookingRequestHandler) Submit(w http.ResponseWriter, r *http.Request) {
	webUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.SubmitIndividualBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.service.Submit(r.Context(), webUserID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, result)
}

// ListForWebUser handles GET /api/v1/web/bookings
func (h *IndividualBookingRequestHandler) ListForWebUser(w http.ResponseWriter, r *http.Request) {
	webUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	p := utils.ParsePagination(r)
	requests, total, err := h.service.ListForWebUser(r.Context(), webUserID, p.Page, p.PageSize)
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

// GetForWebUser handles GET /api/v1/web/bookings/{id}
func (h *IndividualBookingRequestHandler) GetForWebUser(w http.ResponseWriter, r *http.Request) {
	webUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	req, err := h.service.GetForWebUser(r.Context(), id, webUserID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, req)
}

// CancelForWebUser handles PATCH /api/v1/web/bookings/{id}/cancel
func (h *IndividualBookingRequestHandler) CancelForWebUser(w http.ResponseWriter, r *http.Request) {
	webUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	if err := h.service.CancelForWebUser(r.Context(), id, webUserID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Request cancelled"})
}

// ─── Backoffice ───────────────────────────────────────────────────────────────

// List handles GET /api/v1/booking-requests/individual
func (h *IndividualBookingRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	p := utils.ParsePagination(r)
	status := r.URL.Query().Get("status")

	requests, total, err := h.service.List(r.Context(), orgID, status, p.Page, p.PageSize)
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

// GetByID handles GET /api/v1/booking-requests/individual/{id}
func (h *IndividualBookingRequestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

// Approve handles PUT /api/v1/booking-requests/individual/{id}/approve
func (h *IndividualBookingRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	booking, err := h.service.Approve(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, booking)
}

// Reject handles PUT /api/v1/booking-requests/individual/{id}/reject
func (h *IndividualBookingRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
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

// Cancel handles PUT /api/v1/booking-requests/individual/{id}/cancel
func (h *IndividualBookingRequestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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
