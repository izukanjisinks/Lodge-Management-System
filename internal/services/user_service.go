package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/utils/email"
	"lodge-system/internal/utils/password"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type UserService struct {
	repo                  *repository.UserRepository
	roleRepo              *repository.RoleRepository
	passwordPolicyService *PasswordPolicyService
	emailService          *email.EmailService
}

func NewUserService(repo *repository.UserRepository, roleRepo *repository.RoleRepository) *UserService {
	return &UserService{repo: repo, roleRepo: roleRepo}
}

func (s *UserService) SetEmailService(emailService *email.EmailService) {
	s.emailService = emailService
}

func (s *UserService) SetPasswordPolicyService(policyService *PasswordPolicyService) {
	s.passwordPolicyService = policyService
}

func (s *UserService) Register(ctx context.Context, user *models.User) error {
	orgID := uuid.Nil
	if user.OrgID != nil {
		orgID = *user.OrgID
	}
	exists, err := s.repo.EmailExists(user.Email, orgID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("email already registered")
	}

	if s.passwordPolicyService != nil {
		if err := s.passwordPolicyService.ValidateNewPassword(ctx, uuid.Nil, user.Password, "", orgID); err != nil {
			return err
		}
	}

	plainPassword := user.Password
	hashed, err := utils.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashed
	user.IsActive = true
	now := time.Now()
	user.PasswordChangedAt = &now

	if user.RoleName != "" {
		role, err := s.roleRepo.GetRoleByName(user.RoleName)
		if err != nil {
			return fmt.Errorf("role %q not found: %w", user.RoleName, err)
		}
		user.RoleID = &role.RoleID
		user.Role = role
	}

	if user.Role != nil && user.Role.Name == models.RoleAdmin {
		user.BranchID = nil
	}

	if s.passwordPolicyService != nil {
		user.PasswordExpiresAt = s.passwordPolicyService.CalculatePasswordExpiry(ctx, orgID)
	}

	if err := s.repo.Create(user); err != nil {
		return err
	}

	if s.passwordPolicyService != nil {
		_ = s.passwordPolicyService.RecordPasswordChange(ctx, user.UserID, hashed)
	}

	if s.emailService != nil {
		userEmail := user.Email
		fullName := user.FullName
		go func() {
			htmlBody := email.WelcomeUserTemplate(fullName, userEmail, plainPassword)
			if err := s.emailService.SendEmail([]string{userEmail}, "Welcome to Lodge Management System", htmlBody); err != nil {
				fmt.Printf("WARNING: Failed to send welcome email to %s: %v\n", userEmail, err)
			}
		}()
	}

	return nil
}

func (s *UserService) Login(ctx context.Context, emailAddr, pwd string) (map[string]interface{}, error) {
	return s.LoginWithOrg(ctx, emailAddr, pwd, uuid.Nil)
}

