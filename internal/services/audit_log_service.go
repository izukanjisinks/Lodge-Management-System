package services

import (
	"context"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type AuditLogService struct {
	repo *repository.AuditLogRepository
}

func NewAuditLogService(repo *repository.AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

func (s *AuditLogService) List(ctx context.Context, orgID uuid.UUID, entityType, entityID, action string, from, to *time.Time, page, pageSize int) ([]models.AuditLog, int, error) {
	return s.repo.List(orgID, entityType, entityID, action, from, to, page, pageSize)
}
