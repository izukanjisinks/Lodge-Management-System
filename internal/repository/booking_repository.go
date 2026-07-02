package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type BookingRepository struct {
	db *sql.DB
}

func NewBookingRepository() *BookingRepository {
	return &BookingRepository{db: database.DB}
}

func (r *BookingRepository) Begin() (*sql.Tx, error) {
	return r.db.Begin()
}

func (r *BookingRepository) Create(tx *sql.Tx, b *models.Booking) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now

	var metadata interface{}
	if len(b.Metadata) > 0 {
		metadata = []byte(b.Metadata)
	}

	documents := b.Documents
	if documents == nil {
		documents = []string{}
	}

	return tx.QueryRow(`
		INSERT INTO bookings (
			id, org_id, branch_id,
			booking_type, booker_type,
			booker_name, booker_email, booker_phone,
			web_user_id, cor_profile_id, company_id, request_id, venue_id,
			total_amount, status, special_requests, overstayed, documents, metadata,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		) RETURNING booking_number`,
		b.ID, b.OrgID, b.BranchID,
		b.BookingType, b.BookerType,
		b.BookerName, b.BookerEmail, b.BookerPhone,
		b.WebUserID, b.CorProfileID, b.CompanyID, b.RequestID, b.VenueID,
		b.TotalAmount, b.Status, b.SpecialRequests, b.Overstayed, pq.Array(documents), metadata,
		now, now,
	).Scan(&b.BookingNumber)
}

// CreateOrPromote inserts a new booking, OR — if b.ID is already set and names an
// existing pending booking — promotes that placeholder in place (updating its
// fields and flipping pending → the supplied status). Child rows (attendees,
// assignments, events, orders) attach to the same ID either way.
//
// This is the materialise choke point under the single-phase model: a customer
// submission creates a pending booking up-front; on workflow approval the same
// booking is promoted rather than duplicated. Staff walk-ins pass a nil ID and
// always insert.
func (r *BookingRepository) CreateOrPromote(tx *sql.Tx, b *models.Booking) error {
	if b.ID != uuid.Nil {
		var existingNumber string
		err := tx.QueryRow(`
			SELECT booking_number FROM bookings
			WHERE id = $1 AND status = 'pending'`, b.ID).Scan(&existingNumber)
		if err == nil {
			// Promote the placeholder in place.
			b.BookingNumber = existingNumber
			b.UpdatedAt = time.Now()

			var metadata interface{}
			if len(b.Metadata) > 0 {
				metadata = []byte(b.Metadata)
			}
			_, uerr := tx.Exec(`
				UPDATE bookings SET
					branch_id = $2, booking_type = $3, booker_type = $4,
					booker_name = $5, booker_email = $6, booker_phone = $7,
					web_user_id = $8, cor_profile_id = $9, company_id = $10, venue_id = $11,
					total_amount = $12, status = $13, special_requests = $14,
					metadata = $15, updated_at = $16
				WHERE id = $1`,
				b.ID, b.BranchID, b.BookingType, b.BookerType,
				b.BookerName, b.BookerEmail, b.BookerPhone,
				b.WebUserID, b.CorProfileID, b.CompanyID, b.VenueID,
				b.TotalAmount, b.Status, b.SpecialRequests,
				metadata, b.UpdatedAt,
			)
			return uerr
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// b.ID set but no pending row — fall through to a normal insert with that ID.
	}
	return r.Create(tx, b)
}

// CreatePending inserts a parent-only booking in the 'pending' state with the full
// submission envelope stored in metadata and no child rows. It runs in its own
// transaction (submission is a single insert). The generated ID is set on b and is
// later used as the workflow reference and the promote target on approval.
func (r *BookingRepository) CreatePending(b *models.Booking) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	b.Status = models.BookingStatusPending
	if err := r.Create(tx, b); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *BookingRepository) GetByID(id, orgID uuid.UUID) (*models.Booking, error) {
	row := r.db.QueryRow(`
		SELECT b.id, b.booking_number, b.org_id, b.branch_id,
		       b.booking_type, b.booker_type,
		       b.booker_name, b.booker_email, b.booker_phone,
		       b.web_user_id, b.cor_profile_id, b.company_id, b.request_id, b.venue_id,
		       COALESCE(cd.company_name, '')      AS company_name,
		       COALESCE(cp.first_name || ' ' || cp.last_name, '') AS profile_name,
		       COALESCE(v.name, '')               AS venue_name,
		       b.total_amount, b.status, b.special_requests, b.overstayed,
		       b.created_at, b.updated_at, b.metadata, b.documents
		FROM bookings b
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		LEFT JOIN cor_profiles        cp ON cp.id = b.cor_profile_id
		LEFT JOIN venues              v  ON v.id  = b.venue_id
		WHERE b.id = $1 AND b.org_id = $2`, id, orgID)
	return scanBooking(row)
}

