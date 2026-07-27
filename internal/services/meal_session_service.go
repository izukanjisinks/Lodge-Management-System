package services

import (
	"context"
	"errors"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

// MealCollectionService covers resident meal collection: recurring buffet
// sessions, RFID card assignments, and the collect (scan/typed-ID → charge)
// flow. Methods are split across meal_session_service.go, meal_card_service.go,
// and meal_collect_service.go, grouped the same way as the underlying repos.
type MealCollectionService struct {
	sessions    *repository.MealSessionRepository
	cards       *repository.MealCardRepository
	collections *repository.MealCollectionRepository
	invoice     *repository.InvoiceRepository
	menu        *repository.MenuRepository
	attendee    *repository.BookingAttendeeRepository
}

func NewMealCollectionService(
	sessions *repository.MealSessionRepository,
	cards *repository.MealCardRepository,
	collections *repository.MealCollectionRepository,
	invoice *repository.InvoiceRepository,
	menu *repository.MenuRepository,
	attendee *repository.BookingAttendeeRepository,
) *MealCollectionService {
	return &MealCollectionService{sessions: sessions, cards: cards, collections: collections, invoice: invoice, menu: menu, attendee: attendee}
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

var validMealPeriods = map[string]bool{"breakfast": true, "brunch": true, "lunch": true, "dinner": true, "supper": true}
var validSessionStatus = map[string]bool{
	models.MealSessionStatusScheduled: true, models.MealSessionStatusOpen: true,
	models.MealSessionStatusClosed: true, models.MealSessionStatusCancelled: true,
}

func (s *MealCollectionService) CreateSession(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.MealSessionCreateRequest) (*models.ResidentMealSession, error) {
	if !validMealPeriods[req.MealPeriod] {
		return nil, errors.New("invalid meal period")
	}
	if req.BuffetMenuItemID == uuid.Nil {
		return nil, errors.New("buffet_menu_item_id is required")
	}
	if _, err := s.menu.GetMenuItemByID(req.BuffetMenuItemID, orgID); err != nil {
		return nil, errors.New("buffet menu item not found")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, errors.New("start_time and end_time are required")
	}
	session := &models.ResidentMealSession{
		MealPeriod:         req.MealPeriod,
		BuffetMenuItemID:   req.BuffetMenuItemID,
		StartTime:          req.StartTime,
		EndTime:            req.EndTime,
		DaysOfWeek:         req.DaysOfWeek,
		AutoOpenClose:      req.AutoOpenClose,
		Status:             models.MealSessionStatusScheduled,
		GracePeriodMinutes: 15,
	}
	if session.DaysOfWeek == nil {
		session.DaysOfWeek = []string{}
	}
	if err := s.sessions.Create(session, orgID, branchID); err != nil {
		return nil, err
	}
	return s.sessions.GetByID(session.ID, orgID)
}

func (s *MealCollectionService) GetSession(ctx context.Context, id, orgID uuid.UUID) (*models.ResidentMealSession, error) {
	sess, err := s.sessions.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("meal session not found")
	}
	return sess, nil
}

func (s *MealCollectionService) ListSessions(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status, mealPeriod string, page, pageSize int) (*models.PaginatedMealSessions, error) {
	data, total, err := s.sessions.List(orgID, branchID, status, mealPeriod, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &models.PaginatedMealSessions{Data: data, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *MealCollectionService) UpdateSession(ctx context.Context, id, orgID uuid.UUID, req *models.MealSessionUpdateRequest) (*models.ResidentMealSession, error) {
	if req.MealPeriod != nil && !validMealPeriods[*req.MealPeriod] {
		return nil, errors.New("invalid meal period")
	}
	if req.BuffetMenuItemID != nil {
		if _, err := s.menu.GetMenuItemByID(*req.BuffetMenuItemID, orgID); err != nil {
			return nil, errors.New("buffet menu item not found")
		}
	}
	if err := s.sessions.Update(id, orgID, req); err != nil {
		return nil, notFoundOr(err, "meal session not found")
	}
	return s.sessions.GetByID(id, orgID)
}

func (s *MealCollectionService) UpdateSessionStatus(ctx context.Context, id, orgID uuid.UUID, status string) (*models.ResidentMealSession, error) {
	if !validSessionStatus[status] {
		return nil, errors.New("invalid status")
	}
	if err := s.sessions.UpdateStatus(id, orgID, status); err != nil {
		return nil, notFoundOr(err, "meal session not found")
	}
	return s.sessions.GetByID(id, orgID)
}

func (s *MealCollectionService) UpdateGracePeriod(ctx context.Context, id, orgID uuid.UUID, minutes int) (*models.ResidentMealSession, error) {
	if minutes < 0 {
		return nil, errors.New("grace period must be >= 0")
	}
	if err := s.sessions.UpdateGracePeriod(id, orgID, minutes); err != nil {
		return nil, notFoundOr(err, "meal session not found")
	}
	return s.sessions.GetByID(id, orgID)
}

func (s *MealCollectionService) DeleteSession(ctx context.Context, id, orgID uuid.UUID) error {
	if err := s.sessions.Delete(id, orgID); err != nil {
		return notFoundOr(err, "meal session not found")
	}
	return nil
}

func (s *MealCollectionService) ListCollections(ctx context.Context, sessionID, orgID uuid.UUID) ([]models.MealCollectionLogEntry, error) {
	return s.collections.ListBySession(sessionID, orgID)
}
