package models

import (
	"time"

	"github.com/google/uuid"
)

type Branch struct {
	ID            uuid.UUID `json:"id"`
	OrgID         uuid.UUID `json:"org_id"`
	Name          string    `json:"name"`
	BranchCode    string    `json:"branch_code"`
	StreetAddress *string   `json:"street_address"`
	City          *string   `json:"city"`
	Country       *string   `json:"country"`
	Location      *string   `json:"location"`
	Phone         *string   `json:"phone"`
	Email         *string   `json:"email"`
	IsActive      bool      `json:"is_active"`
	IsMain        bool      `json:"is_main"`
	Parking       bool      `json:"parking"`
	Restaurant    bool      `json:"restaurant"`
	CheckInTime   *string   `json:"check_in_time"`
	CheckOutTime  *string   `json:"check_out_time"`
	// Receipt printer — one physical printer per branch. PrinterIP nil/empty
	// means no printer is configured for this branch yet.
	PrinterIP   *string   `json:"printer_ip"`
	PrinterPort int       `json:"printer_port"`
	PrinterName *string   `json:"printer_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateBranchRequest struct {
	Name          string  `json:"name"`
	BranchCode    string  `json:"branch_code"`
	StreetAddress string  `json:"street_address,omitempty"`
	City          string  `json:"city,omitempty"`
	Country       string  `json:"country,omitempty"`
	Location      string  `json:"location,omitempty"`
	Phone         string  `json:"phone,omitempty"`
	Email         string  `json:"email,omitempty"`
	Parking       bool    `json:"parking,omitempty"`
	Restaurant    bool    `json:"restaurant,omitempty"`
	CheckInTime   *string `json:"check_in_time,omitempty"`
	CheckOutTime  *string `json:"check_out_time,omitempty"`
	PrinterIP     string  `json:"printer_ip,omitempty"`
	PrinterPort   int     `json:"printer_port,omitempty"`
	PrinterName   string  `json:"printer_name,omitempty"`
}

type UpdateBranchRequest struct {
	Name          *string `json:"name,omitempty"`
	BranchCode    *string `json:"branch_code,omitempty"`
	StreetAddress *string `json:"street_address,omitempty"`
	City          *string `json:"city,omitempty"`
	Country       *string `json:"country,omitempty"`
	Location      *string `json:"location,omitempty"`
	Phone         *string `json:"phone,omitempty"`
	Email         *string `json:"email,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
	IsMain        *bool   `json:"is_main,omitempty"`
	Parking       *bool   `json:"parking,omitempty"`
	Restaurant    *bool   `json:"restaurant,omitempty"`
	CheckInTime   *string `json:"check_in_time,omitempty"`
	CheckOutTime  *string `json:"check_out_time,omitempty"`
	PrinterIP     *string `json:"printer_ip,omitempty"`
	PrinterPort   *int    `json:"printer_port,omitempty"`
	PrinterName   *string `json:"printer_name,omitempty"`
}