// LoginWithOrg handles both steps of the multi-org login flow.
// When orgID is uuid.Nil, it looks up all users with that email across orgs.
// - 0 matches → 401
// - 1 match   → proceed to password check
// - 2+ matches → return org list for selection (no password check)
// When orgID is provided, it scopes the lookup to that specific org.
func (s *UserService) LoginWithOrg(ctx context.Context, emailAddr, pwd string, orgID uuid.UUID) (map[string]interface{}, error) {
	var user *models.User

	if orgID == uuid.Nil {
		matches, err := s.repo.GetAllByEmail(emailAddr)
		if err != nil || len(matches) == 0 {
			return nil, errors.New("invalid email or password")
		}
		if len(matches) > 1 {
			orgs := make([]map[string]interface{}, len(matches))
			for i, m := range matches {
				orgs[i] = map[string]interface{}{
					"org_id": m.OrgID,
					"name":   m.OrgName,
				}
			}
			return map[string]interface{}{
				"requires_org_selection": true,
				"organizations":          orgs,
			}, nil
		}
		user, err = s.repo.GetUserByEmail(emailAddr)
		if err != nil {
			return nil, errors.New("invalid email or password")
		}
	} else {
		var err error
		user, err = s.repo.GetByEmailAndOrg(emailAddr, orgID)
		if err != nil {
			return nil, errors.New("invalid email or password")
		}
	}

	if !user.IsActive {
		return nil, errors.New("account is inactive")
	}

	userOrgID := uuid.Nil
	if user.OrgID != nil {
		userOrgID = *user.OrgID
	}

	if s.passwordPolicyService != nil {
		locked, reason := s.passwordPolicyService.CheckAccountLockout(ctx, user)
		if locked {
			return nil, errors.New(reason)
		}
	}

	if err := utils.ComparePasswords(user.Password, pwd); err != nil {
		if s.passwordPolicyService != nil {
			user.FailedLoginAttempts++
			if s.passwordPolicyService.ShouldLockAccount(ctx, user.FailedLoginAttempts, userOrgID) {
				lockoutTime := s.passwordPolicyService.CalculateLockoutTime(ctx, userOrgID)
				user.LockedUntil = &lockoutTime
			}
			_ = s.repo.Update(user)
		}
		return nil, errors.New("invalid email or password")
	}

	now := time.Now()
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	user.LastLoginAt = &now
	_ = s.repo.Update(user)

	var passwordExpired bool
	var passwordExpiringSoon bool
	var daysUntilExpiry int
	if s.passwordPolicyService != nil {
		passwordExpired, passwordExpiringSoon, daysUntilExpiry = s.passwordPolicyService.CheckPasswordExpiry(ctx, user)
		if passwordExpired {
			return nil, errors.New("password has expired, please change your password")
		}
	}

	var tokenExpiry time.Duration
	if s.passwordPolicyService != nil {
		tokenExpiry = time.Duration(s.passwordPolicyService.GetSessionTimeout(ctx, userOrgID)) * time.Second
	} else {
		tokenExpiry = 24 * time.Hour
	}

	roleName := ""
	if user.Role != nil {
		roleName = user.Role.Name
	}
	orgIDVal := uuid.Nil
	if user.OrgID != nil {
		orgIDVal = *user.OrgID
	}

	token, err := utils.GenerateStaffToken(user.Email, user.UserID, orgIDVal, user.BranchID, roleName)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"token":      token,
		"expires_at": time.Now().Add(tokenExpiry),
		"user": map[string]interface{}{
			"user_id":         user.UserID,
			"org_id":          orgIDVal,
			"org_name":        user.OrgName,
			"org_logo_url":    user.OrgLogoURL,
			"full_name":       user.FullName,
			"email":           user.Email,
			"role":            user.Role,
			"is_active":       user.IsActive,
			"change_password": user.ChangePassword,
		},
	}

	if passwordExpiringSoon {
		response["password_warning"] = map[string]interface{}{
			"message":           "Your password is expiring soon",
			"days_until_expiry": daysUntilExpiry,
		}
	}

	return response, nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]models.User, error) {
	return s.repo.GetAllUsers()
}

func (s *UserService) ListUsers(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, search string, roleID *uuid.UUID, isActive *bool, page, pageSize int) ([]models.User, int, error) {
	return s.repo.List(orgID, branchID, search, roleID, isActive, page, pageSize)
}

func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.GetUserByID(id)
}

func (s *UserService) UpdateUser(ctx context.Context, updates *models.User) (*models.User, error) {
	existing, err := s.repo.GetUserByID(updates.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if updates.Email != "" && updates.Email != existing.Email {
		orgID := uuid.Nil
		if existing.OrgID != nil {
			orgID = *existing.OrgID
		}
		exists, err := s.repo.EmailExists(updates.Email, orgID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("email already in use")
		}
		existing.Email = updates.Email
	}

	if updates.RoleID != nil {
		existing.RoleID = updates.RoleID
	}
	existing.IsActive = updates.IsActive

	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}
	return s.repo.GetUserByID(existing.UserID)
}

// UpdateUserFull handles the frontend PUT /users/{id} payload: full_name, email, role name, status, optional password.
// callerID is the ID of the user making the request — used to decide whether to send a password-change notification email.
// If callerID == id (self-edit) no email is sent; if an admin changes someone else's password the user is notified.
func (s *UserService) UpdateUserFull(ctx context.Context, id uuid.UUID, callerID uuid.UUID, fullName, newEmail, pwd, roleName, status string, branchID *uuid.UUID) (*models.User, error) {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if fullName != "" {
		user.FullName = fullName
	}

	if newEmail != "" && newEmail != user.Email {
		orgID := uuid.Nil
		if user.OrgID != nil {
			orgID = *user.OrgID
		}
		exists, err := s.repo.EmailExists(newEmail, orgID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("email already in use")
		}
		user.Email = newEmail
	}

	if roleName != "" {
		role, err := s.roleRepo.GetRoleByName(roleName)
		if err != nil {
			return nil, fmt.Errorf("role %q not found: %w", roleName, err)
		}
		user.RoleID = &role.RoleID
		if role.Name == models.RoleAdmin {
			user.BranchID = nil
		}
	}

	if status != "" {
		user.IsActive = status != "inactive"
	}

	if branchID != nil {
		if user.Role != nil && user.Role.Name == models.RoleAdmin {
			return nil, errors.New("admin users cannot be assigned to a branch")
		}
		user.BranchID = branchID
	}

	updateOrgID := uuid.Nil
	if user.OrgID != nil {
		updateOrgID = *user.OrgID
	}

	if pwd != "" {
		if s.passwordPolicyService != nil {
			if err := s.passwordPolicyService.ValidateNewPassword(ctx, id, pwd, user.Password, updateOrgID); err != nil {
				return nil, err
			}
		}
		hashed, err := utils.HashPassword(pwd)
		if err != nil {
			return nil, err
		}
		user.Password = hashed
		now := time.Now()
		user.PasswordChangedAt = &now
		if s.passwordPolicyService != nil {
			user.PasswordExpiresAt = s.passwordPolicyService.CalculatePasswordExpiry(ctx, updateOrgID)
			_ = s.passwordPolicyService.RecordPasswordChange(ctx, id, hashed)
		}

		// Notify the user only when an admin/manager changed their password, not when self-editing
		if s.emailService != nil && callerID != id {
			go func(toEmail, rawPwd string) {
				body := email.PasswordResetTemplate(rawPwd)
				if err := s.emailService.SendEmail([]string{toEmail}, "Your Password Has Been Reset", body); err != nil {
					fmt.Printf("warning: failed to send password reset email to %s: %v\n", toEmail, err)
				}
			}(user.Email, pwd)
		}
	}

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}
	return s.repo.GetUserByID(id)
}

