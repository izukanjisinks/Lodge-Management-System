package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type CorporateBookingRequestService struct {
	requestRepo *repository.CorporateBookingRequestRepository
	guestRepo   *repository.CorporateGuestRepository
	corProfile  *CorProfileService
	workflow    *WorkflowService
	venueRepo   *repository.VenueRepository
	menuRepo    *repository.MenuRepository
	booking     *BookingService
}

func NewCorporateBookingRequestService(
	requestRepo *repository.CorporateBookingRequestRepository,
	guestRepo *repository.CorporateGuestRepository,
	corProfile *CorProfileService,
) *CorporateBookingRequestService {
	return &CorporateBookingRequestService{
		requestRepo: requestRepo,
		guestRepo:   guestRepo,
		corProfile:  corProfile,
	}
}

func (s *CorporateBookingRequestService) SetWorkflowService(svc *WorkflowService) {
	s.workflow = svc
}

func (s *CorporateBookingRequestService) SetVenueRepository(repo *repository.VenueRepository) {
	s.venueRepo = repo
}

func (s *CorporateBookingRequestService) SetMenuRepository(repo *repository.MenuRepository) {
	s.menuRepo = repo
}

// SetBookingService wires the booking service so approving an event/conference
// request auto-creates the booking from the guest's chosen venue.
func (s *CorporateBookingRequestService) SetBookingService(svc *BookingService) {
	s.booking = svc
}

// validateVenue ensures a guest-chosen venue exists and is in service.
func (s *CorporateBookingRequestService) validateVenue(orgID, venueID uuid.UUID) error {
	if venueID == uuid.Nil {
		return errors.New("venue_id is required")
	}
	if s.venueRepo == nil {
		return nil
	}
	venue, err := s.venueRepo.GetByID(venueID, orgID)
	if err != nil {
		return errors.New("selected venue not found")
	}
	if !venue.IsAvailable {
		return errors.New("selected venue is not available")
	}
	return nil
}

// venueBranchID resolves the lodge branch a venue physically belongs to. This is
// the branch an event booking inherits — distinct from the request's branch_id,
// which references the corporate client's branch (cor_branch_details).
func (s *CorporateBookingRequestService) venueBranchID(orgID, venueID uuid.UUID) *uuid.UUID {
	if s.venueRepo == nil || venueID == uuid.Nil {
		return nil
	}
	venue, err := s.venueRepo.GetByID(venueID, orgID)
	if err != nil {
		return nil
	}
	return venue.BranchID
}

// ─── Submission ───────────────────────────────────────────────────────────────

func (s *CorporateBookingRequestService) SubmitAccommodation(orgID uuid.UUID, webUserID uuid.UUID, req *models.SubmitAccommodationRequest) (*models.Booking, error) {
	// Validate required fields
	if req.BookedBy.Email == "" || req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name and email are required")
	}
	if req.Company == nil || req.Company.Name == "" {
		return nil, errors.New("company name is required")
	}
	if req.Accommodation == nil {
		return nil, errors.New("accommodation block is required")
	}
	if req.Accommodation.RoomCount < 1 {
		return nil, errors.New("room_count must be at least 1")
	}
	if req.Accommodation.CheckIn == "" || req.Accommodation.CheckOut == "" {
		return nil, errors.New("check_in and check_out are required")
	}

	// Validate attendants if detailed mode
	if req.ParticipantMode == "detailed" && len(req.Attendants) == 0 {
		return nil, errors.New("attendants are required in detailed mode")
	}

	// Map the nested company/branch/profile into the inputs ResolveChain expects,
	// then get-or-create the company → branch → profile chain.
	company := models.CorBookingCompanyInput{
		CompanyName: req.Company.Name,
		TPIN:        req.Company.TPIN,
		Industry:    req.Company.Industry,
	}

	var branch *models.CorBookingBranchInput
	if req.Company.BranchName != "" {
		branch = &models.CorBookingBranchInput{
			Name: req.Company.BranchName,
		}
	}

	// Split full name into first/last for profile lookup.
	firstName, lastName := splitName(req.BookedBy.Name)
	profile := models.CorBookingProfileInput{
		FirstName:  firstName,
		LastName:   lastName,
		Email:      req.BookedBy.Email,
		Phone:      req.BookedBy.Phone,
		JobTitle:   req.BookedBy.JobTitle,
		ManNumber:  req.BookedBy.ManNumber,
		Department: req.Company.DepartmentName,
	}

	companyID, _, corProfileID, err := s.corProfile.ResolveChain(orgID, company, branch, profile)
	if err != nil {
		return nil, err
	}

	// Store entire payload as-is
	payloadBytes, _ := json.Marshal(req)
	payload := json.RawMessage(payloadBytes)

	var webUserIDPtr *uuid.UUID
	if webUserID != uuid.Nil {
		webUserIDPtr = &webUserID
	}

	booking, err := s.booking.SubmitPending(&models.PendingBookingInput{
		OrgID:        orgID,
		BranchID:     req.BranchID, // lodge branch from URL, not cor_branch_details ID
		WebUserID:    webUserIDPtr,
		CorProfileID: &corProfileID,
		CompanyID:    &companyID,
		BookerType:   models.BookerTypeCorporate,
		BookerName:   req.BookedBy.Name,
		BookingType:  models.CorporateBookingTypeAccommodation,
		Documents:    req.Documents,
		Metadata:     payload,
	})
	if err != nil {
		return nil, err
	}

	s.startWorflow(booking, req.Company.Name)
	return booking, nil
}

