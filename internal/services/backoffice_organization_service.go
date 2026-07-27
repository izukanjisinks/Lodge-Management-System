package services

import (
	"context"
	"errors"
	"fmt"

	"lodge-system/internal/database"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/utils/email"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type BackofficeOrganizationService struct {
	orgRepo      *repository.OrganizationRepository
	userRepo     *repository.UserRepository
	roleRepo     *repository.RoleRepository
	branchRepo   *repository.BranchRepository
	emailService *email.EmailService
}

func NewBackofficeOrganizationService(
	orgRepo *repository.OrganizationRepository,
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
	branchRepo *repository.BranchRepository,
) *BackofficeOrganizationService {
	return &BackofficeOrganizationService{
		orgRepo:    orgRepo,
		userRepo:   userRepo,
		roleRepo:   roleRepo,
		branchRepo: branchRepo,
	}
}

func (s *BackofficeOrganizationService) SetEmailService(svc *email.EmailService) {
	s.emailService = svc
}

func (s *BackofficeOrganizationService) List(ctx context.Context) ([]models.Organization, error) {
	return s.orgRepo.List()
}

func (s *BackofficeOrganizationService) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	return s.orgRepo.GetByID(id)
}

func (s *BackofficeOrganizationService) Update(ctx context.Context, id uuid.UUID, req models.OrgDetails) (*models.Organization, error) {
	org, err := s.orgRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("organization not found")
	}
	if req.Name != "" {
		org.Name = req.Name
	}
	if req.Email != "" {
		org.Email = req.Email
	}
	if req.Phone != "" {
		org.Phone = req.Phone
	}
	if req.StreetAddress != "" {
		org.StreetAddress = req.StreetAddress
	}
	if req.City != "" {
		org.City = req.City
	}
	if req.Country != "" {
		org.Country = req.Country
	}
	if req.Location != "" {
		org.Location = req.Location
	}
	if req.LogoURL != "" {
		org.LogoURL = req.LogoURL
	}
	if req.Parking != nil {
		org.Parking = *req.Parking
	}
	if req.Restaurant != nil {
		org.Restaurant = *req.Restaurant
	}
	if req.CheckInTime != nil {
		org.CheckInTime = req.CheckInTime
	}
	if req.CheckOutTime != nil {
		org.CheckOutTime = req.CheckOutTime
	}
	if err := s.orgRepo.Update(org); err != nil {
		return nil, err
	}
	return s.orgRepo.GetByID(id)
}

func (s *BackofficeOrganizationService) ToggleStatus(ctx context.Context, id uuid.UUID, isActive bool) (*models.Organization, error) {
	org, err := s.orgRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("organization not found")
	}
	org.IsActive = isActive
	if err := s.orgRepo.Update(org); err != nil {
		return nil, err
	}
	return s.orgRepo.GetByID(id)
}

func (s *BackofficeOrganizationService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.orgRepo.Delete(id)
}

// Provision creates an organization and its first admin user in a single transaction.
// The admin password is randomly generated and emailed to the admin.
func (s *BackofficeOrganizationService) Provision(ctx context.Context, req models.ProvisionOrgRequest) (*models.Organization, *models.User, error) {
	if req.Organization.Name == "" {
		return nil, nil, errors.New("organization name is required")
	}
	if req.Admin.FullName == "" || req.Admin.Email == "" {
		return nil, nil, errors.New("admin full_name and email are required")
	}

	adminRole, err := s.roleRepo.GetRoleByName(models.RoleAdmin)
	if err != nil {
		return nil, nil, fmt.Errorf("admin role not found: %w", err)
	}

	password, err := utils.GenerateRandomPassword()
	if err != nil {
		return nil, nil, err
	}

	hashed, err := utils.HashPassword(password)
	if err != nil {
		return nil, nil, err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	org := &models.Organization{
		Name:          req.Organization.Name,
		Email:         req.Organization.Email,
		Phone:         req.Organization.Phone,
		StreetAddress: req.Organization.StreetAddress,
		City:          req.Organization.City,
		Country:       req.Organization.Country,
		Location:      req.Organization.Location,
		LogoURL:       req.Organization.LogoURL,
	}
	if req.Organization.Parking != nil {
		org.Parking = *req.Organization.Parking
	}
	if req.Organization.Restaurant != nil {
		org.Restaurant = *req.Organization.Restaurant
	}
	if req.Organization.CheckInTime != nil {
		org.CheckInTime = req.Organization.CheckInTime
	}
	if req.Organization.CheckOutTime != nil {
		org.CheckOutTime = req.Organization.CheckOutTime
	}
	if err = s.orgRepo.CreateTx(tx, org); err != nil {
		return nil, nil, fmt.Errorf("failed to create organization: %w", err)
	}

	admin := &models.User{
		FullName:       req.Admin.FullName,
		Email:          req.Admin.Email,
		Password:       hashed,
		RoleID:         &adminRole.RoleID,
		Role:           adminRole,
		IsActive:       true,
		ChangePassword: true,
		OrgID:          &org.ID,
	}
	if err = s.userRepo.CreateTx(tx, admin); err != nil {
		return nil, nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}

	// Seed the main branch from the org's details
	mainBranch := &models.Branch{
		OrgID:        org.ID,
		Name:         org.Name,
		BranchCode:   "MAIN",
		IsActive:     true,
		IsMain:       true,
		Parking:      false,
		Restaurant:   false,
		CheckInTime:  func() *string { s := "14:00"; return &s }(),
		CheckOutTime: func() *string { s := "10:00"; return &s }(),
	}
	if org.StreetAddress != "" {
		mainBranch.StreetAddress = &org.StreetAddress
	}
	if org.City != "" {
		mainBranch.City = &org.City
	}
	if org.Country != "" {
		mainBranch.Country = &org.Country
	}
	if org.Location != "" {
		mainBranch.Location = &org.Location
	}
	if org.Phone != "" {
		mainBranch.Phone = &org.Phone
	}
	if org.Email != "" {
		mainBranch.Email = &org.Email
	}
	_ = s.branchRepo.Create(mainBranch)

	if s.emailService != nil {
		go func() {
			body := email.WelcomeUserTemplate(req.Admin.FullName, req.Admin.Email, password)
			if sendErr := s.emailService.SendEmail(
				[]string{req.Admin.Email},
				fmt.Sprintf("Your admin account for %s", req.Organization.Name),
				body,
			); sendErr != nil {
				fmt.Printf("warning: failed to send admin welcome email to %s: %v\n", req.Admin.Email, sendErr)
			}
		}()
	}

	return org, admin, nil
}
