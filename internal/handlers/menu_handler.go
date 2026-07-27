package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type MenuHandler struct {
	service interfaces.MenuInterface
}

func NewMenuHandler(service interfaces.MenuInterface) *MenuHandler {
	return &MenuHandler{service: service}
}

// ── Menu ──────────────────────────────────────────────────────────────────────

func (h *MenuHandler) GetMenu(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	p := utils.ParsePagination(r)
	category := r.URL.Query().Get("category")

	menu, err := h.service.GetMenu(r.Context(), orgID, branchID, category, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, menu)
}

func (h *MenuHandler) UpsertMenu(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	p := utils.ParsePagination(r)
	category := r.URL.Query().Get("category")

	var req models.UpdateMenuRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	menu, err := h.service.UpsertMenu(r.Context(), orgID, branchID, &req, category, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, menu)
}

// ── Menu Items ────────────────────────────────────────────────────────────────

func (h *MenuHandler) CreateMenuItem(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req models.CreateMenuItemRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	item, err := h.service.CreateMenuItem(r.Context(), orgID, branchID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, item)
}

func (h *MenuHandler) UpdateMenuItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := uuid.Parse(r.PathValue("item_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.UpdateMenuItemRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	item, err := h.service.UpdateMenuItem(r.Context(), itemID, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, item)
}

func (h *MenuHandler) DeleteMenuItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := uuid.Parse(r.PathValue("item_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.DeleteMenuItem(r.Context(), itemID, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Menu item deleted successfully"})
}

// ── Guest (public) ────────────────────────────────────────────────────────────

func (h *MenuHandler) GuestGetMenu(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("org_id")
	if orgIDStr == "" {
		utils.RespondError(w, http.StatusBadRequest, "org_id is required")
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid org_id")
		return
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

	p := utils.ParsePagination(r)
	category := r.URL.Query().Get("category")

	menu, err := h.service.GuestGetMenu(r.Context(), orgID, branchID, category, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Menu not found")
		return
	}
	utils.RespondJSON(w, http.StatusOK, menu)
}
