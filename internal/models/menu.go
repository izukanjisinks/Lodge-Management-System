package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Menu struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	BranchID    *uuid.UUID `json:"branch_id,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// MenuItemsPage is the paginated wrapper for menu items embedded in MenuResponse.
type MenuItemsPage struct {
	Data     []MenuItem `json:"data"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
}

// MenuResponse is the full menu payload — menu details plus a paginated items list.
type MenuResponse struct {
	Menu
	Items MenuItemsPage `json:"items"`
}

type MenuItem struct {
	ID          uuid.UUID       `json:"id"`
	MenuID      uuid.UUID       `json:"menu_id"`
	OrgID       uuid.UUID       `json:"org_id"`
	BranchID    *uuid.UUID      `json:"branch_id,omitempty"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Category       string          `json:"category,omitempty"`
	ProductionArea string          `json:"production_area,omitempty"` // kitchen|bakery|bar|grill — station that prepares this item
	ImageURL       *string         `json:"image_url,omitempty"`
	Price          float64         `json:"price"`
	IsAvailable    bool            `json:"is_available"`
	BuffetData     json.RawMessage `json:"buffet_data,omitempty"` // structured buffet payload; only set when category = 'buffet'
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type UpdateMenuRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

type CreateMenuItemRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Category       string          `json:"category,omitempty"`
	ProductionArea string          `json:"production_area,omitempty"`
	ImageURL       *string         `json:"image_url,omitempty"`
	Price          float64         `json:"price"`
	BuffetData     json.RawMessage `json:"buffet_data,omitempty"`
}

type UpdateMenuItemRequest struct {
	Name           *string         `json:"name,omitempty"`
	Description    *string         `json:"description,omitempty"`
	Category       *string         `json:"category,omitempty"`
	ProductionArea *string         `json:"production_area,omitempty"`
	ImageURL       *string         `json:"image_url,omitempty"`
	Price          *float64        `json:"price,omitempty"`
	IsAvailable    *bool           `json:"is_available,omitempty"`
	BuffetData     json.RawMessage `json:"buffet_data,omitempty"`
}
