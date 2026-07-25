package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/pkg/utils"
)

type BackofficeAuthHandler struct {
	service interfaces.BackofficeAuthInterface
}

func NewBackofficeAuthHandler(service interfaces.BackofficeAuthInterface) *BackofficeAuthHandler {
	return &BackofficeAuthHandler{service: service}
}

// Login handles POST /api/v1/backoffice/auth/login
func (h *BackofficeAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, token, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		utils.RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// ChangePassword handles POST /api/v1/backoffice/auth/change-password
func (h *BackofficeAuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.GetBackofficeUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		CurrentPassword string `json:"old_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := utils.DecodeJson(r, &req); err != nil || req.CurrentPassword == "" || req.NewPassword == "" {
		utils.RespondError(w, http.StatusBadRequest, "old password and new password are required")
		return
	}

	if err := h.service.ChangePassword(r.Context(), callerID, req.CurrentPassword, req.NewPassword); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

// Me handles GET /api/v1/backoffice/auth/me
func (h *BackofficeAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.GetBackofficeUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.service.GetByID(r.Context(), callerID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, user)
}
