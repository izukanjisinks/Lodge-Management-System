package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoomTypeSingle     = "single"
	RoomTypeDouble     = "double"
	RoomTypeSuite      = "suite"
	RoomTypeCabin      = "cabin"
	RoomTypeConference = "conference"
)

var ValidRoomTypes = map[string]bool{
	RoomTypeSingle:     true,
	RoomTypeDouble:     true,
	RoomTypeSuite:      true,
	RoomTypeCabin:      true,
	RoomTypeConference: true,
}

type BookedDate struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Status   string `json:"status"`
}

type RoomOrganization struct {
	Name          string `json:"name"`
	Email         string `json:"email,omitempty"`
	StreetAddress string `json:"street_address,omitempty"`
	Phone         string `json:"phone,omitempty"`
	LogoURL       string `json:"logo_url,omitempty"`
}

type Room struct {
	ID            uuid.UUID         `json:"id"`
	OrgID         *uuid.UUID        `json:"org_id,omitempty"`
	BranchID      *uuid.UUID        `json:"branch_id,omitempty"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Capacity      int               `json:"capacity"`
	PricePerNight float64           `json:"price_per_night"`
	Amenities     []string          `json:"amenities"`
	Images        []string          `json:"images"`
	IsAvailable   bool              `json:"is_available"`
	Description   string            `json:"description,omitempty"`
	Organization  *RoomOrganization `json:"organization,omitempty"`
	BookedDates   []BookedDate      `json:"booked_dates,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// RoomStatusOccupant is one guest currently checked into a room (one row per
// active booking_room_assignment with status='checked_in').
type RoomStatusOccupant struct {
	AssignmentID  uuid.UUID `json:"assignment_id"`
	BookingID     uuid.UUID `json:"booking_id"`
	BookingNumber string    `json:"booking_number"`
	Name          string    `json:"name"`
	Phone         string    `json:"phone,omitempty"`
	CheckOut      time.Time `json:"check_out"`
	// Overstaying combines the assignment's own booked check-out date (works
	// until the nightly overstay job auto-extends it forward) with the
	// booking-level "overstayed" flag (durable, but can't pinpoint who in a
	// shared room — so a flagged booking marks all its current occupants).
	Overstaying bool `json:"overstaying"`
}

// RoomStatus is a single room plus whoever is currently checked into it —
// the shape the Room Status board renders directly, no client-side fan-out.
type RoomStatus struct {
	Room      Room                 `json:"room"`
	Occupants []RoomStatusOccupant `json:"occupants"`
}
