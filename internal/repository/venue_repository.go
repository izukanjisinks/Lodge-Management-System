package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type VenueRepository struct {
	db *sql.DB
}

func NewVenueRepository() *VenueRepository {
	return &VenueRepository{db: database.DB}
}

func (r *VenueRepository) Create(venue *models.Venue, orgID uuid.UUID) error {
	venue.ID = uuid.New()
	now := time.Now()
	venue.CreatedAt = now
	venue.UpdatedAt = now
	_, err := r.db.Exec(`
		INSERT INTO venues (id, org_id, branch_id, name, venue_type, capacity, area_sqm, floor, base_rate, rate_type, amenities, images, is_available, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		venue.ID, orgID, venue.BranchID, venue.Name, venue.VenueType, venue.Capacity,
		venue.AreaSqm, venue.Floor, venue.BaseRate, venue.RateType,
		pq.Array(venue.Amenities), pq.Array(venue.Images), venue.IsAvailable, venue.Notes,
		venue.CreatedAt, venue.UpdatedAt,
	)
	return err
}

func (r *VenueRepository) GetByID(id uuid.UUID, orgID uuid.UUID) (*models.Venue, error) {
	row := r.db.QueryRow(`
		SELECT id, org_id, branch_id, name, venue_type, capacity, area_sqm, floor, base_rate, rate_type, amenities, images, is_available, notes, created_at, updated_at
		FROM venues WHERE id = $1 AND org_id = $2`, id, orgID)
	return scanVenue(row)
}

// GetByIDUnscoped looks up a venue by its (globally unique) ID without org scoping.
// Used by the public guest venue-detail endpoint, mirroring RoomRepository.
func (r *VenueRepository) GetByIDUnscoped(id uuid.UUID) (*models.Venue, error) {
	row := r.db.QueryRow(`
		SELECT id, org_id, branch_id, name, venue_type, capacity, area_sqm, floor, base_rate, rate_type, amenities, images, is_available, notes, created_at, updated_at
		FROM venues WHERE id = $1`, id)
	return scanVenue(row)
}

func (r *VenueRepository) List(orgID uuid.UUID, branchID *uuid.UUID, venueType string, isAvailable *bool, from, to *time.Time, page, pageSize int) ([]models.Venue, int, error) {
	args := []interface{}{orgID}
	where := []string{"org_id = $1"}
	i := 2

	if branchID != nil {
		where = append(where, fmt.Sprintf("branch_id = $%d", i))
		args = append(args, *branchID)
		i++
	}
	if venueType != "" {
		where = append(where, fmt.Sprintf("venue_type = $%d", i))
		args = append(args, venueType)
		i++
	}
	if isAvailable != nil {
		where = append(where, fmt.Sprintf("is_available = $%d", i))
		args = append(args, *isAvailable)
		i++
	}
	if clause, newI := venueDateAvailableClause("venues", from, to, i); clause != "" {
		where = append(where, clause)
		if from != nil {
			args = append(args, *from)
		}
		if to != nil {
			args = append(args, *to)
		}
		i = newI
	}

	whereStr := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM venues WHERE %s`, whereStr), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT id, org_id, branch_id, name, venue_type, capacity, area_sqm, floor, base_rate, rate_type, amenities, images, is_available, notes, created_at, updated_at
		FROM venues WHERE %s
		ORDER BY name ASC
		LIMIT $%d OFFSET $%d`, whereStr, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var venues []models.Venue
	for rows.Next() {
		venue, err := scanVenue(rows)
		if err != nil {
			return nil, 0, err
		}
		venues = append(venues, *venue)
	}
	return venues, total, rows.Err()
}

// venueDateAvailableClause returns a NOT EXISTS clause excluding venues that have a
// non-cancelled/non-rejected event reservation overlapping [from, to], along with the
// next placeholder index. Date overlap is inclusive (venue hire is day-based):
// a reservation conflicts when be.start_date <= to AND be.end_date >= from.
// Returns "" when no date window is supplied (from and to are both nil).
func venueDateAvailableClause(venueTable string, from, to *time.Time, i int) (string, int) {
	if from == nil && to == nil {
		return "", i
	}
	conds := []string{
		fmt.Sprintf("be.venue_id = %s.id", venueTable),
		"b.status NOT IN ('cancelled', 'rejected')",
	}
	if from != nil {
		conds = append(conds, fmt.Sprintf("be.end_date >= $%d", i))
		i++
	}
	if to != nil {
		conds = append(conds, fmt.Sprintf("be.start_date <= $%d", i))
		i++
	}
	clause := fmt.Sprintf(`NOT EXISTS (
		SELECT 1 FROM booking_events be
		JOIN bookings b ON b.id = be.booking_id
		WHERE %s
	)`, strings.Join(conds, " AND "))
	return clause, i
}

// GuestList lists available venues for a public org-scoped browse (no auth).
// from/to optionally exclude venues already reserved in that date window.
func (r *VenueRepository) GuestList(orgID uuid.UUID, branchID *uuid.UUID, venueType string, from, to *time.Time) ([]models.Venue, error) {
	args := []interface{}{orgID}
	where := []string{"org_id = $1", "is_available = TRUE"}
	i := 2

	if branchID != nil {
		where = append(where, fmt.Sprintf("branch_id = $%d", i))
		args = append(args, *branchID)
		i++
	}
	if venueType != "" {
		where = append(where, fmt.Sprintf("venue_type = $%d", i))
		args = append(args, venueType)
		i++
	}
	if clause, newI := venueDateAvailableClause("venues", from, to, i); clause != "" {
		where = append(where, clause)
		if from != nil {
			args = append(args, *from)
		}
		if to != nil {
			args = append(args, *to)
		}
		i = newI
	}

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT id, org_id, branch_id, name, venue_type, capacity, area_sqm, floor, base_rate, rate_type, amenities, images, is_available, notes, created_at, updated_at
		FROM venues WHERE %s
		ORDER BY name ASC`, strings.Join(where, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var venues []models.Venue
	for rows.Next() {
		venue, err := scanVenue(rows)
		if err != nil {
			return nil, err
		}
		venues = append(venues, *venue)
	}
	return venues, rows.Err()
}

func (r *VenueRepository) Update(venue *models.Venue, orgID uuid.UUID) error {
	venue.UpdatedAt = time.Now()
	_, err := r.db.Exec(`
		UPDATE venues SET name=$1, venue_type=$2, capacity=$3, area_sqm=$4, floor=$5,
		       base_rate=$6, rate_type=$7, amenities=$8, is_available=$9, notes=$10, updated_at=$11
		WHERE id=$12 AND org_id=$13`,
		venue.Name, venue.VenueType, venue.Capacity, venue.AreaSqm, venue.Floor,
		venue.BaseRate, venue.RateType, pq.Array(venue.Amenities), venue.IsAvailable, venue.Notes,
		venue.UpdatedAt, venue.ID, orgID,
	)
	return err
}

func (r *VenueRepository) UpdateImages(id uuid.UUID, orgID uuid.UUID, images []string) error {
	_, err := r.db.Exec(`UPDATE venues SET images=$1, updated_at=$2 WHERE id=$3 AND org_id=$4`,
		pq.Array(images), time.Now(), id, orgID)
	return err
}

func (r *VenueRepository) SetAvailability(id uuid.UUID, orgID uuid.UUID, available bool) error {
	_, err := r.db.Exec(`UPDATE venues SET is_available=$1, updated_at=$2 WHERE id=$3 AND org_id=$4`,
		available, time.Now(), id, orgID)
	return err
}

func (r *VenueRepository) Delete(id uuid.UUID, orgID uuid.UUID) error {
	res, err := r.db.Exec(`DELETE FROM venues WHERE id=$1 AND org_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("venue not found")
	}
	return nil
}

type venueScanner interface {
	Scan(dest ...interface{}) error
}

func scanVenue(row venueScanner) (*models.Venue, error) {
	var venue models.Venue
	var orgID, branchID sql.NullString
	var areaSqm sql.NullFloat64
	var floor, notes sql.NullString
	err := row.Scan(
		&venue.ID, &orgID, &branchID, &venue.Name, &venue.VenueType, &venue.Capacity,
		&areaSqm, &floor, &venue.BaseRate, &venue.RateType,
		pq.Array(&venue.Amenities), pq.Array(&venue.Images), &venue.IsAvailable, &notes,
		&venue.CreatedAt, &venue.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if orgID.Valid {
		parsed, _ := uuid.Parse(orgID.String)
		venue.OrgID = &parsed
	}
	if branchID.Valid {
		parsed, _ := uuid.Parse(branchID.String)
		venue.BranchID = &parsed
	}
	if areaSqm.Valid {
		venue.AreaSqm = areaSqm.Float64
	}
	if floor.Valid {
		venue.Floor = floor.String
	}
	if notes.Valid {
		venue.Notes = notes.String
	}
	if venue.Amenities == nil {
		venue.Amenities = []string{}
	}
	if venue.Images == nil {
		venue.Images = []string{}
	}
	return &venue, nil
}
