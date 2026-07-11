package handlers

import (
	"encoding/json"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

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
