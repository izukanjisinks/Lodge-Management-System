package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DateOnly is a time.Time that marshals/unmarshals as "YYYY-MM-DD".
type DateOnly struct{ time.Time }

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05.999Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			d.Time = t.UTC().Truncate(24 * time.Hour)
			return nil
		}
	}
	return fmt.Errorf("date must be YYYY-MM-DD or ISO 8601, got %q", s)
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Time.Format("2006-01-02"))
}

// ─── Status constants ─────────────────────────────────────────────────────────

const (
	BookingStatusPending    = "pending"
	BookingStatusConfirmed  = "confirmed"
	BookingStatusCheckedIn  = "checked_in"
	BookingStatusCheckedOut = "checked_out"
	BookingStatusCancelled  = "cancelled"
	BookingStatusRejected   = "rejected"

	BookingTypeRoom       = "accommodation"
	BookingTypeMeals      = "meals"
	BookingTypeConference = "conference"
	BookingTypeEvent      = "event"

	BookerTypeIndividual = "individual"
	BookerTypeCorporate  = "corporate"

	AssignmentStatusPending    = "pending"
	AssignmentStatusConfirmed  = "confirmed"
	AssignmentStatusCheckedIn  = "checked_in"
	AssignmentStatusCheckedOut = "checked_out"
	AssignmentStatusCancelled  = "cancelled"
)

var ValidBookingTransitions = map[string][]string{
	BookingStatusPending:    {BookingStatusConfirmed, BookingStatusCancelled, BookingStatusRejected},
	BookingStatusConfirmed:  {BookingStatusCheckedIn, BookingStatusCancelled},
	BookingStatusCheckedIn:  {BookingStatusCheckedOut},
	BookingStatusCheckedOut: {},
	BookingStatusCancelled:  {},
	BookingStatusRejected:   {},
}

// ─── Core structs ─────────────────────────────────────────────────────────────

type Booking struct {
	ID              uuid.UUID  `json:"id"`
	BookingNumber   string     `json:"booking_number"`
	OrgID           uuid.UUID  `json:"org_id"`
	BranchID        *uuid.UUID `json:"branch_id,omitempty"`
	BookingType     string     `json:"booking_type"`
	BookerType      string     `json:"booker_type"`
	BookerName      string     `json:"booker_name"`
	BookerEmail     string     `json:"booker_email,omitempty"`
	BookerPhone     string     `json:"booker_phone,omitempty"`
	WebUserID       *uuid.UUID `json:"web_user_id,omitempty"`
	CorProfileID    *uuid.UUID `json:"cor_profile_id,omitempty"`
	CompanyID       *uuid.UUID `json:"company_id,omitempty"`
	RequestID       *uuid.UUID `json:"request_id,omitempty"`
	VenueID         *uuid.UUID `json:"venue_id,omitempty"`
	TotalAmount     float64         `json:"total_amount"`
	Status          string          `json:"status"`
	SpecialRequests string          `json:"special_requests,omitempty"`
	Overstayed      bool            `json:"overstayed"`
	Documents       []string        `json:"documents,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	// Joined fields
	CompanyName  string `json:"company_name,omitempty"`
	ProfileName  string `json:"profile_name,omitempty"`
	VenueName    string `json:"venue_name,omitempty"`
	BranchName   string `json:"branch_name,omitempty"`

	// Child data (populated on GetByID)
	Attendees   []BookingAttendee      `json:"attendees,omitempty"`
	Assignments []BookingRoomAssignment `json:"assignments,omitempty"`
	Events      []BookingEvent          `json:"events,omitempty"`
}

// BookingEvent is the venue reservation behind a conference/event booking.
// One per event booking under current scope (single venue, single date range).
type BookingEvent struct {
	ID               uuid.UUID  `json:"id"`
	BookingID        uuid.UUID  `json:"booking_id"`
	VenueID          *uuid.UUID `json:"venue_id,omitempty"`
	EventType        string     `json:"event_type"`
	StartDate        time.Time  `json:"start_date"`
	EndDate          time.Time  `json:"end_date"`
	StartTime        string     `json:"start_time,omitempty"`
	EndTime          string     `json:"end_time,omitempty"`
	PaxCount         int        `json:"pax_count"`
	Price            float64    `json:"price"`
	CateringRequired bool       `json:"catering_required"`
	Notes            string     `json:"notes,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// Joined
	VenueName string `json:"venue_name,omitempty"`
	Days      int    `json:"days,omitempty"`
}

