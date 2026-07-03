package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	CorporateBookingTypeAccommodation = "accommodation"
	CorporateBookingTypeMeals         = "meals"
	CorporateBookingTypeEvent         = "event"

	CorporateBookingStatusPending   = "pending"
	CorporateBookingStatusApproved  = "approved"
	CorporateBookingStatusRejected  = "rejected"
	CorporateBookingStatusCancelled = "cancelled"
)

type CorporateBookingRequest struct {
	ID                   uuid.UUID  `json:"id"`
	OrgID                uuid.UUID  `json:"org_id"`
	BranchID             *uuid.UUID `json:"branch_id,omitempty"`
	CorProfileID         *uuid.UUID `json:"cor_profile_id,omitempty"`
	CompanyID            *uuid.UUID `json:"company_id,omitempty"`
	WebUserID            *uuid.UUID `json:"web_user_id,omitempty"`
	BookingType          string     `json:"booking_type"`
	Status               string     `json:"status"`
	ReasonForBooking     string     `json:"reason_for_booking,omitempty"`
	Notes                string     `json:"notes,omitempty"`
	AuthoriserName       string     `json:"authoriser_name,omitempty"`
	AuthoriserEmail      string     `json:"authoriser_email,omitempty"`
	AuthoriserPhone      string     `json:"authoriser_phone,omitempty"`
	AuthoriserTitle      string     `json:"authoriser_title,omitempty"`
	AuthoriserDepartment string     `json:"authoriser_department,omitempty"`
	AuthoriserGLCode     string     `json:"authoriser_gl_code,omitempty"`
	Documents            []string   `json:"documents,omitempty"`
	Payload              json.RawMessage   `json:"payload,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// Joined fields
	CompanyName string `json:"company_name,omitempty"`
	BranchName  string `json:"branch_name,omitempty"`
	ProfileName string `json:"profile_name,omitempty"`

	// MealsSummary is populated only for meals requests on detail reads. It resolves
	// each menu_item_id in the payload to its current name + price so the back-office
	// can show what was ordered and the estimated cost before approval.
	MealsSummary *MealsRequestSummary `json:"meals_summary,omitempty"`
}

// MealsRequestSummary is the display-ready, price-resolved view of a meals request.
type MealsRequestSummary struct {
	From          string             `json:"from,omitempty"`
	To            string             `json:"to,omitempty"`
	Headcount     int                `json:"headcount,omitempty"`
	DietaryNotes  string             `json:"dietary_notes,omitempty"`
	Guests        []MealsSummaryGuest `json:"guests,omitempty"`        // itemised, per named guest
	BuffetItems   []MealsSummaryItem `json:"buffet_items,omitempty"`  // top-level / shared items
	EstimatedTotal float64           `json:"estimated_total"`
}

type MealsSummaryGuest struct {
	Name               string             `json:"name"`
	IdentificationCard string             `json:"identification_card,omitempty"`
	Items              []MealsSummaryItem `json:"items"`
	Subtotal           float64            `json:"subtotal"`
}

type MealsSummaryItem struct {
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Name       string    `json:"name"`
	Quantity   int       `json:"quantity"`
	UnitPrice  float64   `json:"unit_price"`
	Subtotal   float64   `json:"subtotal"`
	Notes      string    `json:"notes,omitempty"`
}

// ── Submission payloads per booking type ─────────────────────────────────────

type CorBookingCompanyInput struct {
	CompanyName string `json:"company_name"`
	RegNumber   string `json:"reg_number,omitempty"`
	TPIN        string `json:"tpin,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Country     string `json:"country,omitempty"`
}

type CorBookingBranchInput struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

type CorBookingProfileInput struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone,omitempty"`
	JobTitle   string `json:"job_title,omitempty"`
	ManNumber  string `json:"man_number,omitempty"`
	Department string `json:"department,omitempty"`
}

