package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type BranchHandler struct {
	service interfaces.BranchInterface
}

func NewBranchHandler(service interfaces.BranchInterface) *BranchHandler {
	return &BranchHandler{service: service}
}

// List handles GET /api/v1/branches
func (h *BranchHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branches, err := h.service.List(r.Context(), orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve branches")
		return
	}
	utils.RespondJSON(w, http.StatusOK, branches)
}

// Create handles POST /api/v1/branches
func (h *BranchHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	var req models.CreateBranchRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	branch, err := h.service.Create(r.Context(), orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, branch)
}

// GetByID handles GET /api/v1/branches/{id}
func (h *BranchHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}
	branch, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, branch)
}

// Update handles PUT /api/v1/branches/{id}
func (h *BranchHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}
	var req models.UpdateBranchRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	branch, err := h.service.Update(r.Context(), id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, branch)
}

// Delete handles DELETE /api/v1/branches/{id}
func (h *BranchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}
	if err := h.service.Delete(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Branch deleted"})
}
