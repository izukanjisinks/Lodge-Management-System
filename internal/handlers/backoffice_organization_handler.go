package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type BackofficeOrganizationHandler struct {
	service interfaces.BackofficeOrganizationInterface
}

func NewBackofficeOrganizationHandler(service interfaces.BackofficeOrganizationInterface) *BackofficeOrganizationHandler {
	return &BackofficeOrganizationHandler{service: service}
}

func (h *BackofficeOrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.service.List(r.Context())
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve organizations")
		return
	}
	pag := utils.ParsePagination(r)
	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     orgs,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    len(orgs),
	})
}

func (h *BackofficeOrganizationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	org, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Organization not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, org)
}

func (h *BackofficeOrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	var req models.OrgDetails
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	org, err := h.service.Update(r.Context(), id, req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, org)
}

func (h *BackofficeOrganizationHandler) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	org, err := h.service.ToggleStatus(r.Context(), id, req.IsActive)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, org)
}

func (h *BackofficeOrganizationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Organization deleted successfully"})
}

func (h *BackofficeOrganizationHandler) Provision(w http.ResponseWriter, r *http.Request) {
	var req models.ProvisionOrgRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	org, admin, err := h.service.Provision(r.Context(), req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"organization": org,
		"admin": map[string]interface{}{
			"id":        admin.UserID,
			"full_name": admin.FullName,
			"email":     admin.Email,
		},
	})
}
