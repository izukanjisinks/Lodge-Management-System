package handlers

import (
	"lodge-system/internal/interfaces"
	"lodge-system/internal/middleware"
	"lodge-system/pkg/utils"
	"net/http"
	"time"
)

type AuditLogHandler struct {
	service interfaces.AuditLogInterface
}

func NewAuditLogHandler(service interfaces.AuditLogInterface) *AuditLogHandler {
	return &AuditLogHandler{service: service}
}

func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	p := utils.ParsePagination(r)

	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")
	action := r.URL.Query().Get("action")

	var from, to *time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			to = &end
		}
	}

	logs, total, err := h.service.List(r.Context(), orgID, entityType, entityID, action, from, to, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to fetch audit logs")
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     logs,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
	})
}