// buildBookingWhere constructs the shared WHERE clause and args for booking list
// queries. Returns whereStr, args, and the next placeholder index.
func buildBookingWhere(orgID uuid.UUID, bookerType, bookingType, status string, from, to *time.Time) (string, []interface{}, int) {
	args := []interface{}{orgID}
	where := []string{"b.org_id = $1"}
	i := 2

	if bookerType != "" {
		where = append(where, fmt.Sprintf("b.booker_type = $%d", i))
		args = append(args, bookerType)
		i++
	}
	if bookingType != "" {
		where = append(where, fmt.Sprintf("b.booking_type = $%d", i))
		args = append(args, bookingType)
		i++
	} else {
		where = append(where, "b.booking_type <> 'meals'")
	}
	if status != "" {
		where = append(where, fmt.Sprintf("b.status = $%d", i))
		args = append(args, status)
		i++
	} else {
		// Pending bookings await workflow approval; staff act on them via the
		// task screen, not the bookings list. An explicit ?status=pending still
		// surfaces them for anyone who needs that view.
		where = append(where, "b.status <> 'pending'")
	}
	if from != nil {
		where = append(where, fmt.Sprintf("b.created_at >= $%d", i))
		args = append(args, *from)
		i++
	}
	if to != nil {
		where = append(where, fmt.Sprintf("b.created_at <= $%d", i))
		args = append(args, *to)
		i++
	}
	return strings.Join(where, " AND "), args, i
}

// List returns paginated bookings for the back-office table. Only columns
// needed to render the table rows are selected — no metadata, no child joins.
func (r *BookingRepository) List(orgID uuid.UUID, bookerType, bookingType, status string, from, to *time.Time, page, pageSize int) ([]models.Booking, int, error) {
	whereStr, args, i := buildBookingWhere(orgID, bookerType, bookingType, status, from, to)

	var total int
	if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM bookings b WHERE %s`, whereStr), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	limitClause := fmt.Sprintf("LIMIT $%d OFFSET $%d", i, i+1)

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT b.id, b.booking_number, b.org_id, b.branch_id,
		       b.booking_type, b.booker_type,
		       b.booker_name, b.booker_email, b.booker_phone,
		       b.web_user_id, b.cor_profile_id, b.company_id, b.request_id, b.venue_id,
		       COALESCE(cd.company_name, '')                        AS company_name,
		       COALESCE(cp.first_name || ' ' || cp.last_name, '')   AS profile_name,
		       COALESCE(v.name, '')                                  AS venue_name,
		       COALESCE(br.name, '')                                 AS branch_name,
		       b.total_amount, b.status, b.special_requests, b.overstayed,
		       b.created_at, b.updated_at
		FROM bookings b
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		LEFT JOIN cor_profiles        cp ON cp.id = b.cor_profile_id
		LEFT JOIN venues              v  ON v.id  = b.venue_id
		LEFT JOIN branches            br ON br.id = b.branch_id
		WHERE %s
		ORDER BY b.created_at DESC
		%s`, whereStr, limitClause), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var b models.Booking
		var branchID, webUserID, corProfileID, companyID, requestID, venueID uuid.NullUUID
		var bookerEmail, bookerPhone, specialRequests sql.NullString
		var companyName, profileName, venueName, branchName sql.NullString

		if err := rows.Scan(
			&b.ID, &b.BookingNumber, &b.OrgID, &branchID,
			&b.BookingType, &b.BookerType,
			&b.BookerName, &bookerEmail, &bookerPhone,
			&webUserID, &corProfileID, &companyID, &requestID, &venueID,
			&companyName, &profileName, &venueName, &branchName,
			&b.TotalAmount, &b.Status, &specialRequests, &b.Overstayed,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if branchID.Valid    { b.BranchID    = &branchID.UUID }
		if webUserID.Valid   { b.WebUserID   = &webUserID.UUID }
		if corProfileID.Valid { b.CorProfileID = &corProfileID.UUID }
		if companyID.Valid   { b.CompanyID   = &companyID.UUID }
		if requestID.Valid   { b.RequestID   = &requestID.UUID }
		if venueID.Valid     { b.VenueID     = &venueID.UUID }
		if bookerEmail.Valid     { b.BookerEmail     = bookerEmail.String }
		if bookerPhone.Valid     { b.BookerPhone     = bookerPhone.String }
		if specialRequests.Valid { b.SpecialRequests = specialRequests.String }
		if companyName.Valid  { b.CompanyName  = companyName.String }
		if profileName.Valid  { b.ProfileName  = profileName.String }
		if venueName.Valid    { b.VenueName    = venueName.String }
		if branchName.Valid   { b.BranchName   = branchName.String }

		bookings = append(bookings, b)
	}
	return bookings, total, rows.Err()
}

