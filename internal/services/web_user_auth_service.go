package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/utils/email"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type WebUserAuthService struct {
	repo         *repository.WebUserRepository
	emailService *email.EmailService
	policy       *PasswordPolicyService
}

func NewWebUserAuthService(repo *repository.WebUserRepository, policy *PasswordPolicyService) *WebUserAuthService {
	return &WebUserAuthService{repo: repo, policy: policy}
}

func (s *WebUserAuthService) SetEmailService(svc *email.EmailService) {
	s.emailService = svc
}

func (s *WebUserAuthService) Register(ctx context.Context, req *models.WebUserRegisterRequest) (*models.WebUser, error) {
	if req.FullName == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("full_name, email, and password are required")
	}

	exists, err := s.repo.EmailExists(req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("email already registered")
	}

	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	u := &models.WebUser{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
	}
	if err := s.repo.Create(u, hashed); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	if s.emailService != nil {
		go func() {
			body := email.GuestWelcomeTemplate(req.FullName)
			if sendErr := s.emailService.SendEmail([]string{req.Email}, "Welcome", body); sendErr != nil {
				fmt.Printf("warning: failed to send welcome email to %s: %v\n", req.Email, sendErr)
			}
		}()
	}

	return u, nil
}

func (s *WebUserAuthService) Login(ctx context.Context, req *models.WebUserLoginRequest) (*models.WebUser, string, error) {
	if req.Email == "" || req.Password == "" {
		return nil, "", errors.New("email and password are required")
	}

	u, hashed, err := s.repo.GetByEmail(req.Email)
	if err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	if !u.IsActive {
		return nil, "", errors.New("account is inactive")
	}

	if u.IsLocked {
		if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
			return nil, "", errors.New("account is temporarily locked — try again later")
		}
		_ = s.repo.SetLocked(u.ID, nil)
	}

	if err := utils.ComparePasswords(hashed, req.Password); err != nil {
		_ = s.repo.RecordFailedLogin(u.ID)
		u, _, _ = s.repo.GetByEmail(req.Email)
		if s.policy.ShouldLockAccount(ctx, u.FailedLoginAttempts) {
			until := s.policy.CalculateLockoutTime(ctx)
			_ = s.repo.SetLocked(u.ID, &until)
			return nil, "", errors.New("account locked due to too many failed attempts")
		}
		return nil, "", errors.New("invalid credentials")
	}

	_ = s.repo.RecordLogin(u.ID)

	token, err := utils.GenerateGuestToken(u.Email, u.ID)
	if err != nil {
		return nil, "", errors.New("failed to generate token")
	}

	return u, token, nil
}

func (s *WebUserAuthService) GetByID(ctx context.Context, id uuid.UUID) (*models.WebUser, error) {
	return s.repo.GetByID(id)
}

func (s *WebUserAuthService) Update(ctx context.Context, id uuid.UUID, req *models.WebUserUpdateRequest) (*models.WebUser, error) {
	if err := s.repo.Update(id, req); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id)
}

func (s *WebUserAuthService) ChangePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error {
	u, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("user not found")
	}

	_, hashed, err := s.repo.GetByEmail(u.Email)
	if err != nil {
		return err
	}

	if err := utils.ComparePasswords(hashed, oldPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	newHashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.repo.UpdatePassword(id, newHashed)
}
