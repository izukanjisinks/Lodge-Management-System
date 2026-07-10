package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type InvoiceRepository struct {
	db *sql.DB
}

func NewInvoiceRepository() *InvoiceRepository {
	return &InvoiceRepository{db: database.DB}
}

// coalesceLineType defaults an unset line type to "room" (the historical default),
// matching the column's DB default so older callers keep working.
func coalesceLineType(t string) string {
	if t == "" {
		return models.LineTypeRoom
	}
	return t
}

// Create inserts the invoice and all its line items in a single transaction.
func (r *InvoiceRepository) Create(inv *models.Invoice, orgID uuid.UUID) error {
	inv.ID = uuid.New()
	now := time.Now()
	inv.CreatedAt = now
	inv.UpdatedAt = now

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(`
		INSERT INTO invoices
		    (id, invoice_number, booking_id, corporate_client_id, subtotal, tax_rate, tax, total, status, issued_at, due_date, notes, org_id, branch_id, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		inv.ID, inv.InvoiceNumber, inv.BookingID, inv.CorporateClientID,
		inv.Subtotal, inv.TaxRate, inv.TaxAmount, inv.Total,
		inv.Status, inv.IssuedDate, inv.DueDate, inv.Notes,
		orgID, inv.BranchID, inv.Metadata, inv.CreatedAt, inv.UpdatedAt,
	)
	if err != nil {
		return err
	}

	for i := range inv.LineItems {
		inv.LineItems[i].ID = uuid.New()
		inv.LineItems[i].InvoiceID = inv.ID
		inv.LineItems[i].CreatedAt = now
		var paxCount sql.NullInt64
		if inv.LineItems[i].PaxCount != nil {
			paxCount = sql.NullInt64{Int64: int64(*inv.LineItems[i].PaxCount), Valid: true}
		}
		_, err = tx.Exec(`
			INSERT INTO invoice_line_items (id, invoice_id, booking_id, order_id, order_item_id, line_type, description, quantity, unit_price, total, attendee_name, pax_count, service_type, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			inv.LineItems[i].ID, inv.ID, inv.LineItems[i].BookingID,
			inv.LineItems[i].OrderID, inv.LineItems[i].OrderItemID,
			coalesceLineType(inv.LineItems[i].LineType),
			inv.LineItems[i].Description, inv.LineItems[i].Quantity,
			inv.LineItems[i].UnitPrice, inv.LineItems[i].Total,
			sql.NullString{String: inv.LineItems[i].AttendeeName, Valid: inv.LineItems[i].AttendeeName != ""},
			paxCount,
			sql.NullString{String: inv.LineItems[i].ServiceType, Valid: inv.LineItems[i].ServiceType != ""},
			inv.LineItems[i].CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *InvoiceRepository) GetByID(id uuid.UUID, orgID uuid.UUID) (*models.Invoice, error) {
	return r.fetchOne(`WHERE i.id = $1 AND i.org_id = $2`, id, orgID)
}

func (r *InvoiceRepository) GetByBookingID(bookingID uuid.UUID, orgID uuid.UUID) (*models.Invoice, error) {
	return r.fetchOne(`WHERE i.booking_id = $1 AND i.org_id = $2`, bookingID, orgID)
}

func (r *InvoiceRepository) List(orgID uuid.UUID, branchID *uuid.UUID, status, clientType string, page, pageSize int) ([]models.Invoice, int, error) {
	args := []interface{}{orgID}
	where := []string{"i.org_id = $1"}
	i := 2

	if branchID != nil {
		where = append(where, fmt.Sprintf("i.branch_id = $%d", i))
		args = append(args, *branchID)
		i++
	}
	if status != "" {
		where = append(where, fmt.Sprintf("i.status = $%d", i))
		args = append(args, status)
		i++
	}
	if clientType != "" {
		where = append(where, fmt.Sprintf("b.booker_type = $%d", i))
		args = append(args, clientType)
		i++
	}

	whereStr := strings.Join(where, " AND ")

	// The bookings join is required in both queries because the client_type filter
	// and client display columns reference it.
	joins := `
		LEFT JOIN bookings            b  ON b.id = i.booking_id
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id`

	var total int
	if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM invoices i %s WHERE %s`, joins, whereStr), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT %s
		FROM invoices i %s
		WHERE %s
		ORDER BY i.created_at DESC
		LIMIT $%d OFFSET $%d`, invoiceSelectColumns, joins, whereStr, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, err
		}
		lineItems, err := r.fetchLineItems(inv.ID)
		if err != nil {
			return nil, 0, err
		}
		inv.LineItems = lineItems
		invoices = append(invoices, *inv)
	}
	return invoices, total, rows.Err()
}