type CorBookingAuthoriser struct {
	Name       string `json:"name,omitempty"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Title      string `json:"title,omitempty"`
	Department string `json:"department,omitempty"`
	GLCode     string `json:"gl_code,omitempty"`
}

type CorBookingGuestInput struct {
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Email              string `json:"email,omitempty"`
	Phone              string `json:"phone,omitempty"`
	IdentificationCard string `json:"identification_card"`
	CheckIn            string `json:"check_in,omitempty"`
	CheckOut           string `json:"check_out,omitempty"`
	RoomType           string `json:"room_type,omitempty"`
}

// SubmitAccommodationRequest matches the unified envelope from accommodationBooking.js.
// company, approver, booked_by, and accommodation all arrive as nested objects.
type SubmitAccommodationRequest struct {
	OrgID    uuid.UUID  `json:"org_id"`
	BranchID *uuid.UUID `json:"branch_id,omitempty"`

	BookingType    string `json:"booking_type,omitempty"`
	Source         string `json:"source,omitempty"`
	Currency       string `json:"currency,omitempty"`
	BookingContext string `json:"booking_context,omitempty"`

	ParticipantMode  string `json:"participant_mode,omitempty"`
	ParticipantCount *int   `json:"participant_count,omitempty"`

	BookedBy   CorSubmitBookedBy     `json:"booked_by"`
	Attendants []CorBookingAttendant `json:"attendants,omitempty"`

	Company  *CorSubmitCompany  `json:"company,omitempty"`
	Approver *CorSubmitApprover `json:"approver,omitempty"`

	Accommodation *CorSubmitAccommodation `json:"accommodation,omitempty"`

	Documents []string `json:"documents,omitempty"`
}

type CorSubmitBookedBy struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
	JobTitle  string `json:"job_title,omitempty"`
	ManNumber string `json:"man_number,omitempty"`
}

type CorSubmitCompany struct {
	Name           string `json:"name"`
	TPIN           string `json:"tpin,omitempty"`
	Industry       string `json:"industry,omitempty"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	City           string `json:"city,omitempty"`
	StreetAddress  string `json:"street_address,omitempty"`
	BranchName     string `json:"branch_name,omitempty"`
	DepartmentName string `json:"department_name,omitempty"`
	CostCenter     string `json:"cost_center,omitempty"`
	CostCenterType string `json:"cost_center_type,omitempty"`
	GLCode         string `json:"gl_code,omitempty"`
}

type CorSubmitApprover struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	Title string `json:"title,omitempty"`
}

type CorSubmitAccommodation struct {
	CheckIn            string `json:"check_in,omitempty"`
	CheckOut           string `json:"check_out,omitempty"`
	Notes              string `json:"notes,omitempty"`
	RoomCount          int    `json:"room_count,omitempty"`
	RoomTypePreference string `json:"room_type_preference,omitempty"`
}

