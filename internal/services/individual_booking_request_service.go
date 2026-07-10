package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type IndividualBookingRequestService struct {
	requestRepo    *repository.IndividualBookingRequestRepository
	roomRepo       *repository.RoomRepository
	bookingService *BookingService
	workflow       *WorkflowService
}

func NewIndividualBookingRequestService(
	requestRepo *repository.IndividualBookingRequestRepository,
	roomRepo *repository.RoomRepository,
	bookingService *BookingService,
) *IndividualBookingRequestService {
	return &IndividualBookingRequestService{
		requestRepo:    requestRepo,
		roomRepo:       roomRepo,
		bookingService: bookingService,
	}
}

func (s *IndividualBookingRequestService) SetWorkflowService(svc *WorkflowService) {
	s.workflow = svc
}

// ─── Submission ───────────────────────────────────────────────────────────────

func (s *IndividualBookingRequestService) Submit(guestID uuid.UUID, req *models.SubmitIndividualBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" || req.BookedBy.Email == "" {
		return nil, errors.New("booked_by name and email are required")
	}
	if req.Accommodation == nil {
		return nil, errors.New("accommodation block is required")
	}
	if req.Accommodation.CheckIn == "" || req.Accommodation.CheckOut == "" {
		return nil, errors.New("check_in and check_out are required")
	}
	if len(req.Accommodation.Rooms) == 0 {
		return nil, errors.New("at least one room must be selected")
	}

	// Resolve org and branch: prefer the values from the body, otherwise fall back
	// to the first room (the entity being booked carries the authoritative branch).
	orgID := req.OrgID
	branchID := req.BranchID
	if orgID == uuid.Nil || branchID == nil {
		firstRoom, err := s.roomRepo.GetByIDUnscoped(req.Accommodation.Rooms[0].RoomID)
		if err != nil {
			return nil, errors.New("room not found")
		}
		if orgID == uuid.Nil && firstRoom.OrgID != nil {
			orgID = *firstRoom.OrgID
		}
		if branchID == nil {
			branchID = firstRoom.BranchID
		}
	}
	if orgID == uuid.Nil {
		return nil, errors.New("could not resolve org_id")
	}

	// Store the entire envelope as-is in JSONB
	payloadBytes, _ := json.Marshal(req)

	booking, err := s.bookingService.SubmitPending(&models.PendingBookingInput{
		OrgID:       orgID,
		BranchID:    branchID,
		WebUserID:   &guestID,
		BookerType:  models.BookerTypeIndividual,
		BookerName:  req.BookedBy.Name,
		BookerEmail: req.BookedBy.Email,
		BookerPhone: req.BookedBy.Phone,
		BookingType: models.BookingTypeRoom,
		Documents:   req.Documents,
		Metadata:    json.RawMessage(payloadBytes),
	})
	if err != nil {
		return nil, err
	}

	s.startWorkflow(booking, "Room")
	return booking, nil
}

// SubmitEvent stores a standalone individual event request (Flow B). It reuses the
// shared event envelope (no company/approver for individuals), persists the whole
// payload as JSONB with booking_type='event', and starts the approval workflow.
func (s *IndividualBookingRequestService) SubmitEvent(guestID uuid.UUID, req *models.SubmitEventBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" || req.BookedBy.Email == "" {
		return nil, errors.New("booked_by name and email are required")
	}
	if req.Event == nil || len(req.Event.Sessions) == 0 {
		return nil, errors.New("at least one event session is required")
	}
	for i, sess := range req.Event.Sessions {
		if sess.VenueID == "" {
			return nil, fmt.Errorf("session %d is missing a venue", i+1)
		}
		if _, err := uuid.Parse(sess.VenueID); err != nil {
			return nil, fmt.Errorf("session %d has an invalid venue", i+1)
		}
	}

	orgID := req.OrgID
	if orgID == uuid.Nil {
		return nil, errors.New("org_id is required")
	}

	payloadBytes, _ := json.Marshal(req)

	booking, err := s.bookingService.SubmitPending(&models.PendingBookingInput{
		OrgID:       orgID,
		BranchID:    req.BranchID,
		WebUserID:   &guestID,
		BookerType:  models.BookerTypeIndividual,
		BookerName:  req.BookedBy.Name,
		BookerEmail: req.BookedBy.Email,
		BookerPhone: req.BookedBy.Phone,
		BookingType: models.BookingTypeEvent,
		Documents:   req.Documents,
		Metadata:    json.RawMessage(payloadBytes),
	})
	if err != nil {
		return nil, err
	}

	s.startWorkflow(booking, "Event")
	return booking, nil
}

