package handlers

import (
	"net/http"
	"time"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/services"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type OrderHandler struct {
	service *services.OrderService
}

func NewOrderHandler(service *services.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	pag := utils.ParsePagination(r)

	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	orderType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status") // defaults to "open" when blank

	var bookingID *uuid.UUID
	if v := r.URL.Query().Get("booking_id"); v != "" {
		parsed, err := uuid.Parse(v)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid booking_id")
			return
		}
		bookingID = &parsed
	}

	var from, to *time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid from date, expected YYYY-MM-DD")
			return
		}
		from = &t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "Invalid to date, expected YYYY-MM-DD")
			return
		}
		// Include the full day
		end := t.Add(24*time.Hour - time.Second)
		to = &end
	}

	orders, total, err := h.service.List(orgID, branchID, orderType, status, bookingID, from, to, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     orders,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

func (h *OrderHandler) ListInHouseGuests(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	guests, err := h.service.ListCheckedInGuests(orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if guests == nil {
		guests = []repository.InHouseGuest{}
	}
	utils.RespondJSON(w, http.StatusOK, guests)
}

func (h *OrderHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	order, err := h.service.GetByID(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.PlaceOrderRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	branchID, _ := middleware.ResolveBranchID(r)
	order, err := h.service.PlaceOrder(orgID, branchID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) PlaceWalkInOrder(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.PlaceWalkInOrderRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	branchID, _ := middleware.ResolveBranchID(r)
	order, err := h.service.PlaceWalkInOrder(orgID, branchID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) CloseAllOrders(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	n, err := h.service.CloseAllOrders(orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, map[string]any{"closed": n})
}

func (h *OrderHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("item_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.RemoveItem(itemID, id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OrderHandler) UpdateKitchenStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req struct {
		KitchenStatus string `json:"kitchen_status"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	order, err := h.service.UpdateKitchenStatus(id, orgID, req.KitchenStatus)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) AddItems(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.AddOrderItemsRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	order, err := h.service.AddItems(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, order)
}
