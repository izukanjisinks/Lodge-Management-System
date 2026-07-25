package services

import (
	"context"
	"errors"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

// CorProfileService manages the corporate profile chain:
// cor_company_details → cor_branch_details → cor_profiles → corporate_guests
type CorProfileService struct {
	companyRepo *repository.CorCompanyRepository
	branchRepo  *repository.CorBranchRepository
	profileRepo *repository.CorProfileRepository
	guestRepo   *repository.CorporateGuestRepository
}

func NewCorProfileService(
	companyRepo *repository.CorCompanyRepository,
	branchRepo *repository.CorBranchRepository,
	profileRepo *repository.CorProfileRepository,
	guestRepo *repository.CorporateGuestRepository,
) *CorProfileService {
	return &CorProfileService{
		companyRepo: companyRepo,
		branchRepo:  branchRepo,
		profileRepo: profileRepo,
		guestRepo:   guestRepo,
	}
}

// ─── Company ──────────────────────────────────────────────────────────────────

func (s *CorProfileService) GetCompany(ctx context.Context, id, orgID uuid.UUID) (*models.CorCompanyDetails, error) {
	return s.companyRepo.GetByID(id, orgID)
}

func (s *CorProfileService) ListCompanies(ctx context.Context, orgID uuid.UUID, page, pageSize int) ([]models.CorCompanyDetails, int, error) {
	return s.companyRepo.List(orgID, page, pageSize)
}

func (s *CorProfileService) UpdateCompany(ctx context.Context, id, orgID uuid.UUID, req *models.UpdateCorCompanyRequest) (*models.CorCompanyDetails, error) {
	return s.companyRepo.Update(id, orgID, req)
}

// ─── Branch ───────────────────────────────────────────────────────────────────

func (s *CorProfileService) GetBranch(ctx context.Context, id, companyID uuid.UUID) (*models.CorBranchDetails, error) {
	return s.branchRepo.GetByID(id, companyID)
}

func (s *CorProfileService) ListBranches(ctx context.Context, companyID uuid.UUID) ([]models.CorBranchDetails, error) {
	return s.branchRepo.List(companyID)
}

func (s *CorProfileService) CreateBranch(ctx context.Context, companyID uuid.UUID, req *models.CreateCorBranchRequest) (*models.CorBranchDetails, error) {
	if req.Name == "" {
		return nil, errors.New("branch name is required")
	}
	input := &models.CorBookingBranchInput{
		Name:    req.Name,
		Address: req.Address,
		Phone:   req.Phone,
	}
	return s.branchRepo.GetOrCreate(companyID, input)
}

func (s *CorProfileService) UpdateBranch(ctx context.Context, id, companyID uuid.UUID, req *models.UpdateCorBranchRequest) (*models.CorBranchDetails, error) {
	return s.branchRepo.Update(id, companyID, req)
}

// ─── Profile ──────────────────────────────────────────────────────────────────

func (s *CorProfileService) GetProfile(ctx context.Context, id, orgID uuid.UUID) (*models.CorProfile, error) {
	return s.profileRepo.GetByID(id, orgID)
}

func (s *CorProfileService) ListProfiles(ctx context.Context, companyID uuid.UUID, page, pageSize int) ([]models.CorProfile, int, error) {
	return s.profileRepo.List(companyID, page, pageSize)
}

func (s *CorProfileService) CreateProfile(ctx context.Context, orgID uuid.UUID, req *models.CreateCorProfileRequest) (*models.CorProfile, error) {
	if req.FirstName == "" || req.LastName == "" {
		return nil, errors.New("first_name and last_name are required")
	}
	input := models.CorBookingProfileInput{
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Email:      req.Email,
		Phone:      req.Phone,
		JobTitle:   req.JobTitle,
		Department: req.Department,
	}
	return s.profileRepo.GetOrCreate(orgID, req.CompanyID, req.BranchID, input)
}

func (s *CorProfileService) UpdateProfile(ctx context.Context, id, orgID uuid.UUID, req *models.UpdateCorProfileRequest) (*models.CorProfile, error) {
	return s.profileRepo.Update(id, orgID, req)
}

// ─── Corporate Guests ─────────────────────────────────────────────────────────

func (s *CorProfileService) GetGuest(ctx context.Context, id, profileID uuid.UUID) (*models.CorporateGuest, error) {
	return s.guestRepo.GetByID(id, profileID)
}

func (s *CorProfileService) ListGuests(ctx context.Context, profileID uuid.UUID) ([]models.CorporateGuest, error) {
	return s.guestRepo.List(profileID)
}

func (s *CorProfileService) AddGuest(ctx context.Context, profileID uuid.UUID, req *models.CreateCorporateGuestRequest) (*models.CorporateGuest, error) {
	if req.FirstName == "" || req.LastName == "" || req.IdentificationCard == "" {
		return nil, errors.New("first_name, last_name, and identification_card are required")
	}
	input := models.CorBookingGuestInput{
		FirstName:          req.FirstName,
		LastName:           req.LastName,
		Phone:              req.Phone,
		Email:              req.Email,
		IdentificationCard: req.IdentificationCard,
	}
	return s.guestRepo.Create(profileID, input)
}

func (s *CorProfileService) UpdateGuest(ctx context.Context, id, profileID uuid.UUID, req *models.UpdateCorporateGuestRequest) (*models.CorporateGuest, error) {
	return s.guestRepo.Update(id, profileID, req)
}

func (s *CorProfileService) DeleteGuest(ctx context.Context, id, profileID uuid.UUID) error {
	return s.guestRepo.Delete(id, profileID)
}

// ─── Booking submission chain ─────────────────────────────────────────────────

// ResolveChain upserts company → branch → profile in sequence and returns their IDs.
// Called at the start of every corporate booking submission.
func (s *CorProfileService) ResolveChain(ctx context.Context, orgID uuid.UUID, company models.CorBookingCompanyInput, branch *models.CorBookingBranchInput, profile models.CorBookingProfileInput) (companyID uuid.UUID, branchID *uuid.UUID, profileID uuid.UUID, err error) {
	c, err := s.companyRepo.GetOrCreate(orgID, company)
	if err != nil {
		return uuid.Nil, nil, uuid.Nil, errors.New("failed to resolve company: " + err.Error())
	}

	var b *models.CorBranchDetails
	if branch != nil && branch.Name != "" {
		b, err = s.branchRepo.GetOrCreate(c.ID, branch)
		if err != nil {
			return uuid.Nil, nil, uuid.Nil, errors.New("failed to resolve branch: " + err.Error())
		}
	}

	var bID *uuid.UUID
	if b != nil {
		bID = &b.ID
	}

	p, err := s.profileRepo.GetOrCreate(orgID, c.ID, bID, profile)
	if err != nil {
		return uuid.Nil, nil, uuid.Nil, errors.New("failed to resolve profile: " + err.Error())
	}

	return c.ID, bID, p.ID, nil
}
