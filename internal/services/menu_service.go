package services

import (
	"context"
	"database/sql"
	"errors"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type MenuService struct {
	repo *repository.MenuRepository
}

func NewMenuService(repo *repository.MenuRepository) *MenuService {
	return &MenuService{repo: repo}
}

// ── Menu ──────────────────────────────────────────────────────────────────────

func (s *MenuService) GetMenu(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, category string, page, pageSize int) (*models.MenuResponse, error) {
	menu, err := s.repo.GetMenu(orgID, branchID)
	if err != nil {
		return nil, errors.New("menu not found")
	}
	// No menu configured anywhere — return an empty menu rather than failing so the
	// UI can render a "no menu yet" state instead of an error.
	if menu == nil {
		return &models.MenuResponse{
			Items: models.MenuItemsPage{
				Data:     []models.MenuItem{},
				Page:     page,
				PageSize: pageSize,
				Total:    0,
			},
		}, nil
	}
	return s.buildResponse(menu, orgID, category, page, pageSize)
}

func (s *MenuService) UpsertMenu(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.UpdateMenuRequest, category string, page, pageSize int) (*models.MenuResponse, error) {
	menu, err := s.repo.UpsertMenu(orgID, branchID, req)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(menu, orgID, category, page, pageSize)
}

func (s *MenuService) buildResponse(menu *models.Menu, orgID uuid.UUID, category string, page, pageSize int) (*models.MenuResponse, error) {
	items, total, err := s.repo.ListMenuItems(menu.ID, orgID, category, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &models.MenuResponse{
		Menu: *menu,
		Items: models.MenuItemsPage{
			Data:     items,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

// ── Menu Items ────────────────────────────────────────────────────────────────

func (s *MenuService) CreateMenuItem(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.CreateMenuItemRequest) (*models.MenuItem, error) {
	if req.Name == "" {
		return nil, errors.New("name is required")
	}
	if req.Price < 0 {
		return nil, errors.New("price must be >= 0")
	}
	menu, err := s.repo.GetMenu(orgID, branchID)
	if err != nil {
		return nil, errors.New("menu not found")
	}
	item := &models.MenuItem{
		Name:           req.Name,
		Description:    req.Description,
		Category:       req.Category,
		ProductionArea: req.ProductionArea,
		ImageURL:       req.ImageURL,
		Price:          req.Price,
		IsAvailable:    true,
	}
	// Only buffet items carry structured buffet_data; ignore it otherwise so the
	// DB CHECK constraint (buffet_data only for category = 'buffet') is satisfied.
	if req.Category == "buffet" {
		item.BuffetData = req.BuffetData
	}
	if err := s.repo.CreateMenuItem(item, menu.ID, orgID, branchID); err != nil {
		return nil, err
	}
	return s.repo.GetMenuItemByID(item.ID, orgID)
}

func (s *MenuService) GetMenuItemByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*models.MenuItem, error) {
	item, err := s.repo.GetMenuItemByID(id, orgID)
	if err != nil {
		return nil, errors.New("menu item not found")
	}
	return item, nil
}

func (s *MenuService) UpdateMenuItem(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req *models.UpdateMenuItemRequest) (*models.MenuItem, error) {
	if req.Price != nil && *req.Price < 0 {
		return nil, errors.New("price must be >= 0")
	}
	item, err := s.repo.UpdateMenuItem(id, orgID, req)
	if err != nil {
		return nil, errors.New("menu item not found")
	}
	return item, nil
}

func (s *MenuService) DeleteMenuItem(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	if err := s.repo.DeleteMenuItem(id, orgID); err != nil {
		return errors.New("menu item not found")
	}
	return nil
}

// ── Guest (public) ────────────────────────────────────────────────────────────

func (s *MenuService) GuestGetMenu(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, category string, page, pageSize int) (*models.MenuResponse, error) {
	menu, err := s.repo.GuestGetMenu(orgID, branchID)
	if errors.Is(err, sql.ErrNoRows) {
		// No menu configured for this lodge yet — return an empty menu rather
		// than 404 so the guest meals tab simply shows nothing to order.
		return &models.MenuResponse{
			Items: models.MenuItemsPage{Data: []models.MenuItem{}, Page: page, PageSize: pageSize, Total: 0},
		}, nil
	}
	if err != nil {
		return nil, errors.New("menu not found")
	}
	items, total, err := s.repo.ListAvailableMenuItems(menu.ID, category, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &models.MenuResponse{
		Menu: *menu,
		Items: models.MenuItemsPage{
			Data:     items,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}
