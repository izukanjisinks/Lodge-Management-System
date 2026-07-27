package services

import (
	"context"
	"errors"
	"fmt"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/utils/email"
	"lodge-system/internal/utils/password"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type GuestAuthService struct {
	guestRepo    *repository.GuestRepository
	emailService *email.EmailService
}

func NewGuestAuthService(guestRepo *repository.GuestRepository) *GuestAuthService {
	return &GuestAuthService{guestRepo: guestRepo}
}

func (s *GuestAuthService) SetEmailService(svc *email.EmailService) {
	s.emailService = svc
}

func (s *GuestAuthService) Register(ctx context.Context, req *models.GuestRegisterRequest) (*models.Guest, error) {
	if req.FullName == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("full_name, email, and password are required")
	}

	exists, err := s.guestRepo.EmailExists(req.Email)
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

	guest := &models.Guest{
		FullName: req.FullName,
		Email:    req.Email,
		Password: hashed,
		Phone:    req.Phone,
		IsActive: true,
	}

	if err := s.guestRepo.Create(guest); err != nil {
		return nil, fmt.Errorf("failed to create guest account: %w", err)
	}

	if s.emailService != nil {
		go func() {
			body := email.GuestWelcomeTemplate(req.FullName)
			if sendErr := s.emailService.SendEmail([]string{req.Email}, "Welcome to Mwakwanda", body); sendErr != nil {
				fmt.Printf("warning: failed to send guest welcome email to %s: %v\n", req.Email, sendErr)
			}
		}()
	}

	return guest, nil
}

func (s *GuestAuthService) Login(ctx context.Context, emailAddr, password string) (*models.Guest, string, error) {
	guest, err := s.guestRepo.GetByEmail(emailAddr)
	if err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	if !guest.IsActive {
		return nil, "", errors.New("account is inactive")
	}

	if err := utils.ComparePasswords(guest.Password, password); err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	token, err := utils.GenerateGuestToken(guest.Email, guest.ID)
	if err != nil {
		return nil, "", err
	}

	return guest, token, nil
}

func (s *GuestAuthService) GetByID(ctx context.Context, id uuid.UUID) (*models.Guest, error) {
	return s.guestRepo.GetByID(id)
}

// ResetPassword generates a new password for the guest and emails it to them.
// Always returns nil to avoid leaking whether the email exists.
func (s *GuestAuthService) ResetPassword(ctx context.Context, emailAddr string) error {
	fmt.Printf("[ResetPassword] request for email: %s\n", emailAddr)

	guest, err := s.guestRepo.GetByEmail(emailAddr)
	if err != nil {
		fmt.Printf("[ResetPassword] guest not found for email %s: %v\n", emailAddr, err)
		return nil
	}
	fmt.Printf("[ResetPassword] guest found: %s (id: %s)\n", guest.Email, guest.ID)

	newPassword, err := password.GenerateTemporaryPassword()
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	fmt.Printf("[ResetPassword] generated new password for %s\n", guest.Email)

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.guestRepo.UpdatePassword(guest.ID, hashed); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	fmt.Printf("[ResetPassword] password updated in DB for %s\n", guest.Email)

	if s.emailService == nil {
		fmt.Printf("[ResetPassword] email service is nil — skipping email send\n")
		return nil
	}

	go func() {
		fmt.Printf("[ResetPassword] sending email to %s\n", guest.Email)
		body := email.GuestPasswordResetTemplate(guest.FullName, newPassword)
		if sendErr := s.emailService.SendEmail([]string{guest.Email}, "Password Reset — Mwakwanda", body); sendErr != nil {
			fmt.Printf("[ResetPassword] failed to send email to %s: %v\n", guest.Email, sendErr)
		} else {
			fmt.Printf("[ResetPassword] email sent successfully to %s\n", guest.Email)
		}
	}()

	return nil
}

func (s *GuestAuthService) UpdateProfile(ctx context.Context, id uuid.UUID, req *models.GuestUpdateRequest) (*models.Guest, error) {
	guest, err := s.guestRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("guest not found")
	}

	if req.FullName != "" {
		guest.FullName = req.FullName
	}
	if req.Phone != "" {
		guest.Phone = req.Phone
	}

	if err := s.guestRepo.Update(guest); err != nil {
		return nil, err
	}
	return s.guestRepo.GetByID(id)
}
