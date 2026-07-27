package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

// MealCollectionHandler serves resident meal collection: recurring buffet
// sessions, RFID card assignments, and the collect flow. Methods are split
// across meal_session_handler.go, meal_card_handler.go, and
// meal_collect_handler.go, grouped the same way as the underlying service.
type MealCollectionHandler struct {
	service interfaces.MealCollectionInterface
}

func NewMealCollectionHandler(service interfaces.MealCollectionInterface) *MealCollectionHandler {
	return &MealCollectionHandler{service: service}
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

func (h *MealCollectionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	p := utils.ParsePagination(r)
	result, err := h.service.ListSessions(r.Context(), orgID, branchID,
		r.URL.Query().Get("status"), r.URL.Query().Get("meal_period"), p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, result)
}

func (h *MealCollectionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	sess, err := h.service.GetSession(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, sess)
}

func (h *MealCollectionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req models.MealSessionCreateRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := h.service.CreateSession(r.Context(), orgID, branchID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, sess)
}

func (h *MealCollectionHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	var req models.MealSessionUpdateRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := h.service.UpdateSession(r.Context(), id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, sess)
}

func (h *MealCollectionHandler) UpdateSessionStatus(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	var req models.MealSessionStatusRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := h.service.UpdateSessionStatus(r.Context(), id, orgID, req.Status)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, sess)
}

func (h *MealCollectionHandler) UpdateGracePeriod(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	var req models.UpdateGracePeriodRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := h.service.UpdateGracePeriod(r.Context(), id, orgID, req.GracePeriodMinutes)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, sess)
}

func (h *MealCollectionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if err := h.service.DeleteSession(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "session deleted"})
}

func (h *MealCollectionHandler) ListCollections(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	logs, err := h.service.ListCollections(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, logs)
}
