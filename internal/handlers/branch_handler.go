package handlers

import (
	"encoding/base64"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/services"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type BranchHandler struct {
	service        *services.BranchService
	printerService *services.PrinterService
}

func NewBranchHandler(service *services.BranchService, printerService *services.PrinterService) *BranchHandler {
	return &BranchHandler{service: service, printerService: printerService}
}

// List handles GET /api/v1/branches
func (h *BranchHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branches, err := h.service.List(orgID)
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
	branch, err := h.service.Create(orgID, &req)
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
	branch, err := h.service.GetByID(id, orgID)
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
	// Branch-scoped staff (branch_admin, manager) may only update their own
	// branch. Org-level admins have no branch_id on their JWT and can update any
	// branch in their org.
	if callerBranch := middleware.GetBranchIDFromContext(r.Context()); callerBranch != nil && *callerBranch != id {
		utils.RespondError(w, http.StatusForbidden, "You can only update your own branch")
		return
	}
	var req models.UpdateBranchRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	branch, err := h.service.Update(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, branch)
}

// TestPrint handles POST /api/v1/branches/{id}/printer/test — sends a short
// ESC/POS test receipt to the branch's configured printer.
func (h *BranchHandler) TestPrint(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}
	if callerBranch := middleware.GetBranchIDFromContext(r.Context()); callerBranch != nil && *callerBranch != id {
		utils.RespondError(w, http.StatusForbidden, "You can only test your own branch's printer")
		return
	}
	if err := h.printerService.TestPrint(id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadGateway, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Test print sent"})
}

// TestPrintJob handles GET /api/v1/branches/{id}/printer/test-job — returns
// the printer's ip/port and a pre-rendered ESC/POS test receipt (base64) for
// a caller with local network access (the Electron terminal app) to send
// itself, rather than the server dialing a printer it may not be able to reach.
func (h *BranchHandler) TestPrintJob(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}
	if callerBranch := middleware.GetBranchIDFromContext(r.Context()); callerBranch != nil && *callerBranch != id {
		utils.RespondError(w, http.StatusForbidden, "You can only test your own branch's printer")
		return
	}
	job, err := h.printerService.BuildTestPrintJob(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ip":          job.IP,
		"port":        job.Port,
		"data_base64": base64.StdEncoding.EncodeToString(job.Data),
	})
}

// Delete handles DELETE /api/v1/branches/{id}
func (h *BranchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}
	if err := h.service.Delete(id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Branch deleted"})
}
