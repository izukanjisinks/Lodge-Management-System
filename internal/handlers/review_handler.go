package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"
)

type ReviewHandler struct {
	service interfaces.ReviewInterface
}

func NewReviewHandler(service interfaces.ReviewInterface) *ReviewHandler {
	return &ReviewHandler{service: service}
}

// Submit handles POST /api/v1/guest/reviews
func (h *ReviewHandler) Submit(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*models.User)
	if !ok || user == nil {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.SubmitReviewRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	review, err := h.service.Submit(r.Context(), user.UserID, &req)
	if err != nil {
		switch err.Error() {
		case "forbidden":
			utils.RespondError(w, http.StatusForbidden, "Access denied")
		default:
			utils.RespondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.RespondJSON(w, http.StatusCreated, review)
}

// GetSummary handles GET /api/v1/reviews/summary
func (h *ReviewHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.service.GetSummary(r.Context())
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to load rating summary")
		return
	}
	utils.RespondJSON(w, http.StatusOK, summary)
}
