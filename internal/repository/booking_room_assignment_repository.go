package repository

import (
	"database/sql"
	"fmt"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type BookingRoomAssignmentRepository struct {
	db *sql.DB
}

func NewBookingRoomAssignmentRepository() *BookingRoomAssignmentRepository {
	return &BookingRoomAssignmentRepository{db: database.DB}
}

func (r *BookingRoomAssignmentRepository) CreateInTx(tx *sql.Tx, a *models.BookingRoomAssignment) error {
	a.ID = uuid.New()
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now

	return tx.QueryRow(`
		INSERT INTO booking_room_assignments (
			id, booking_id, room_id, attendee_id,
			check_in, check_out, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`,
		a.ID, a.BookingID, a.RoomID, a.AttendeeID,
		a.CheckIn, a.CheckOut, a.Status, now, now,
	).Scan(&a.ID)
}

func (r *BookingRoomAssignmentRepository) ListByBookingID(bookingID uuid.UUID) ([]models.BookingRoomAssignment, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.booking_id, a.room_id, a.attendee_id,
		       a.check_in, a.check_out, a.status, a.checked_in_at, a.checked_out_at,
		       a.created_at, a.updated_at,
		       r.name AS room_name,
		       COALESCE(att.full_name, '') AS attendee_name,
		       (a.check_out - a.check_in) AS nights,
		       (a.check_out - a.check_in) * r.price_per_night AS room_cost
		FROM booking_room_assignments a
		JOIN rooms r ON r.id = a.room_id
		LEFT JOIN booking_attendees att ON att.id = a.attendee_id
		WHERE a.booking_id = $1
		ORDER BY a.check_in, r.name`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []models.BookingRoomAssignment
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, *a)
	}
	return assignments, rows.Err()
}

func (r *BookingRoomAssignmentRepository) GetByID(id, bookingID uuid.UUID) (*models.BookingRoomAssignment, error) {
	row := r.db.QueryRow(`
		SELECT a.id, a.booking_id, a.room_id, a.attendee_id,
		       a.check_in, a.check_out, a.status, a.checked_in_at, a.checked_out_at,
		       a.created_at, a.updated_at,
		       r.name AS room_name,
		       COALESCE(att.full_name, '') AS attendee_name,
		       (a.check_out - a.check_in) AS nights,
		       (a.check_out - a.check_in) * r.price_per_night AS room_cost
		FROM booking_room_assignments a
		JOIN rooms r ON r.id = a.room_id
		LEFT JOIN booking_attendees att ON att.id = a.attendee_id
		WHERE a.id = $1 AND a.booking_id = $2`, id, bookingID)
	return scanAssignment(row)
}

func (r *BookingRoomAssignmentRepository) Update(id, bookingID uuid.UUID, req *models.UpdateRoomAssignmentRequest) (*models.BookingRoomAssignment, error) {
	_, err := r.db.Exec(`
		UPDATE booking_room_assignments SET
			room_id   = COALESCE($1, room_id),
			check_in  = COALESCE($2, check_in),
			check_out = COALESCE($3, check_out),
			updated_at = $4
		WHERE id = $5 AND booking_id = $6`,
		req.RoomID, req.CheckIn, req.CheckOut, time.Now(), id, bookingID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id, bookingID)
}

func (r *BookingRoomAssignmentRepository) UpdateStatus(id, bookingID uuid.UUID, status string) error {
	_, err := r.db.Exec(assignmentStatusUpdateSQL, status, time.Now(), id, bookingID)
	return err
}

func (r *BookingRoomAssignmentRepository) UpdateStatusTx(tx *sql.Tx, id, bookingID uuid.UUID, status string) error {
	_, err := tx.Exec(assignmentStatusUpdateSQL, status, time.Now(), id, bookingID)
	return err
}

// assignmentStatusUpdateSQL updates status and stamps the actual check-in/out
// timestamp when transitioning into checked_in / checked_out. The timestamps are
// only set the first time (COALESCE keeps an existing value), leaving the booked
// check_in/check_out dates untouched. $1=status $2=now $3=id $4=booking_id.
const assignmentStatusUpdateSQL = `
	UPDATE booking_room_assignments SET
		status = $1::text,
		checked_in_at  = CASE WHEN $1::text = 'checked_in'  THEN COALESCE(checked_in_at, $2)  ELSE checked_in_at  END,
		checked_out_at = CASE WHEN $1::text = 'checked_out' THEN COALESCE(checked_out_at, $2) ELSE checked_out_at END,
		updated_at = $2
	WHERE id = $3 AND booking_id = $4`

// RoomOccupiedByOtherBookingTx reports whether, within the transaction, the room
// of the given assignment is currently physically occupied (status = checked_in
// and not yet checked out) by a guest belonging to a DIFFERENT booking. Used to
// prevent checking a guest into a room another booking's guest is still in.
// Returns the occupying booking's number for a clear error message.
func (r *BookingRoomAssignmentRepository) RoomOccupiedByOtherBookingTx(tx *sql.Tx, assignmentID, bookingID uuid.UUID) (bool, string, error) {
	var occupied bool
	var otherBookingNumber sql.NullString
	err := tx.QueryRow(`
		SELECT EXISTS (
		         SELECT 1 FROM booking_room_assignments other
		         WHERE other.room_id = self.room_id
		           AND other.booking_id != self.booking_id
		           AND other.status = 'checked_in'
		           AND other.checked_out_at IS NULL
		       ),
		       (
		         SELECT bk.booking_number
		         FROM booking_room_assignments other
		         JOIN bookings bk ON bk.id = other.booking_id
		         WHERE other.room_id = self.room_id
		           AND other.booking_id != self.booking_id
		           AND other.status = 'checked_in'
		           AND other.checked_out_at IS NULL
		         LIMIT 1
		       )
		FROM booking_room_assignments self
		WHERE self.id = $1 AND self.booking_id = $2`, assignmentID, bookingID).
		Scan(&occupied, &otherBookingNumber)
	if err != nil {
		return false, "", err
	}
	return occupied, otherBookingNumber.String, nil
}

// StatusCountsTx returns, within a transaction, the number of non-cancelled
// assignments for a booking and how many of those are checked out. Used to roll
// the parent booking's status up from its room assignments.
func (r *BookingRoomAssignmentRepository) StatusCountsTx(tx *sql.Tx, bookingID uuid.UUID) (active, checkedOut int, err error) {
	err = tx.QueryRow(`
		SELECT
			COUNT(*) FILTER (WHERE status != 'cancelled'),
			COUNT(*) FILTER (WHERE status = 'checked_out')
		FROM booking_room_assignments
		WHERE booking_id = $1`, bookingID).Scan(&active, &checkedOut)
	return active, checkedOut, err
}

func (r *BookingRoomAssignmentRepository) Delete(id, bookingID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM booking_room_assignments WHERE id=$1 AND booking_id=$2`, id, bookingID)
	return err
}

