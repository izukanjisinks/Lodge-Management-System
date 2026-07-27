package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type GuestAuthHandler struct {
	guestAuthService interfaces.GuestAuthInterface
	orgRepo          *repository.OrganizationRepository
	branchRepo       *repository.BranchRepository
}

func NewGuestAuthHandler(guestAuthService interfaces.GuestAuthInterface, orgRepo *repository.OrganizationRepository, branchRepo *repository.BranchRepository) *GuestAuthHandler {
	return &GuestAuthHandler{guestAuthService: guestAuthService, orgRepo: orgRepo, branchRepo: branchRepo}
}

// Register handles POST /api/v1/guest/auth/register
func (h *GuestAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.GuestRegisterRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	guest, err := h.guestAuthService.Register(r.Context(), &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := utils.GenerateGuestToken(guest.Email, guest.ID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	utils.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"guest": guest,
		"token": token,
	})
}

// Login handles POST /api/v1/guest/auth/login
func (h *GuestAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	guest, token, err := h.guestAuthService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		utils.RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"guest": guest,
		"token": token,
	})
}

// Me handles GET /api/v1/guest/me
func (h *GuestAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	guestID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	guest, err := h.guestAuthService.GetByID(r.Context(), guestID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Guest not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, guest)
}

// UpdateProfile handles PUT /api/v1/guest/me
func (h *GuestAuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	guestID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.GuestUpdateRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	guest, err := h.guestAuthService.UpdateProfile(r.Context(), guestID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, guest)
}

// GetLodge handles GET /api/v1/guest/lodges/{org_id} — public, no auth required.
func (h *GuestAuthHandler) GetLodge(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("org_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid org_id")
		return
	}
	lodge, err := h.orgRepo.GetByID(id)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Lodge not found")
		return
	}
	branches, _ := h.branchRepo.List(id)
	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"lodge":    lodge,
		"branches": branches,
	})
}

// ListLodges handles GET /api/v1/guest/lodges — public, no auth required.
func (h *GuestAuthHandler) ListLodges(w http.ResponseWriter, r *http.Request) {
	pg := utils.ParsePagination(r)
	lodges, total, err := h.orgRepo.ListPublic(pg.Page, pg.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve lodges")
		return
	}

	type lodgeWithBranches struct {
		models.Organization
		Branches []models.Branch `json:"branches"`
	}
	data := make([]lodgeWithBranches, len(lodges))
	for i, org := range lodges {
		branches, _ := h.branchRepo.List(org.ID)
		data[i] = lodgeWithBranches{Organization: org, Branches: branches}
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     data,
		Page:     pg.Page,
		PageSize: pg.PageSize,
		Total:    total,
	})
}

// ResetPassword handles POST /api/v1/guest/auth/reset-password — public, no auth required.
func (h *GuestAuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := utils.DecodeJson(r, &req); err != nil || req.Email == "" {
		utils.RespondError(w, http.StatusBadRequest, "email is required")
		return
	}

	if err := h.guestAuthService.ResetPassword(r.Context(), req.Email); err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to process reset request")
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "If an account exists for that email, a new password has been sent",
	})
}
