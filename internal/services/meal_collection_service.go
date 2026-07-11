package services

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

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

func (s *MealCollectionService) CreateSession(orgID uuid.UUID, branchID *uuid.UUID, req *models.MealSessionCreateRequest) (*models.ResidentMealSession, error) {
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

func (s *MealCollectionService) GetSession(id, orgID uuid.UUID) (*models.ResidentMealSession, error) {
	sess, err := s.sessions.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("meal session not found")
	}
	return sess, nil
}

func (s *MealCollectionService) ListSessions(orgID uuid.UUID, branchID *uuid.UUID, status, mealPeriod string, page, pageSize int) (*models.PaginatedMealSessions, error) {
	data, total, err := s.sessions.List(orgID, branchID, status, mealPeriod, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &models.PaginatedMealSessions{Data: data, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *MealCollectionService) UpdateSession(id, orgID uuid.UUID, req *models.MealSessionUpdateRequest) (*models.ResidentMealSession, error) {
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

func (s *MealCollectionService) UpdateSessionStatus(id, orgID uuid.UUID, status string) (*models.ResidentMealSession, error) {
	if !validSessionStatus[status] {
		return nil, errors.New("invalid status")
	}
	if err := s.sessions.UpdateStatus(id, orgID, status); err != nil {
		return nil, notFoundOr(err, "meal session not found")
	}
	return s.sessions.GetByID(id, orgID)
}

func (s *MealCollectionService) UpdateGracePeriod(id, orgID uuid.UUID, minutes int) (*models.ResidentMealSession, error) {
	if minutes < 0 {
		return nil, errors.New("grace period must be >= 0")
	}
	if err := s.sessions.UpdateGracePeriod(id, orgID, minutes); err != nil {
		return nil, notFoundOr(err, "meal session not found")
	}
	return s.sessions.GetByID(id, orgID)
}

func (s *MealCollectionService) DeleteSession(id, orgID uuid.UUID) error {
	if err := s.sessions.Delete(id, orgID); err != nil {
		return notFoundOr(err, "meal session not found")
	}
	return nil
}

func (s *MealCollectionService) ListCollections(sessionID, orgID uuid.UUID) ([]models.MealCollectionLogEntry, error) {
	return s.collections.ListBySession(sessionID, orgID)
}

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

func (s *MealCollectionService) GetCurrentStay(orgID, roomID uuid.UUID) (*models.CurrentStay, error) {
	stay, err := s.collections.GetCurrentStay(orgID, roomID)
	if err != nil {
		return nil, err
	}
	if stay == nil {
		return nil, errors.New("no current stay for this room")
	}
	return stay, nil
}

// ─── Collect ──────────────────────────────────────────────────────────────────

// Collect resolves the scanned/typed input to a resident, gates on the session
// being open (within grace), posts a buffet charge to the room's booking invoice,
// and logs it. Idempotent on idempotency_key.
func (s *MealCollectionService) Collect(orgID, sessionID uuid.UUID, staffID uuid.UUID, staffName string, req *models.MealCollectRequest) (*models.MealCollectionResult, error) {
	if req.Input == "" {
		return nil, errors.New("input is required")
	}
	if req.IdempotencyKey == "" {
		return nil, errors.New("idempotency_key is required")
	}

	// Idempotency: a prior collection with this key means the same physical scan —
	// return its result without charging again.
	if prior, err := s.collections.FindByIdempotencyKey(orgID, req.IdempotencyKey); err == nil && prior != nil {
		return s.resultFromLog(prior), nil
	}

	session, err := s.sessions.GetByID(sessionID, orgID)
	if err != nil {
		return nil, errors.New("meal session not found")
	}
	if session.Status != models.MealSessionStatusOpen {
		return &models.MealCollectionResult{Result: models.CollectResultSessionClosed, Message: "This session is not open for collection."}, nil
	}

	// Resolve input → resident, and the billing target (room's current booking).
	method := models.CollectMethodTyped
	var cardUID string
	var resident *models.ResidentSummary

	if card, cardErr := s.cards.GetActiveByUID(req.Input, orgID); cardErr == nil && card != nil {
		method = models.CollectMethodCard
		cardUID = card.CardUID
		if card.Role == models.CardRoleRoomService {
			return &models.MealCollectionResult{Result: models.CollectResultNotPermitted, Message: "Room Service cards can't be used to collect meals."}, nil
		}
		stay, sErr := s.collections.ResolveRoomCurrentStay(orgID, card.RoomID)
		if sErr != nil {
			return nil, sErr
		}
		if stay == nil {
			return &models.MealCollectionResult{Result: models.CollectResultNotFound, Message: fmt.Sprintf("%s has no current guest — nothing to charge.", card.RoomName)}, nil
		}
		// Display the card holder's name if known, else the room's lead guest.
		if card.HolderName != "" {
			stay.FullName = card.HolderName
			if card.AttendeeID != nil {
				stay.AttendeeID = card.AttendeeID
			}
			if card.IdentificationCard != "" {
				stay.IdentificationCard = card.IdentificationCard
			}
		}
		if stay.RoomName == "" {
			stay.RoomName = card.RoomName
		}
		resident = stay
	} else {
		// Typed ID → current residents.
		matches, mErr := s.collections.ResolveByIDCard(orgID, req.Input)
		if mErr != nil {
			return nil, mErr
		}
		if len(matches) == 0 {
			return &models.MealCollectionResult{Result: models.CollectResultNotFound, Message: "No resident found for this card or ID."}, nil
		}
		if len(matches) > 1 {
			return &models.MealCollectionResult{Result: models.CollectResultAmbiguous, Candidates: matches, Message: fmt.Sprintf("%d residents share this ID — select the correct one.", len(matches))}, nil
		}
		resident = &matches[0]
	}

	return s.chargeAndLog(orgID, session, req, method, cardUID, resident, staffID, staffName)
}

func (s *MealCollectionService) chargeAndLog(orgID uuid.UUID, session *models.ResidentMealSession, req *models.MealCollectRequest, method, cardUID string, resident *models.ResidentSummary, staffID uuid.UUID, staffName string) (*models.MealCollectionResult, error) {
	// Buffet price = the linked menu item's current price.
	menuItem, err := s.menu.GetMenuItemByID(session.BuffetMenuItemID, orgID)
	if err != nil {
		return nil, errors.New("buffet menu item not found")
	}
	amount := menuItem.Price

	inv, err := s.invoice.GetByBookingID(resident.BookingID, orgID)
	if err != nil {
		return &models.MealCollectionResult{Result: models.CollectResultNotFound, Message: "No accommodation invoice found for this booking."}, nil
	}

	// Reserve the log row FIRST — the unique (org_id, idempotency_key) index makes
	// this the atomic dedupe point, so a concurrent/retried scan with the same key
	// can never produce a second charge. Only charge if we won the reservation.
	collectedAt := parseClientTime(req.ClientCollectedAt)
	bookingID := resident.BookingID
	logEntry := &models.MealCollectionLogEntry{
		MealSessionID:      session.ID,
		AttendeeID:         resident.AttendeeID,
		ResidentName:       resident.FullName,
		IdentificationCard: resident.IdentificationCard,
		Method:             method,
		CardUID:            cardUID,
		RoomName:           resident.RoomName,
		CollectedBy:        staffID.String(),
		CollectedByName:    staffName,
		CollectedAt:        collectedAt,
	}
	inserted, err := s.collections.Reserve(logEntry, orgID, &bookingID, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to log collection: %w", err)
	}
	if !inserted {
		// Lost the race — the key already exists. Return the prior result, no charge.
		if prior, pErr := s.collections.FindByIdempotencyKey(orgID, req.IdempotencyKey); pErr == nil && prior != nil {
			return s.resultFromLog(prior), nil
		}
		return &models.MealCollectionResult{Result: models.CollectResultMatched, Message: "Already collected (duplicate scan ignored)."}, nil
	}

	desc := fmt.Sprintf("%s — %s", menuItem.Name, session.MealPeriod)
	if resident.FullName != "" {
		desc = fmt.Sprintf("%s (%s)", desc, resident.FullName)
	}
	line := &models.InvoiceLineItem{
		BookingID:    &resident.BookingID,
		LineType:     models.LineTypeMeal,
		Description:  desc,
		Quantity:     1,
		UnitPrice:    amount,
		Total:        amount,
		AttendeeName: resident.FullName,
		ServiceType:  "buffet",
	}
	if err := s.invoice.AppendOrderLineItem(inv.ID, orgID, line); err != nil {
		return nil, fmt.Errorf("failed to post charge: %w", err)
	}
	// Backfill the charge details onto the reserved log row.
	_ = s.collections.SetCharge(logEntry.ID, orgID, amount, inv.ID, line.ID)

	return &models.MealCollectionResult{
		Result:   models.CollectResultMatched,
		Resident: resident,
		Charge:   &models.MealCollectionCharge{InvoiceID: inv.ID, InvoiceNumber: inv.InvoiceNumber, LineItemID: line.ID, Amount: amount},
		Message:  fmt.Sprintf("Charged ZMW %.2f to %s.", amount, roomOrBooking(resident)),
	}, nil
}

func (s *MealCollectionService) resultFromLog(e *models.MealCollectionLogEntry) *models.MealCollectionResult {
	res := &models.MealCollectionResult{Result: models.CollectResultMatched, Message: "Already collected (duplicate scan ignored)."}
	if e.InvoiceID != nil && e.Amount != nil {
		res.Charge = &models.MealCollectionCharge{InvoiceID: *e.InvoiceID, Amount: *e.Amount}
	}
	res.Resident = &models.ResidentSummary{FullName: e.ResidentName, IdentificationCard: e.IdentificationCard, RoomName: e.RoomName, AttendeeID: e.AttendeeID}
	return res
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func notFoundOr(err error, msg string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New(msg)
	}
	return err
}

func parseClientTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
}

func roomOrBooking(r *models.ResidentSummary) string {
	if r.RoomName != "" {
		return r.RoomName
	}
	return "booking"
}