// SubmitMeal stores a standalone individual meal request (Flow B). The full
// envelope is persisted as JSONB with booking_type='meals' and the approval
// workflow is started immediately.
func (s *IndividualBookingRequestService) SubmitMeal(guestID uuid.UUID, req *models.SubmitMealBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" || req.BookedBy.Email == "" {
		return nil, errors.New("booked_by name and email are required")
	}
	if req.Meal == nil || len(req.Meal.Sessions) == 0 {
		return nil, errors.New("at least one meal session is required")
	}
	if req.OrgID == uuid.Nil {
		return nil, errors.New("org_id is required")
	}

	payloadBytes, _ := json.Marshal(req)

	booking, err := s.bookingService.SubmitPending(&models.PendingBookingInput{
		OrgID:       req.OrgID,
		BranchID:    req.BranchID,
		WebUserID:   &guestID,
		BookerType:  models.BookerTypeIndividual,
		BookerName:  req.BookedBy.Name,
		BookerEmail: req.BookedBy.Email,
		BookerPhone: req.BookedBy.Phone,
		BookingType: models.BookingTypeMeals,
		Documents:   req.Documents,
		Metadata:    json.RawMessage(payloadBytes),
	})
	if err != nil {
		return nil, err
	}

	s.startWorkflow(booking, "Meal")
	return booking, nil
}

// WalkInMeal creates a confirmed individual meal booking directly (no pending state,
// no approval workflow) — for a walk-in guest booking in person. Materialises orders
// immediately via CreateIndividualMeal with no web user attached.
func (s *IndividualBookingRequestService) WalkInMeal(orgID uuid.UUID, req *models.SubmitMealBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name is required")
	}
	if req.Meal == nil || len(req.Meal.Sessions) == 0 {
		return nil, errors.New("at least one meal session is required")
	}
	req.OrgID = orgID
	metadata, _ := json.Marshal(req)
	return s.bookingService.CreateIndividualMeal(orgID, nil, nil, req, metadata)
}

// WalkInEvent creates a confirmed individual event booking directly (no pending
// state, no approval workflow). Materialises event sessions immediately.
func (s *IndividualBookingRequestService) WalkInEvent(orgID uuid.UUID, req *models.SubmitEventBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name is required")
	}
	if req.Event == nil || len(req.Event.Sessions) == 0 {
		return nil, errors.New("at least one event session is required")
	}
	req.OrgID = orgID
	metadata, _ := json.Marshal(req)
	return s.bookingService.CreateIndividualEvent(orgID, nil, nil, req, metadata)
}

// WalkInAccommodation creates a confirmed individual accommodation booking directly
// (no pending state, no approval) with one attendee + room assignment per room slot.
// Supports multiple named guests, each assigned their own room.
func (s *IndividualBookingRequestService) WalkInAccommodation(orgID uuid.UUID, branchID *uuid.UUID, req *models.SubmitIndividualBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name is required")
	}
	if req.Accommodation == nil || len(req.Accommodation.Rooms) == 0 {
		return nil, errors.New("at least one room is required")
	}
	req.OrgID = orgID
	if branchID != nil {
		req.BranchID = branchID
	}
	metadata, _ := json.Marshal(req)
	return s.bookingService.CreateIndividualRooms(
		orgID, branchID, uuid.Nil, nil,
		req.BookedBy.Name, req.BookedBy.Email, req.BookedBy.Phone,
		req, metadata,
	)
}

// ─── Web user ─────────────────────────────────────────────────────────────────

// GetForWebUser returns a single booking owned by the web user, shaped as a request.
func (s *IndividualBookingRequestService) GetForWebUser(id, webUserID uuid.UUID) (*models.IndividualBookingRequest, error) {
	b, err := s.bookingService.GetByIDUnscoped(id)
	if err != nil || b.WebUserID == nil || *b.WebUserID != webUserID {
		return nil, errors.New("booking not found")
	}
	return bookingToIndividualRequest(b), nil
}

// ListForWebUser returns the web user's bookings shaped as requests. Retained for the
// legacy /web/bookings endpoint; the website now reads /web/my-bookings directly.
func (s *IndividualBookingRequestService) ListForWebUser(webUserID uuid.UUID, page, pageSize int) ([]models.IndividualBookingRequest, int, error) {
	bookings, total, err := s.bookingService.ListForWebUser(webUserID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.IndividualBookingRequest, 0, len(bookings))
	for i := range bookings {
		out = append(out, *bookingToIndividualRequest(&bookings[i]))
	}
	return out, total, nil
}

