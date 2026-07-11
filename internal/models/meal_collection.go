package models

import (
	"time"

	"github.com/google/uuid"
)

// ─── Meal sessions (recurring buffet schedule templates) ──────────────────────

const (
	MealSessionStatusScheduled = "scheduled"
	MealSessionStatusOpen      = "open"
	MealSessionStatusClosed    = "closed"
	MealSessionStatusCancelled = "cancelled"

	MealCardStatusActive   = "active"
	MealCardStatusInactive = "inactive"
	MealCardStatusReplaced = "replaced"
	MealCardStatusVoid     = "void"

	CardRoleResident    = "resident"
	CardRoleRoomService = "room_service"

	CollectMethodCard  = "card"
	CollectMethodTyped = "typed"
)

type ResidentMealSession struct {
	ID                 uuid.UUID  `json:"id"`
	OrgID              uuid.UUID  `json:"org_id"`
	BranchID           *uuid.UUID `json:"branch_id,omitempty"`
	MealPeriod         string     `json:"meal_period"`
	BuffetMenuItemID   uuid.UUID  `json:"buffet_menu_item_id"`
	BuffetName         string     `json:"buffet_name,omitempty"` // denormalized from menu item
	StartTime          string     `json:"start_time"`            // HH:MM
	EndTime            string     `json:"end_time"`              // HH:MM
	DaysOfWeek         []string   `json:"days_of_week"`
	AutoOpenClose      bool       `json:"auto_open_close"`
	Status             string     `json:"status"`
	GracePeriodMinutes int        `json:"grace_period_minutes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type MealSessionCreateRequest struct {
	MealPeriod       string    `json:"meal_period"`
	BuffetMenuItemID uuid.UUID `json:"buffet_menu_item_id"`
	StartTime        string    `json:"start_time"`
	EndTime          string    `json:"end_time"`
	AutoOpenClose    bool      `json:"auto_open_close"`
	DaysOfWeek       []string  `json:"days_of_week"`
}

type MealSessionUpdateRequest struct {
	MealPeriod       *string    `json:"meal_period,omitempty"`
	BuffetMenuItemID *uuid.UUID `json:"buffet_menu_item_id,omitempty"`
	StartTime        *string    `json:"start_time,omitempty"`
	EndTime          *string    `json:"end_time,omitempty"`
	AutoOpenClose    *bool      `json:"auto_open_close,omitempty"`
	DaysOfWeek       []string   `json:"days_of_week,omitempty"`
}

type MealSessionStatusRequest struct {
	Status string `json:"status"`
}

type UpdateGracePeriodRequest struct {
	GracePeriodMinutes int `json:"grace_period_minutes"`
}

type PaginatedMealSessions struct {
	Data     []ResidentMealSession `json:"data"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Total    int                   `json:"total"`
}

// ─── Meal cards ───────────────────────────────────────────────────────────────

type MealCard struct {
	ID                 uuid.UUID  `json:"id"`
	OrgID              uuid.UUID  `json:"org_id"`
	BranchID           *uuid.UUID `json:"branch_id,omitempty"`
	CardUID            string     `json:"card_uid"`
	RoomID             uuid.UUID  `json:"room_id"`
	RoomName           string     `json:"room_name,omitempty"`
	Role               string     `json:"role"`
	AttendeeID         *uuid.UUID `json:"attendee_id,omitempty"`
	HolderName         string     `json:"holder_name,omitempty"`
	IdentificationCard string     `json:"identification_card,omitempty"`
	Status             string     `json:"status"`
	ReplacedCardID     *uuid.UUID `json:"replaced_card_id,omitempty"`
	IssuedAt           time.Time  `json:"issued_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type AssignCardRequest struct {
	CardUID    string     `json:"card_uid"`
	RoomID     uuid.UUID  `json:"room_id"`
	Role       string     `json:"role"`
	AttendeeID *uuid.UUID `json:"attendee_id,omitempty"`
}

// UpdateCardRequest is a partial patch. AttendeeID uses a pointer-to-pointer style
// via ClearAttendee so an explicit null can clear the occupant (undefined leaves
// it unchanged).
type UpdateCardRequest struct {
	RoomID        *uuid.UUID `json:"room_id,omitempty"`
	Role          *string    `json:"role,omitempty"`
	AttendeeID    *uuid.UUID `json:"attendee_id,omitempty"`
	ClearAttendee bool       `json:"-"`                // set when attendee_id was explicitly null
	Status        *string    `json:"status,omitempty"` // active|inactive
}

type ReplaceCardRequest struct {
	NewCardUID string `json:"new_card_uid"`
	Reason     string `json:"reason,omitempty"`
}

// ─── Collect ──────────────────────────────────────────────────────────────────

const (
	CollectResultMatched       = "matched"
	CollectResultNotFound      = "not_found"
	CollectResultAmbiguous     = "ambiguous"
	CollectResultSessionClosed = "session_not_open"
	CollectResultNotPermitted  = "not_permitted"
)

type MealCollectRequest struct {
	Input             string `json:"input"`
	IdempotencyKey    string `json:"idempotency_key"`
	ClientCollectedAt string `json:"client_collected_at"`
}

type ResidentSummary struct {
	AttendeeID         *uuid.UUID `json:"attendee_id,omitempty"`
	FullName           string     `json:"full_name"`
	IdentificationCard string     `json:"identification_card,omitempty"`
	RoomName           string     `json:"room_name,omitempty"`
	BookingID          uuid.UUID  `json:"booking_id"`
	BookingNumber      string     `json:"booking_number,omitempty"`
}

type MealCollectionCharge struct {
	InvoiceID     uuid.UUID `json:"invoice_id"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
	LineItemID    uuid.UUID `json:"line_item_id"`
	Amount        float64   `json:"amount"`
}

type MealCollectionResult struct {
	Result     string                `json:"result"`
	Resident   *ResidentSummary      `json:"resident,omitempty"`
	Candidates []ResidentSummary     `json:"candidates,omitempty"`
	Charge     *MealCollectionCharge `json:"charge,omitempty"`
	Message    string                `json:"message"`
}

type MealCollectionLogEntry struct {
	ID                 uuid.UUID  `json:"id"`
	MealSessionID      uuid.UUID  `json:"meal_session_id"`
	AttendeeID         *uuid.UUID `json:"attendee_id,omitempty"`
	ResidentName       string     `json:"resident_name,omitempty"`
	IdentificationCard string     `json:"identification_card,omitempty"`
	Method             string     `json:"method"`
	CardUID            string     `json:"card_uid,omitempty"`
	RoomName           string     `json:"room_name,omitempty"`
	Amount             *float64   `json:"amount,omitempty"`
	InvoiceID          *uuid.UUID `json:"invoice_id,omitempty"`
	CollectedBy        string     `json:"collected_by"`
	CollectedByName    string     `json:"collected_by_name,omitempty"`
	CollectedAt        time.Time  `json:"collected_at"`
	SyncedAt           *time.Time `json:"synced_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ─── Current stay (powers the card dialog's occupant dropdown) ────────────────

type CurrentStay struct {
	BookingID     uuid.UUID         `json:"booking_id"`
	BookingNumber string            `json:"booking_number,omitempty"`
	RoomID        uuid.UUID         `json:"room_id"`
	RoomName      string            `json:"room_name,omitempty"`
	Attendees     []BookingAttendee `json:"attendees"`
}

// ─── Booking charges primitive ────────────────────────────────────────────────

type PostBookingChargeRequest struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Quantity    int     `json:"quantity,omitempty"`  // defaults to 1
	LineType    string  `json:"line_type,omitempty"` // defaults to meal
}
