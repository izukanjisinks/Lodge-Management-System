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

type BackofficeAuthService struct {
	repo         *repository.BackofficeUserRepository
	emailService *email.EmailService
}

func NewBackofficeAuthService(repo *repository.BackofficeUserRepository) *BackofficeAuthService {
	return &BackofficeAuthService{repo: repo}
}

func (s *BackofficeAuthService) SetEmailService(svc *email.EmailService) {
	s.emailService = svc
}

func (s *BackofficeAuthService) GetByID(ctx context.Context, id uuid.UUID) (*models.BackofficeUser, error) {
	return s.repo.GetByID(id)
}

func (s *BackofficeAuthService) Login(ctx context.Context, emailAddr, password string) (*models.BackofficeUser, string, error) {
	user, err := s.repo.GetByEmail(emailAddr)
	if err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	if !user.IsActive {
		return nil, "", errors.New("account is inactive")
	}

	if err := utils.ComparePasswords(user.Password, password); err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	token, err := utils.GenerateBackofficeToken(user.Email, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *BackofficeAuthService) ChangePassword(ctx context.Context, id uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("user not found")
	}

	if err := utils.ComparePasswords(user.Password, currentPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.Password = hashed
	user.ChangePassword = false
	return s.repo.Update(user)
}

func (s *BackofficeAuthService) ResetPassword(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("user not found")
	}

	newPassword, err := utils.GenerateRandomPassword()
	if err != nil {
		return err
	}

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.Password = hashed
	user.ChangePassword = true
	if err := s.repo.Update(user); err != nil {
		return err
	}

	if s.emailService != nil {
		go func() {
			body := email.PasswordResetTemplate(newPassword)
			if sendErr := s.emailService.SendEmail([]string{user.Email}, "Your Password Has Been Reset", body); sendErr != nil {
				fmt.Printf("warning: failed to send password reset email to %s: %v\n", user.Email, sendErr)
			}
		}()
	}

	return nil
}