func (r *InvoiceRepository) UpdateDueDate(bookingID uuid.UUID, orgID uuid.UUID, dueDate time.Time) error {
	_, err := r.db.Exec(`
		UPDATE invoices SET due_date=$1, updated_at=$2
		WHERE booking_id=$3 AND org_id=$4`,
		dueDate, time.Now(), bookingID, orgID,
	)
	return err
}

// UpdateRoomLineItem updates the room accommodation line item (the one with no order_id)
// and recalculates the invoice totals atomically.
func (r *InvoiceRepository) UpdateRoomLineItem(bookingID uuid.UUID, orgID uuid.UUID, nights int, pricePerNight float64, roomName string) error {
	inv, err := r.GetByBookingID(bookingID, orgID)
	if err != nil {
		return err
	}

	total := float64(nights) * pricePerNight
	description := fmt.Sprintf("%s — %d night(s) @ %.2f/night", roomName, nights, pricePerNight)
	now := time.Now()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(`
		UPDATE invoice_line_items
		SET description=$1, quantity=$2, unit_price=$3, total=$4
		WHERE invoice_id=$5 AND order_id IS NULL`,
		description, nights, pricePerNight, total, inv.ID,
	)
	if err != nil {
		return err
	}

	// Recalculate invoice totals from all line items
	_, err = tx.Exec(`
		UPDATE invoices
		SET subtotal   = (SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1),
		    tax        = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * tax_rate / 100, 2),
		    total      = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * (1 + tax_rate / 100), 2),
		    due_date   = $2,
		    updated_at = $2
		WHERE id = $1`, inv.ID, now)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *InvoiceRepository) UpdateStatus(id uuid.UUID, orgID uuid.UUID, status string, paidDate *time.Time, notes *string, proofURL *string) error {
	now := time.Now()
	_, err := r.db.Exec(`
		UPDATE invoices
		SET status=$1, paid_date=$2, notes=COALESCE($3, notes),
		    proof_of_payment_url=COALESCE($4, proof_of_payment_url), updated_at=$5
		WHERE id=$6 AND org_id=$7`,
		status, paidDate, notes, proofURL, now, id, orgID,
	)
	return err
}

// invoiceSelectColumns is the shared SELECT list for reading invoices. Client display
// fields come from the denormalized booking row (booker_name/email) and the company
// join for corporate bookings — there is no longer a separate client profile table.
const invoiceSelectColumns = `
	i.id, i.invoice_number, i.booking_id, i.corporate_client_id,
	COALESCE(b.cor_profile_id, b.company_id, b.id) AS client_id,
	COALESCE(b.booker_type, 'individual')          AS client_type,
	COALESCE(NULLIF(cd.company_name, ''), b.booker_name, '') AS client_name,
	COALESCE(b.booker_email, '')                   AS client_email,
	i.subtotal, i.tax_rate, i.tax, i.total,
	i.status, i.issued_at, i.due_date, i.paid_date, i.proof_of_payment_url, i.notes,
	i.metadata, i.created_at, i.updated_at`

