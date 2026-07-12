package services

import (
	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type RoleService struct {
	userRepo *repository.UserRepository
	roleRepo *repository.RoleRepository
}

func NewRoleService(userRepo *repository.UserRepository, roleRepo *repository.RoleRepository) *RoleService {
	return &RoleService{userRepo: userRepo, roleRepo: roleRepo}
}

func (s *RoleService) InitializePredefinedRoles() error {
	for _, r := range models.GetPredefinedRoles() {
		_, err := s.roleRepo.GetRoleByName(r.Name)
		if err != nil {
			if err := s.userRepo.CreateRole(&r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *RoleService) GetAllRoles(orgID uuid.UUID) ([]models.Role, error) {
	return s.roleRepo.GetAllRoles(orgID)
}
