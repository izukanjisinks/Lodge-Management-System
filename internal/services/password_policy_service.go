package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"lodge-system/internal/models"
	"lodge-system/internal/repositories"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Password policy errors
var (
	ErrPasswordExpired  = errors.New("password has expired")
	ErrPasswordReused   = errors.New("password was used recently and cannot be reused")
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooWeak  = errors.New("password does not meet complexity requirements")
)

type PasswordPolicyService struct {
	policyRepo  *repositories.PasswordPolicyRepository
	historyRepo *repositories.PasswordHistoryRepository
	policy      *models.PasswordPolicy               // global default cache
	orgPolicies map[uuid.UUID]*models.PasswordPolicy // per-org cache
}

func NewPasswordPolicyService(
	policyRepo *repositories.PasswordPolicyRepository,
	historyRepo *repositories.PasswordHistoryRepository,
) *PasswordPolicyService {
	service := &PasswordPolicyService{
		policyRepo:  policyRepo,
		historyRepo: historyRepo,
		orgPolicies: make(map[uuid.UUID]*models.PasswordPolicy),
	}

	// Load global default policy at startup
	if err := service.LoadGlobalPolicy(context.Background()); err != nil {
		service.policy = models.DefaultPasswordPolicy()
	}

	return service
}

// LoadGlobalPolicy loads the global default password policy from the database.
func (s *PasswordPolicyService) LoadGlobalPolicy(ctx context.Context) error {
	policy, err := s.policyRepo.Get()
	if err != nil {
		return err
	}
	s.policy = policy
	return nil
}

// GetPolicy returns the effective policy for the given org: org-specific if one exists, otherwise global default.
func (s *PasswordPolicyService) GetPolicy(ctx context.Context, orgID ...uuid.UUID) *models.PasswordPolicy {
	if len(orgID) > 0 && orgID[0] != uuid.Nil {
		id := orgID[0]
		if p, ok := s.orgPolicies[id]; ok {
			return p
		}
		p, err := s.policyRepo.GetByOrg(id)
		if err == nil {
			s.orgPolicies[id] = p
			return p
		}
	}
	return s.policy
}

// UpsertPolicy creates or updates the org-specific policy, seeding from the global default on first creation.
func (s *PasswordPolicyService) UpsertPolicy(ctx context.Context, orgID uuid.UUID, req *models.CreatePasswordPolicyRequest) (*models.PasswordPolicy, error) {
	// Start from the org's existing policy (or global default if none yet)
	policy, err := s.policyRepo.GetByOrg(orgID)
	if err != nil || policy.OrgID == nil {
		// No org-specific row yet — clone the global default as the starting point
		base := s.policy
		clone := *base
		policy = &clone
		policy.ID = uuid.New()
		policy.OrgID = &orgID
		policy.CreatedAt = time.Now()
	}

	if req.MinLength != nil {
		policy.MinLength = *req.MinLength
	}
	if req.RequireUppercase != nil {
		policy.RequireUppercase = *req.RequireUppercase
	}
	if req.RequireLowercase != nil {
		policy.RequireLowercase = *req.RequireLowercase
	}
	if req.RequireNumbers != nil {
		policy.RequireNumbers = *req.RequireNumbers
	}
	if req.RequireSpecialChars != nil {
		policy.RequireSpecialChars = *req.RequireSpecialChars
	} else if req.RequireSpecial != nil {
		policy.RequireSpecialChars = *req.RequireSpecial
	}
	if req.MaxFailedAttempts != nil {
		policy.MaxFailedAttempts = *req.MaxFailedAttempts
	} else if req.MaxAttempts != nil {
		policy.MaxFailedAttempts = *req.MaxAttempts
	}
	if req.LockoutDurationMins != nil {
		policy.LockoutDurationMins = *req.LockoutDurationMins
	} else if req.LockoutMinutes != nil {
		policy.LockoutDurationMins = *req.LockoutMinutes
	}
	if req.PasswordExpiryDays != nil {
		policy.PasswordExpiryDays = req.PasswordExpiryDays
	} else if req.ExpiryDays != nil {
		policy.PasswordExpiryDays = req.ExpiryDays
	}
	if req.OTPLength != nil {
		policy.OTPLength = *req.OTPLength
	}
	if req.OTPExpiryMins != nil {
		policy.OTPExpiryMins = *req.OTPExpiryMins
	}
	if req.SessionTimeoutMins != nil {
		policy.SessionTimeoutMins = *req.SessionTimeoutMins
	}

	policy.UpdatedAt = time.Now()

	if err := s.validatePolicyRanges(policy); err != nil {
		return nil, err
	}

	if err := s.policyRepo.UpsertForOrg(orgID, policy); err != nil {
		return nil, err
	}

	s.orgPolicies[orgID] = policy
	return policy, nil
}

// validatePolicyRanges validates that policy values are within acceptable ranges
func (s *PasswordPolicyService) validatePolicyRanges(policy *models.PasswordPolicy) error {
	if policy.MinLength < 6 || policy.MinLength > 128 {
		return fmt.Errorf("min_length must be between 6 and 128")
	}
	if policy.MaxFailedAttempts < 1 || policy.MaxFailedAttempts > 100 {
		return fmt.Errorf("max_failed_attempts must be between 1 and 100")
	}
	if policy.LockoutDurationMins < 1 || policy.LockoutDurationMins > 10080 {
		return fmt.Errorf("lockout_duration_mins must be between 1 and 10080 (1 week)")
	}
	if policy.PasswordExpiryDays != nil && (*policy.PasswordExpiryDays < 1 || *policy.PasswordExpiryDays > 365) {
		return fmt.Errorf("password_expiry_days must be between 1 and 365 or null")
	}
	if policy.OTPLength < 4 || policy.OTPLength > 10 {
		return fmt.Errorf("otp_length must be between 4 and 10")
	}
	if policy.OTPExpiryMins < 1 || policy.OTPExpiryMins > 60 {
		return fmt.Errorf("otp_expiry_mins must be between 1 and 60")
	}
	if policy.SessionTimeoutMins < 1 || policy.SessionTimeoutMins > 10080 {
		return fmt.Errorf("session_timeout_mins must be between 1 and 10080 (1 week)")
	}
	return nil
}

