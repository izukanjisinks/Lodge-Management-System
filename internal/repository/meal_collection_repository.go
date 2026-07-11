package repository

import (
	"database/sql"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type MealCollectionRepository struct {
	db *sql.DB
}

func NewMealCollectionRepository() *MealCollectionRepository {
	return &MealCollectionRepository{db: database.DB}
}

// FindByIdempotencyKey returns a prior collection for this key, or nil if none.
// Used to make /collect and offline /sync idempotent (dedupe accidental double-taps).
func (r *MealCollectionRepository) FindByIdempotencyKey(orgID uuid.UUID, key string) (*models.MealCollectionLogEntry, error) {
	e, err := r.scanLog(r.db.QueryRow(mealCollectionLogSelect+` WHERE mc.org_id = $1 AND mc.idempotency_key = $2`, orgID, key))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// Reserve inserts the collection log row up front, relying on the unique
// (org_id, idempotency_key) index to atomically reject a duplicate scan. Returns
// (false, nil) when the key already exists — the caller should treat it as a
// duplicate and NOT charge. The charge/invoice fields are filled in later by
// SetCharge once the invoice line has actually been posted.
func (r *MealCollectionRepository) Reserve(e *models.MealCollectionLogEntry, orgID uuid.UUID, bookingID *uuid.UUID, idempotencyKey string) (inserted bool, err error) {
	e.ID = uuid.New()
	now := time.Now()
	e.CreatedAt = now
	res, err := r.db.Exec(`
		INSERT INTO meal_collections
		    (id, org_id, meal_session_id, booking_id, attendee_id, resident_name, identification_card,
		     method, card_uid, room_name, idempotency_key, collected_by, collected_by_name, collected_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)
		ON CONFLICT (org_id, idempotency_key) DO NOTHING`,
		e.ID, orgID, e.MealSessionID, bookingID, e.AttendeeID, nullString(e.ResidentName), nullString(e.IdentificationCard),
		e.Method, nullString(e.CardUID), nullString(e.RoomName), idempotencyKey,
		nullUUID(e.CollectedBy), nullString(e.CollectedByName), e.CollectedAt,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetCharge fills in the amount/invoice/line-item on a reserved collection row
// after the invoice line has been posted.
func (r *MealCollectionRepository) SetCharge(id, orgID uuid.UUID, amount float64, invoiceID, lineItemID uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE meal_collections SET amount=$1, invoice_id=$2, invoice_line_item_id=$3 WHERE id=$4 AND org_id=$5`,
		amount, invoiceID, lineItemID, id, orgID)
	return err
}

const mealCollectionLogSelect = `
	SELECT mc.id, mc.meal_session_id, mc.attendee_id, COALESCE(mc.resident_name,''),
	       COALESCE(mc.identification_card,''), mc.method, COALESCE(mc.card_uid,''),
	       COALESCE(mc.room_name,''), mc.amount, mc.invoice_id,
	       COALESCE(mc.collected_by::text,''), COALESCE(mc.collected_by_name,''),
	       mc.collected_at, mc.synced_at, mc.created_at
	FROM meal_collections mc`

func (r *MealCollectionRepository) scanLog(row interface{ Scan(...interface{}) error }) (*models.MealCollectionLogEntry, error) {
	var e models.MealCollectionLogEntry
	var attendeeID, invoiceID uuid.NullUUID
	var amount sql.NullFloat64
	var syncedAt sql.NullTime
	if err := row.Scan(
		&e.ID, &e.MealSessionID, &attendeeID, &e.ResidentName, &e.IdentificationCard,
		&e.Method, &e.CardUID, &e.RoomName, &amount, &invoiceID,
		&e.CollectedBy, &e.CollectedByName, &e.CollectedAt, &syncedAt, &e.CreatedAt,
	); err != nil {
		return nil, err
	}
	if attendeeID.Valid {
		e.AttendeeID = &attendeeID.UUID
	}
	if invoiceID.Valid {
		e.InvoiceID = &invoiceID.UUID
	}
	if amount.Valid {
		e.Amount = &amount.Float64
	}
	if syncedAt.Valid {
		e.SyncedAt = &syncedAt.Time
	}
	return &e, nil
}

func (r *MealCollectionRepository) ListBySession(sessionID, orgID uuid.UUID) ([]models.MealCollectionLogEntry, error) {
	rows, err := r.db.Query(mealCollectionLogSelect+` WHERE mc.org_id = $1 AND mc.meal_session_id = $2 ORDER BY mc.collected_at DESC`, orgID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.MealCollectionLogEntry{}
	for rows.Next() {
		e, err := r.scanLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// ─── Resolution helpers (used by /collect) ────────────────────────────────────

// ResolveRoomCurrentStay returns the current in-house booking + occupant info for a
// room: the assignment that is checked in and not yet checked out. Nil when none.
func (r *MealCollectionRepository) ResolveRoomCurrentStay(orgID, roomID uuid.UUID) (*models.ResidentSummary, error) {
	var s models.ResidentSummary
	var attendeeID uuid.NullUUID
	var idCard, leadName, roomName sql.NullString
	err := r.db.QueryRow(`
		SELECT b.id, b.booking_number, ro.name,
		       lead.id, lead.full_name, lead.identification_card
		FROM booking_room_assignments a
		JOIN bookings b  ON b.id = a.booking_id
		JOIN rooms    ro ON ro.id = a.room_id
		LEFT JOIN LATERAL (
		    SELECT id, full_name, identification_card FROM booking_attendees
		    WHERE booking_id = b.id ORDER BY is_lead_contact DESC LIMIT 1
		) lead ON TRUE
		WHERE a.room_id = $1 AND b.org_id = $2
		  AND a.checked_in_at IS NOT NULL AND a.checked_out_at IS NULL
		ORDER BY a.checked_in_at DESC LIMIT 1`, roomID, orgID).
		Scan(&s.BookingID, &s.BookingNumber, &roomName, &attendeeID, &leadName, &idCard)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if attendeeID.Valid {
		s.AttendeeID = &attendeeID.UUID
	}
	if leadName.Valid {
		s.FullName = leadName.String
	}
	if idCard.Valid {
		s.IdentificationCard = idCard.String
	}
	if roomName.Valid {
		s.RoomName = roomName.String
	}
	return &s, nil
}

// ResolveByIDCard finds current in-house residents whose identification_card matches
// the typed input. More than one → ambiguous.
func (r *MealCollectionRepository) ResolveByIDCard(orgID uuid.UUID, idCard string) ([]models.ResidentSummary, error) {
	rows, err := r.db.Query(`
		SELECT att.id, att.full_name, att.identification_card, ro.name, b.id, b.booking_number
		FROM booking_attendees att
		JOIN bookings b ON b.id = att.booking_id AND b.org_id = $1
		JOIN booking_room_assignments a ON a.booking_id = b.id
		    AND a.checked_in_at IS NOT NULL AND a.checked_out_at IS NULL
		    AND (a.attendee_id = att.id OR a.attendee_id IS NULL)
		JOIN rooms ro ON ro.id = a.room_id
		WHERE att.identification_card = $2`, orgID, idCard)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[uuid.UUID]bool{}
	var out []models.ResidentSummary
	for rows.Next() {
		var s models.ResidentSummary
		var attID uuid.UUID
		var idc, roomName, bookingNum sql.NullString
		if err := rows.Scan(&attID, &s.FullName, &idc, &roomName, &s.BookingID, &bookingNum); err != nil {
			return nil, err
		}
		if seen[attID] {
			continue
		}
		seen[attID] = true
		s.AttendeeID = &attID
		s.IdentificationCard = idc.String
		s.RoomName = roomName.String
		s.BookingNumber = bookingNum.String
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetCurrentStay returns the full current stay (booking + all attendees) for a room,
// powering the card dialog occupant picker. Nil when the room has no active stay.
func (r *MealCollectionRepository) GetCurrentStay(orgID, roomID uuid.UUID) (*models.CurrentStay, error) {
	var cs models.CurrentStay
	var bookingNum, roomName sql.NullString
	err := r.db.QueryRow(`
		SELECT b.id, b.booking_number, ro.id, ro.name
		FROM booking_room_assignments a
		JOIN bookings b  ON b.id = a.booking_id
		JOIN rooms    ro ON ro.id = a.room_id
		WHERE a.room_id = $1 AND b.org_id = $2
		  AND a.checked_in_at IS NOT NULL AND a.checked_out_at IS NULL
		ORDER BY a.checked_in_at DESC LIMIT 1`, roomID, orgID).
		Scan(&cs.BookingID, &bookingNum, &cs.RoomID, &roomName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cs.BookingNumber = bookingNum.String
	cs.RoomName = roomName.String

	rows, err := r.db.Query(`
		SELECT id, booking_id, full_name, COALESCE(email,''), COALESCE(phone,''),
		       COALESCE(identification_card,''), is_lead_contact
		FROM booking_attendees WHERE booking_id = $1 ORDER BY is_lead_contact DESC, full_name`, cs.BookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cs.Attendees = []models.BookingAttendee{}
	for rows.Next() {
		var a models.BookingAttendee
		if err := rows.Scan(&a.ID, &a.BookingID, &a.FullName, &a.Email, &a.Phone, &a.IdentificationCard, &a.IsLeadContact); err != nil {
			return nil, err
		}
		cs.Attendees = append(cs.Attendees, a)
	}
	return &cs, rows.Err()
}

func nullUUID(s string) interface{} {
	if s == "" {
		return nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return id
	}
	return nil
}
