package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type CorProfileHandler struct {
	service interfaces.CorProfileInterface
}

func NewCorProfileHandler(service interfaces.CorProfileInterface) *CorProfileHandler {
	return &CorProfileHandler{service: service}
}

// ─── Companies ────────────────────────────────────────────────────────────────

// ListCompanies handles GET /api/v1/clients/companies
func (h *CorProfileHandler) ListCompanies(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	p := utils.ParsePagination(r)

	companies, total, err := h.service.ListCompanies(r.Context(), orgID, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     companies,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
	})
}

// GetCompany handles GET /api/v1/clients/companies/{id}
func (h *CorProfileHandler) GetCompany(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}

	company, err := h.service.GetCompany(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	branches, _ := h.service.ListBranches(r.Context(), id)
	profiles, _, _ := h.service.ListProfiles(r.Context(), id, 1, 100)

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"company":  company,
		"branches": branches,
		"profiles": profiles,
	})
}

// UpdateCompany handles PUT /api/v1/clients/companies/{id}
func (h *CorProfileHandler) UpdateCompany(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}

	var req models.UpdateCorCompanyRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	company, err := h.service.UpdateCompany(r.Context(), id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, company)
}

// ─── Branches ─────────────────────────────────────────────────────────────────

// ListBranches handles GET /api/v1/clients/companies/{id}/branches
func (h *CorProfileHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}

	branches, err := h.service.ListBranches(r.Context(), companyID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, branches)
}

// CreateBranch handles POST /api/v1/clients/companies/{id}/branches
func (h *CorProfileHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}

	var req models.CreateCorBranchRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	branch, err := h.service.CreateBranch(r.Context(), companyID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, branch)
}

// UpdateBranch handles PUT /api/v1/clients/companies/{id}/branches/{branch_id}
func (h *CorProfileHandler) UpdateBranch(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}
	branchID, err := uuid.Parse(r.PathValue("branch_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}

	var req models.UpdateCorBranchRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	branch, err := h.service.UpdateBranch(r.Context(), branchID, companyID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, branch)
}

// ─── Profiles ─────────────────────────────────────────────────────────────────

// ListProfiles handles GET /api/v1/clients/companies/{id}/profiles
func (h *CorProfileHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}
	p := utils.ParsePagination(r)

	profiles, total, err := h.service.ListProfiles(r.Context(), companyID, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     profiles,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
	})
}

// GetProfile handles GET /api/v1/clients/profiles/{id}
func (h *CorProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid profile ID")
		return
	}

	profile, err := h.service.GetProfile(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	guests, _ := h.service.ListGuests(r.Context(), id)

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"profile": profile,
		"guests":  guests,
	})
}

// CreateProfile handles POST /api/v1/clients/companies/{id}/profiles
func (h *CorProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	companyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid company ID")
		return
	}

	var req models.CreateCorProfileRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.CompanyID = companyID

	profile, err := h.service.CreateProfile(r.Context(), orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, profile)
}

// UpdateProfile handles PUT /api/v1/clients/profiles/{id}
func (h *CorProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid profile ID")
		return
	}

	var req models.UpdateCorProfileRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, profile)
}

// ─── Corporate Guests ─────────────────────────────────────────────────────────

// ListGuests handles GET /api/v1/clients/profiles/{id}/guests
func (h *CorProfileHandler) ListGuests(w http.ResponseWriter, r *http.Request) {
	profileID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid profile ID")
		return
	}

	guests, err := h.service.ListGuests(r.Context(), profileID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, guests)
}

// AddGuest handles POST /api/v1/clients/profiles/{id}/guests
func (h *CorProfileHandler) AddGuest(w http.ResponseWriter, r *http.Request) {
	profileID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid profile ID")
		return
	}

	var req models.CreateCorporateGuestRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	guest, err := h.service.AddGuest(r.Context(), profileID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, guest)
}

// UpdateGuest handles PUT /api/v1/clients/profiles/{id}/guests/{guest_id}
func (h *CorProfileHandler) UpdateGuest(w http.ResponseWriter, r *http.Request) {
	profileID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid profile ID")
		return
	}
	guestID, err := uuid.Parse(r.PathValue("guest_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid guest ID")
		return
	}

	var req models.UpdateCorporateGuestRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	guest, err := h.service.UpdateGuest(r.Context(), guestID, profileID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, guest)
}

// DeleteGuest handles DELETE /api/v1/clients/profiles/{id}/guests/{guest_id}
func (h *CorProfileHandler) DeleteGuest(w http.ResponseWriter, r *http.Request) {
	profileID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid profile ID")
		return
	}
	guestID, err := uuid.Parse(r.PathValue("guest_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid guest ID")
		return
	}

	if err := h.service.DeleteGuest(r.Context(), guestID, profileID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Guest removed successfully"})
}
