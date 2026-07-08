package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

const (
	OrderTypeInHouse = "in_house"
	OrderTypeWalkIn  = "walk_in"

	OrderStatusOpen   = "open"
	OrderStatusClosed = "closed"

	KitchenStatusNew      = "new"
	KitchenStatusPreparing = "preparing"
	KitchenStatusReady    = "ready"
)

type Order struct {
	ID            uuid.UUID    `json:"id"`
	OrgID         uuid.UUID    `json:"org_id"`
	BranchID      *uuid.UUID   `json:"branch_id,omitempty"`
	BookingID     *uuid.UUID   `json:"booking_id,omitempty"`
	AttendeeID    *uuid.UUID   `json:"attendee_id,omitempty"`
	BookingNumber string       `json:"booking_number,omitempty"`
	RoomName      string       `json:"room_name,omitempty"`
	ClientName    string       `json:"client_name,omitempty"`
	CompanyName   string       `json:"company_name,omitempty"`
	AttendeeName  string       `json:"attendee_name,omitempty"`
	OrderNumber   string       `json:"order_number"`
	Type          string       `json:"type"`
	Status        string       `json:"status"`
	KitchenStatus string       `json:"kitchen_status"`
	Notes         string       `json:"notes,omitempty"`
	ScheduledFor  *time.Time   `json:"scheduled_for,omitempty"` // meal session date
	MealPeriod    string       `json:"meal_period,omitempty"`   // breakfast|lunch|dinner
	ServingTime   string       `json:"serving_time,omitempty"`  // HH:MM — scheduled serving time
	Total         float64      `json:"total"`
	Items         []OrderItem  `json:"items,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// NullDate wraps sql.NullTime for scanning a nullable DATE column.
type NullDate = sql.NullTime

type OrderItem struct {
	ID           uuid.UUID  `json:"id"`
	OrderID      uuid.UUID  `json:"order_id"`
	MenuItemID   uuid.UUID  `json:"menu_item_id"`
	AttendeeID   *uuid.UUID `json:"attendee_id,omitempty"`
	AttendeeName string     `json:"attendee_name,omitempty"`
	ItemName     string     `json:"item_name,omitempty"`
	Quantity     int        `json:"quantity"`
	UnitPrice    float64    `json:"unit_price"`
	Subtotal     float64    `json:"subtotal"`
	Notes        string     `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// PlaceOrderRequest creates a new in-house order tied to a booking.
type PlaceOrderRequest struct {
	BookingID  uuid.UUID  `json:"booking_id"`
	AttendeeID *uuid.UUID `json:"attendee_id,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Items      []PlaceOrderItemRequest `json:"items"`
}

// PlaceWalkInOrderRequest creates a new walk-in order with no booking.
type PlaceWalkInOrderRequest struct {
	Notes string                 `json:"notes,omitempty"`
	Items []PlaceOrderItemRequest `json:"items"`
}

type PlaceOrderItemRequest struct {
	MenuItemID uuid.UUID  `json:"menu_item_id"`
	AttendeeID *uuid.UUID `json:"attendee_id,omitempty"`
	Quantity   int        `json:"quantity"`
	Notes      string     `json:"notes,omitempty"`
}

// AddOrderItemsRequest appends items to an existing order.
type AddOrderItemsRequest struct {
	Items []PlaceOrderItemRequest `json:"items"`
}