// ListForExport fetches all matching bookings for CSV export. Includes the lead
// room assignment (room name, dates, nights) needed for the individual CSV
// columns. No metadata, no pagination cap.
func (r *BookingRepository) ListForExport(orgID uuid.UUID, bookerType, bookingType, status string, from, to *time.Time) ([]models.Booking, error) {
	whereStr, args, _ := buildBookingWhere(orgID, bookerType, bookingType, status, from, to)

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT b.id, b.booking_number, b.org_id, b.branch_id,
		       b.booking_type, b.booker_type,
		       b.booker_name, b.booker_email, b.booker_phone,
		       b.web_user_id, b.cor_profile_id, b.company_id, b.request_id, b.venue_id,
		       COALESCE(cd.company_name, '')                        AS company_name,
		       COALESCE(cp.first_name || ' ' || cp.last_name, '')   AS profile_name,
		       COALESCE(v.name, '')                                  AS venue_name,
		       COALESCE(br.name, '')                                 AS branch_name,
		       b.total_amount, b.status, b.special_requests, b.overstayed,
		       b.created_at, b.updated_at,
		       asg.id, asg.room_id, asg.check_in, asg.check_out, asg.status, COALESCE(r.name, '')
		FROM bookings b
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		LEFT JOIN cor_profiles        cp ON cp.id = b.cor_profile_id
		LEFT JOIN venues              v  ON v.id  = b.venue_id
		LEFT JOIN branches            br ON br.id = b.branch_id
		LEFT JOIN LATERAL (
		    SELECT bra.id, bra.room_id, bra.check_in, bra.check_out, bra.status
		    FROM booking_room_assignments bra
		    WHERE bra.booking_id = b.id
		    ORDER BY bra.check_in ASC LIMIT 1
		) asg ON TRUE
		LEFT JOIN rooms r ON r.id = asg.room_id
		WHERE %s
		ORDER BY b.created_at DESC`, whereStr), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var b models.Booking
		var branchID, webUserID, corProfileID, companyID, requestID, venueID uuid.NullUUID
		var bookerEmail, bookerPhone, specialRequests sql.NullString
		var companyName, profileName, venueName, branchName sql.NullString
		var asgID, asgRoomID uuid.NullUUID
		var asgCheckIn, asgCheckOut sql.NullTime
		var asgStatus, asgRoomName sql.NullString

		if err := rows.Scan(
			&b.ID, &b.BookingNumber, &b.OrgID, &branchID,
			&b.BookingType, &b.BookerType,
			&b.BookerName, &bookerEmail, &bookerPhone,
			&webUserID, &corProfileID, &companyID, &requestID, &venueID,
			&companyName, &profileName, &venueName, &branchName,
			&b.TotalAmount, &b.Status, &specialRequests, &b.Overstayed,
			&b.CreatedAt, &b.UpdatedAt,
			&asgID, &asgRoomID, &asgCheckIn, &asgCheckOut, &asgStatus, &asgRoomName,
		); err != nil {
			return nil, err
		}
		if branchID.Valid    { b.BranchID    = &branchID.UUID }
		if webUserID.Valid   { b.WebUserID   = &webUserID.UUID }
		if corProfileID.Valid { b.CorProfileID = &corProfileID.UUID }
		if companyID.Valid   { b.CompanyID   = &companyID.UUID }
		if requestID.Valid   { b.RequestID   = &requestID.UUID }
		if venueID.Valid     { b.VenueID     = &venueID.UUID }
		if bookerEmail.Valid     { b.BookerEmail     = bookerEmail.String }
		if bookerPhone.Valid     { b.BookerPhone     = bookerPhone.String }
		if specialRequests.Valid { b.SpecialRequests = specialRequests.String }
		if companyName.Valid  { b.CompanyName  = companyName.String }
		if profileName.Valid  { b.ProfileName  = profileName.String }
		if venueName.Valid    { b.VenueName    = venueName.String }
		if branchName.Valid   { b.BranchName   = branchName.String }

		if asgID.Valid {
			nights := 0
			if asgCheckIn.Valid && asgCheckOut.Valid {
				nights = int(asgCheckOut.Time.Sub(asgCheckIn.Time).Hours() / 24)
			}
			a := models.BookingRoomAssignment{
				ID:        asgID.UUID,
				BookingID: b.ID,
				Status:    asgStatus.String,
				RoomName:  asgRoomName.String,
				Nights:    nights,
			}
			if asgRoomID.Valid  { a.RoomID   = asgRoomID.UUID }
			if asgCheckIn.Valid { a.CheckIn  = asgCheckIn.Time }
			if asgCheckOut.Valid { a.CheckOut = asgCheckOut.Time }
			b.Assignments = []models.BookingRoomAssignment{a}
		}

		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

func (r *BookingRepository) GetByIDUnscoped(id uuid.UUID) (*models.Booking, error) {
	row := r.db.QueryRow(`
		SELECT b.id, b.booking_number, b.org_id, b.branch_id,
		       b.booking_type, b.booker_type,
		       b.booker_name, b.booker_email, b.booker_phone,
		       b.web_user_id, b.cor_profile_id, b.company_id, b.request_id, b.venue_id,
		       COALESCE(cd.company_name, '')                   AS company_name,
		       COALESCE(cp.first_name || ' ' || cp.last_name, '') AS profile_name,
		       COALESCE(v.name, '')                            AS venue_name,
		       b.total_amount, b.status, b.special_requests, b.overstayed,
		       b.created_at, b.updated_at, b.metadata, b.documents
		FROM bookings b
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		LEFT JOIN cor_profiles        cp ON cp.id = b.cor_profile_id
		LEFT JOIN venues              v  ON v.id  = b.venue_id
		WHERE b.id = $1`, id)
	return scanBooking(row)
}

func (r *BookingRepository) ListByWebUserID(webUserID uuid.UUID, page, pageSize int) ([]models.Booking, int, error) {
	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM bookings WHERE web_user_id = $1`, webUserID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`
		SELECT b.id, b.booking_number, b.org_id, b.branch_id,
		       b.booking_type, b.booker_type,
		       b.booker_name, b.booker_email, b.booker_phone,
		       b.web_user_id, b.cor_profile_id, b.company_id, b.request_id, b.venue_id,
		       COALESCE(cd.company_name, '')                      AS company_name,
		       COALESCE(cp.first_name || ' ' || cp.last_name, '') AS profile_name,
		       COALESCE(v.name, '')                               AS venue_name,
		       b.total_amount, b.status, b.special_requests, b.overstayed,
		       b.created_at, b.updated_at, b.metadata, b.documents
		FROM bookings b
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		LEFT JOIN cor_profiles        cp ON cp.id = b.cor_profile_id
		LEFT JOIN venues              v  ON v.id  = b.venue_id
		WHERE b.web_user_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`, webUserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, 0, err
		}
		bookings = append(bookings, *b)
	}
	return bookings, total, rows.Err()
}