// SubmitEventBooking handles the standalone event envelope (Flow B) from
// eventBooking.js for corporate bookers. It maps the nested company/approver/
// booked_by into the ResolveChain inputs, stores the whole envelope as JSONB, and
// starts the approval workflow.
func (s *CorporateBookingRequestService) SubmitEventBooking(orgID uuid.UUID, webUserID uuid.UUID, req *models.SubmitEventBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Email == "" || req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name and email are required")
	}
	if req.Company == nil || req.Company.Name == "" {
		return nil, errors.New("company name is required")
	}
	if req.Event == nil || len(req.Event.Sessions) == 0 {
		return nil, errors.New("at least one event session is required")
	}
	for i, sess := range req.Event.Sessions {
		if sess.VenueID == "" {
			return nil, fmt.Errorf("session %d is missing a venue", i+1)
		}
		venueID, err := uuid.Parse(sess.VenueID)
		if err != nil {
			return nil, fmt.Errorf("session %d has an invalid venue", i+1)
		}
		if err := s.validateVenue(orgID, venueID); err != nil {
			return nil, err
		}
	}

	company := models.CorBookingCompanyInput{
		CompanyName: req.Company.Name,
		TPIN:        req.Company.TPIN,
		Industry:    req.Company.Industry,
	}
	var branch *models.CorBookingBranchInput
	if req.Company.BranchName != "" {
		branch = &models.CorBookingBranchInput{Name: req.Company.BranchName}
	}
	firstName, lastName := splitName(req.BookedBy.Name)
	profile := models.CorBookingProfileInput{
		FirstName:  firstName,
		LastName:   lastName,
		Email:      req.BookedBy.Email,
		Phone:      req.BookedBy.Phone,
		JobTitle:   req.BookedBy.JobTitle,
		ManNumber:  req.BookedBy.ManNumber,
		Department: req.Company.DepartmentName,
	}

	companyID, _, profileID, err := s.corProfile.ResolveChain(orgID, company, branch, profile)
	if err != nil {
		return nil, err
	}

	payloadBytes, _ := json.Marshal(req)

	var webUserIDPtr *uuid.UUID
	if webUserID != uuid.Nil {
		webUserIDPtr = &webUserID
	}

	booking, err := s.booking.SubmitPending(&models.PendingBookingInput{
		OrgID:        orgID,
		BranchID:     req.BranchID,
		WebUserID:    webUserIDPtr,
		CorProfileID: &profileID,
		CompanyID:    &companyID,
		BookerType:   models.BookerTypeCorporate,
		BookerName:   req.BookedBy.Name,
		BookingType:  models.CorporateBookingTypeEvent,
		Documents:    req.Documents,
		Metadata:     json.RawMessage(payloadBytes),
	})
	if err != nil {
		return nil, err
	}

	s.startWorflow(booking, req.Company.Name)
	return booking, nil
}

