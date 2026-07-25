package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/utils/password"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type PasswordPolicyHandler struct {
	policyService interfaces.PasswordPolicyInterface
	userService   interfaces.UserInterface
}

func NewPasswordPolicyHandler(policyService interfaces.PasswordPolicyInterface, userService interfaces.UserInterface) *PasswordPolicyHandler {
	return &PasswordPolicyHandler{
		policyService: policyService,
		userService:   userService,
	}
}

// GetPasswordPolicy retrieves the org-specific policy, falling back to the global default.
func (h *PasswordPolicyHandler) GetPasswordPolicy(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	policy := h.policyService.GetPolicy(r.Context(), orgID)
	if policy == nil {
		utils.RespondError(w, http.StatusNotFound, "No password policy configured")
		return
	}

	utils.RespondJSON(w, http.StatusOK, policy)
}

// UpdatePasswordPolicy updates or creates the global password policy
func (h *PasswordPolicyHandler) UpdatePasswordPolicy(w http.ResponseWriter, r *http.Request) {
	// Only super_admin or hr_manager can update password policy
	user, ok := r.Context().Value(middleware.UserKey).(*models.User)
	if !ok || user == nil {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Check if user has permission
	if user.Role == nil || (user.Role.Name != models.RoleAdmin && user.Role.Name != models.RoleBranchAdmin) {
		utils.RespondError(w, http.StatusForbidden, "Insufficient permissions to update password policy")
		return
	}

	var req models.CreatePasswordPolicyRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	policy, err := h.policyService.UpsertPolicy(r.Context(), orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, policy)
}

// ChangePassword allows authenticated user to change their password
func (h *PasswordPolicyHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*models.User)
	if !ok || user == nil {
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

	if req.OldPassword == "" || req.NewPassword == "" {
		utils.RespondError(w, http.StatusBadRequest, "Old password and new password are required")
		return
	}

	if err := h.userService.ChangePassword(r.Context(), user.UserID, req.OldPassword, req.NewPassword); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Password changed successfully",
	})
}

// GeneratePassword generates a password based on the current password policy
func (h *PasswordPolicyHandler) GeneratePassword(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	policy := h.policyService.GetPolicy(r.Context(), orgID)
	if policy == nil {
		utils.RespondError(w, http.StatusNotFound, "No password policy configured")
		return
	}

	// Generate password using the policy settings
	generatedPassword, err := password.GeneratePassword(
		policy.MinLength,
		policy.RequireUppercase,
		policy.RequireLowercase,
		policy.RequireNumbers,
		policy.RequireSpecialChars,
	)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to generate password")
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"password": generatedPassword,
	})
}

// ResetUserPassword allows admin to reset a user's password
func (h *PasswordPolicyHandler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	// Only super_admin or hr_manager can reset passwords
	user, ok := r.Context().Value(middleware.UserKey).(*models.User)
	if !ok || user == nil {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if user.Role == nil || (user.Role.Name != models.RoleAdmin && user.Role.Name != models.RoleBranchAdmin) {
		utils.RespondError(w, http.StatusForbidden, "Insufficient permissions to reset passwords")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserID == "" {
		utils.RespondError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := h.userService.ResetPassword(r.Context(), userID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Password reset successfully. A temporary password has been sent to the user's email. User must change password on next login.",
	})
}