// ValidateNewPassword validates a new password against the org's policy and history.
func (s *PasswordPolicyService) ValidateNewPassword(ctx context.Context, userID uuid.UUID, newPassword, currentPasswordHash string, orgID ...uuid.UUID) error {
	policy := s.GetPolicy(ctx, orgID...)

	if len(newPassword) < policy.MinLength {
		return fmt.Errorf("%w: minimum %d characters required", ErrPasswordTooShort, policy.MinLength)
	}

	if err := s.validateComplexity(newPassword, policy); err != nil {
		return err
	}

	// Check if reusing current password
	if currentPasswordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(currentPasswordHash), []byte(newPassword)); err == nil {
			return ErrPasswordReused
		}
	}

	// Check password history (last 10 passwords)
	history, err := s.historyRepo.GetRecentByUserID(userID, 10)
	if err == nil {
		for _, h := range history {
			if err := bcrypt.CompareHashAndPassword([]byte(h.PasswordHash), []byte(newPassword)); err == nil {
				return fmt.Errorf("%w: cannot reuse any of your last 10 passwords", ErrPasswordReused)
			}
		}
	}

	return nil
}

// validateComplexity checks if password meets the given policy's complexity requirements.
func (s *PasswordPolicyService) validateComplexity(password string, policy *models.PasswordPolicy) error {
	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	var missing []string
	if policy.RequireUppercase && !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if policy.RequireLowercase && !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if policy.RequireNumbers && !hasNumber {
		missing = append(missing, "number")
	}
	if policy.RequireSpecialChars && !hasSpecial {
		missing = append(missing, "special character")
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: password must contain at least one %s", ErrPasswordTooWeak, strings.Join(missing, ", "))
	}

	return nil
}

// RecordPasswordChange records a password change in history and updates user fields
func (s *PasswordPolicyService) RecordPasswordChange(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	// Add to password history
	history := &models.PasswordHistory{
		ID:           uuid.New(),
		UserID:       userID,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}

	if err := s.historyRepo.Create(history); err != nil {
		return err
	}

	// Clean up old history (keep only last 10)
	return s.historyRepo.DeleteOldHistory(userID, 10)
}

// CalculatePasswordExpiry calculates when a password will expire based on the org's policy.
func (s *PasswordPolicyService) CalculatePasswordExpiry(ctx context.Context, orgID ...uuid.UUID) *time.Time {
	policy := s.GetPolicy(ctx, orgID...)
	if policy.PasswordExpiryDays == nil {
		return nil
	}
	expiryTime := time.Now().AddDate(0, 0, *policy.PasswordExpiryDays)
	return &expiryTime
}

// CheckPasswordExpiry checks if a user's password is expired or expiring soon
func (s *PasswordPolicyService) CheckPasswordExpiry(ctx context.Context, user *models.User) (expired bool, expiringSoon bool, daysUntilExpiry int) {
	if user.PasswordExpiresAt == nil {
		return false, false, 0 // Never expires
	}

	now := time.Now()
	if user.PasswordExpiresAt.Before(now) {
		return true, false, 0 // Already expired
	}

	daysUntil := int(user.PasswordExpiresAt.Sub(now).Hours() / 24)
	if daysUntil <= 14 {
		return false, true, daysUntil // Expiring within 14 days
	}

	return false, false, daysUntil
}

// CheckAccountLockout checks if account is locked and returns lock status
func (s *PasswordPolicyService) CheckAccountLockout(ctx context.Context, user *models.User) (locked bool, reason string) {
	// Check permanent lock (admin action)
	if user.IsLocked {
		return true, "Account has been locked by administrator"
	}

	// Check temporary lock (failed attempts)
	if user.LockedUntil != nil {
		if user.LockedUntil.After(time.Now()) {
			minutesRemaining := int(user.LockedUntil.Sub(time.Now()).Minutes())
			return true, fmt.Sprintf("Account is temporarily locked. Try again in %d minutes", minutesRemaining)
		}
		// Lock has expired, account is unlocked
	}

	return false, ""
}

// ShouldLockAccount determines if account should be locked based on failed attempts.
func (s *PasswordPolicyService) ShouldLockAccount(ctx context.Context, failedAttempts int, orgID ...uuid.UUID) bool {
	return failedAttempts >= s.GetPolicy(ctx, orgID...).MaxFailedAttempts
}

// CalculateLockoutTime calculates when the lockout period expires.
func (s *PasswordPolicyService) CalculateLockoutTime(ctx context.Context, orgID ...uuid.UUID) time.Time {
	return time.Now().Add(time.Duration(s.GetPolicy(ctx, orgID...).LockoutDurationMins) * time.Minute)
}

// GetSessionTimeout returns the session timeout duration in seconds.
func (s *PasswordPolicyService) GetSessionTimeout(ctx context.Context, orgID ...uuid.UUID) int {
	return s.GetPolicy(ctx, orgID...).SessionTimeoutMins * 60
}
