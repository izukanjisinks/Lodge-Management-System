package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"
)

type OrganizationSettingsHandler struct {
	service interfaces.OrganizationSettingsInterface
}

func NewOrganizationSettingsHandler(service interfaces.OrganizationSettingsInterface) *OrganizationSettingsHandler {
	return &OrganizationSettingsHandler{service: service}
}

func (h *OrganizationSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	settings, err := h.service.Get(r.Context(), orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to fetch settings")
		return
	}
	utils.RespondJSON(w, http.StatusOK, settings)
}

func (h *OrganizationSettingsHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.UpdateOrganizationSettingsRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings, err := h.service.Upsert(r.Context(), orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}
	utils.RespondJSON(w, http.StatusOK, settings)
}