// SubmitMealBooking handles the standalone meal envelope (Flow B) from
// mealBooking.js for corporate bookers. Stores the whole envelope as JSONB and
// starts the approval workflow.
func (s *CorporateBookingRequestService) SubmitMealBooking(orgID uuid.UUID, webUserID uuid.UUID, req *models.SubmitMealBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Email == "" || req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name and email are required")
	}
	if req.Company == nil || req.Company.Name == "" {
		return nil, errors.New("company name is required")
	}
	if req.Meal == nil || len(req.Meal.Sessions) == 0 {
		return nil, errors.New("at least one meal session is required")
	}

	company := models.CorBookingCompanyInput{
		CompanyName: req.Company.Name,
		TPIN:        req.Company.TPIN,
		Industry:    req.Company.Industry,
	}
	var branch *models.CorBookingBranchInput
	if req.Company.BranchName != "" {
		branch = &models.CorBookingBranchInput{Name: req.Company.BranchName}
	}
	firstName, lastName := splitName(req.BookedBy.Name)
	profile := models.CorBookingProfileInput{
		FirstName:  firstName,
		LastName:   lastName,
		Email:      req.BookedBy.Email,
		Phone:      req.BookedBy.Phone,
		JobTitle:   req.BookedBy.JobTitle,
		ManNumber:  req.BookedBy.ManNumber,
		Department: req.Company.DepartmentName,
	}

	companyID, _, profileID, err := s.corProfile.ResolveChain(orgID, company, branch, profile)
	if err != nil {
		return nil, err
	}

	payloadBytes, _ := json.Marshal(req)

	var webUserIDPtr *uuid.UUID
	if webUserID != uuid.Nil {
		webUserIDPtr = &webUserID
	}

	booking, err := s.booking.SubmitPending(&models.PendingBookingInput{
		OrgID:        orgID,
		BranchID:     req.BranchID,
		WebUserID:    webUserIDPtr,
		CorProfileID: &profileID,
		CompanyID:    &companyID,
		BookerType:   models.BookerTypeCorporate,
		BookerName:   req.BookedBy.Name,
		BookingType:  models.CorporateBookingTypeMeals,
		Documents:    req.Documents,
		Metadata:     json.RawMessage(payloadBytes),
	})
	if err != nil {
		return nil, err
	}

	s.startWorflow(booking, req.Company.Name)
	return booking, nil
}

// resolveCorporateChain get-or-creates the company → branch → profile chain from a
// booked_by + company block, returning the resolved company and profile IDs. Shared
// by the website submit and the staff walk-in paths.
func (s *CorporateBookingRequestService) resolveCorporateChain(orgID uuid.UUID, comp *models.CorSubmitCompany, bookedBy models.CorSubmitBookedBy) (companyID, profileID uuid.UUID, err error) {
	company := models.CorBookingCompanyInput{
		CompanyName: comp.Name,
		TPIN:        comp.TPIN,
		Industry:    comp.Industry,
	}
	var branch *models.CorBookingBranchInput
	if comp.BranchName != "" {
		branch = &models.CorBookingBranchInput{Name: comp.BranchName}
	}
	firstName, lastName := splitName(bookedBy.Name)
	profile := models.CorBookingProfileInput{
		FirstName:  firstName,
		LastName:   lastName,
		Email:      bookedBy.Email,
		Phone:      bookedBy.Phone,
		JobTitle:   bookedBy.JobTitle,
		ManNumber:  bookedBy.ManNumber,
		Department: comp.DepartmentName,
	}
	companyID, _, profileID, err = s.corProfile.ResolveChain(orgID, company, branch, profile)
	return companyID, profileID, err
}

// WalkInMeal creates a confirmed corporate meal booking directly (no pending state,
// no approval workflow) — for a corporate client booking in person at the desk. It
// resolves the company/profile chain then materialises via CreateCorporateMeal.
func (s *CorporateBookingRequestService) WalkInMeal(orgID uuid.UUID, req *models.SubmitMealBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name is required")
	}
	if req.Company == nil || req.Company.Name == "" {
		return nil, errors.New("company name is required")
	}
	if req.Meal == nil || len(req.Meal.Sessions) == 0 {
		return nil, errors.New("at least one meal session is required")
	}
	companyID, profileID, err := s.resolveCorporateChain(orgID, req.Company, req.BookedBy)
	if err != nil {
		return nil, err
	}
	return s.booking.CreateCorporateMeal(orgID, &profileID, &companyID, nil, nil, req)
}