// InvoiceAssignmentRow carries the data invoice generation needs per room assignment.
// CheckIn/CheckOut are the booked dates; CheckedInAt/CheckedOutAt are the actual
// times (nil until the guest is checked in/out). Invoice generation bills actual
// nights when both actual timestamps are present, else falls back to booked dates.
type InvoiceAssignmentRow struct {
	RoomName      string
	AttendeeName  string
	CheckIn       time.Time
	CheckOut      time.Time
	CheckedInAt   *time.Time
	CheckedOutAt  *time.Time
	PricePerNight float64
}

// GetAssignmentsForInvoice returns all non-cancelled assignments for a booking with room pricing.
func (r *BookingRoomAssignmentRepository) GetAssignmentsForInvoice(bookingID uuid.UUID) ([]InvoiceAssignmentRow, error) {
	rows, err := r.db.Query(`
		SELECT ro.name, COALESCE(att.full_name, ''),
		       a.check_in, a.check_out, a.checked_in_at, a.checked_out_at, ro.price_per_night
		FROM booking_room_assignments a
		JOIN rooms ro ON ro.id = a.room_id
		LEFT JOIN booking_attendees att ON att.id = a.attendee_id
		WHERE a.booking_id = $1 AND a.status != 'cancelled'
		ORDER BY a.check_in`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []InvoiceAssignmentRow
	for rows.Next() {
		var row InvoiceAssignmentRow
		var checkedInAt, checkedOutAt sql.NullTime
		if err := rows.Scan(&row.RoomName, &row.AttendeeName, &row.CheckIn, &row.CheckOut, &checkedInAt, &checkedOutAt, &row.PricePerNight); err != nil {
			return nil, err
		}
		if checkedInAt.Valid {
			row.CheckedInAt = &checkedInAt.Time
		}
		if checkedOutAt.Valid {
			row.CheckedOutAt = &checkedOutAt.Time
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// RoomAssignability is the outcome of checking whether a room can take another
// assignment for a given booking over a date range.
type RoomAssignability struct {
	OK           bool
	OtherBooking bool // an overlapping assignment belongs to a DIFFERENT booking
	CapacityFull bool // the room is at capacity with same-booking guests
	Occupied     int  // current overlapping same-booking assignments
	Capacity     int  // the room's capacity
}

// CanAssignRoom enforces the sharing rule: a room may only be shared by guests
// under the SAME booking, and only up to the room's capacity. Any overlapping
// assignment from a different booking blocks it outright.
func (r *BookingRoomAssignmentRepository) CanAssignRoom(roomID, bookingID uuid.UUID, checkIn, checkOut time.Time, excludeID *uuid.UUID) (RoomAssignability, error) {
	var res RoomAssignability

	// Room capacity.
	if err := r.db.QueryRow(`SELECT capacity FROM rooms WHERE id = $1`, roomID).Scan(&res.Capacity); err != nil {
		return res, err
	}

	args := []interface{}{roomID, checkOut, checkIn, bookingID}
	excludeClause := ""
	if excludeID != nil {
		args = append(args, *excludeID)
		excludeClause = fmt.Sprintf(" AND id != $%d", len(args))
	}

	// Count overlapping active assignments, split by whether they belong to a
	// different booking vs the same booking.
	var otherBooking, sameBooking int
	err := r.db.QueryRow(fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE booking_id != $4) AS other_booking,
			COUNT(*) FILTER (WHERE booking_id  = $4) AS same_booking
		FROM booking_room_assignments
		WHERE room_id = $1
		  AND status IN ('pending','confirmed','checked_in')
		  AND check_in  < $2
		  AND check_out > $3%s`, excludeClause), args...).Scan(&otherBooking, &sameBooking)
	if err != nil {
		return res, err
	}

	res.Occupied = sameBooking
	if otherBooking > 0 {
		res.OtherBooking = true
		return res, nil
	}
	if sameBooking >= res.Capacity {
		res.CapacityFull = true
		return res, nil
	}
	res.OK = true
	return res, nil
}

// IsRoomAvailable checks no active assignment overlaps the requested dates for the given room.
func (r *BookingRoomAssignmentRepository) IsRoomAvailable(roomID uuid.UUID, checkIn, checkOut time.Time, excludeID *uuid.UUID) (bool, error) {
	args := []interface{}{roomID, checkOut, checkIn}
	excludeClause := ""
	if excludeID != nil {
		args = append(args, *excludeID)
		excludeClause = fmt.Sprintf(" AND id != $%d", len(args))
	}

	var count int
	err := r.db.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*) FROM booking_room_assignments
		WHERE room_id = $1
		  AND status IN ('pending','confirmed','checked_in')
		  AND check_in  < $2
		  AND check_out > $3%s`, excludeClause), args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// SumRoomCosts returns the total room cost across all assignments for a booking.
func (r *BookingRoomAssignmentRepository) SumRoomCosts(bookingID uuid.UUID) (float64, error) {
	var total float64
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM((a.check_out - a.check_in) * ro.price_per_night), 0)
		FROM booking_room_assignments a
		JOIN rooms ro ON ro.id = a.room_id
		WHERE a.booking_id = $1
		  AND a.status != 'cancelled'`, bookingID).Scan(&total)
	return total, err
}

func (r *BookingRoomAssignmentRepository) SumRoomCostsInTx(tx *sql.Tx, bookingID uuid.UUID) (float64, error) {
	var total float64
	err := tx.QueryRow(`
		SELECT COALESCE(SUM((a.check_out - a.check_in) * ro.price_per_night), 0)
		FROM booking_room_assignments a
		JOIN rooms ro ON ro.id = a.room_id
		WHERE a.booking_id = $1
		  AND a.status != 'cancelled'`, bookingID).Scan(&total)
	return total, err
}

type assignmentScanner interface {
	Scan(dest ...interface{}) error
}

func scanAssignment(row assignmentScanner) (*models.BookingRoomAssignment, error) {
	var a models.BookingRoomAssignment
	var attendeeID uuid.NullUUID
	var attendeeName sql.NullString
	var checkedInAt, checkedOutAt sql.NullTime

	err := row.Scan(
		&a.ID, &a.BookingID, &a.RoomID, &attendeeID,
		&a.CheckIn, &a.CheckOut, &a.Status, &checkedInAt, &checkedOutAt,
		&a.CreatedAt, &a.UpdatedAt,
		&a.RoomName, &attendeeName, &a.Nights, &a.RoomCost,
	)
	if err != nil {
		return nil, err
	}
	if attendeeID.Valid {
		a.AttendeeID = &attendeeID.UUID
	}
	if attendeeName.Valid {
		a.AttendeeName = attendeeName.String
	}
	if checkedInAt.Valid {
		a.CheckedInAt = &checkedInAt.Time
	}
	if checkedOutAt.Valid {
		a.CheckedOutAt = &checkedOutAt.Time
	}
	return &a, nil
}
