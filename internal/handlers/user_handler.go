package handlers

import (
	"lodge-system/internal/interfaces"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type UserHandler struct {
	service     interfaces.UserInterface
	roleService interfaces.RoleInterface
}

func NewUserHandler(service interfaces.UserInterface, roleService interfaces.RoleInterface) *UserHandler {
	return &UserHandler{service: service, roleService: roleService}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FullName string     `json:"full_name"`
		Email    string     `json:"email"`
		Password string     `json:"password"`
		Role     string     `json:"role"`
		Status   string     `json:"status"`
		BranchID *uuid.UUID `json:"branch_id,omitempty"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.FullName == "" || req.Email == "" || req.Password == "" || req.Role == "" {
		utils.RespondError(w, http.StatusBadRequest, "full_name, email, password, and role are required")
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	// Branch-scoped admins automatically assign users to their branch.
	// Org-level admins can optionally specify a branch_id in the body.
	branchID := middleware.GetBranchIDFromContext(r.Context())
	if branchID == nil {
		branchID = req.BranchID
	}

	user := &models.User{
		FullName: req.FullName,
		Email:    req.Email,
		Password: req.Password,
		RoleName: req.Role,
		IsActive: req.Status != "inactive",
		OrgID:    &orgID,
		BranchID: branchID,
	}

	if err := h.service.Register(r.Context(), user); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, userResponse(user))
}

func (h *UserHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pag := utils.ParsePagination(r)
	search := r.URL.Query().Get("search")

	var roleID *uuid.UUID
	if roleIDStr := r.URL.Query().Get("role_id"); roleIDStr != "" {
		if id, err := uuid.Parse(roleIDStr); err == nil {
			roleID = &id
		}
	}

	var isActive *bool
	if activeStr := r.URL.Query().Get("is_active"); activeStr != "" {
		val := activeStr == "true"
		isActive = &val
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	users, total, err := h.service.ListUsers(r.Context(), orgID, branchID, search, roleID, isActive, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve users")
		return
	}

	data := make([]map[string]interface{}, len(users))
	for i := range users {
		data[i] = userResponse(&users[i])
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     data,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), id)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	utils.RespondJSON(w, http.StatusOK, userResponse(user))
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		FullName string     `json:"full_name"`
		Email    string     `json:"email"`
		Password string     `json:"password"`
		Role     string     `json:"role"`
		Status   string     `json:"status"`
		BranchID *uuid.UUID `json:"branch_id"`
	}
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Branch-scoped admins cannot reassign users to a different branch.
	if req.BranchID != nil {
		if callerBranch := middleware.GetBranchIDFromContext(r.Context()); callerBranch != nil {
			utils.RespondError(w, http.StatusForbidden, "Branch admins cannot reassign users to a different branch")
			return
		}
	}

	callerID, _ := middleware.GetUserIDFromContext(r.Context())
	user, err := h.service.UpdateUserFull(r.Context(), id, callerID, req.FullName, req.Email, req.Password, req.Role, req.Status, req.BranchID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, userResponse(user))
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*models.User)
	if !ok || user == nil {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	utils.RespondJSON(w, http.StatusOK, userResponse(user))
}

func (h *UserHandler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		RoleID string `json:"role_id"`
	}
	if err := utils.DecodeJson(r, &req); err != nil || req.RoleID == "" {
		utils.RespondError(w, http.StatusBadRequest, "role_id is required")
		return
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid role ID")
		return
	}

	user, err := h.service.ChangeUserRole(r.Context(), id, roleID)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, userResponse(user))
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	if err := h.service.DeleteUser(r.Context(), id, orgID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

func (h *UserHandler) Lock(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := h.service.LockUser(r.Context(), id); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "User account locked successfully"})
}

func (h *UserHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	roles, err := h.roleService.GetAllRoles(r.Context(), orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve roles")
		return
	}

	callerRole, _ := middleware.GetRoleFromContext(r.Context())
	if callerRole != models.RoleAdmin {
		filtered := roles[:0]
		for _, role := range roles {
			if role.Name != models.RoleAdmin {
				filtered = append(filtered, role)
			}
		}
		roles = filtered
	}

	utils.RespondJSON(w, http.StatusOK, roles)
}

func (h *UserHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := h.service.UnlockUser(r.Context(), id); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "User account unlocked successfully"})
}