func (r *BookingRepository) UpdateStatus(tx *sql.Tx, id, orgID uuid.UUID, status string) error {
	_, err := tx.Exec(`
		UPDATE bookings SET status=$1, updated_at=$2 WHERE id=$3 AND org_id=$4`,
		status, time.Now(), id, orgID)
	return err
}

func (r *BookingRepository) UpdateTotalAmount(tx *sql.Tx, id, orgID uuid.UUID, amount float64) error {
	_, err := tx.Exec(`
		UPDATE bookings SET total_amount=$1, updated_at=$2 WHERE id=$3 AND org_id=$4`,
		amount, time.Now(), id, orgID)
	return err
}

// UpdateVenueAndTotal pins the chosen venue and hire charge on an event booking.
// The booking row is inserted before the venue is resolved, so this writes both
// back in one update during materialisation.
func (r *BookingRepository) UpdateVenueAndTotal(tx *sql.Tx, id, orgID, venueID uuid.UUID, amount float64) error {
	_, err := tx.Exec(`
		UPDATE bookings SET venue_id=$1, total_amount=$2, updated_at=$3 WHERE id=$4 AND org_id=$5`,
		venueID, amount, time.Now(), id, orgID)
	return err
}

func (r *BookingRepository) ExtendCheckout(id, orgID uuid.UUID, newDate time.Time) error {
	_, err := r.db.Exec(`
		UPDATE bookings SET updated_at=$1 WHERE id=$2 AND org_id=$3`,
		time.Now(), id, orgID)
	return err
}

