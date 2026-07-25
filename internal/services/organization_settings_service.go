package services

import (
	"context"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type OrganizationSettingsService struct {
	repo *repository.OrganizationSettingsRepository
}

func NewOrganizationSettingsService(repo *repository.OrganizationSettingsRepository) *OrganizationSettingsService {
	return &OrganizationSettingsService{repo: repo}
}

func (s *OrganizationSettingsService) Get(ctx context.Context, orgID uuid.UUID) (*models.OrganizationSettings, error) {
	return s.repo.GetForOrg(orgID)
}

func (s *OrganizationSettingsService) Upsert(ctx context.Context, orgID uuid.UUID, req *models.UpdateOrganizationSettingsRequest) (*models.OrganizationSettings, error) {
	return s.repo.Upsert(orgID, req)
}
