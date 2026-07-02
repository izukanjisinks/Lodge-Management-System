package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ClientTypeIndividual = "individual"
	ClientTypeCorporate  = "corporate"
	ClientStatusActive   = "active"
	ClientStatusInactive = "inactive"
)

type IndividualClient struct {
	ID                uuid.UUID `json:"id"`
	FullName          string    `json:"full_name"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	IDPassportNumber  string    `json:"id_passport_number"`
	Nationality       string    `json:"nationality,omitempty"`
	Status            string    `json:"status"`
	Notes             string    `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CorporateClient is the back-office "Corporate Clients" view model. It is served
// from cor_company_details (the table corporate bookings populate + dedup on
// (org_id, reg_number, tpin)), with the contact person/email/phone enriched from a
// representative cor_profiles row for the company. CompanyRegNumber maps to
// cor_company_details.reg_number.
type CorporateClient struct {
	ID                uuid.UUID `json:"id"`
	CompanyName       string    `json:"company_name"`
	ContactPerson     string    `json:"contact_person"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	CompanyRegNumber  string    `json:"company_reg_number"`
	TPIN              string    `json:"tpin,omitempty"`
	Industry          string    `json:"industry,omitempty"`
	Country           string    `json:"country,omitempty"`
	Status            string    `json:"status"`
	Notes             string    `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type CorporateBookingGuest struct {
	BookingID     string `json:"booking_id"`
	BookingNumber string `json:"booking_number"`
	ClientName    string `json:"client_name"`
	RoomName      string `json:"room_name"`
	CheckIn       string `json:"check_in"`
	CheckOut      string `json:"check_out"`
	Guests        int    `json:"guests"`
	Status        string `json:"status"`
}

type CorporateClientWithBookings struct {
	*CorporateClient
	Documents []string                `json:"documents"`
	Guests    []CorporateBookingGuest `json:"guests"`
}
