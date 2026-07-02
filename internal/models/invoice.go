package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	InvoiceStatusDraft     = "draft"
	InvoiceStatusIssued    = "issued"
	InvoiceStatusPaid      = "paid"
	InvoiceStatusOverdue   = "overdue"
	InvoiceStatusCancelled = "cancelled"
)

var ValidInvoiceTransitions = map[string][]string{
	InvoiceStatusDraft:     {InvoiceStatusIssued, InvoiceStatusCancelled},
	InvoiceStatusIssued:    {InvoiceStatusPaid, InvoiceStatusOverdue, InvoiceStatusCancelled},
	InvoiceStatusOverdue:   {InvoiceStatusPaid, InvoiceStatusCancelled},
	InvoiceStatusPaid:      {},
	InvoiceStatusCancelled: {},
}

const (
	LineTypeRoom  = "room"
	LineTypeMeal  = "meal"
	LineTypeEvent = "event"
)

type InvoiceLineItem struct {
	ID          uuid.UUID  `json:"id"`
	InvoiceID   uuid.UUID  `json:"invoice_id"`
	BookingID   *uuid.UUID `json:"booking_id,omitempty"`
	OrderID     *uuid.UUID `json:"order_id,omitempty"`
	OrderItemID *uuid.UUID `json:"order_item_id,omitempty"`
	LineType    string     `json:"line_type,omitempty"`
	Description string     `json:"description"`
	Quantity    int        `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	Total       float64    `json:"total"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Invoice struct {
	ID                uuid.UUID         `json:"id"`
	InvoiceNumber     string            `json:"invoice_number"`
	BookingID         *uuid.UUID        `json:"booking_id,omitempty"`
	CorporateClientID *uuid.UUID        `json:"corporate_client_id,omitempty"`
	ClientID          uuid.UUID         `json:"client_id"`
	ClientName        string            `json:"client_name"`
	ClientType        string            `json:"client_type"`
	ClientEmail       string            `json:"client_email,omitempty"`
	// Corporate billing fields extracted from metadata
	ClientTPIN       string `json:"client_tpin,omitempty"`
	ClientDepartment string `json:"client_department,omitempty"`
	GLCode           string `json:"gl_code,omitempty"`
	CostCenter       string `json:"cost_center,omitempty"`
	CostCenterType   string `json:"cost_center_type,omitempty"`
	InternalOrder    string `json:"internal_order,omitempty"`
	// Approver fields extracted from metadata
	ApproverName  string `json:"approver_name,omitempty"`
	ApproverEmail string `json:"approver_email,omitempty"`
	BranchID          *uuid.UUID        `json:"branch_id,omitempty"`
	LineItems         []InvoiceLineItem `json:"line_items"`
	Subtotal          float64           `json:"subtotal"`
	TaxRate           float64           `json:"tax_rate"`
	TaxAmount         float64           `json:"tax_amount"`
	Total             float64           `json:"total_amount"`
	Status            string            `json:"status"`
	IssuedDate        *time.Time        `json:"issued_date,omitempty"`
	DueDate           *time.Time        `json:"due_date,omitempty"`
	PaidDate             *time.Time        `json:"paid_date,omitempty"`
	ProofOfPaymentURL    string            `json:"proof_of_payment_url,omitempty"`
	Notes                string            `json:"notes,omitempty"`
	Metadata             json.RawMessage   `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// invoiceMetadataShape mirrors the booking metadata payload for field extraction.
type InvoiceMetadataShape struct {
	Company struct {
		TPIN           string `json:"tpin"`
		DepartmentName string `json:"department_name"`
		GLCode         string `json:"gl_code"`
		CostCenter     string `json:"cost_center"`
		CostCenterType string `json:"cost_center_type"`
	} `json:"company"`
	Approver struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"approver"`
}

// HydrateFromMetadata extracts flat billing fields from the stored JSONB payload.
func (inv *Invoice) HydrateFromMetadata() {
	if len(inv.Metadata) == 0 {
		return
	}
	var m InvoiceMetadataShape
	if err := json.Unmarshal(inv.Metadata, &m); err != nil {
		return
	}
	inv.ClientTPIN = m.Company.TPIN
	inv.ClientDepartment = m.Company.DepartmentName
	inv.GLCode = m.Company.GLCode
	inv.CostCenter = m.Company.CostCenter
	inv.CostCenterType = m.Company.CostCenterType
	// cost_center_type drives which of CostCenter / InternalOrder is active
	if m.Company.CostCenterType == "internal_order" {
		inv.InternalOrder = m.Company.CostCenter
		inv.CostCenter = ""
	}
	inv.ApproverName = m.Approver.Name
	inv.ApproverEmail = m.Approver.Email
}

type UpdateInvoiceStatusRequest struct {
	Status            string  `json:"status"`
	PaidDateStr       string  `json:"paid_date,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	ProofOfPaymentURL *string `json:"proof_of_payment_url,omitempty"`

	// Resolved from PaidDateStr after JSON decode.
	PaidDate *time.Time `json:"-"`
}

// ParsePaidDate parses PaidDateStr ("2006-01-02" or RFC 3339) into PaidDate.
func (r *UpdateInvoiceStatusRequest) ParsePaidDate() {
	if r.PaidDateStr == "" {
		return
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, r.PaidDateStr); err == nil {
			r.PaidDate = &t
			return
		}
	}
}