// fetchOne is a shared helper used by GetByID and GetByBookingID.
func (r *InvoiceRepository) fetchOne(whereClause string, args ...interface{}) (*models.Invoice, error) {
	row := r.db.QueryRow(fmt.Sprintf(`
		SELECT %s
		FROM invoices i
		LEFT JOIN bookings            b  ON b.id = i.booking_id
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		%s`, invoiceSelectColumns, whereClause), args...)

	inv, err := scanInvoice(row)
	if err != nil {
		return nil, err
	}

	inv.LineItems, err = r.fetchLineItems(inv.ID)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

// AppendOrderLineItem adds a line item linked to a closed order onto the booking's invoice.
// The invoice totals (subtotal, tax, total) are recalculated atomically.
func (r *InvoiceRepository) AppendOrderLineItem(invoiceID uuid.UUID, orgID uuid.UUID, item *models.InvoiceLineItem) error {
	item.ID = uuid.New()
	item.InvoiceID = invoiceID
	item.CreatedAt = time.Now()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var paxCount sql.NullInt64
	if item.PaxCount != nil {
		paxCount = sql.NullInt64{Int64: int64(*item.PaxCount), Valid: true}
	}
	_, err = tx.Exec(`
		INSERT INTO invoice_line_items (id, invoice_id, booking_id, order_id, order_item_id, line_type, description, quantity, unit_price, total, attendee_name, pax_count, service_type, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		item.ID, invoiceID, item.BookingID, item.OrderID, item.OrderItemID,
		coalesceLineType(item.LineType), item.Description, item.Quantity, item.UnitPrice, item.Total,
		sql.NullString{String: item.AttendeeName, Valid: item.AttendeeName != ""}, paxCount, sql.NullString{String: item.ServiceType, Valid: item.ServiceType != ""},
		item.CreatedAt,
	)
	if err != nil {
		return err
	}

	// Recalculate invoice totals from all line items
	_, err = tx.Exec(`
		UPDATE invoices
		SET subtotal   = (SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1),
		    tax        = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * tax_rate / 100, 2),
		    total      = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * (1 + tax_rate / 100), 2),
		    updated_at = $2
		WHERE id = $1 AND org_id = $3`, invoiceID, time.Now(), orgID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// RemoveOrderLineItem deletes the invoice line item tied to a specific order item
// (matched by order_item_id) and recalculates the invoice totals atomically.
// It is a no-op if the invoice or line item doesn't exist.
func (r *InvoiceRepository) RemoveOrderLineItem(bookingID uuid.UUID, orgID uuid.UUID, orderItemID uuid.UUID) error {
	inv, err := r.GetByBookingID(bookingID, orgID)
	if err != nil {
		return nil // no invoice yet, nothing to do
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(`DELETE FROM invoice_line_items WHERE invoice_id=$1 AND order_item_id=$2`, inv.ID, orderItemID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE invoices
		SET subtotal   = (SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1),
		    tax        = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * tax_rate / 100, 2),
		    total      = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * (1 + tax_rate / 100), 2),
		    updated_at = $2
		WHERE id = $1`, inv.ID, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ReplaceRoomLineItems atomically swaps a booking invoice's room lines for a fresh
// set and recalculates totals + due date. Only room lines (line_type='room') are
// touched — meal/event/adjustment lines are preserved. It is a guarded no-op when
// no invoice exists or the invoice is not a draft (never rewrite an issued/paid
// document). Returns whether a regeneration actually happened.
func (r *InvoiceRepository) ReplaceRoomLineItems(bookingID, orgID uuid.UUID, items []models.InvoiceLineItem, dueDate *time.Time) (bool, error) {
	inv, err := r.GetByBookingID(bookingID, orgID)
	if err != nil {
		return false, nil // no invoice yet
	}
	if inv.Status != models.InvoiceStatusDraft {
		return false, nil // don't touch issued/paid/overdue/cancelled invoices
	}

	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM invoice_line_items WHERE invoice_id=$1 AND line_type=$2`,
		inv.ID, models.LineTypeRoom); err != nil {
		return false, err
	}

	now := time.Now()
	for i := range items {
		bID := bookingID
		if _, err = tx.Exec(`
			INSERT INTO invoice_line_items (id, invoice_id, booking_id, order_id, order_item_id, line_type, description, quantity, unit_price, total, created_at)
			VALUES ($1,$2,$3,NULL,NULL,$4,$5,$6,$7,$8,$9)`,
			uuid.New(), inv.ID, bID, models.LineTypeRoom,
			items[i].Description, items[i].Quantity, items[i].UnitPrice, items[i].Total, now,
		); err != nil {
			return false, err
		}
	}

	if _, err = tx.Exec(`
		UPDATE invoices
		SET subtotal   = (SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1),
		    tax        = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * tax_rate / 100, 2),
		    total      = ROUND((SELECT COALESCE(SUM(total), 0) FROM invoice_line_items WHERE invoice_id = $1) * (1 + tax_rate / 100), 2),
		    due_date   = COALESCE($3, due_date),
		    updated_at = $2
		WHERE id = $1 AND org_id = $4`, inv.ID, now, dueDate, orgID); err != nil {
		return false, err
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *InvoiceRepository) fetchLineItems(invoiceID uuid.UUID) ([]models.InvoiceLineItem, error) {
	rows, err := r.db.Query(`
		SELECT id, invoice_id, booking_id, order_id, order_item_id, line_type, description, quantity, unit_price, total, attendee_name, pax_count, service_type, created_at
		FROM invoice_line_items WHERE invoice_id = $1
		ORDER BY booking_id NULLS FIRST, created_at ASC`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.InvoiceLineItem
	for rows.Next() {
		var item models.InvoiceLineItem
		var bookingID, orderID, orderItemID uuid.NullUUID
		var attendeeName, serviceType sql.NullString
		var paxCount sql.NullInt64
		if err := rows.Scan(&item.ID, &item.InvoiceID, &bookingID, &orderID, &orderItemID, &item.LineType, &item.Description, &item.Quantity, &item.UnitPrice, &item.Total, &attendeeName, &paxCount, &serviceType, &item.CreatedAt); err != nil {
			return nil, err
		}
		if bookingID.Valid {
			item.BookingID = &bookingID.UUID
		}
		if orderID.Valid {
			item.OrderID = &orderID.UUID
		}
		if orderItemID.Valid {
			item.OrderItemID = &orderItemID.UUID
		}
		if attendeeName.Valid {
			item.AttendeeName = attendeeName.String
		}
		if paxCount.Valid {
			v := int(paxCount.Int64)
			item.PaxCount = &v
		}
		if serviceType.Valid {
			item.ServiceType = serviceType.String
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.InvoiceLineItem{}
	}
	return items, rows.Err()
}

// GenerateInvoiceNumber produces a sequential human-readable number like INV-2026-0001.
// GenerateInvoiceNumber returns the next INV-<year>-<NNNN> number for the current
// year. It derives NNNN from the MAX existing suffix (not COUNT) so deleting an
// invoice never causes the next number to collide with an existing one.
func (r *InvoiceRepository) GenerateInvoiceNumber() (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("INV-%d-", year)
	var maxSeq int
	// Parse the numeric suffix of this year's invoice numbers and take the max.
	err := r.db.QueryRow(`
		SELECT COALESCE(MAX(CAST(substring(invoice_number FROM '[0-9]+$') AS INTEGER)), 0)
		FROM invoices
		WHERE invoice_number LIKE $1`, prefix+"%").Scan(&maxSeq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxSeq+1), nil
}

type invoiceScanner interface {
	Scan(dest ...interface{}) error
}

func scanInvoice(row invoiceScanner) (*models.Invoice, error) {
	var inv models.Invoice
	var bookingID, corporateClientID uuid.NullUUID
	var clientEmail, notes, proofURL sql.NullString
	var issuedAt, dueDate, paidDate sql.NullTime
	var metadata []byte
	err := row.Scan(
		&inv.ID, &inv.InvoiceNumber, &bookingID, &corporateClientID,
		&inv.ClientID, &inv.ClientType, &inv.ClientName, &clientEmail,
		&inv.Subtotal, &inv.TaxRate, &inv.TaxAmount, &inv.Total,
		&inv.Status, &issuedAt, &dueDate, &paidDate, &proofURL, &notes,
		&metadata, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if bookingID.Valid {
		inv.BookingID = &bookingID.UUID
	}
	if corporateClientID.Valid {
		inv.CorporateClientID = &corporateClientID.UUID
	}
	if clientEmail.Valid {
		inv.ClientEmail = clientEmail.String
	}
	if proofURL.Valid {
		inv.ProofOfPaymentURL = proofURL.String
	}
	if notes.Valid {
		inv.Notes = notes.String
	}
	if issuedAt.Valid {
		inv.IssuedDate = &issuedAt.Time
	}
	if dueDate.Valid {
		inv.DueDate = &dueDate.Time
	}
	if paidDate.Valid {
		inv.PaidDate = &paidDate.Time
	}
	if len(metadata) > 0 {
		inv.Metadata = metadata
		inv.HydrateFromMetadata()
	}
	return &inv, nil
}