// CorBookingAttendant is a shared attendee in the corporate booking
// (used for both accommodation and events/meals).
type CorBookingAttendant struct {
	FullName       string `json:"full_name"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	IDNumber       string `json:"id_number,omitempty"`
	DietaryNotes   string `json:"dietary_notes,omitempty"`
	Company        string `json:"company,omitempty"`
	IsLeadContact  bool   `json:"is_lead_contact"`
}

// CorMealItemInput is a menu-item selection on a meals request. Only the item id
// and quantity are sent — the price is looked up server-side from menu_items at
// materialise time, never trusted from the client.
type CorMealItemInput struct {
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Quantity   int       `json:"quantity"`
	Notes      string    `json:"notes,omitempty"`
}

type CorMealGuestInput struct {
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	Email              string             `json:"email,omitempty"`
	IdentificationCard string             `json:"identification_card,omitempty"`
	Items              []CorMealItemInput `json:"items,omitempty"`
}

// SubmitMealsRequest supports two interchangeable shapes that share one engine —
// every selection is a menu item with a quantity, priced from menu_items:
//   - Itemised: named Guests, each with their own Items (per-guest selections).
//   - Buffet:   a Headcount and top-level Items (e.g. one buffet item × headcount).
// Both may appear in a single request; at least one source of items is required.
type SubmitMealsRequest struct {
	OrgID            uuid.UUID             `json:"org_id"`
	BranchID         *uuid.UUID            `json:"branch_id,omitempty"`
	Company          CorBookingCompanyInput `json:"company"`
	Branch           *CorBookingBranchInput `json:"branch,omitempty"`
	Profile          CorBookingProfileInput `json:"booked_by"`
	ReasonForBooking string                `json:"reason_for_booking,omitempty"`
	From             string                `json:"from"`
	To               string                `json:"to"`
	Headcount        int                   `json:"headcount,omitempty"`
	Items            []CorMealItemInput    `json:"items,omitempty"`
	DietaryNotes     string                `json:"dietary_notes,omitempty"`
	Authoriser       *CorBookingAuthoriser `json:"authoriser,omitempty"`
	Guests           []CorMealGuestInput   `json:"guests,omitempty"`
	Documents        []string              `json:"documents,omitempty"`
}

type CorConferenceGuestInput struct {
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Email              string `json:"email,omitempty"`
	IdentificationCard string `json:"identification_card,omitempty"`
}

// MaterialiseGuestAssignment maps a guest (by index in payload.guests) to a real room picked by staff.
type MaterialiseGuestAssignment struct {
	GuestIndex int       `json:"guest_index"`
	RoomID     uuid.UUID `json:"room_id"`
	RoomName   string    `json:"room_name,omitempty"`
	RoomType   string    `json:"room_type,omitempty"`
}

// MaterialiseRequest is the body for POST /api/v1/booking-requests/:id/materialise.
// Accommodation requests use Assignments; conference/event requests use Event.
type MaterialiseRequest struct {
	Assignments []MaterialiseGuestAssignment `json:"assignments,omitempty"`
	Event       *MaterialiseEvent            `json:"event,omitempty"`
}

// MaterialiseEvent is the staff-supplied venue + pricing when turning an approved
// conference/event request into a booking. Price is optional — if zero, the booking
// service falls back to the venue's base_rate.
type MaterialiseEvent struct {
	VenueID   uuid.UUID `json:"venue_id"`
	StartDate string    `json:"start_date,omitempty"`
	EndDate   string    `json:"end_date,omitempty"`
	Price     float64   `json:"price,omitempty"`
}

type SubmitEventRequest struct {
	OrgID            uuid.UUID                 `json:"org_id"`
	BranchID         *uuid.UUID                `json:"branch_id,omitempty"`
	Company          CorBookingCompanyInput     `json:"company"`
	Branch           *CorBookingBranchInput     `json:"branch,omitempty"`
	Profile          CorBookingProfileInput     `json:"booked_by"`
	VenueID          uuid.UUID                 `json:"venue_id"`
	ReasonForBooking string                    `json:"reason_for_booking,omitempty"`
	EventType        string                    `json:"event_type"`
	StartDate        string                    `json:"start_date"`
	EndDate          string                    `json:"end_date,omitempty"`
	StartTime        string                    `json:"start_time"`
	EndTime          string                    `json:"end_time,omitempty"`
	Headcount        int                       `json:"headcount"`
	CateringRequired bool                      `json:"catering_required"`
	Notes            string                    `json:"notes,omitempty"`
	Authoriser       *CorBookingAuthoriser     `json:"authoriser,omitempty"`
	Guests           []CorConferenceGuestInput  `json:"guests,omitempty"`
	Documents        []string                  `json:"documents,omitempty"`
}

// ── Standalone event booking (Flow B) ────────────────────────────────────────
// SubmitEventBookingRequest is the unified envelope from eventBooking.js. Both
// individual and corporate events use this shape; corporate adds company/approver.
type SubmitEventBookingRequest struct {
	OrgID    uuid.UUID  `json:"org_id"`
	BranchID *uuid.UUID `json:"branch_id,omitempty"`

	BookingType    string `json:"booking_type,omitempty"`    // "event"
	Source         string `json:"source,omitempty"`
	Currency       string `json:"currency,omitempty"`
	BookingContext string `json:"booking_context,omitempty"` // "individual" | "corporate"

	ParticipantMode  string `json:"participant_mode,omitempty"`
	ParticipantCount *int   `json:"participant_count,omitempty"`

	BookedBy   CorSubmitBookedBy     `json:"booked_by"`
	Attendants []CorBookingAttendant `json:"attendants,omitempty"`

	Company  *CorSubmitCompany  `json:"company,omitempty"`  // corporate only
	Approver *CorSubmitApprover `json:"approver,omitempty"` // corporate only

	Event *EventBlock `json:"event,omitempty"`

	Documents []string        `json:"documents,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// ── Standalone meal booking (Flow B) ─────────────────────────────────────────
// SubmitMealBookingRequest is the unified envelope from mealBooking.js. Both
// individual and corporate meals use this shape; corporate adds company/approver.
type SubmitMealBookingRequest struct {
	OrgID    uuid.UUID  `json:"org_id"`
	BranchID *uuid.UUID `json:"branch_id,omitempty"`

	BookingType    string `json:"booking_type,omitempty"`    // "meal"
	Source         string `json:"source,omitempty"`
	Currency       string `json:"currency,omitempty"`
	BookingContext string `json:"booking_context,omitempty"` // "individual" | "corporate"

	ParticipantMode  string `json:"participant_mode,omitempty"`
	ParticipantCount *int   `json:"participant_count,omitempty"`

	BookedBy   CorSubmitBookedBy     `json:"booked_by"`
	Attendants []CorBookingAttendant `json:"attendants,omitempty"`

	Company  *CorSubmitCompany  `json:"company,omitempty"`
	Approver *CorSubmitApprover `json:"approver,omitempty"`

	Meal *MealBlock `json:"meal,omitempty"`

	Documents []string `json:"documents,omitempty"`
}

// MealBlock is the schedule + sessions for a standalone meal booking.
type MealBlock struct {
	ReasonForBooking string        `json:"reason_for_booking,omitempty"`
	MealMode         string        `json:"meal_mode,omitempty"`    // "standalone"
	StartDate        string        `json:"start_date,omitempty"`
	EndDate          string        `json:"end_date,omitempty"`
	ScheduleMode     string        `json:"schedule_mode,omitempty"` // "uniform" | "per_day"
	Notes            string        `json:"notes,omitempty"`
	Sessions         []MealSession `json:"sessions,omitempty"`
}

// MealSession maps onto one Order row at materialise time.
type MealSession struct {
	SessionName       string `json:"session_name,omitempty"`
	MealDate          string `json:"meal_date,omitempty"`    // YYYY-MM-DD
	MealPeriod        string `json:"meal_period,omitempty"`  // "breakfast"|"lunch"|"dinner"
	ServingTime       string `json:"serving_time,omitempty"` // HH:MM — scheduled serving time
	ServiceType       string `json:"service_type,omitempty"` // "buffet"|"a_la_carte"|...
	PaxCount          int    `json:"pax_count,omitempty"`
	MenuItemID        string `json:"menu_item_id,omitempty"` // headcount/buffet mode: the buffet item
	DietaryNotes      string `json:"dietary_notes,omitempty"`
	ArrangementsNotes string `json:"arrangements_notes,omitempty"`

	// Detailed mode only — one entry per attendant.
	IndividualOrders []MealIndividualOrder `json:"individual_orders,omitempty"`
}

// MealIndividualOrder is a single attendant's item selection within a session.
type MealIndividualOrder struct {
	AttendantIdx int    `json:"attendant_idx"`
	MenuItemID   string `json:"menu_item_id"`
	Quantity     int    `json:"quantity"`
	Notes        string `json:"notes,omitempty"`
}

// EventBlock is the schedule + sessions for a standalone event booking.
type EventBlock struct {
	ReasonForBooking string         `json:"reason_for_booking,omitempty"`
	StartDate        string         `json:"start_date,omitempty"`
	EndDate          string         `json:"end_date,omitempty"`
	ScheduleMode     string         `json:"schedule_mode,omitempty"` // "uniform" | "per_day"
	Notes            string         `json:"notes,omitempty"`
	Sessions         []EventSession `json:"sessions,omitempty"`
}

// EventSession maps one-to-one onto a booking_events row at materialise time.
type EventSession struct {
	EventName           string `json:"event_name,omitempty"`
	EventType           string `json:"event_type,omitempty"`
	EventDate           string `json:"event_date,omitempty"`
	StartTime           string `json:"start_time,omitempty"`
	EndTime             string `json:"end_time,omitempty"`
	ExpectedAttendees   int    `json:"expected_attendees,omitempty"`
	SetupType           string `json:"setup_type,omitempty"`
	VenueID             string `json:"venue_id,omitempty"`
	VenueName           string `json:"venue_name,omitempty"`
	VenueCapacity       *int   `json:"venue_capacity,omitempty"`
	PricingBasis        string `json:"pricing_basis,omitempty"`
	SpecialRequirements string `json:"special_requirements,omitempty"`
}
