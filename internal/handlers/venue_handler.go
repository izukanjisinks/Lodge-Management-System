package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type VenueHandler struct {
	service interfaces.VenueInterface
}

func NewVenueHandler(service interfaces.VenueInterface) *VenueHandler {
	return &VenueHandler{service: service}
}

func (h *VenueHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	pag := utils.ParsePagination(r)
	venueType := r.URL.Query().Get("venue_type")

	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var isAvailable *bool
	if v := r.URL.Query().Get("is_available"); v != "" {
		b := v == "true"
		isAvailable = &b
	}

	// Optional [from, to] window: excludes venues already reserved in that range.
	from, to := parseDateRange(r)

	venues, total, err := h.service.List(r.Context(), orgID, branchID, venueType, isAvailable, from, to, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     venues,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

// GuestList handles GET /api/v1/guest/venues — public, org_id required as a query param.
func (h *VenueHandler) GuestList(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("org_id")
	if orgIDStr == "" {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid org_id")
		return
	}

	var branchID *uuid.UUID
	if v := r.URL.Query().Get("branch_id"); v != "" {
		parsed, err := uuid.Parse(v)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "invalid branch_id")
			return
		}
		branchID = &parsed
	}

	venueType := r.URL.Query().Get("venue_type")

	// Optional [from, to] window: excludes venues already reserved in that range.
	from, to := parseDateRange(r)

	venues, err := h.service.GuestList(r.Context(), orgID, branchID, venueType, from, to)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, venues)
}

// GuestGetByID handles the public GET /api/v1/guest/venues/{id}. Venue IDs are
// globally unique, so the lookup is unscoped (no auth, no org_id) — mirroring the
// public room-detail endpoint.
func (h *VenueHandler) GuestGetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid venue ID")
		return
	}

	venue, err := h.service.GetByIDUnscoped(r.Context(), id)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Venue not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, venue)
}

func (h *VenueHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid venue ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	venue, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Venue not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, venue)
}

func (h *VenueHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	var venue models.Venue
	if err := utils.DecodeJson(r, &venue); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Branch-scoped staff have their branch fixed by the JWT; org-level admins
	// may optionally supply branch_id in the body.
	if branchID := middleware.GetBranchIDFromContext(r.Context()); branchID != nil {
		venue.BranchID = branchID
	}

	if err := h.service.Create(r.Context(), &venue, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, venue)
}

func (h *VenueHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid venue ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var updates models.Venue
	if err := utils.DecodeJson(r, &updates); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	venue, err := h.service.Update(r.Context(), id, orgID, &updates)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, venue)
}

func (h *VenueHandler) UpdateImages(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid venue ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req struct {
		Images []string `json:"images"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	venue, err := h.service.UpdateImages(r.Context(), id, orgID, req.Images)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, venue)
}

func (h *VenueHandler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid venue ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req struct {
		IsAvailable bool `json:"is_available"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.SetAvailability(r.Context(), id, orgID, req.IsAvailable); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"id":           id,
		"is_available": req.IsAvailable,
	})
}

func (h *VenueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid venue ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.Delete(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Venue deleted successfully"})
}
