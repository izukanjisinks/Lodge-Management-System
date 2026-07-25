package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"lodge-system/internal/interfaces"
	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

// WalkInBookingHandler serves staff-authed, materialise-immediately booking
// creation for meals and events. Unlike the guest (website) endpoints, these skip
// the pending → approval workflow: a walk-in guest is booking in person, so the
// booking is confirmed and its children (orders / event sessions) are created on
// the spot. It accepts the same envelopes the website uses, branching on
// booking_context (individual | corporate).
type WalkInBookingHandler struct {
	individual interfaces.IndividualBookingRequestInterface
	corporate  interfaces.CorporateBookingRequestInterface
}

func NewWalkInBookingHandler(
	individual interfaces.IndividualBookingRequestInterface,
	corporate interfaces.CorporateBookingRequestInterface,
) *WalkInBookingHandler {
	return &WalkInBookingHandler{individual: individual, corporate: corporate}
}

// SubmitMeal handles POST /api/v1/bookings/meal (staff auth).
func (h *WalkInBookingHandler) SubmitMeal(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req models.SubmitMealBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	// Staff context is the source of truth for org/branch — never trust the body.
	req.OrgID = orgID
	if branchID != nil {
		req.BranchID = branchID
	}

	booking, err := h.createMeal(r.Context(), orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, booking)
}

// SubmitEvent handles POST /api/v1/bookings/event (staff auth).
func (h *WalkInBookingHandler) SubmitEvent(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req models.SubmitEventBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.OrgID = orgID
	if branchID != nil {
		req.BranchID = branchID
	}

	booking, err := h.createEvent(r.Context(), orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, booking)
}

// walkInAccommodationBody is the SubmitAccommodationRequest envelope plus the
// per-attendant room assignments needed to materialise immediately.
type walkInAccommodationBody struct {
	models.SubmitAccommodationRequest
	Assignments []models.MaterialiseGuestAssignment `json:"assignments"`
}

// SubmitAccommodation handles POST /api/v1/bookings/accommodation (staff auth).
// Both individual and corporate: multiple named guests, each with their own room.
// The body is read once and decoded per context (the envelopes differ).
func (h *WalkInBookingHandler) SubmitAccommodation(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var probe struct {
		BookingContext string `json:"booking_context"`
	}
	if jErr := json.Unmarshal(body, &probe); jErr != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var booking *models.Booking
	if probe.BookingContext == "individual" {
		var req models.SubmitIndividualBookingRequest
		if jErr := json.Unmarshal(body, &req); jErr != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		booking, err = h.individual.WalkInAccommodation(r.Context(), orgID, branchID, &req)
	} else {
		var cBody walkInAccommodationBody
		if jErr := json.Unmarshal(body, &cBody); jErr != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		cBody.OrgID = orgID
		if branchID != nil {
			cBody.BranchID = branchID
		}
		matReq := &models.MaterialiseRequest{Assignments: cBody.Assignments}
		booking, err = h.corporate.WalkInAccommodation(r.Context(), orgID, branchID, &cBody.SubmitAccommodationRequest, matReq)
	}
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, booking)
}

func (h *WalkInBookingHandler) createMeal(ctx context.Context, orgID uuid.UUID, req *models.SubmitMealBookingRequest) (*models.Booking, error) {
	if req.BookingContext == "corporate" {
		return h.corporate.WalkInMeal(ctx, orgID, req)
	}
	return h.individual.WalkInMeal(ctx, orgID, req)
}

func (h *WalkInBookingHandler) createEvent(ctx context.Context, orgID uuid.UUID, req *models.SubmitEventBookingRequest) (*models.Booking, error) {
	if req.BookingContext == "corporate" {
		return h.corporate.WalkInEvent(ctx, orgID, req)
	}
	return h.individual.WalkInEvent(ctx, orgID, req)
}
