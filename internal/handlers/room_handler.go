package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"
	"time"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type RoomHandler struct {
	service interfaces.RoomInterface
}

func NewRoomHandler(service interfaces.RoomInterface) *RoomHandler {
	return &RoomHandler{service: service}
}

func (h *RoomHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	pag := utils.ParsePagination(r)
	roomType := r.URL.Query().Get("type")

	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var isAvailable *bool
	if v := r.URL.Query().Get("is_available"); v != "" {
		b := v == "true"
		isAvailable = &b
	}

	rooms, total, err := h.service.List(r.Context(), orgID, branchID, roomType, isAvailable, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     rooms,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

// Status handles GET /api/v1/rooms/status — every room with whoever is
// currently checked into it, for the Room Status board. Replaces the
// frontend's old rooms + bookings(x2) + per-booking-detail fan-out with one call.
func (h *RoomHandler) Status(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	statuses, err := h.service.ListRoomStatus(r.Context(), orgID, branchID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, statuses)
}

// GuestList handles GET /api/v1/guest/rooms — public, org_id and branch_id are optional filters
func (h *RoomHandler) GuestList(w http.ResponseWriter, r *http.Request) {
	var orgID *uuid.UUID
	if orgIDStr := r.URL.Query().Get("org_id"); orgIDStr != "" {
		parsed, err := uuid.Parse(orgIDStr)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid org_id")
			return
		}
		orgID = &parsed
	}

	var branchID *uuid.UUID
	if v := r.URL.Query().Get("branch_id"); v != "" {
		parsed, err := uuid.Parse(v)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid branch_id")
			return
		}
		branchID = &parsed
	}

	roomType := r.URL.Query().Get("type")
	checkInStr := r.URL.Query().Get("check_in")
	checkOutStr := r.URL.Query().Get("check_out")

	if checkInStr != "" && checkOutStr != "" {
		if orgID == nil {
			utils.RespondError(w, http.StatusBadRequest, "org_id is required when filtering by date range")
			return
		}
		checkIn, err := time.Parse("2006-01-02", checkInStr)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "invalid check_in date, expected YYYY-MM-DD")
			return
		}
		checkOut, err := time.Parse("2006-01-02", checkOutStr)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "invalid check_out date, expected YYYY-MM-DD")
			return
		}
		rooms, err := h.service.ListAvailable(r.Context(), *orgID, branchID, checkIn, checkOut, roomType)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
			Data:     rooms,
			Page:     1,
			PageSize: len(rooms),
			Total:    len(rooms),
		})
		return
	}

	pag := utils.ParsePagination(r)
	orgName := r.URL.Query().Get("org_name")

	var isAvailable *bool
	if v := r.URL.Query().Get("is_available"); v != "" {
		b := v == "true"
		isAvailable = &b
	}

	rooms, total, err := h.service.GuestList(r.Context(), orgID, branchID, roomType, orgName, isAvailable, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     rooms,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

// GuestGetByID handles GET /api/v1/guest/rooms/{id} — public, no org required (unscoped lookup)
func (h *RoomHandler) GuestGetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}

	room, err := h.service.GetByIDUnscoped(r.Context(), id)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Room not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, room)
}

// GuestListAvailable handles GET /api/v1/guest/rooms/available — public, org_id from query param.
func (h *RoomHandler) GuestListAvailable(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("org_id")
	if orgIDStr == "" {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid org_id")
		return
	}

	var branchID *uuid.UUID
	if v := r.URL.Query().Get("branch_id"); v != "" {
		parsed, err := uuid.Parse(v)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "invalid branch_id")
			return
		}
		branchID = &parsed
	}

	checkInStr := r.URL.Query().Get("check_in")
	checkOutStr := r.URL.Query().Get("check_out")
	roomType := r.URL.Query().Get("type")

	if checkInStr == "" || checkOutStr == "" {
		utils.RespondError(w, http.StatusBadRequest, "check_in and check_out are required (YYYY-MM-DD)")
		return
	}

	checkIn, err := time.Parse("2006-01-02", checkInStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid check_in date, expected YYYY-MM-DD")
		return
	}
	checkOut, err := time.Parse("2006-01-02", checkOutStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid check_out date, expected YYYY-MM-DD")
		return
	}

	rooms, err := h.service.ListAvailable(r.Context(), orgID, branchID, checkIn, checkOut, roomType)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, rooms)
}

func (h *RoomHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	checkInStr := r.URL.Query().Get("check_in")
	checkOutStr := r.URL.Query().Get("check_out")
	roomType := r.URL.Query().Get("type")

	if checkInStr == "" || checkOutStr == "" {
		utils.RespondError(w, http.StatusBadRequest, "check_in and check_out query params are required (YYYY-MM-DD)")
		return
	}

	checkIn, err := time.Parse("2006-01-02", checkInStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid check_in date, expected YYYY-MM-DD")
		return
	}
	checkOut, err := time.Parse("2006-01-02", checkOutStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid check_out date, expected YYYY-MM-DD")
		return
	}

	rooms, err := h.service.ListAvailable(r.Context(), orgID, branchID, checkIn, checkOut, roomType)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, rooms)
}

func (h *RoomHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	room, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Room not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	var room models.Room
	if err := utils.DecodeJson(r, &room); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Branch-scoped staff have their branch fixed by the JWT; org-level admins
	// may optionally supply branch_id in the body.
	if branchID := middleware.GetBranchIDFromContext(r.Context()); branchID != nil {
		room.BranchID = branchID
	}

	if err := h.service.Create(r.Context(), &room, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, room)
}

func (h *RoomHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var updates models.Room
	if err := utils.DecodeJson(r, &updates); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	room, err := h.service.Update(r.Context(), id, orgID, &updates)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) UpdateImages(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req struct {
		Images []string `json:"images"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	room, err := h.service.UpdateImages(r.Context(), id, orgID, req.Images)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req struct {
		IsAvailable bool `json:"is_available"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.SetAvailability(r.Context(), id, orgID, req.IsAvailable); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"id":           id,
		"is_available": req.IsAvailable,
	})
}

func (h *RoomHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.Delete(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Room deleted successfully"})
}
