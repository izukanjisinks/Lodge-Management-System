package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"
)

type WebUserAuthHandler struct {
	service interfaces.WebUserAuthInterface
}

func NewWebUserAuthHandler(service interfaces.WebUserAuthInterface) *WebUserAuthHandler {
	return &WebUserAuthHandler{service: service}
}

// Register handles POST /api/v1/web/auth/register
func (h *WebUserAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.WebUserRegisterRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.service.Register(r.Context(), &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"user": user,
	})
}

// Login handles POST /api/v1/web/auth/login
func (h *WebUserAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.WebUserLoginRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, token, err := h.service.Login(r.Context(), &req)
	if err != nil {
		utils.RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// GetProfile handles GET /api/v1/web/profile
func (h *WebUserAuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.service.GetByID(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, user)
}

// UpdateProfile handles PUT /api/v1/web/profile
func (h *WebUserAuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.WebUserUpdateRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.service.Update(r.Context(), userID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, user)
}

// ChangePassword handles PUT /api/v1/web/auth/change-password
func (h *WebUserAuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}
