package services

import (
	"errors"

	"lodge-system/internal/models"

	"github.com/google/uuid"
)

// ─── Cards ────────────────────────────────────────────────────────────────────

var validCardRoles = map[string]bool{models.CardRoleResident: true, models.CardRoleRoomService: true}

func (s *MealCollectionService) AssignCard(orgID uuid.UUID, branchID *uuid.UUID, req *models.AssignCardRequest) (*models.MealCard, error) {
	if req.CardUID == "" {
		return nil, errors.New("card_uid is required")
	}
	if req.RoomID == uuid.Nil {
		return nil, errors.New("room_id is required")
	}
	role := req.Role
	if role == "" {
		role = models.CardRoleResident
	}
	if !validCardRoles[role] {
		return nil, errors.New("invalid card role")
	}
	// Reject a duplicate active UID up front for a clean error (the unique index is
	// the hard guard).
	if existing, _ := s.cards.GetActiveByUID(req.CardUID, orgID); existing != nil {
		return nil, errors.New("a card with this UID is already active")
	}
	card := &models.MealCard{CardUID: req.CardUID, RoomID: req.RoomID, Role: role, AttendeeID: req.AttendeeID}
	if req.AttendeeID != nil {
		if att, err := s.attendee.GetByIDUnscoped(*req.AttendeeID); err == nil {
			card.HolderName = att.FullName
			card.IdentificationCard = att.IdentificationCard
		}
	}
	if err := s.cards.Create(card, orgID, branchID); err != nil {
		return nil, err
	}
	return s.cards.GetByID(card.ID, orgID)
}

func (s *MealCollectionService) ListCards(orgID uuid.UUID, roomID *uuid.UUID, status string) ([]models.MealCard, error) {
	return s.cards.List(orgID, roomID, status)
}

func (s *MealCollectionService) UpdateCard(id, orgID uuid.UUID, req *models.UpdateCardRequest) (*models.MealCard, error) {
	if req.Role != nil && !validCardRoles[*req.Role] {
		return nil, errors.New("invalid card role")
	}
	if req.Status != nil && *req.Status != models.MealCardStatusActive && *req.Status != models.MealCardStatusInactive {
		return nil, errors.New("status patch may only set active or inactive")
	}
	if err := s.cards.Update(id, orgID, req); err != nil {
		return nil, notFoundOr(err, "card not found")
	}
	// If a new occupant was set, denormalize their name/ID onto the card.
	if !req.ClearAttendee && req.AttendeeID != nil {
		if att, err := s.attendee.GetByIDUnscoped(*req.AttendeeID); err == nil {
			_ = s.cards.SetHolder(id, orgID, att.ID, att.FullName, att.IdentificationCard)
		}
	}
	return s.cards.GetByID(id, orgID)
}

func (s *MealCollectionService) ReplaceCard(id, orgID uuid.UUID, req *models.ReplaceCardRequest) (*models.MealCard, error) {
	if req.NewCardUID == "" {
		return nil, errors.New("new_card_uid is required")
	}
	if existing, _ := s.cards.GetActiveByUID(req.NewCardUID, orgID); existing != nil {
		return nil, errors.New("a card with this UID is already active")
	}
	card, err := s.cards.Replace(id, orgID, req.NewCardUID)
	if err != nil {
		return nil, notFoundOr(err, "card not found")
	}
	return card, nil
}

func (s *MealCollectionService) VoidCard(id, orgID uuid.UUID) (*models.MealCard, error) {
	if err := s.cards.Void(id, orgID); err != nil {
		return nil, notFoundOr(err, "card not found")
	}
	return s.cards.GetByID(id, orgID)
}