// WalkInEvent creates a confirmed corporate event booking directly (no pending state,
// no approval workflow). Resolves the company/profile chain then materialises via
// CreateCorporateEvent.
func (s *CorporateBookingRequestService) WalkInEvent(orgID uuid.UUID, req *models.SubmitEventBookingRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name is required")
	}
	if req.Company == nil || req.Company.Name == "" {
		return nil, errors.New("company name is required")
	}
	if req.Event == nil || len(req.Event.Sessions) == 0 {
		return nil, errors.New("at least one event session is required")
	}
	companyID, profileID, err := s.resolveCorporateChain(orgID, req.Company, req.BookedBy)
	if err != nil {
		return nil, err
	}
	return s.booking.CreateCorporateEvent(orgID, &profileID, &companyID, nil, nil, req)
}

// WalkInAccommodation creates a confirmed corporate accommodation booking directly
// (no pending state, no approval). Resolves the company/profile chain, then
// materialises the booking + per-attendant room assignments. matReq carries one
// assignment (guest_index → room_id) per attendant in the envelope.
func (s *CorporateBookingRequestService) WalkInAccommodation(orgID uuid.UUID, branchID *uuid.UUID, req *models.SubmitAccommodationRequest, matReq *models.MaterialiseRequest) (*models.Booking, error) {
	if req.BookedBy.Name == "" {
		return nil, errors.New("booked_by name is required")
	}
	if req.Company == nil || req.Company.Name == "" {
		return nil, errors.New("company name is required")
	}
	if req.Accommodation == nil {
		return nil, errors.New("accommodation block is required")
	}
	if matReq == nil || len(matReq.Assignments) == 0 {
		return nil, errors.New("room assignments are required")
	}
	companyID, profileID, err := s.resolveCorporateChain(orgID, req.Company, req.BookedBy)
	if err != nil {
		return nil, err
	}
	return s.booking.CreateCorporateAccommodation(orgID, branchID, &profileID, &companyID, req, matReq)
}

// ─── Backoffice ───────────────────────────────────────────────────────────────

