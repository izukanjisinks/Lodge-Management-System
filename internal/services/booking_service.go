package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type BookingService struct {
	bookingRepo    *repository.BookingRepository
	attendeeRepo   *repository.BookingAttendeeRepository
	assignmentRepo *repository.BookingRoomAssignmentRepository
	requestRepo    *repository.CorporateBookingRequestRepository
	guestRepo      *repository.CorporateGuestRepository
	eventRepo      *repository.BookingEventRepository
	venueRepo      *repository.VenueRepository
	orderRepo      *repository.OrderRepository
	clientRepo     *repository.ClientRepository
	invoiceSvc     *InvoiceService
}

func NewBookingService(
	bookingRepo *repository.BookingRepository,
	attendeeRepo *repository.BookingAttendeeRepository,
	assignmentRepo *repository.BookingRoomAssignmentRepository,
	requestRepo *repository.CorporateBookingRequestRepository,
	guestRepo *repository.CorporateGuestRepository,
	eventRepo *repository.BookingEventRepository,
	venueRepo *repository.VenueRepository,
) *BookingService {
	return &BookingService{
		bookingRepo:    bookingRepo,
		attendeeRepo:   attendeeRepo,
		assignmentRepo: assignmentRepo,
		requestRepo:    requestRepo,
		guestRepo:      guestRepo,
		eventRepo:      eventRepo,
		venueRepo:      venueRepo,
	}
}

// SetInvoiceService wires the invoice service so confirmed bookings auto-generate
// a draft invoice. Optional — if unset, invoice generation is skipped.
func (s *BookingService) SetInvoiceService(inv *InvoiceService) {
	s.invoiceSvc = inv
}

// SetOrderRepository wires the orders repo so approved meals requests materialise
// into orders (one per named guest, plus a buffet order for top-level items).
func (s *BookingService) SetOrderRepository(repo *repository.OrderRepository) {
	s.orderRepo = repo
}

// SetClientRepository wires the client repo so approved bookings populate the
// org-scoped individual client registry (deduped on ID/passport number).
// Optional — if unset, client materialisation is skipped.
func (s *BookingService) SetClientRepository(repo *repository.ClientRepository) {
	s.clientRepo = repo
}

// SubmitPending creates a parent-only booking in the 'pending' state from a customer
// submission. The full envelope is stored in metadata and decoded later at workflow
// approval to materialise children (rooms / event sessions / meal orders). No child
// rows or inventory are created here — a pending booking holds nothing until approved.
func (s *BookingService) SubmitPending(ctx context.Context, in *models.PendingBookingInput) (*models.Booking, error) {
	if in.BookerName == "" {
		return nil, errors.New("booker_name is required")
	}
	if in.BranchID == nil {
		return nil, errors.New("branch is required")
	}
	b := &models.Booking{
		OrgID:        in.OrgID,
		BranchID:     in.BranchID,
		BookingType:  in.BookingType,
		BookerType:   in.BookerType,
		BookerName:   in.BookerName,
		BookerEmail:  in.BookerEmail,
		BookerPhone:  in.BookerPhone,
		WebUserID:    in.WebUserID,
		CorProfileID: in.CorProfileID,
		CompanyID:    in.CompanyID,
		Status:       models.BookingStatusPending,
		Documents:    in.Documents,
		Metadata:     in.Metadata,
	}
	if err := s.bookingRepo.CreatePending(b); err != nil {
		return nil, err
	}
	return b, nil
}

// GetForApproval loads a booking by ID for the workflow approval/materialise paths.
func (s *BookingService) GetForApproval(ctx context.Context, id, orgID uuid.UUID) (*models.Booking, error) {
	return s.bookingRepo.GetByID(id, orgID)
}

// GetByIDUnscoped loads a booking by ID without org scoping (used by web-user reads
// that authorise via web_user_id ownership instead).
func (s *BookingService) GetByIDUnscoped(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	return s.bookingRepo.GetByIDUnscoped(id)
}

// CancelForWebUser cancels a pending booking owned by the given web user. Only the
// owner can cancel, and only while still pending. Returns the booking's org ID so the
// caller can cancel the associated workflow.
func (s *BookingService) CancelForWebUser(ctx context.Context, id, webUserID uuid.UUID) (uuid.UUID, error) {
	b, err := s.bookingRepo.GetByIDUnscoped(id)
	if err != nil {
		return uuid.Nil, errors.New("booking not found")
	}
	if b.WebUserID == nil || *b.WebUserID != webUserID {
		return uuid.Nil, errors.New("booking not found")
	}
	if b.Status != models.BookingStatusPending {
		return uuid.Nil, errors.New("only pending bookings can be cancelled")
	}
	if err := s.SetStatus(ctx, id, b.OrgID, models.BookingStatusCancelled, models.BookingStatusPending); err != nil {
		return uuid.Nil, err
	}
	return b.OrgID, nil
}

// SetStatus transitions a booking to newStatus, but only if it is currently in
// fromStatus (an optimistic guard so a reject can't clobber an already-confirmed
// booking). Used by the reject/cancel paths on pending bookings.
func (s *BookingService) SetStatus(ctx context.Context, id, orgID uuid.UUID, newStatus, fromStatus string) error {
	b, err := s.bookingRepo.GetByID(id, orgID)
	if err != nil {
		return errors.New("booking not found")
	}
	if b.Status != fromStatus {
		return fmt.Errorf("booking is %s, expected %s", b.Status, fromStatus)
	}
	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.bookingRepo.UpdateStatus(tx, id, orgID, newStatus); err != nil {
		return err
	}
	return tx.Commit()
}

// generateInvoice is a best-effort hook: a booking that successfully commits should
// not be rolled back just because invoice generation failed. Errors are swallowed
// (the invoice can be regenerated), but generation runs after commit so the booking,
// attendees, and assignments are all visible.
func (s *BookingService) generateInvoice(bookingID, orgID uuid.UUID) {
	if s.invoiceSvc == nil {
		return
	}
	_ = s.invoiceSvc.GenerateForBooking(context.Background(), bookingID, orgID)
}

// ─── Individual booking ───────────────────────────────────────────────────────

