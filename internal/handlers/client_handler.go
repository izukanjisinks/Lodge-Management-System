package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type ClientHandler struct {
	service interfaces.ClientInterface
}

func NewClientHandler(service interfaces.ClientInterface) *ClientHandler {
	return &ClientHandler{service: service}
}

// ─── Individual ───────────────────────────────────────────────────────────────

func (h *ClientHandler) ListIndividual(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	pag := utils.ParsePagination(r)
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	clients, total, err := h.service.ListIndividual(r.Context(), orgID, search, status, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     clients,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

func (h *ClientHandler) GetIndividualByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	client, err := h.service.GetIndividualByID(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Individual client not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, client)
}

func (h *ClientHandler) CreateIndividual(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	var c models.IndividualClient
	if err := utils.DecodeJson(r, &c); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.CreateIndividual(r.Context(), orgID, &c); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, c)
}

func (h *ClientHandler) UpdateIndividual(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var updates models.IndividualClient
	if err := utils.DecodeJson(r, &updates); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	client, err := h.service.UpdateIndividual(r.Context(), id, orgID, &updates)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, client)
}

func (h *ClientHandler) DeleteIndividual(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.DeleteIndividual(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Individual client deleted successfully"})
}

func (h *ClientHandler) LookupIndividualByIDNumber(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	idNumber := r.URL.Query().Get("id_number")

	client, err := h.service.LookupIndividualByIDNumber(r.Context(), orgID, idNumber)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "client not found")
		return
	}
	utils.RespondJSON(w, http.StatusOK, client)
}

// ─── Corporate ────────────────────────────────────────────────────────────────

func (h *ClientHandler) ListCorporate(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	pag := utils.ParsePagination(r)
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	clients, total, err := h.service.ListCorporate(r.Context(), orgID, search, status, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     clients,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

func (h *ClientHandler) GetCorporateByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	client, err := h.service.GetCorporateByID(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Corporate client not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, client)
}

func (h *ClientHandler) CreateCorporate(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	var c models.CorporateClient
	if err := utils.DecodeJson(r, &c); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.CreateCorporate(r.Context(), orgID, &c); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, c)
}

func (h *ClientHandler) UpdateCorporate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var updates models.CorporateClient
	if err := utils.DecodeJson(r, &updates); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	client, err := h.service.UpdateCorporate(r.Context(), id, orgID, &updates)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, client)
}

func (h *ClientHandler) SearchCorporate(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	search := r.URL.Query().Get("search")

	clients, err := h.service.SearchCorporate(r.Context(), orgID, search)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, clients)
}

func (h *ClientHandler) DeleteCorporate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.DeleteCorporate(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Corporate client deleted successfully"})
}

func (h *ClientHandler) UpsertDocuments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req struct {
		URLs []string `json:"urls"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	doc, err := h.service.UpsertDocuments(r.Context(), id, orgID, req.URLs)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, doc)
}

func (h *ClientHandler) GetCorporateWithBookings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	result, err := h.service.GetCorporateWithBookings(r.Context(), id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, result)
}
