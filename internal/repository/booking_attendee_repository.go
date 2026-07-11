package repository

import (
	"database/sql"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type BookingAttendeeRepository struct {
	db *sql.DB
}

func NewBookingAttendeeRepository() *BookingAttendeeRepository {
	return &BookingAttendeeRepository{db: database.DB}
}

func (r *BookingAttendeeRepository) CreateInTx(tx *sql.Tx, a *models.BookingAttendee) error {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()

	return tx.QueryRow(`
		INSERT INTO booking_attendees (
			id, booking_id, corporate_guest_id,
			full_name, email, phone, identification_card,
			dietary_notes, special_needs, is_lead_contact, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		a.ID, a.BookingID, a.CorporateGuestID,
		a.FullName, a.Email, a.Phone, a.IdentificationCard,
		a.DietaryNotes, a.SpecialNeeds, a.IsLeadContact, a.CreatedAt,
	).Scan(&a.ID)
}

func (r *BookingAttendeeRepository) ListByBookingID(bookingID uuid.UUID) ([]models.BookingAttendee, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.booking_id, a.corporate_guest_id,
		       a.full_name, a.email, a.phone, a.identification_card,
		       a.dietary_notes, a.special_needs, a.is_lead_contact, a.created_at
		FROM booking_attendees a
		WHERE a.booking_id = $1
		ORDER BY a.is_lead_contact DESC, a.full_name`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendees []models.BookingAttendee
	for rows.Next() {
		a, err := scanAttendee(rows)
		if err != nil {
			return nil, err
		}
		attendees = append(attendees, *a)
	}
	return attendees, rows.Err()
}

func (r *BookingAttendeeRepository) GetByID(id, bookingID uuid.UUID) (*models.BookingAttendee, error) {
	row := r.db.QueryRow(`
		SELECT a.id, a.booking_id, a.corporate_guest_id,
		       a.full_name, a.email, a.phone, a.identification_card,
		       a.dietary_notes, a.special_needs, a.is_lead_contact, a.created_at
		FROM booking_attendees a
		WHERE a.id = $1 AND a.booking_id = $2`, id, bookingID)
	return scanAttendee(row)
}

// GetByIDUnscoped looks up an attendee by ID alone (attendee IDs are globally
// unique). Used where the caller has an attendee_id but not its booking_id.
func (r *BookingAttendeeRepository) GetByIDUnscoped(id uuid.UUID) (*models.BookingAttendee, error) {
	row := r.db.QueryRow(`
		SELECT a.id, a.booking_id, a.corporate_guest_id,
		       a.full_name, a.email, a.phone, a.identification_card,
		       a.dietary_notes, a.special_needs, a.is_lead_contact, a.created_at
		FROM booking_attendees a
		WHERE a.id = $1`, id)
	return scanAttendee(row)
}

func (r *BookingAttendeeRepository) Update(id, bookingID uuid.UUID, req *models.UpdateAttendeeRequest) (*models.BookingAttendee, error) {
	_, err := r.db.Exec(`
		UPDATE booking_attendees SET
			full_name           = COALESCE($1, full_name),
			email               = COALESCE($2, email),
			phone               = COALESCE($3, phone),
			identification_card = COALESCE($4, identification_card),
			dietary_notes       = COALESCE($5, dietary_notes),
			special_needs       = COALESCE($6, special_needs),
			is_lead_contact     = COALESCE($7, is_lead_contact)
		WHERE id = $8 AND booking_id = $9`,
		req.FullName, req.Email, req.Phone, req.IdentificationCard,
		req.DietaryNotes, req.SpecialNeeds, req.IsLeadContact,
		id, bookingID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id, bookingID)
}

func (r *BookingAttendeeRepository) Delete(id, bookingID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM booking_attendees WHERE id=$1 AND booking_id=$2`, id, bookingID)
	return err
}

func scanAttendee(row bookingScanner) (*models.BookingAttendee, error) {
	var a models.BookingAttendee
	var corporateGuestID uuid.NullUUID
	var email, phone, identificationCard, dietaryNotes, specialNeeds sql.NullString

	err := row.Scan(
		&a.ID, &a.BookingID, &corporateGuestID,
		&a.FullName, &email, &phone, &identificationCard,
		&dietaryNotes, &specialNeeds, &a.IsLeadContact, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if corporateGuestID.Valid {
		a.CorporateGuestID = &corporateGuestID.UUID
	}
	if email.Valid {
		a.Email = email.String
	}
	if phone.Valid {
		a.Phone = phone.String
	}
	if identificationCard.Valid {
		a.IdentificationCard = identificationCard.String
	}
	if dietaryNotes.Valid {
		a.DietaryNotes = dietaryNotes.String
	}
	if specialNeeds.Valid {
		a.SpecialNeeds = specialNeeds.String
	}
	return &a, nil
}
