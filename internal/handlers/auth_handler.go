package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type AuthHandler struct {
	userService interfaces.UserInterface
}

func NewAuthHandler(userService interfaces.UserInterface) *AuthHandler {
	return &AuthHandler{userService: userService}
}

func (h *AuthHandler) AdminUserHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		utils.RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.userService.GetByEmail(r.Context(), req.Email)

	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       user.UserID,
		"email":         user.Email,
		"password_hash": user.Password,
		"is_active":     user.IsActive,
		"role_id":       user.RoleID,
	})

}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		OrgID    string `json:"org_id,omitempty"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		utils.RespondError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	orgID := uuid.Nil
	if req.OrgID != "" {
		if parsed, err := uuid.Parse(req.OrgID); err == nil {
			orgID = parsed
		}
	}

	result, err := h.userService.LoginWithOrg(r.Context(), req.Email, req.Password, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, result)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*models.User)
	if !ok || user == nil {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	utils.RespondJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// JWT is stateless; client-side logout by discarding the token
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Status   string `json:"status"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.FullName == "" || req.Email == "" || req.Password == "" || req.Role == "" {
		utils.RespondError(w, http.StatusBadRequest, "full_name, email, password, and role are required")
		return
	}

	user := &models.User{
		FullName: req.FullName,
		Email:    req.Email,
		Password: req.Password,
		RoleName: req.Role,
		IsActive: req.Status != "inactive",
	}

	if err := h.userService.Register(r.Context(), user); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, userResponse(user))
}
