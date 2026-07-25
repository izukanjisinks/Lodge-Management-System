package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type OrganizationHandler struct {
	service interfaces.BackofficeOrganizationInterface
}

func NewOrganizationHandler(service interfaces.BackofficeOrganizationInterface) *OrganizationHandler {
	return &OrganizationHandler{service: service}
}

func (h *OrganizationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

func (h *OrganizationHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	org, err := h.service.GetByID(r.Context(), orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Organization not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, org)
}

func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.OrgDetails
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	org, err := h.service.Update(r.Context(), orgID, req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, org)
}