type BookingAttendee struct {
	ID               uuid.UUID  `json:"id"`
	BookingID        uuid.UUID  `json:"booking_id"`
	CorporateGuestID *uuid.UUID `json:"corporate_guest_id,omitempty"`
	FullName         string     `json:"full_name"`
	Email            string     `json:"email,omitempty"`
	Phone            string     `json:"phone,omitempty"`
	IdentificationCard string   `json:"identification_card,omitempty"`
	DietaryNotes     string     `json:"dietary_notes,omitempty"`
	SpecialNeeds     string     `json:"special_needs,omitempty"`
	IsLeadContact    bool       `json:"is_lead_contact"`
	CreatedAt        time.Time  `json:"created_at"`

	// Joined
	RoomAssignment *BookingRoomAssignment `json:"room_assignment,omitempty"`
}

type BookingRoomAssignment struct {
	ID         uuid.UUID  `json:"id"`
	BookingID  uuid.UUID  `json:"booking_id"`
	RoomID     uuid.UUID  `json:"room_id"`
	AttendeeID *uuid.UUID `json:"attendee_id,omitempty"`
	CheckIn    time.Time  `json:"check_in"`  // booked check-in date
	CheckOut   time.Time  `json:"check_out"` // booked check-out date
	Status     string     `json:"status"`
	// Actual timestamps recorded when staff press check-in / check-out. Nil until
	// pressed. Distinct from the booked CheckIn/CheckOut dates above.
	CheckedInAt  *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt *time.Time `json:"checked_out_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Joined
	RoomName    string  `json:"room_name,omitempty"`
	AttendeeName string `json:"attendee_name,omitempty"`
	Nights      int     `json:"nights,omitempty"`
	RoomCost    float64 `json:"room_cost,omitempty"`
}

// ─── Request structs ──────────────────────────────────────────────────────────

type CreateIndividualBookingRequest struct {
	// Booker — existing web_user or walk-in details
	WebUserID          *uuid.UUID `json:"web_user_id,omitempty"`
	BookerName         string     `json:"booker_name"`
	BookerEmail        string     `json:"booker_email,omitempty"`
	BookerPhone        string     `json:"booker_phone,omitempty"`
	IdentificationCard string     `json:"identification_card,omitempty"`

	// Room stay
	RoomID          uuid.UUID       `json:"room_id"`
	CheckIn         DateOnly        `json:"check_in"`
	CheckOut        DateOnly        `json:"check_out"`
	SpecialRequests string          `json:"special_requests,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`

	// PromoteID names the pending booking to promote in place (set at workflow
	// approval, so the placeholder created at submission becomes the confirmed
	// booking rather than spawning a duplicate). Nil for staff walk-ins.
	PromoteID *uuid.UUID `json:"-"`
}

type CreateAttendeeRequest struct {
	CorporateGuestID *uuid.UUID `json:"corporate_guest_id,omitempty"`
	FullName         string     `json:"full_name"`
	Email            string     `json:"email,omitempty"`
	Phone            string     `json:"phone,omitempty"`
	IdentificationCard string   `json:"identification_card,omitempty"`
	DietaryNotes     string     `json:"dietary_notes,omitempty"`
	SpecialNeeds     string     `json:"special_needs,omitempty"`
	IsLeadContact    bool       `json:"is_lead_contact"`
}

type UpdateAttendeeRequest struct {
	FullName           *string `json:"full_name,omitempty"`
	Email              *string `json:"email,omitempty"`
	Phone              *string `json:"phone,omitempty"`
	IdentificationCard *string `json:"identification_card,omitempty"`
	DietaryNotes       *string `json:"dietary_notes,omitempty"`
	SpecialNeeds       *string `json:"special_needs,omitempty"`
	IsLeadContact      *bool   `json:"is_lead_contact,omitempty"`
}

type CreateRoomAssignmentRequest struct {
	RoomID     uuid.UUID  `json:"room_id"`
	AttendeeID *uuid.UUID `json:"attendee_id,omitempty"`
	CheckIn    DateOnly   `json:"check_in"`
	CheckOut   DateOnly   `json:"check_out"`
}

type UpdateRoomAssignmentRequest struct {
	RoomID   *uuid.UUID `json:"room_id,omitempty"`
	CheckIn  *DateOnly  `json:"check_in,omitempty"`
	CheckOut *DateOnly  `json:"check_out,omitempty"`
}

type UpdateBookingStatusRequest struct {
	Status string `json:"status"`
}