func (r *BookingRepository) MarkOverstayed(id, orgID uuid.UUID) error {
	_, err := r.db.Exec(`
		UPDATE bookings SET overstayed=TRUE, updated_at=$1 WHERE id=$2 AND org_id=$3`,
		time.Now(), id, orgID)
	return err
}

// OverdueBookingRef carries details the nightly job needs to flag overstayed bookings.
type OverdueBookingRef struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	BookingNumber string
	BookerName    string
	OriginalCheckOut time.Time
}

func (r *BookingRepository) FindOverdueCheckouts(orgIDs []uuid.UUID) ([]OverdueBookingRef, error) {
	rows, err := r.db.Query(`
		SELECT b.id, b.org_id, b.booking_number, b.booker_name,
		       MAX(a.check_out) AS latest_checkout
		FROM bookings b
		JOIN booking_room_assignments a ON a.booking_id = b.id
		WHERE b.status = 'checked_in'
		  AND a.check_out < CURRENT_DATE
		  AND b.org_id = ANY($1)
		GROUP BY b.id, b.org_id, b.booking_number, b.booker_name`, orgIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []OverdueBookingRef
	for rows.Next() {
		var ref OverdueBookingRef
		if err := rows.Scan(&ref.ID, &ref.OrgID, &ref.BookingNumber, &ref.BookerName, &ref.OriginalCheckOut); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

type bookingScanner interface {
	Scan(dest ...interface{}) error
}

func scanBooking(row bookingScanner) (*models.Booking, error) {
	var b models.Booking
	var branchID, webUserID, corProfileID, companyID, requestID, venueID uuid.NullUUID
	var bookerEmail, bookerPhone, specialRequests sql.NullString
	var companyName, profileName, venueName sql.NullString
	var metadata []byte

	err := row.Scan(
		&b.ID, &b.BookingNumber, &b.OrgID, &branchID,
		&b.BookingType, &b.BookerType,
		&b.BookerName, &bookerEmail, &bookerPhone,
		&webUserID, &corProfileID, &companyID, &requestID, &venueID,
		&companyName, &profileName, &venueName,
		&b.TotalAmount, &b.Status, &specialRequests, &b.Overstayed,
		&b.CreatedAt, &b.UpdatedAt,
		&metadata, pq.Array(&b.Documents),
	)
	if err != nil {
		return nil, err
	}
	if branchID.Valid {
		b.BranchID = &branchID.UUID
	}
	if webUserID.Valid {
		b.WebUserID = &webUserID.UUID
	}
	if corProfileID.Valid {
		b.CorProfileID = &corProfileID.UUID
	}
	if companyID.Valid {
		b.CompanyID = &companyID.UUID
	}
	if requestID.Valid {
		b.RequestID = &requestID.UUID
	}
	if venueID.Valid {
		b.VenueID = &venueID.UUID
	}
	if bookerEmail.Valid {
		b.BookerEmail = bookerEmail.String
	}
	if bookerPhone.Valid {
		b.BookerPhone = bookerPhone.String
	}
	if specialRequests.Valid {
		b.SpecialRequests = specialRequests.String
	}
	if companyName.Valid {
		b.CompanyName = companyName.String
	}
	if profileName.Valid {
		b.ProfileName = profileName.String
	}
	if venueName.Valid {
		b.VenueName = venueName.String
	}
	if len(metadata) > 0 {
		b.Metadata = metadata
	}
	return &b, nil
}