// GetByID returns a pending corporate booking shaped as a CorporateBookingRequest so
// the existing back-office task screen (which reads .payload/.booking_type) keeps
// working. Single-phase: the id is a booking ID and the payload comes from metadata.
func (s *CorporateBookingRequestService) GetByID(id, orgID uuid.UUID) (*models.CorporateBookingRequest, error) {
	if s.booking == nil {
		return nil, errors.New("booking service not configured")
	}
	b, err := s.booking.GetForApproval(id, orgID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	req := bookingToCorporateRequest(b)
	if req.BookingType == models.CorporateBookingTypeMeals {
		req.MealsSummary = s.buildMealsSummary(orgID, req.Payload)
	}
	return req, nil
}

// bookingToCorporateRequest maps a booking back into the legacy request shape the
// back-office expects (payload ← metadata, status mapped to request vocabulary).
func bookingToCorporateRequest(b *models.Booking) *models.CorporateBookingRequest {
	return &models.CorporateBookingRequest{
		ID:           b.ID,
		OrgID:        b.OrgID,
		BranchID:     b.BranchID,
		CorProfileID: b.CorProfileID,
		CompanyID:    b.CompanyID,
		WebUserID:    b.WebUserID,
		BookingType:  corporateTypeFromBooking(b.BookingType),
		Status:       requestStatusFromBooking(b.Status),
		Documents:    b.Documents,
		Payload:      b.Metadata,
		ProfileName:  b.BookerName,
		CompanyName:  b.CompanyName,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}

// requestStatusFromBooking maps a booking status to the request-status vocabulary the
// back-office task UI was written against.
func requestStatusFromBooking(s string) string {
	switch s {
	case models.BookingStatusPending:
		return models.CorporateBookingStatusPending
	case models.BookingStatusRejected:
		return models.CorporateBookingStatusRejected
	case models.BookingStatusCancelled:
		return models.CorporateBookingStatusCancelled
	default:
		// confirmed / checked_in / checked_out all mean "approved & materialised"
		return models.CorporateBookingStatusApproved
	}
}

// buildMealsSummary resolves each menu_item_id in a meals payload to its current
// name + price (from menu_items, the same source billing uses) and computes
// per-line, per-guest, and grand totals for back-office display. Items whose menu
// item no longer exists are shown with a fallback name and zero price.
func (s *CorporateBookingRequestService) buildMealsSummary(orgID uuid.UUID, payload json.RawMessage) *models.MealsRequestSummary {
	if s.menuRepo == nil || len(payload) == 0 {
		return nil
	}
	var p models.SubmitMealsRequest
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}

	// Cache lookups so a shared item is only fetched once.
	cache := map[uuid.UUID]*models.MenuItem{}
	resolve := func(in models.CorMealItemInput) models.MealsSummaryItem {
		mi, ok := cache[in.MenuItemID]
		if !ok {
			mi, _ = s.menuRepo.GetMenuItemByID(in.MenuItemID, orgID)
			cache[in.MenuItemID] = mi
		}
		item := models.MealsSummaryItem{
			MenuItemID: in.MenuItemID,
			Quantity:   in.Quantity,
			Notes:      in.Notes,
			Name:       "(unknown item)",
		}
		if mi != nil {
			item.Name = mi.Name
			item.UnitPrice = mi.Price
		}
		item.Subtotal = item.UnitPrice * float64(in.Quantity)
		return item
	}

	summary := &models.MealsRequestSummary{
		From:         p.From,
		To:           p.To,
		Headcount:    p.Headcount,
		DietaryNotes: p.DietaryNotes,
	}

	for _, g := range p.Guests {
		if len(g.Items) == 0 {
			continue
		}
		sg := models.MealsSummaryGuest{
			Name:               strings.TrimSpace(g.FirstName + " " + g.LastName),
			IdentificationCard: g.IdentificationCard,
		}
		for _, in := range g.Items {
			line := resolve(in)
			sg.Items = append(sg.Items, line)
			sg.Subtotal += line.Subtotal
		}
		summary.EstimatedTotal += sg.Subtotal
		summary.Guests = append(summary.Guests, sg)
	}

	for _, in := range p.Items {
		line := resolve(in)
		summary.BuffetItems = append(summary.BuffetItems, line)
		summary.EstimatedTotal += line.Subtotal
	}

	return summary
}

// List returns pending corporate bookings shaped as requests for the back-office
// "booking requests" screen. Single-phase: pending bookings ARE the requests.
func (s *CorporateBookingRequestService) List(orgID uuid.UUID, bookingType, status string, page, pageSize int) ([]models.CorporateBookingRequest, int, error) {
	if s.booking == nil {
		return nil, 0, errors.New("booking service not configured")
	}
	// A request screen shows things awaiting action → pending bookings.
	bookings, total, err := s.booking.List(orgID, models.BookerTypeCorporate, bookingType, models.BookingStatusPending, nil, nil, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.CorporateBookingRequest, 0, len(bookings))
	for i := range bookings {
		out = append(out, *bookingToCorporateRequest(&bookings[i]))
	}
	return out, total, nil
}

// ApproveFromWorkflow / RejectFromWorkflow satisfy the workflow's
// BookingRequestApprover interface, delegating to the same Approve/Reject the
// request endpoints use.
func (s *CorporateBookingRequestService) ApproveFromWorkflow(id, orgID uuid.UUID) error {
	return s.Approve(id, orgID)
}

func (s *CorporateBookingRequestService) RejectFromWorkflow(id, orgID uuid.UUID) error {
	return s.Reject(id, orgID)
}

// Approve promotes a pending corporate booking (id is the booking ID). Event and
// meals bookings are self-contained (venue/menu chosen at submission) and promote in
// place to confirmed immediately. Accommodation needs staff to assign rooms, so it
// stays pending here — the materialise endpoint promotes it once rooms are assigned.
func (s *CorporateBookingRequestService) Approve(id, orgID uuid.UUID) error {
	if s.booking == nil {
		return errors.New("booking service not configured")
	}
	b, err := s.booking.GetForApproval(id, orgID)
	if err != nil {
		return errors.New("booking not found")
	}
	if b.Status == models.BookingStatusConfirmed {
		// Accommodation: materialise already promoted this booking before the
		// workflow step fired. Nothing left to do.
		return nil
	}
	if b.Status != models.BookingStatusPending {
		return errors.New("only pending bookings can be approved")
	}

	switch b.BookingType {
	case models.CorporateBookingTypeEvent:
		var lodgeBranchID *uuid.UUID
		var envelope models.SubmitEventBookingRequest
		if json.Unmarshal(b.Metadata, &envelope) == nil && envelope.Event != nil && len(envelope.Event.Sessions) > 0 {
			if vID, err := uuid.Parse(envelope.Event.Sessions[0].VenueID); err == nil {
				lodgeBranchID = s.venueBranchID(orgID, vID)
			}
		} else {
			var legacy models.SubmitEventRequest
			_ = json.Unmarshal(b.Metadata, &legacy)
			lodgeBranchID = s.venueBranchID(orgID, legacy.VenueID)
		}
		if _, err := s.booking.CreateFromBooking(orgID, lodgeBranchID, id, nil); err != nil {
			return fmt.Errorf("approved but booking promotion failed: %w", err)
		}

	case models.CorporateBookingTypeMeals:
		var envelope models.SubmitMealBookingRequest
		if json.Unmarshal(b.Metadata, &envelope) == nil && envelope.Meal != nil && len(envelope.Meal.Sessions) > 0 {
			if _, err := s.booking.CreateCorporateMeal(orgID, b.CorProfileID, b.CompanyID, b.WebUserID, &id, &envelope); err != nil {
				return fmt.Errorf("approved but meals promotion failed: %w", err)
			}
		} else {
			if _, err := s.booking.CreateFromBooking(orgID, nil, id, nil); err != nil {
				return fmt.Errorf("approved but booking promotion failed: %w", err)
			}
		}

	default:
		// Accommodation: leave pending until staff assign rooms via the materialise
		// endpoint. Nothing to do here — approval is recorded by the workflow itself.
	}
	return nil
}

// Reject marks a pending corporate booking as rejected.
func (s *CorporateBookingRequestService) Reject(id, orgID uuid.UUID) error {
	if s.booking == nil {
		return errors.New("booking service not configured")
	}
	return s.booking.SetStatus(id, orgID, models.BookingStatusRejected, models.BookingStatusPending)
}

// Cancel marks a pending corporate booking as cancelled.
func (s *CorporateBookingRequestService) Cancel(id, orgID uuid.UUID) error {
	if s.booking == nil {
		return errors.New("booking service not configured")
	}
	return s.booking.SetStatus(id, orgID, models.BookingStatusCancelled, models.BookingStatusPending)
}

// ─── Workflow ─────────────────────────────────────────────────────────────────

// startWorflow kicks off the booking-approval workflow for a pending corporate
// booking. TaskID is the booking ID (single-phase): a terminal approve/reject
// promotes or rejects that same booking row. companyName is the display label.
func (s *CorporateBookingRequestService) startWorflow(b *models.Booking, companyName string) {
	if s.workflow == nil {
		return
	}
	go func() {
		taskDetails := models.TaskDetails{
			TaskID:          b.ID.String(),
			TaskRef:         b.ID.String()[:8],
			TaskType:        "corporate_booking",
			TaskDescription: "Corporate " + b.BookingType + " booking request from " + companyName,
			SenderDetails: models.SenderDetails{
				SenderID:   b.ID.String(),
				SenderName: companyName,
				Position:   b.BookingType,
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

// splitName splits a full name string into first and last name.
// Everything after the first space is treated as the last name.
// If there is no space, the whole string is the first name.
func splitName(full string) (first, last string) {
	full = strings.TrimSpace(full)
	i := strings.Index(full, " ")
	if i < 0 {
		return full, ""
	}
	return strings.TrimSpace(full[:i]), strings.TrimSpace(full[i+1:])
}