func (s *BookingService) CreateIndividual(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.CreateIndividualBookingRequest, metadata json.RawMessage) (*models.Booking, error) {
	if req.BookerName == "" {
		return nil, errors.New("booker_name is required")
	}
	if req.RoomID == uuid.Nil {
		return nil, errors.New("room_id is required")
	}
	if req.CheckIn.IsZero() || req.CheckOut.IsZero() {
		return nil, errors.New("check_in and check_out are required")
	}
	if !req.CheckOut.After(req.CheckIn.Time) {
		return nil, errors.New("check_out must be after check_in")
	}

	available, err := s.assignmentRepo.IsRoomAvailable(req.RoomID, req.CheckIn.Time, req.CheckOut.Time, nil)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, errors.New("room is not available for the selected dates")
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	b := &models.Booking{
		OrgID:           orgID,
		BranchID:        branchID,
		BookingType:     models.BookingTypeRoom,
		BookerType:      models.BookerTypeIndividual,
		BookerName:      req.BookerName,
		BookerEmail:     req.BookerEmail,
		BookerPhone:     req.BookerPhone,
		WebUserID:       req.WebUserID,
		Status:          models.BookingStatusConfirmed,
		SpecialRequests: req.SpecialRequests,
		Metadata:        metadata,
	}
	if req.PromoteID != nil {
		b.ID = *req.PromoteID
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	// Single attendee — the booker themselves
	attendee := &models.BookingAttendee{
		BookingID:          b.ID,
		FullName:           req.BookerName,
		Email:              req.BookerEmail,
		Phone:              req.BookerPhone,
		IdentificationCard: req.IdentificationCard,
		IsLeadContact:      true,
	}
	if err = s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
		return nil, fmt.Errorf("failed to create attendee: %w", err)
	}

	// Populate the individual client registry (deduped on ID number). Skip if
	// no ID number was supplied — it's the dedup key.
	if s.clientRepo != nil && req.IdentificationCard != "" {
		client := &models.IndividualClient{
			FullName:         req.BookerName,
			Email:            req.BookerEmail,
			Phone:            req.BookerPhone,
			IDPassportNumber: req.IdentificationCard,
		}
		if err = s.clientRepo.FindOrCreateIndividualInTx(tx, orgID, client); err != nil {
			return nil, fmt.Errorf("failed to upsert client: %w", err)
		}
	}

	// Single room assignment
	assignment := &models.BookingRoomAssignment{
		BookingID:  b.ID,
		RoomID:     req.RoomID,
		AttendeeID: &attendee.ID,
		CheckIn:    req.CheckIn.Time,
		CheckOut:   req.CheckOut.Time,
		Status:     models.AssignmentStatusConfirmed,
	}
	if err = s.assignmentRepo.CreateInTx(tx, assignment); err != nil {
		return nil, fmt.Errorf("failed to create room assignment: %w", err)
	}

	// Set total_amount from room cost (must use tx — assignment not committed yet)
	total, costErr := s.assignmentRepo.SumRoomCostsInTx(tx, b.ID)
	if costErr == nil && total > 0 {
		_ = s.bookingRepo.UpdateTotalAmount(tx, b.ID, orgID, total)
		b.TotalAmount = total
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	s.generateInvoice(b.ID, orgID)

	b.Attendees = []models.BookingAttendee{*attendee}
	b.Assignments = []models.BookingRoomAssignment{*assignment}
	return b, nil
}

// CreateIndividualRooms materialises an approved individual accommodation request
// that may contain multiple rooms. Each slot in rooms maps to one attendant (by
// slot.AttendantIdx into attendants); if the attendant list is shorter than the
// room list the booker details are used as a fallback for unmatched slots.
func (s *BookingService) CreateIndividualRooms(ctx context.Context,
	orgID uuid.UUID,
	branchID *uuid.UUID,
	promoteID uuid.UUID,
	webUserID *uuid.UUID,
	bookerName, bookerEmail, bookerPhone string,
	envelope *models.SubmitIndividualBookingRequest,
	metadata json.RawMessage,
) (*models.Booking, error) {
	acc := envelope.Accommodation
	if acc == nil || len(acc.Rooms) == 0 {
		return nil, errors.New("no rooms in accommodation block")
	}

	checkIn := models.DateOnly{}
	if err := checkIn.UnmarshalJSON([]byte(`"` + acc.CheckIn + `"`)); err != nil {
		return nil, fmt.Errorf("invalid check_in: %w", err)
	}
	checkOut := models.DateOnly{}
	if err := checkOut.UnmarshalJSON([]byte(`"` + acc.CheckOut + `"`)); err != nil {
		return nil, fmt.Errorf("invalid check_out: %w", err)
	}
	if !checkOut.After(checkIn.Time) {
		return nil, errors.New("check_out must be after check_in")
	}

	// Availability check for every room before opening the transaction.
	for i, slot := range acc.Rooms {
		ok, err := s.assignmentRepo.IsRoomAvailable(slot.RoomID, checkIn.Time, checkOut.Time, nil)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("room %d (%s) is not available for the selected dates", i+1, slot.RoomID)
		}
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	b := &models.Booking{
		ID:          promoteID,
		OrgID:       orgID,
		BranchID:    branchID,
		BookingType: models.BookingTypeRoom,
		BookerType:  models.BookerTypeIndividual,
		BookerName:  bookerName,
		BookerEmail: bookerEmail,
		BookerPhone: bookerPhone,
		WebUserID:   webUserID,
		Status:      models.BookingStatusConfirmed,
		Metadata:    metadata,
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to promote booking: %w", err)
	}

	attendants := envelope.Attendants
	var createdAttendees []models.BookingAttendee
	var createdAssignments []models.BookingRoomAssignment

	for i, slot := range acc.Rooms {
		// Resolve attendant for this slot.
		var attendeeName, attendeeEmail, attendeePhone, attendeeIDNum string
		isLead := i == 0
		if slot.AttendantIdx >= 0 && slot.AttendantIdx < len(attendants) {
			a := attendants[slot.AttendantIdx]
			attendeeName = a.FullName
			attendeeEmail = a.Email
			attendeePhone = a.Phone
			attendeeIDNum = a.IDNumber
			isLead = a.IsLeadContact
		} else {
			// Fallback to booker for unmatched slots.
			attendeeName = bookerName
			attendeeEmail = bookerEmail
			attendeePhone = bookerPhone
		}

		attendee := &models.BookingAttendee{
			BookingID:          b.ID,
			FullName:           attendeeName,
			Email:              attendeeEmail,
			Phone:              attendeePhone,
			IdentificationCard: attendeeIDNum,
			IsLeadContact:      isLead,
		}
		if err = s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
			return nil, fmt.Errorf("failed to create attendee for slot %d: %w", i+1, err)
		}
		createdAttendees = append(createdAttendees, *attendee)

		// Populate the individual client registry (deduped on ID number). Skip
		// attendees without an ID number — it's the dedup key.
		if s.clientRepo != nil && attendeeIDNum != "" {
			client := &models.IndividualClient{
				FullName:         attendeeName,
				Email:            attendeeEmail,
				Phone:            attendeePhone,
				IDPassportNumber: attendeeIDNum,
			}
			if err = s.clientRepo.FindOrCreateIndividualInTx(tx, orgID, client); err != nil {
				return nil, fmt.Errorf("failed to upsert client for slot %d: %w", i+1, err)
			}
		}

		assignment := &models.BookingRoomAssignment{
			BookingID:  b.ID,
			RoomID:     slot.RoomID,
			AttendeeID: &attendee.ID,
			CheckIn:    checkIn.Time,
			CheckOut:   checkOut.Time,
			Status:     models.AssignmentStatusConfirmed,
		}
		if err = s.assignmentRepo.CreateInTx(tx, assignment); err != nil {
			return nil, fmt.Errorf("failed to create assignment for slot %d: %w", i+1, err)
		}
		createdAssignments = append(createdAssignments, *assignment)
	}

	total, costErr := s.assignmentRepo.SumRoomCostsInTx(tx, b.ID)
	if costErr == nil && total > 0 {
		_ = s.bookingRepo.UpdateTotalAmount(tx, b.ID, orgID, total)
		b.TotalAmount = total
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	s.generateInvoice(b.ID, orgID)

	b.Attendees = createdAttendees
	b.Assignments = createdAssignments
	return b, nil
}

// CreateIndividualEvent materialises an approved individual event request into a
// booking: it creates the bookings spine (booker_type individual) then delegates
// to the shared multi-session engine to create the booking_events + attendees.
// branchID is the lodge branch the first session's venue sits at.
func (s *BookingService) CreateIndividualEvent(ctx context.Context,
	orgID uuid.UUID,
	webUserID *uuid.UUID,
	promoteID *uuid.UUID,
	envelope *models.SubmitEventBookingRequest,
	metadata json.RawMessage,
) (*models.Booking, error) {
	if envelope.Event == nil || len(envelope.Event.Sessions) == 0 {
		return nil, errors.New("at least one event session is required")
	}

	// The booking lives on the lodge branch the first session's venue sits at.
	var branchID *uuid.UUID
	if vID, perr := uuid.Parse(envelope.Event.Sessions[0].VenueID); perr == nil {
		if venue, verr := s.venueRepo.GetByID(vID, orgID); verr == nil {
			branchID = venue.BranchID
		}
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	b := &models.Booking{
		OrgID:       orgID,
		BranchID:    branchID,
		BookingType: models.BookingTypeEvent,
		BookerType:  models.BookerTypeIndividual,
		BookerName:  envelope.BookedBy.Name,
		BookerEmail: envelope.BookedBy.Email,
		BookerPhone: envelope.BookedBy.Phone,
		WebUserID:   webUserID,
		Status:      models.BookingStatusConfirmed,
		Metadata:    metadata,
	}
	if promoteID != nil {
		b.ID = *promoteID
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	if _, err = s.materialiseEventSessions(tx, orgID, b, envelope.Event, envelope.Attendants); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	s.generateInvoice(b.ID, orgID)
	return b, nil
}

// ─── Corporate booking (promote a pending booking) ────────────────────────────

// CreateFromBooking promotes a pending corporate booking (bookingID) into a confirmed
// booking, materialising children from the envelope stored in the booking's metadata.
// It builds a CorporateBookingRequest shim from the booking so the shared materialise
// body (which predates single-phase) keeps working unchanged.
func (s *BookingService) CreateFromBooking(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, bookingID uuid.UUID, matReq *models.MaterialiseRequest) (*models.Booking, error) {
	b, err := s.bookingRepo.GetByID(bookingID, orgID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	if b.Status != models.BookingStatusPending {
		return nil, errors.New("only pending bookings can be materialised")
	}

	req := &models.CorporateBookingRequest{
		ID:              b.ID,
		OrgID:           orgID,
		BranchID:        b.BranchID,
		CorProfileID:    b.CorProfileID,
		CompanyID:       b.CompanyID,
		WebUserID:       b.WebUserID,
		BookingType:     corporateTypeFromBooking(b.BookingType),
		Status:          models.CorporateBookingStatusApproved,
		Payload:         b.Metadata,
		ProfileName:     b.BookerName,
		AuthoriserEmail: b.BookerEmail,
		AuthoriserPhone: b.BookerPhone,
	}
	return s.createCorporateBooking(orgID, branchID, req, matReq, b.WebUserID, &bookingID)
}

// CreateCorporateAccommodation materialises a corporate accommodation envelope
// directly into a confirmed booking + room assignments, without a pending state.
// The caller supplies the resolved company/profile IDs and the per-attendant room
// assignments (matReq). Used by the staff walk-in path.
func (s *BookingService) CreateCorporateAccommodation(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, corProfileID, companyID *uuid.UUID, envelope *models.SubmitAccommodationRequest, matReq *models.MaterialiseRequest) (*models.Booking, error) {
	payloadBytes, _ := json.Marshal(envelope)
	req := &models.CorporateBookingRequest{
		OrgID:           orgID,
		BranchID:        branchID,
		CorProfileID:    corProfileID,
		CompanyID:       companyID,
		BookingType:     models.CorporateBookingTypeAccommodation,
		Status:          models.CorporateBookingStatusApproved,
		Payload:         json.RawMessage(payloadBytes),
		ProfileName:     envelope.BookedBy.Name,
		AuthoriserEmail: envelope.BookedBy.Email,
		AuthoriserPhone: envelope.BookedBy.Phone,
	}
	return s.createCorporateBooking(orgID, branchID, req, matReq, nil, nil)
}

// createCorporateBooking is the shared materialise body. promoteID, when set,
// promotes that pending booking in place rather than inserting a new row.
func (s *BookingService) createCorporateBooking(orgID uuid.UUID, branchID *uuid.UUID, req *models.CorporateBookingRequest, matReq *models.MaterialiseRequest, webUserID *uuid.UUID, promoteID *uuid.UUID) (*models.Booking, error) {
	var err error

	// Only accommodation requests require room assignments
	if req.BookingType == models.CorporateBookingTypeAccommodation {
		if matReq == nil || len(matReq.Assignments) == 0 {
			return nil, errors.New("room assignments are required for accommodation requests")
		}
	}

	// Decode payload for accommodation validation
	var payload models.SubmitAccommodationRequest
	var attendants []models.CorBookingAttendant
	var checkIn, checkOut time.Time
	if req.BookingType == models.CorporateBookingTypeAccommodation && req.Payload != nil {
		if jsonErr := json.Unmarshal(req.Payload, &payload); jsonErr == nil && payload.Accommodation != nil {
			attendants = payload.Attendants
			// Parse shared check-in/check-out from the accommodation block
			layouts := []string{"2006-01-02", time.RFC3339}
			for _, l := range layouts {
				if t, e := time.Parse(l, payload.Accommodation.CheckIn); e == nil {
					checkIn = t
					break
				}
			}
			for _, l := range layouts {
				if t, e := time.Parse(l, payload.Accommodation.CheckOut); e == nil {
					checkOut = t
					break
				}
			}
		}
	}

	// Validate all attendants have an assignment (accommodation only)
	if req.BookingType == models.CorporateBookingTypeAccommodation {
		if len(matReq.Assignments) != len(attendants) {
			return nil, fmt.Errorf("expected %d room assignments, got %d", len(attendants), len(matReq.Assignments))
		}
		if checkIn.IsZero() || checkOut.IsZero() {
			return nil, errors.New("invalid check_in/check_out dates in accommodation block")
		}
		// Build index → room map and validate rooms are available
		for _, a := range matReq.Assignments {
			if a.GuestIndex < 0 || a.GuestIndex >= len(attendants) {
				return nil, fmt.Errorf("guest_index %d is out of range", a.GuestIndex)
			}
			available, avErr := s.assignmentRepo.IsRoomAvailable(a.RoomID, checkIn, checkOut, nil)
			if avErr != nil {
				return nil, avErr
			}
			if !available {
				attName := attendants[a.GuestIndex].FullName
				if attName == "" {
					attName = fmt.Sprintf("attendant %d", a.GuestIndex)
				}
				return nil, fmt.Errorf("room %s is not available for %s", a.RoomID, attName)
			}
		}
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Metadata = the full request payload as-is, enriched with assigned_rooms
	// for accommodation (which are only known at materialise time).
	meta := req.Payload
	if req.BookingType == models.CorporateBookingTypeAccommodation && payload.Accommodation != nil && len(matReq.Assignments) > 0 {
		type assignedRoom struct {
			GuestIndex int    `json:"guest_index"`
			GuestName  string `json:"guest_name,omitempty"`
			RoomID     string `json:"room_id"`
			RoomName   string `json:"room_name,omitempty"`
			RoomType   string `json:"room_type,omitempty"`
		}
		nights := int(checkOut.Sub(checkIn).Hours() / 24)
		assignedRooms := make([]assignedRoom, 0, len(matReq.Assignments))
		for _, a := range matReq.Assignments {
			guestName := ""
			if a.GuestIndex < len(attendants) {
				guestName = attendants[a.GuestIndex].FullName
			}
			assignedRooms = append(assignedRooms, assignedRoom{
				GuestIndex: a.GuestIndex,
				GuestName:  guestName,
				RoomID:     a.RoomID.String(),
				RoomName:   a.RoomName,
				RoomType:   a.RoomType,
			})
		}
		// Merge assigned_rooms and nights into the existing payload map
		var m map[string]interface{}
		if json.Unmarshal(req.Payload, &m) == nil {
			m["assigned_rooms"] = assignedRooms
			m["nights"] = nights
			meta, _ = json.Marshal(m)
		}
	}

	b := &models.Booking{
		OrgID:        orgID,
		BranchID:     branchID,
		BookingType:  bookingTypeFromRequest(req.BookingType),
		BookerType:   models.BookerTypeCorporate,
		BookerName:   req.ProfileName,
		BookerEmail:  req.AuthoriserEmail,
		BookerPhone:  req.AuthoriserPhone,
		CorProfileID: req.CorProfileID,
		CompanyID:    req.CompanyID,
		WebUserID:    webUserID,
		Status:       models.BookingStatusConfirmed,
		Metadata:     meta,
	}
	if promoteID != nil {
		b.ID = *promoteID
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	// For accommodation: create attendees + assignments from the shared attendant list
	if req.BookingType == models.CorporateBookingTypeAccommodation && req.CorProfileID != nil {
		assignmentMap := make(map[int]uuid.UUID, len(matReq.Assignments))
		for _, a := range matReq.Assignments {
			assignmentMap[a.GuestIndex] = a.RoomID
		}

		for i, att := range attendants {
			// Create attendee from the shared attendant
			attendee := &models.BookingAttendee{
				BookingID:          b.ID,
				FullName:           att.FullName,
				Email:              att.Email,
				Phone:              att.Phone,
				IdentificationCard: att.IDNumber,
				IsLeadContact:      att.IsLeadContact,
			}
			if err = s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
				return nil, fmt.Errorf("failed to create attendee: %w", err)
			}

			roomID := assignmentMap[i]
			attendeeID := attendee.ID
			assignment := &models.BookingRoomAssignment{
				BookingID:  b.ID,
				RoomID:     roomID,
				AttendeeID: &attendeeID,
				CheckIn:    checkIn,
				CheckOut:   checkOut,
				Status:     models.AssignmentStatusConfirmed,
			}
			if err = s.assignmentRepo.CreateInTx(tx, assignment); err != nil {
				return nil, fmt.Errorf("failed to create room assignment: %w", err)
			}
		}

		// Sum total amount from room costs (must use tx — assignments not committed yet)
		total, costErr := s.assignmentRepo.SumRoomCostsInTx(tx, b.ID)
		if costErr == nil && total > 0 {
			_ = s.bookingRepo.UpdateTotalAmount(tx, b.ID, orgID, total)
			b.TotalAmount = total
		}
	}

	// For events: new envelope (Flow B) carries multiple sessions, each its own
	// venue → one booking_events row per session via the shared engine. Older
	// single-venue requests fall back to materialiseEvent.
	if req.BookingType == models.CorporateBookingTypeEvent {
		var envelope models.SubmitEventBookingRequest
		if json.Unmarshal(req.Payload, &envelope) == nil && envelope.Event != nil && len(envelope.Event.Sessions) > 0 {
			if _, err = s.materialiseEventSessions(tx, orgID, b, envelope.Event, envelope.Attendants); err != nil {
				return nil, err
			}
		} else if err = s.materialiseEvent(tx, orgID, b, req, matReq); err != nil {
			return nil, err
		}
	}

	// For meals: create an attendee per named guest inside the tx so orders can
	// reference them. Orders themselves are created after commit (the order repo
	// opens its own transaction). guestAttendeeIDs maps guest index → attendee id.
	var mealsPayload models.SubmitMealsRequest
	var guestAttendeeIDs []uuid.UUID
	if req.BookingType == models.CorporateBookingTypeMeals {
		guestAttendeeIDs, err = s.materialiseMealAttendees(tx, b, req, &mealsPayload)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	// Meals orders are created post-commit: one order per named guest (carrying
	// their attendee_id + items) plus one buffet order for top-level items. Prices
	// are looked up from menu_items by the order repo.
	if req.BookingType == models.CorporateBookingTypeMeals {
		if err = s.materialiseMealOrders(orgID, b.ID, &mealsPayload, guestAttendeeIDs, b.BranchID); err != nil {
			return nil, err
		}
	}

	s.generateInvoice(b.ID, orgID)

	return b, nil
}

// materialiseMealAttendees decodes the meals payload and creates one attendee per
// named guest (buffet-only requests create none). Returns attendee ids aligned with
// payload.Guests by index.
func (s *BookingService) materialiseMealAttendees(
	tx *sql.Tx,
	b *models.Booking,
	req *models.CorporateBookingRequest,
	payload *models.SubmitMealsRequest,
) ([]uuid.UUID, error) {
	if req.Payload != nil {
		if jsonErr := json.Unmarshal(req.Payload, payload); jsonErr != nil {
			return nil, fmt.Errorf("invalid meals payload: %w", jsonErr)
		}
	}

	attendeeIDs := make([]uuid.UUID, len(payload.Guests))
	for i, g := range payload.Guests {
		attendee := &models.BookingAttendee{
			BookingID:          b.ID,
			FullName:           strings.TrimSpace(g.FirstName + " " + g.LastName),
			Email:              g.Email,
			IdentificationCard: g.IdentificationCard,
			IsLeadContact:      i == 0,
		}
		if err := s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
			return nil, fmt.Errorf("failed to create meal attendee: %w", err)
		}
		attendeeIDs[i] = attendee.ID
	}
	return attendeeIDs, nil
}

// materialiseMealOrders turns the meals payload into orders. Each named guest with
// selections gets their own order (linked to their attendee); top-level items become
// a single buffet order with no attendee. The order repo snapshots prices from
// menu_items, so any client-sent price is irrelevant.
func (s *BookingService) materialiseMealOrders(
	orgID, bookingID uuid.UUID,
	payload *models.SubmitMealsRequest,
	guestAttendeeIDs []uuid.UUID,
	branchID *uuid.UUID,
) error {
	if s.orderRepo == nil {
		return errors.New("orders are not configured; cannot materialise meals booking")
	}

	toOrderItems := func(items []models.CorMealItemInput) []models.PlaceOrderItemRequest {
		out := make([]models.PlaceOrderItemRequest, 0, len(items))
		for _, it := range items {
			if it.Quantity <= 0 {
				continue
			}
			out = append(out, models.PlaceOrderItemRequest{
				MenuItemID: it.MenuItemID,
				Quantity:   it.Quantity,
				Notes:      it.Notes,
			})
		}
		return out
	}

	// Per-guest orders.
	for i, g := range payload.Guests {
		items := toOrderItems(g.Items)
		if len(items) == 0 {
			continue
		}
		attendeeID := guestAttendeeIDs[i]
		order := &models.Order{
			BranchID:   branchID,
			BookingID:  &bookingID,
			AttendeeID: &attendeeID,
			Type:       models.OrderTypeInHouse,
		}
		if _, err := s.orderRepo.Create(order, items, orgID); err != nil {
			return fmt.Errorf("failed to create order for guest %s: %w", g.FirstName, err)
		}
	}

	// Buffet / shared order from top-level items (no attendee).
	if buffet := toOrderItems(payload.Items); len(buffet) > 0 {
		order := &models.Order{
			BranchID:  branchID,
			BookingID: &bookingID,
			Type:      models.OrderTypeInHouse,
			Notes:     "Buffet / shared order",
		}
		if _, err := s.orderRepo.Create(order, buffet, orgID); err != nil {
			return fmt.Errorf("failed to create buffet order: %w", err)
		}
	}

	return nil
}

// materialiseEvent seeds the named roster (if any) + the booking_events venue
// reservation for an event booking. The guest's chosen venue comes from the stored
// request payload; staff may override venue/price/dates via matReq.Event (optional).
// Price defaults to the venue's base_rate.
func (s *BookingService) materialiseEvent(
	tx *sql.Tx,
	orgID uuid.UUID,
	b *models.Booking,
	req *models.CorporateBookingRequest,
	matReq *models.MaterialiseRequest,
) error {
	d := eventDetailsFromRequest(req)

	// Venue: staff override (matReq.Event) wins, else the guest's chosen venue.
	venueID := d.VenueID
	var overrideStart, overrideEnd string
	var overridePrice float64
	if matReq != nil && matReq.Event != nil {
		if matReq.Event.VenueID != uuid.Nil {
			venueID = matReq.Event.VenueID
		}
		overrideStart, overrideEnd = matReq.Event.StartDate, matReq.Event.EndDate
		overridePrice = matReq.Event.Price
	}
	if venueID == uuid.Nil {
		return errors.New("a venue is required to materialise an event booking")
	}

	venue, err := s.venueRepo.GetByID(venueID, orgID)
	if err != nil {
		return errors.New("venue not found")
	}

	// The roster is optional: a booker may name every guest, name a few, or none at
	// all and rely on headcount. We create an attendee row for each named guest.
	for i, g := range d.Guests {
		attendee := &models.BookingAttendee{
			BookingID:          b.ID,
			FullName:           strings.TrimSpace(g.FirstName + " " + g.LastName),
			Email:              g.Email,
			IdentificationCard: g.IdentificationCard,
			IsLeadContact:      i == 0,
		}
		if err := s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
			return fmt.Errorf("failed to create attendee: %w", err)
		}
	}

	// Dates: staff override wins, else fall back to the payload.
	startDate := parseEventDate(overrideStart, d.StartDate)
	endDate := parseEventDate(overrideEnd, d.EndDate)
	if endDate.Before(startDate) {
		endDate = startDate
	}

	price := overridePrice
	if price <= 0 {
		price = venue.BaseRate
	}

	// pax: an explicit headcount is the real figure (rosters may be partial); fall
	// back to the named-guest count when no headcount was given.
	paxCount := d.Headcount
	if paxCount <= 0 {
		paxCount = len(d.Guests)
	}

	bookingEvent := &models.BookingEvent{
		BookingID:        b.ID,
		VenueID:          &venueID,
		EventType:        d.EventType,
		StartDate:        startDate,
		EndDate:          endDate,
		StartTime:        d.StartTime,
		EndTime:          d.EndTime,
		PaxCount:         paxCount,
		Price:            price,
		CateringRequired: d.CateringRequired,
	}
	if err := s.eventRepo.CreateInTx(tx, bookingEvent); err != nil {
		return fmt.Errorf("failed to create booking event: %w", err)
	}

	// Pin the venue on the booking and roll the hire charge into total_amount.
	days := int(endDate.Sub(startDate).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	total := price * float64(days)
	b.VenueID = &venueID
	b.TotalAmount = total
	if err := s.bookingRepo.UpdateVenueAndTotal(tx, b.ID, orgID, venueID, total); err != nil {
		return fmt.Errorf("failed to pin venue on booking: %w", err)
	}

	b.Events = []models.BookingEvent{*bookingEvent}
	return nil
}

// materialiseEventSessions is the shared engine for standalone event bookings
// (Flow B), used by both corporate and individual materialise. For an already-
// created booking spine `b`, it:
//   - creates one booking_events row per session (each with its own venue/date/time)
//   - creates a booking_attendees row per named attendant (headcount-only = none)
//   - rolls each session's hire charge into the booking total
//
// A session's venue is required; pricing falls back to the venue's base_rate when
// the session doesn't carry an explicit price (pricing_basis is advisory metadata
// kept in the payload — staff confirm final pricing). Returns the total amount.
func (s *BookingService) materialiseEventSessions(
	tx *sql.Tx,
	orgID uuid.UUID,
	b *models.Booking,
	event *models.EventBlock,
	attendants []models.CorBookingAttendant,
) (float64, error) {
	if event == nil || len(event.Sessions) == 0 {
		return 0, errors.New("at least one event session is required")
	}

	// Named roster → real attendees. Headcount-only bookings name no one and rely
	// on each session's pax_count.
	for i, a := range attendants {
		attendee := &models.BookingAttendee{
			BookingID:          b.ID,
			FullName:           a.FullName,
			Email:              a.Email,
			Phone:              a.Phone,
			IdentificationCard: a.IDNumber,
			DietaryNotes:       a.DietaryNotes,
			IsLeadContact:      a.IsLeadContact || i == 0,
		}
		if err := s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
			return 0, fmt.Errorf("failed to create attendee: %w", err)
		}
	}

	var total float64
	var created []models.BookingEvent
	var firstVenueID *uuid.UUID

	for idx, sess := range event.Sessions {
		venueID, err := uuid.Parse(sess.VenueID)
		if err != nil || venueID == uuid.Nil {
			return 0, fmt.Errorf("session %d is missing a valid venue", idx+1)
		}
		venue, err := s.venueRepo.GetByID(venueID, orgID)
		if err != nil {
			return 0, fmt.Errorf("venue for session %d not found", idx+1)
		}

		// Sessions are single-day: event_date is both start and end.
		day := parseEventDate("", sess.EventDate)

		eventType := sess.EventType
		if eventType == "" {
			eventType = "event"
		}

		price := venue.BaseRate
		total += price

		be := &models.BookingEvent{
			BookingID: b.ID,
			VenueID:   &venueID,
			EventType: eventType,
			StartDate: day,
			EndDate:   day,
			StartTime: sess.StartTime,
			EndTime:   sess.EndTime,
			PaxCount:  sess.ExpectedAttendees,
			Price:     price,
			Notes:     sess.SpecialRequirements,
		}
		if err := s.eventRepo.CreateInTx(tx, be); err != nil {
			return 0, fmt.Errorf("failed to create booking event for session %d: %w", idx+1, err)
		}
		created = append(created, *be)
		if firstVenueID == nil {
			firstVenueID = &venueID
		}
	}

	// Pin the first session's venue on the booking (the headline venue) and roll the
	// summed hire charges into total_amount.
	if firstVenueID != nil {
		if err := s.bookingRepo.UpdateVenueAndTotal(tx, b.ID, orgID, *firstVenueID, total); err != nil {
			return 0, fmt.Errorf("failed to pin venue on booking: %w", err)
		}
		b.VenueID = firstVenueID
	}
	b.TotalAmount = total
	b.Events = created
	return total, nil
}

// ─── Flow B: standalone meals materialise ────────────────────────────────────

// CreateIndividualMeal materialises an approved individual meal request into a
// bookings record + one order per session (per-attendant for detailed mode,
// per-session for headcount/buffet mode).
func (s *BookingService) CreateIndividualMeal(ctx context.Context, orgID uuid.UUID, webUserID *uuid.UUID, promoteID *uuid.UUID, envelope *models.SubmitMealBookingRequest, metadata json.RawMessage) (*models.Booking, error) {
	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	b := &models.Booking{
		OrgID:       orgID,
		BranchID:    envelope.BranchID,
		BookingType: models.BookingTypeMeals,
		BookerType:  models.BookerTypeIndividual,
		BookerName:  envelope.BookedBy.Name,
		BookerEmail: envelope.BookedBy.Email,
		BookerPhone: envelope.BookedBy.Phone,
		WebUserID:   webUserID,
		Status:      models.BookingStatusConfirmed,
		Metadata:    metadata,
	}
	if promoteID != nil {
		b.ID = *promoteID
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to create meals booking: %w", err)
	}

	// Create attendees from the envelope's attendants list.
	attendeeIDs, err := s.materialiseFlowBAttendees(tx, b, envelope.Attendants)
	if err != nil {
		return nil, err
	}

	if err = s.materialiseMealSessions(tx, orgID, b.ID, envelope.Meal, envelope.Attendants, attendeeIDs, b.BranchID); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	go s.generateInvoice(b.ID, orgID)
	return s.bookingRepo.GetByID(b.ID, orgID)
}

// CreateCorporateEvent materialises a corporate event envelope (Flow B) directly
// into a confirmed booking + event sessions, without going through the pending
// state. Mirrors CreateCorporateMeal for the event session engine. Used by the
// staff walk-in path where no approval step is needed.
func (s *BookingService) CreateCorporateEvent(ctx context.Context, orgID uuid.UUID, corProfileID, companyID *uuid.UUID, webUserID *uuid.UUID, promoteID *uuid.UUID, envelope *models.SubmitEventBookingRequest) (*models.Booking, error) {
	if envelope.Event == nil || len(envelope.Event.Sessions) == 0 {
		return nil, errors.New("at least one event session is required")
	}

	// The booking lives on the lodge branch the first session's venue sits at,
	// falling back to the envelope's branch.
	branchID := envelope.BranchID
	if vID, perr := uuid.Parse(envelope.Event.Sessions[0].VenueID); perr == nil {
		if venue, verr := s.venueRepo.GetByID(vID, orgID); verr == nil && venue.BranchID != nil {
			branchID = venue.BranchID
		}
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Store the full submitted envelope (not the stripped summary) so the web user's
	// reservation history survives confirmation; the invoice hydrator still finds
	// company/approver at the same top-level keys.
	eventMeta, _ := json.Marshal(envelope)

	b := &models.Booking{
		OrgID:        orgID,
		BranchID:     branchID,
		BookingType:  models.BookingTypeEvent,
		BookerType:   models.BookerTypeCorporate,
		BookerName:   envelope.BookedBy.Name,
		BookerEmail:  envelope.BookedBy.Email,
		BookerPhone:  envelope.BookedBy.Phone,
		CorProfileID: corProfileID,
		CompanyID:    companyID,
		WebUserID:    webUserID,
		Status:       models.BookingStatusConfirmed,
		Metadata:     json.RawMessage(eventMeta),
	}
	if promoteID != nil {
		b.ID = *promoteID
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to create event booking: %w", err)
	}

	if _, err = s.materialiseEventSessions(tx, orgID, b, envelope.Event, envelope.Attendants); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	go s.generateInvoice(b.ID, orgID)
	return s.bookingRepo.GetByID(b.ID, orgID)
}

// CreateCorporateMeal materialises an approved corporate meal request (Flow B)
// into a bookings record + orders using the same session engine.
func (s *BookingService) CreateCorporateMeal(ctx context.Context, orgID uuid.UUID, corProfileID, companyID *uuid.UUID, webUserID *uuid.UUID, promoteID *uuid.UUID, envelope *models.SubmitMealBookingRequest) (*models.Booking, error) {
	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Store the full submitted envelope as metadata so the web user's reservation
	// history stays intact after confirmation. The invoice hydrator reads company/
	// approver from the same top-level keys, so it keeps working. (Previously this
	// stored only buildMealMetadata's stripped summary, which dropped the meal
	// sessions / attendants and broke the reservation view.)
	mealMeta, _ := json.Marshal(envelope)

	b := &models.Booking{
		OrgID:        orgID,
		BookingType:  models.BookingTypeMeals,
		BookerType:   models.BookerTypeCorporate,
		BookerName:   envelope.BookedBy.Name,
		BookerEmail:  envelope.BookedBy.Email,
		BookerPhone:  envelope.BookedBy.Phone,
		CorProfileID: corProfileID,
		CompanyID:    companyID,
		WebUserID:    webUserID,
		Status:       models.BookingStatusConfirmed,
		Metadata:     json.RawMessage(mealMeta),
	}
	if envelope.BranchID != nil {
		b.BranchID = envelope.BranchID
	}
	if promoteID != nil {
		b.ID = *promoteID
	}
	if err = s.bookingRepo.CreateOrPromote(tx, b); err != nil {
		return nil, fmt.Errorf("failed to create meals booking: %w", err)
	}

	attendeeIDs, err := s.materialiseFlowBAttendees(tx, b, envelope.Attendants)
	if err != nil {
		return nil, err
	}

	if err = s.materialiseMealSessions(tx, orgID, b.ID, envelope.Meal, envelope.Attendants, attendeeIDs, b.BranchID); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	go s.generateInvoice(b.ID, orgID)
	return s.bookingRepo.GetByID(b.ID, orgID)
}

// materialiseFlowBAttendees creates booking_attendees rows from the shared
// attendants slice (used by both event and meal Flow B paths).
func (s *BookingService) materialiseFlowBAttendees(tx *sql.Tx, b *models.Booking, attendants []models.CorBookingAttendant) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(attendants))
	for i, a := range attendants {
		att := &models.BookingAttendee{
			BookingID:     b.ID,
			FullName:      a.FullName,
			Email:         a.Email,
			Phone:         a.Phone,
			DietaryNotes:  a.DietaryNotes,
			IsLeadContact: a.IsLeadContact,
		}
		if err := s.attendeeRepo.CreateInTx(tx, att); err != nil {
			return nil, fmt.Errorf("failed to create attendee: %w", err)
		}
		ids[i] = att.ID
	}
	return ids, nil
}

// materialiseMealSessions turns a MealBlock's sessions into orders. For each session:
//   - Detailed mode (individual_orders present): one Order per attendant per session.
//   - Headcount/buffet mode (menu_item_id present): one Order for the whole session,
//     quantity = pax_count, attributed to the booking (no attendee_id).
func (s *BookingService) materialiseMealSessions(
	tx *sql.Tx,
	orgID, bookingID uuid.UUID,
	meal *models.MealBlock,
	attendants []models.CorBookingAttendant,
	attendeeIDs []uuid.UUID,
	branchID *uuid.UUID,
) error {
	if s.orderRepo == nil {
		return errors.New("order repository not configured")
	}
	if meal == nil {
		return errors.New("meal block is required")
	}

	for _, sess := range meal.Sessions {
		scheduledFor, err := time.Parse("2006-01-02", sess.MealDate)
		if err != nil {
			return fmt.Errorf("invalid meal_date %q: %w", sess.MealDate, err)
		}

		if len(sess.IndividualOrders) > 0 {
			// Detailed mode — one order per session; each item carries its attendee_id.
			var items []models.PlaceOrderItemRequest
			for _, io := range sess.IndividualOrders {
				menuItemID, parseErr := uuid.Parse(io.MenuItemID)
				if parseErr != nil || io.Quantity <= 0 {
					continue
				}
				var attendeeID *uuid.UUID
				if io.AttendantIdx >= 0 && io.AttendantIdx < len(attendeeIDs) {
					aid := attendeeIDs[io.AttendantIdx]
					attendeeID = &aid
				}
				items = append(items, models.PlaceOrderItemRequest{
					MenuItemID: menuItemID,
					AttendeeID: attendeeID,
					Quantity:   io.Quantity,
					Notes:      io.Notes,
				})
			}
			if len(items) > 0 {
				order := &models.Order{
					BranchID:     branchID,
					BookingID:    &bookingID,
					Type:         models.OrderTypeInHouse,
					ScheduledFor: &scheduledFor,
					MealPeriod:   sess.MealPeriod,
					ServingTime:  sess.ServingTime,
					ServiceType:  sess.ServiceType,
					Notes:        sess.DietaryNotes,
				}
				if err := s.orderRepo.CreateInTx(tx, order, items, orgID); err != nil {
					return fmt.Errorf("failed to create detailed order for session %s: %w", sess.MealDate, err)
				}
			}
		} else if sess.MenuItemID != "" {
			// Headcount/buffet mode — one order for the whole session.
			menuItemID, parseErr := uuid.Parse(sess.MenuItemID)
			if parseErr != nil {
				return fmt.Errorf("invalid menu_item_id for session %s: %w", sess.MealDate, parseErr)
			}
			pax := sess.PaxCount
			if pax <= 0 {
				pax = 1
			}
			serviceType := sess.ServiceType
			if serviceType == "" {
				serviceType = "buffet"
			}
			order := &models.Order{
				BranchID:     branchID,
				BookingID:    &bookingID,
				Type:         models.OrderTypeInHouse,
				ScheduledFor: &scheduledFor,
				MealPeriod:   sess.MealPeriod,
				ServingTime:  sess.ServingTime,
				PaxCount:     &pax,
				ServiceType:  serviceType,
				Notes:        sess.DietaryNotes,
			}
			// A buffet is billed as a flat package (quantity = 1), not per head. The
			// cover count is kept on the order (PaxCount) for display context only.
			items := []models.PlaceOrderItemRequest{{MenuItemID: menuItemID, Quantity: 1}}
			if err := s.orderRepo.CreateInTx(tx, order, items, orgID); err != nil {
				return fmt.Errorf("failed to create buffet order for session %s: %w", sess.MealDate, err)
			}
		}
		// If neither detailed nor a menu_item_id, the session is noted-only — no order.
	}
	return nil
}

// eventPayloadDetails holds everything materialiseEvent needs out of a stored event
// request payload.
type eventPayloadDetails struct {
	Guests           []models.CorConferenceGuestInput
	EventType        string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Headcount        int
	CateringRequired bool
	VenueID          uuid.UUID
}

// eventDetailsFromRequest unpacks the stored event request payload. event_type defaults
// to "event" when the payload doesn't specify one.
func eventDetailsFromRequest(req *models.CorporateBookingRequest) eventPayloadDetails {
	d := eventPayloadDetails{EventType: "event"}
	if req.Payload == nil {
		return d
	}
	var p models.SubmitEventRequest
	if json.Unmarshal(req.Payload, &p) == nil {
		d.Guests = p.Guests
		d.StartDate, d.EndDate = p.StartDate, p.EndDate
		d.StartTime, d.EndTime = p.StartTime, p.EndTime
		d.Headcount = p.Headcount
		d.CateringRequired = p.CateringRequired
		d.VenueID = p.VenueID
		if p.EventType != "" {
			d.EventType = p.EventType
		}
	}
	return d
}

func parseEventDate(override, fallback string) time.Time {
	for _, s := range []string{override, fallback} {
		if s == "" {
			continue
		}
		for _, l := range []string{"2006-01-02", time.RFC3339} {
			if t, err := time.Parse(l, s); err == nil {
				return t.UTC().Truncate(24 * time.Hour)
			}
		}
	}
	return time.Now().UTC().Truncate(24 * time.Hour)
}

func parseGuestDates(g models.CorBookingGuestInput) (checkIn, checkOut time.Time, err error) {
	layouts := []string{"2006-01-02", time.RFC3339}
	for _, l := range layouts {
		if t, e := time.Parse(l, g.CheckIn); e == nil {
			checkIn = t
			break
		}
	}
	for _, l := range layouts {
		if t, e := time.Parse(l, g.CheckOut); e == nil {
			checkOut = t
			break
		}
	}
	if checkIn.IsZero() || checkOut.IsZero() {
		err = fmt.Errorf("invalid check_in/check_out dates for guest %s %s", g.FirstName, g.LastName)
	}
	return
}

func bookingTypeFromRequest(t string) string {
	switch t {
	case models.CorporateBookingTypeMeals:
		return models.BookingTypeMeals
	case models.CorporateBookingTypeEvent:
		return models.BookingTypeEvent
	default:
		return models.BookingTypeRoom
	}
}

// corporateTypeFromBooking maps a booking_type back to the corporate request type the
// shared materialise body switches on.
func corporateTypeFromBooking(t string) string {
	switch t {
	case models.BookingTypeMeals:
		return models.CorporateBookingTypeMeals
	case models.BookingTypeEvent:
		return models.CorporateBookingTypeEvent
	default:
		return models.CorporateBookingTypeAccommodation
	}
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func (s *BookingService) GetByID(ctx context.Context, id, orgID uuid.UUID) (*models.Booking, error) {
	b, err := s.bookingRepo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	b.Attendees, _ = s.attendeeRepo.ListByBookingID(id)
	b.Assignments, _ = s.assignmentRepo.ListByBookingID(id)
	b.Events, _ = s.eventRepo.ListByBookingID(id)
	return b, nil
}

func (s *BookingService) List(ctx context.Context, orgID uuid.UUID, bookerType, bookingType, status string, from, to *time.Time, page, pageSize int) ([]models.Booking, int, error) {
	return s.bookingRepo.List(orgID, bookerType, bookingType, status, from, to, page, pageSize)
}

func (s *BookingService) ListForExport(ctx context.Context, orgID uuid.UUID, bookerType, bookingType, status string, from, to *time.Time) ([]models.Booking, error) {
	return s.bookingRepo.ListForExport(orgID, bookerType, bookingType, status, from, to)
}

func (s *BookingService) ListForWebUser(ctx context.Context, webUserID uuid.UUID, page, pageSize int) ([]models.Booking, int, error) {
	return s.bookingRepo.ListByWebUserID(webUserID, page, pageSize)
}

// ─── Status transitions ───────────────────────────────────────────────────────

func (s *BookingService) updateStatus(id, orgID uuid.UUID, newStatus string) error {
	b, err := s.bookingRepo.GetByID(id, orgID)
	if err != nil {
		return errors.New("booking not found")
	}

	allowed := models.ValidBookingTransitions[b.Status]
	for _, next := range allowed {
		if next == newStatus {
			tx, txErr := s.bookingRepo.Begin()
			if txErr != nil {
				return txErr
			}
			defer tx.Rollback()
			if err := s.bookingRepo.UpdateStatus(tx, id, orgID, newStatus); err != nil {
				return err
			}
			return tx.Commit()
		}
	}
	return fmt.Errorf("cannot transition booking from %s to %s", b.Status, newStatus)
}

func (s *BookingService) CheckIn(ctx context.Context, id, orgID uuid.UUID) error {
	return s.updateStatus(id, orgID, models.BookingStatusCheckedIn)
}

func (s *BookingService) CheckOut(ctx context.Context, id, orgID uuid.UUID) error {
	return s.updateStatus(id, orgID, models.BookingStatusCheckedOut)
}

func (s *BookingService) Cancel(ctx context.Context, id, orgID uuid.UUID) error {
	return s.updateStatus(id, orgID, models.BookingStatusCancelled)
}

func (s *BookingService) UpdateStatus(ctx context.Context, id, orgID uuid.UUID, newStatus string) error {
	return s.updateStatus(id, orgID, newStatus)
}

// ─── Room assignments ─────────────────────────────────────────────────────────

// checkRoomAssignable enforces the sharing rule at assignment time: a room may
// only be shared by guests under the SAME booking, and only up to its capacity.
// An overlapping assignment from a different booking blocks it outright.
func (s *BookingService) checkRoomAssignable(roomID, bookingID uuid.UUID, checkIn, checkOut time.Time, excludeID *uuid.UUID) error {
	res, err := s.assignmentRepo.CanAssignRoom(roomID, bookingID, checkIn, checkOut, excludeID)
	if err != nil {
		return err
	}
	if res.OtherBooking {
		return errors.New("room is already assigned to a different booking for the selected dates")
	}
	if res.CapacityFull {
		return fmt.Errorf("room capacity (%d) reached for this booking on the selected dates", res.Capacity)
	}
	return nil
}

func (s *BookingService) AssignRoom(ctx context.Context, id, orgID uuid.UUID, req *models.CreateRoomAssignmentRequest) (*models.BookingRoomAssignment, error) {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return nil, errors.New("booking not found")
	}
	if req.RoomID == uuid.Nil {
		return nil, errors.New("room_id is required")
	}
	if !req.CheckOut.After(req.CheckIn.Time) {
		return nil, errors.New("check_out must be after check_in")
	}

	if err := s.checkRoomAssignable(req.RoomID, id, req.CheckIn.Time, req.CheckOut.Time, nil); err != nil {
		return nil, err
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	assignment := &models.BookingRoomAssignment{
		BookingID:  id,
		RoomID:     req.RoomID,
		AttendeeID: req.AttendeeID,
		CheckIn:    req.CheckIn.Time,
		CheckOut:   req.CheckOut.Time,
		Status:     models.AssignmentStatusConfirmed,
	}
	if err = s.assignmentRepo.CreateInTx(tx, assignment); err != nil {
		return nil, err
	}

	// Recalculate booking total
	total, _ := s.assignmentRepo.SumRoomCosts(id)
	_ = s.bookingRepo.UpdateTotalAmount(tx, id, orgID, total)

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return assignment, nil
}

func (s *BookingService) ListAssignments(ctx context.Context, id, orgID uuid.UUID) ([]models.BookingRoomAssignment, error) {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return nil, errors.New("booking not found")
	}
	return s.assignmentRepo.ListByBookingID(id)
}

func (s *BookingService) UpdateAssignment(ctx context.Context, id, orgID, assignmentID uuid.UUID, req *models.UpdateRoomAssignmentRequest) (*models.BookingRoomAssignment, error) {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return nil, errors.New("booking not found")
	}

	existing, err := s.assignmentRepo.GetByID(assignmentID, id)
	if err != nil {
		return nil, errors.New("assignment not found")
	}

	roomID := existing.RoomID
	if req.RoomID != nil {
		roomID = *req.RoomID
	}
	checkIn := existing.CheckIn
	if req.CheckIn != nil {
		checkIn = req.CheckIn.Time
	}
	checkOut := existing.CheckOut
	if req.CheckOut != nil {
		checkOut = req.CheckOut.Time
	}

	if err := s.checkRoomAssignable(roomID, id, checkIn, checkOut, &assignmentID); err != nil {
		return nil, err
	}

	return s.assignmentRepo.Update(assignmentID, id, req)
}

func (s *BookingService) RemoveAssignment(ctx context.Context, id, orgID, assignmentID uuid.UUID) error {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return errors.New("booking not found")
	}
	return s.assignmentRepo.Delete(assignmentID, id)
}

func (s *BookingService) CheckInAssignment(ctx context.Context, id, orgID, assignmentID uuid.UUID) error {
	b, err := s.bookingRepo.GetByID(id, orgID)
	if err != nil {
		return errors.New("booking not found")
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// A room may only be shared within the same booking. Block check-in if another
	// booking's guest is currently physically in this room (checked in, not yet out).
	occupied, otherBooking, err := s.assignmentRepo.RoomOccupiedByOtherBookingTx(tx, assignmentID, id)
	if err != nil {
		return err
	}
	if occupied {
		if otherBooking != "" {
			return fmt.Errorf("room is currently occupied by another booking (%s); guest cannot check in", otherBooking)
		}
		return errors.New("room is currently occupied by another booking; guest cannot check in")
	}

	if err := s.assignmentRepo.UpdateStatusTx(tx, assignmentID, id, models.AssignmentStatusCheckedIn); err != nil {
		return err
	}

	// Roll the booking up: the first guest to check in moves a confirmed booking
	// to checked_in. (No-op if it's already checked_in.)
	if b.Status == models.BookingStatusConfirmed {
		if err := s.bookingRepo.UpdateStatus(tx, id, orgID, models.BookingStatusCheckedIn); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *BookingService) CheckOutAssignment(ctx context.Context, id, orgID, assignmentID uuid.UUID) error {
	b, err := s.bookingRepo.GetByID(id, orgID)
	if err != nil {
		return errors.New("booking not found")
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.assignmentRepo.UpdateStatusTx(tx, assignmentID, id, models.AssignmentStatusCheckedOut); err != nil {
		return err
	}

	// Roll the booking up: only mark the whole booking checked_out once every
	// non-cancelled assignment has checked out. While guests remain in-house the
	// booking stays checked_in (or is promoted to it if it was still confirmed).
	active, checkedOut, err := s.assignmentRepo.StatusCountsTx(tx, id)
	if err != nil {
		return err
	}
	bookingComplete := false
	switch {
	case active > 0 && checkedOut == active:
		bookingComplete = true
		if b.Status != models.BookingStatusCheckedOut {
			if err := s.bookingRepo.UpdateStatus(tx, id, orgID, models.BookingStatusCheckedOut); err != nil {
				return err
			}
		}
	case b.Status == models.BookingStatusConfirmed:
		// Edge case: a guest checks out without the booking ever being marked
		// checked_in (e.g. assignment checked in directly). Keep status coherent.
		if err := s.bookingRepo.UpdateStatus(tx, id, orgID, models.BookingStatusCheckedIn); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Once the last guest has checked out, regenerate the (draft) invoice from
	// actual nights stayed. Best-effort and post-commit: the actual checkout must
	// be visible, and a regeneration failure must not undo the checkout itself.
	if bookingComplete && s.invoiceSvc != nil {
		_ = s.invoiceSvc.RegenerateRoomInvoice(context.Background(), id, orgID)
	}

	return nil
}

// ─── Attendees ────────────────────────────────────────────────────────────────

func (s *BookingService) ListAttendees(ctx context.Context, id, orgID uuid.UUID) ([]models.BookingAttendee, error) {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return nil, errors.New("booking not found")
	}
	return s.attendeeRepo.ListByBookingID(id)
}

func (s *BookingService) AddAttendee(ctx context.Context, id, orgID uuid.UUID, req *models.CreateAttendeeRequest) (*models.BookingAttendee, error) {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return nil, errors.New("booking not found")
	}

	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	attendee := &models.BookingAttendee{
		BookingID:          id,
		CorporateGuestID:   req.CorporateGuestID,
		FullName:           req.FullName,
		Email:              req.Email,
		Phone:              req.Phone,
		IdentificationCard: req.IdentificationCard,
		DietaryNotes:       req.DietaryNotes,
		SpecialNeeds:       req.SpecialNeeds,
		IsLeadContact:      req.IsLeadContact,
	}
	if err = s.attendeeRepo.CreateInTx(tx, attendee); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return attendee, nil
}

func (s *BookingService) UpdateAttendee(ctx context.Context, id, orgID, attendeeID uuid.UUID, req *models.UpdateAttendeeRequest) (*models.BookingAttendee, error) {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return nil, errors.New("booking not found")
	}
	return s.attendeeRepo.Update(attendeeID, id, req)
}

func (s *BookingService) RemoveAttendee(ctx context.Context, id, orgID, attendeeID uuid.UUID) error {
	if _, err := s.bookingRepo.GetByID(id, orgID); err != nil {
		return errors.New("booking not found")
	}
	return s.attendeeRepo.Delete(attendeeID, id)
}

// ─── Overdue (nightly job) ────────────────────────────────────────────────────

func (s *BookingService) FindOverdueCheckouts(ctx context.Context, orgIDs []uuid.UUID) ([]repository.OverdueBookingRef, error) {
	return s.bookingRepo.FindOverdueCheckouts(orgIDs)
}

func (s *BookingService) MarkOverstayed(ctx context.Context, id, orgID uuid.UUID) error {
	return s.bookingRepo.MarkOverstayed(id, orgID)
}

func (s *BookingService) RecalculateTotal(ctx context.Context, id, orgID uuid.UUID) error {
	total, err := s.assignmentRepo.SumRoomCosts(id)
	if err != nil {
		return err
	}
	tx, err := s.bookingRepo.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.bookingRepo.UpdateTotalAmount(tx, id, orgID, total); err != nil {
		return err
	}
	return tx.Commit()
}
