package handlers

import (
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

// ─── Collect ──────────────────────────────────────────────────────────────────

func (h *MealCollectionHandler) Collect(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	staffID, _ := middleware.GetUserIDFromContext(r.Context())
	staffName, _ := middleware.GetEmailFromContext(r.Context())
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	var req models.MealCollectRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.Collect(orgID, sessionID, staffID, staffName, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, result)
}

// ─── Current stay ─────────────────────────────────────────────────────────────

func (h *MealCollectionHandler) GetCurrentStay(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid room id")
		return
	}
	stay, err := h.service.GetCurrentStay(orgID, roomID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, stay)
}