func (s *UserService) GetByEmail(ctx context.Context, emailAddr string) (*models.User, error) {
	return s.repo.GetUserByEmail(emailAddr)
}

func (s *UserService) ChangeUserRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) (*models.User, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	user.RoleID = &roleID
	if err := s.repo.Update(user); err != nil {
		return nil, err
	}
	return s.repo.GetUserByID(userID)
}

func (s *UserService) DeactivateUser(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	user.IsActive = false
	return s.repo.Update(user)
}

func (s *UserService) DeleteUser(ctx context.Context, id, orgID uuid.UUID) error {
	_, err := s.repo.GetUserByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	return s.repo.Delete(id, orgID)
}

func (s *UserService) LockUser(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	user.IsLocked = true
	user.LockedUntil = nil
	return s.repo.Update(user)
}

func (s *UserService) UnlockUser(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	user.IsLocked = false
	user.LockedUntil = nil
	user.FailedLoginAttempts = 0
	return s.repo.Update(user)
}

func (s *UserService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	changeOrgID := uuid.Nil
	if user.OrgID != nil {
		changeOrgID = *user.OrgID
	}

	if err := utils.ComparePasswords(user.Password, oldPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	if s.passwordPolicyService != nil {
		if err := s.passwordPolicyService.ValidateNewPassword(ctx, userID, newPassword, user.Password, changeOrgID); err != nil {
			return err
		}
	}

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.Password = hashed
	now := time.Now()
	user.PasswordChangedAt = &now
	user.ChangePassword = false

	if s.passwordPolicyService != nil {
		user.PasswordExpiresAt = s.passwordPolicyService.CalculatePasswordExpiry(ctx, changeOrgID)
	}

	if err := s.repo.Update(user); err != nil {
		return err
	}

	if s.passwordPolicyService != nil {
		_ = s.passwordPolicyService.RecordPasswordChange(ctx, userID, hashed)
	}

	return nil
}

func (s *UserService) ResetPassword(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	resetOrgID := uuid.Nil
	if user.OrgID != nil {
		resetOrgID = *user.OrgID
	}

	newPassword, err := password.GenerateTemporaryPassword()
	if err != nil {
		return fmt.Errorf("failed to generate temporary password: %w", err)
	}

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.Password = hashed
	now := time.Now()
	user.PasswordChangedAt = &now
	user.ChangePassword = true
	user.FailedLoginAttempts = 0
	user.IsLocked = false
	user.LockedUntil = nil

	if s.passwordPolicyService != nil {
		user.PasswordExpiresAt = s.passwordPolicyService.CalculatePasswordExpiry(ctx, resetOrgID)
	}

	if err := s.repo.Update(user); err != nil {
		return err
	}

	if s.passwordPolicyService != nil {
		_ = s.passwordPolicyService.RecordPasswordChange(ctx, userID, hashed)
	}

	if s.emailService != nil {
		userEmail := user.Email
		go func() {
			htmlBody := email.PasswordResetTemplate(newPassword)
			if err := s.emailService.SendEmail([]string{userEmail}, "Your Password Has Been Reset", htmlBody); err != nil {
				fmt.Printf("WARNING: Failed to send password reset email to %s: %v\n", userEmail, err)
			}
		}()
	}

	return nil
}