// CancelForWebUser cancels a pending booking the web user owns (single-phase: the id
// is a booking ID) and cancels the associated workflow instance.
func (s *IndividualBookingRequestService) CancelForWebUser(id, webUserID uuid.UUID) error {
	orgID, err := s.bookingService.CancelForWebUser(id, webUserID)
	if err != nil {
		return err
	}
	if s.workflow != nil {
		_ = s.workflow.CancelInstance(id.String(), orgID.String())
	}
	return nil
}

// ─── Backoffice ───────────────────────────────────────────────────────────────

// GetByID returns a pending individual booking shaped as an IndividualBookingRequest
// so the back-office task screen (which reads .payload/.booking_type) keeps working.
// Single-phase: the id is a booking ID and payload comes from metadata.
func (s *IndividualBookingRequestService) GetByID(id, orgID uuid.UUID) (*models.IndividualBookingRequest, error) {
	b, err := s.bookingService.GetForApproval(id, orgID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	return bookingToIndividualRequest(b), nil
}

// List returns pending individual bookings shaped as requests for the back-office
// "booking requests" screen.
func (s *IndividualBookingRequestService) List(orgID uuid.UUID, status string, page, pageSize int) ([]models.IndividualBookingRequest, int, error) {
	bookings, total, err := s.bookingService.List(orgID, models.BookerTypeIndividual, "", models.BookingStatusPending, nil, nil, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.IndividualBookingRequest, 0, len(bookings))
	for i := range bookings {
		out = append(out, *bookingToIndividualRequest(&bookings[i]))
	}
	return out, total, nil
}

// bookingToIndividualRequest maps a booking back into the legacy request shape.
func bookingToIndividualRequest(b *models.Booking) *models.IndividualBookingRequest {
	return &models.IndividualBookingRequest{
		ID:          b.ID,
		OrgID:       b.OrgID,
		WebUserID:   b.WebUserID,
		BookerName:  b.BookerName,
		BookerEmail: b.BookerEmail,
		BookerPhone: b.BookerPhone,
		BookingType: b.BookingType,
		Status:      requestStatusFromBooking(b.Status),
		Documents:   b.Documents,
		Payload:     b.Metadata,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}

// ApproveFromWorkflow adapts Approve to the workflow's BookingRequestApprover
// interface — it materialises the booking but discards the returned record, since
// the workflow only needs the success/failure signal.
func (s *IndividualBookingRequestService) ApproveFromWorkflow(id, orgID uuid.UUID) error {
	_, err := s.Approve(id, orgID)
	return err
}

// RejectFromWorkflow adapts Reject to the workflow's BookingRequestApprover interface.
func (s *IndividualBookingRequestService) RejectFromWorkflow(id, orgID uuid.UUID) error {
	return s.Reject(id, orgID)
}

// Approve promotes a pending individual booking (id is the booking ID) into a
// confirmed booking, materialising children from the envelope stored in the
// booking's metadata. The pending booking row is promoted in place — no new row.
func (s *IndividualBookingRequestService) Approve(id, orgID uuid.UUID) (*models.Booking, error) {
	b, err := s.bookingService.GetForApproval(id, orgID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	if b.Status != models.BookingStatusPending {
		return nil, errors.New("only pending bookings can be approved")
	}
	if b.WebUserID == nil {
		b.WebUserID = nil // explicit: pending bookings from staff have none
	}

	// Meal bookings materialise into orders (one per session).
	if b.BookingType == models.BookingTypeMeals {
		var envelope models.SubmitMealBookingRequest
		if jsonErr := json.Unmarshal(b.Metadata, &envelope); jsonErr != nil || envelope.Meal == nil {
			return nil, errors.New("invalid meal booking payload")
		}
		return s.bookingService.CreateIndividualMeal(orgID, b.WebUserID, &id, &envelope, b.Metadata)
	}

	// Event bookings materialise via the shared multi-session engine.
	if b.BookingType == models.BookingTypeEvent {
		var envelope models.SubmitEventBookingRequest
		if jsonErr := json.Unmarshal(b.Metadata, &envelope); jsonErr != nil || envelope.Event == nil {
			return nil, errors.New("invalid event booking payload")
		}
		return s.bookingService.CreateIndividualEvent(orgID, b.WebUserID, &id, &envelope, b.Metadata)
	}

	// Accommodation: decode the stored envelope (new shape first, legacy fallback).
	var newPayload models.SubmitIndividualBookingRequest
	if jsonErr := json.Unmarshal(b.Metadata, &newPayload); jsonErr == nil && newPayload.Accommodation != nil {
		if len(newPayload.Accommodation.Rooms) == 0 {
			return nil, errors.New("no rooms in accommodation block")
		}

		// Resolve branch from the first room when not set on the booking.
		branchID := b.BranchID
		if branchID == nil {
			firstRoom, roomErr := s.roomRepo.GetByIDUnscoped(newPayload.Accommodation.Rooms[0].RoomID)
			if roomErr != nil {
				return nil, errors.New("room not found")
			}
			branchID = firstRoom.BranchID
		}

		return s.bookingService.CreateIndividualRooms(
			orgID,
			branchID,
			id,
			b.WebUserID,
			b.BookerName, b.BookerEmail, b.BookerPhone,
			&newPayload,
			b.Metadata,
		)
	}

	// Legacy single-room payload shape.
	var legacy models.IndividualBookingPayload
	if jsonErr := json.Unmarshal(b.Metadata, &legacy); jsonErr != nil {
		return nil, errors.New("invalid booking payload")
	}

	room, err := s.roomRepo.GetByIDUnscoped(legacy.RoomID)
	if err != nil {
		return nil, errors.New("room not found")
	}

	checkIn := models.DateOnly{}
	if err := checkIn.UnmarshalJSON([]byte(`"` + legacy.CheckIn + `"`)); err != nil {
		return nil, fmt.Errorf("invalid check_in date: %w", err)
	}
	checkOut := models.DateOnly{}
	if err := checkOut.UnmarshalJSON([]byte(`"` + legacy.CheckOut + `"`)); err != nil {
		return nil, fmt.Errorf("invalid check_out date: %w", err)
	}

	nights := int(checkOut.Sub(checkIn.Time).Hours() / 24)
	meta := b.Metadata
	var mMap map[string]interface{}
	if json.Unmarshal(b.Metadata, &mMap) == nil {
		mMap["room_name"] = room.Name
		mMap["room_type"] = room.Type
		mMap["nights"] = nights
		if enriched, jsonErr := json.Marshal(mMap); jsonErr == nil {
			meta = enriched
		}
	}

	bookingReq := &models.CreateIndividualBookingRequest{
		WebUserID:   b.WebUserID,
		BookerName:  b.BookerName,
		BookerEmail: b.BookerEmail,
		BookerPhone: b.BookerPhone,
		RoomID:      legacy.RoomID,
		CheckIn:     checkIn,
		CheckOut:    checkOut,
		Metadata:    meta,
		PromoteID:   &id,
	}

	return s.bookingService.CreateIndividual(orgID, room.BranchID, bookingReq, bookingReq.Metadata)
}

// Reject marks a pending individual booking as rejected.
func (s *IndividualBookingRequestService) Reject(id, orgID uuid.UUID) error {
	return s.bookingService.SetStatus(id, orgID, models.BookingStatusRejected, models.BookingStatusPending)
}

// Cancel marks a pending individual booking as cancelled.
func (s *IndividualBookingRequestService) Cancel(id, orgID uuid.UUID) error {
	return s.bookingService.SetStatus(id, orgID, models.BookingStatusCancelled, models.BookingStatusPending)
}

// ─── Metadata builders ────────────────────────────────────────────────────────


// ─── Workflow ─────────────────────────────────────────────────────────────────

// startWorkflow kicks off the booking-approval workflow for a pending booking. The
// workflow's TaskID is the booking ID itself (single-phase): a terminal approve/reject
// promotes or rejects that same booking row.
func (s *IndividualBookingRequestService) startWorkflow(b *models.Booking, label string) {
	if s.workflow == nil {
		return
	}
	go func() {
		taskDetails := models.TaskDetails{
			TaskID:          b.ID.String(),
			TaskRef:         b.ID.String()[:8],
			TaskType:        "individual_booking",
			TaskDescription: fmt.Sprintf("%s booking request from %s", label, b.BookerName),
			SenderDetails: models.SenderDetails{
				SenderID:   b.ID.String(),
				SenderName: b.BookerName,
				Position:   "Guest",
				Department: "Guest",
			},
		}
		if b.BranchID != nil {
			taskDetails.BranchID = b.BranchID.String()
		}
		if _, err := s.workflow.InitiateWorkflow(
			models.WorkflowTypeBookingApproval,
			taskDetails,
			b.ID.String(),
			"medium",
			nil,
			b.OrgID.String(),
		); err != nil {
			_ = err
		}
	}()
}
