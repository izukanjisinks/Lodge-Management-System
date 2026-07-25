package services

import (
	"context"
	"errors"
	"fmt"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/utils/email"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type BackofficeUserService struct {
	repo         *repository.BackofficeUserRepository
	emailService *email.EmailService
}

func NewBackofficeUserService(repo *repository.BackofficeUserRepository) *BackofficeUserService {
	return &BackofficeUserService{repo: repo}
}

func (s *BackofficeUserService) SetEmailService(svc *email.EmailService) {
	s.emailService = svc
}

func (s *BackofficeUserService) List(ctx context.Context) ([]models.BackofficeUser, error) {
	return s.repo.List()
}

func (s *BackofficeUserService) GetByID(ctx context.Context, id uuid.UUID) (*models.BackofficeUser, error) {
	return s.repo.GetByID(id)
}

func (s *BackofficeUserService) Create(ctx context.Context, req models.CreateBackofficeUserRequest) (*models.BackofficeUser, error) {
	if req.FullName == "" || req.Email == "" {
		return nil, errors.New("full_name and email are required")
	}

	exists, err := s.repo.EmailExists(req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("email already registered")
	}

	password, err := utils.GenerateRandomPassword()
	if err != nil {
		return nil, err
	}

	hashed, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	u := &models.BackofficeUser{
		FullName:       req.FullName,
		Email:          req.Email,
		Password:       hashed,
		IsActive:       true,
		ChangePassword: true,
	}

	if err := s.repo.Create(u); err != nil {
		return nil, err
	}

	if s.emailService != nil {
		go func() {
			body := email.BackofficeWelcomeTemplate(req.FullName, req.Email, password)
			if sendErr := s.emailService.SendEmail([]string{req.Email}, "You've been granted Backoffice Portal access", body); sendErr != nil {
				fmt.Printf("warning: failed to send backoffice welcome email to %s: %v\n", req.Email, sendErr)
			}
		}()
	}

	return u, nil
}

func (s *BackofficeUserService) Update(ctx context.Context, id uuid.UUID, req models.UpdateBackofficeUserRequest) (*models.BackofficeUser, error) {
	u, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if req.FullName != "" {
		u.FullName = req.FullName
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	if err := s.repo.Update(u); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id)
}

func (s *BackofficeUserService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(id)
}

func (s *BackofficeUserService) ResetPassword(ctx context.Context, id uuid.UUID) error {
	u, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("user not found")
	}

	password, err := utils.GenerateRandomPassword()
	if err != nil {
		return err
	}

	hashed, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	u.Password = hashed
	u.ChangePassword = true
	if err := s.repo.Update(u); err != nil {
		return err
	}

	if s.emailService != nil {
		go func() {
			body := email.PasswordResetTemplate(password)
			if sendErr := s.emailService.SendEmail([]string{u.Email}, "Your Password Has Been Reset", body); sendErr != nil {
				fmt.Printf("warning: failed to send password reset email to %s: %v\n", u.Email, sendErr)
			}
		}()
	}

	return nil
}

func (s *BackofficeUserService) Lock(ctx context.Context, id uuid.UUID) error {
	u, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	u.IsLocked = true
	return s.repo.Update(u)
}

func (s *BackofficeUserService) Unlock(ctx context.Context, id uuid.UUID) error {
	u, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	u.IsLocked = false
	u.LockedUntil = nil
	u.FailedLoginAttempts = 0
	return s.repo.Update(u)
}
