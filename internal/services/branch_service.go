package services

import (
	"context"
	"errors"
	"strings"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type BranchService struct {
	repo *repository.BranchRepository
}

func NewBranchService(repo *repository.BranchRepository) *BranchService {
	return &BranchService{repo: repo}
}

func (s *BranchService) Create(ctx context.Context, orgID uuid.UUID, req *models.CreateBranchRequest) (*models.Branch, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("branch name is required")
	}
	if strings.TrimSpace(req.BranchCode) == "" {
		return nil, errors.New("branch_code is required")
	}
	b := &models.Branch{
		OrgID:      orgID,
		Name:       strings.TrimSpace(req.Name),
		BranchCode: strings.ToUpper(strings.TrimSpace(req.BranchCode)),
		IsActive:   true,
		IsMain:     false,
	}
	if v := strings.TrimSpace(req.StreetAddress); v != "" {
		b.StreetAddress = &v
	}
	if v := strings.TrimSpace(req.City); v != "" {
		b.City = &v
	}
	if v := strings.TrimSpace(req.Country); v != "" {
		b.Country = &v
	}
	if v := strings.TrimSpace(req.Location); v != "" {
		b.Location = &v
	}
	if v := strings.TrimSpace(req.Phone); v != "" {
		b.Phone = &v
	}
	if v := strings.TrimSpace(req.Email); v != "" {
		b.Email = &v
	}
	b.Parking = req.Parking
	b.Restaurant = req.Restaurant
	if req.CheckInTime != nil {
		b.CheckInTime = req.CheckInTime
	}
	if req.CheckOutTime != nil {
		b.CheckOutTime = req.CheckOutTime
	}
	if err := s.repo.Create(b); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, errors.New("branch_code already exists for this organization")
		}
		return nil, err
	}
	return b, nil
}

func (s *BranchService) GetByID(ctx context.Context, id, orgID uuid.UUID) (*models.Branch, error) {
	b, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("branch not found")
	}
	return b, nil
}

func (s *BranchService) List(ctx context.Context, orgID uuid.UUID) ([]models.Branch, error) {
	return s.repo.List(orgID)
}

func (s *BranchService) Update(ctx context.Context, id, orgID uuid.UUID, req *models.UpdateBranchRequest) (*models.Branch, error) {
	b, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("branch not found")
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, errors.New("branch name cannot be empty")
		}
		b.Name = strings.TrimSpace(*req.Name)
	}
	if req.BranchCode != nil {
		if strings.TrimSpace(*req.BranchCode) == "" {
			return nil, errors.New("branch_code cannot be empty")
		}
		b.BranchCode = strings.ToUpper(strings.TrimSpace(*req.BranchCode))
	}
	if req.StreetAddress != nil {
		v := strings.TrimSpace(*req.StreetAddress)
		b.StreetAddress = &v
	}
	if req.City != nil {
		v := strings.TrimSpace(*req.City)
		b.City = &v
	}
	if req.Country != nil {
		v := strings.TrimSpace(*req.Country)
		b.Country = &v
	}
	if req.Location != nil {
		v := strings.TrimSpace(*req.Location)
		b.Location = &v
	}
	if req.Phone != nil {
		v := strings.TrimSpace(*req.Phone)
		b.Phone = &v
	}
	if req.Email != nil {
		v := strings.TrimSpace(*req.Email)
		b.Email = &v
	}
	if req.IsActive != nil {
		b.IsActive = *req.IsActive
	}
	if req.IsMain != nil {
		b.IsMain = *req.IsMain
	}
	if req.Parking != nil {
		b.Parking = *req.Parking
	}
	if req.Restaurant != nil {
		b.Restaurant = *req.Restaurant
	}
	if req.CheckInTime != nil {
		b.CheckInTime = req.CheckInTime
	}
	if req.CheckOutTime != nil {
		b.CheckOutTime = req.CheckOutTime
	}
	if err := s.repo.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *BranchService) Delete(ctx context.Context, id, orgID uuid.UUID) error {
	return s.repo.Delete(id, orgID)
}
