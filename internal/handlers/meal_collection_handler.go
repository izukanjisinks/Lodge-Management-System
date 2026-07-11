package handlers

import (
	"encoding/json"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/services"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type MealCollectionHandler struct {
	service *services.MealCollectionService
}

func NewMealCollectionHandler(service *services.MealCollectionService) *MealCollectionHandler {
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
	result, err := h.service.ListSessions(orgID, branchID,
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
	sess, err := h.service.GetSession(id, orgID)
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
	sess, err := h.service.CreateSession(orgID, branchID, &req)
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
	sess, err := h.service.UpdateSession(id, orgID, &req)
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
	sess, err := h.service.UpdateSessionStatus(id, orgID, req.Status)
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
	sess, err := h.service.UpdateGracePeriod(id, orgID, req.GracePeriodMinutes)
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
	if err := h.service.DeleteSession(id, orgID); err != nil {
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
	logs, err := h.service.ListCollections(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, logs)
}

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

// ─── Cards ────────────────────────────────────────────────────────────────────

func (h *MealCollectionHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	var roomID *uuid.UUID
	if v := r.URL.Query().Get("room_id"); v != "" {
		if parsed, err := uuid.Parse(v); err == nil {
			roomID = &parsed
		}
	}
	cards, err := h.service.ListCards(orgID, roomID, r.URL.Query().Get("status"))
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, cards)
}

func (h *MealCollectionHandler) AssignCard(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req models.AssignCardRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	card, err := h.service.AssignCard(orgID, branchID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, card)
}

func (h *MealCollectionHandler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	// Decode into a raw map first so we can distinguish attendee_id: null (clear)
	// from an omitted attendee_id (leave as-is).
	var raw map[string]json.RawMessage
	if err := utils.DecodeJson(r, &raw); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var req models.UpdateCardRequest
	if v, ok := raw["room_id"]; ok {
		var id uuid.UUID
		if json.Unmarshal(v, &id) == nil {
			req.RoomID = &id
		}
	}
	if v, ok := raw["role"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			req.Role = &s
		}
	}
	if v, ok := raw["status"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			req.Status = &s
		}
	}
	if v, ok := raw["attendee_id"]; ok {
		if string(v) == "null" {
			req.ClearAttendee = true
		} else {
			var id uuid.UUID
			if json.Unmarshal(v, &id) == nil {
				req.AttendeeID = &id
			}
		}
	}
	card, err := h.service.UpdateCard(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, card)
}

func (h *MealCollectionHandler) ReplaceCard(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	var req models.ReplaceCardRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	card, err := h.service.ReplaceCard(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, card)
}

func (h *MealCollectionHandler) VoidCard(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	card, err := h.service.VoidCard(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, card)
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
