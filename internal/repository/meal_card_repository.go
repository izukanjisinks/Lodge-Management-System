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

type MealCardRepository struct {
	db *sql.DB
}

func NewMealCardRepository() *MealCardRepository {
	return &MealCardRepository{db: database.DB}
}

const mealCardSelect = `
	SELECT c.id, c.org_id, c.branch_id, c.card_uid, c.room_id,
	       COALESCE(r.name, '') AS room_name,
	       c.role, c.attendee_id, COALESCE(c.holder_name, ''), COALESCE(c.identification_card, ''),
	       c.status, c.replaced_card_id, c.issued_at, c.created_at, c.updated_at
	FROM meal_cards c
	LEFT JOIN rooms r ON r.id = c.room_id`

func scanMealCard(row interface{ Scan(...interface{}) error }) (*models.MealCard, error) {
	var c models.MealCard
	var branchID, attendeeID, replacedID uuid.NullUUID
	if err := row.Scan(
		&c.ID, &c.OrgID, &branchID, &c.CardUID, &c.RoomID, &c.RoomName,
		&c.Role, &attendeeID, &c.HolderName, &c.IdentificationCard,
		&c.Status, &replacedID, &c.IssuedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if branchID.Valid {
		c.BranchID = &branchID.UUID
	}
	if attendeeID.Valid {
		c.AttendeeID = &attendeeID.UUID
	}
	if replacedID.Valid {
		c.ReplacedCardID = &replacedID.UUID
	}
	return &c, nil
}

func (r *MealCardRepository) Create(c *models.MealCard, orgID uuid.UUID, branchID *uuid.UUID) error {
	c.ID = uuid.New()
	now := time.Now()
	err := r.db.QueryRow(`
		INSERT INTO meal_cards
		    (id, org_id, branch_id, card_uid, room_id, role, attendee_id, holder_name,
		     identification_card, status, issued_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',$10,$10,$10)
		RETURNING id`,
		c.ID, orgID, branchID, c.CardUID, c.RoomID, c.Role, c.AttendeeID,
		nullString(c.HolderName), nullString(c.IdentificationCard), now,
	).Scan(&c.ID)
	return err
}

func (r *MealCardRepository) GetByID(id, orgID uuid.UUID) (*models.MealCard, error) {
	return scanMealCard(r.db.QueryRow(mealCardSelect+` WHERE c.id = $1 AND c.org_id = $2`, id, orgID))
}

// GetActiveByUID resolves a scanned card UID to its active assignment.
func (r *MealCardRepository) GetActiveByUID(cardUID string, orgID uuid.UUID) (*models.MealCard, error) {
	return scanMealCard(r.db.QueryRow(mealCardSelect+` WHERE c.card_uid = $1 AND c.org_id = $2 AND c.status = 'active'`, cardUID, orgID))
}

func (r *MealCardRepository) List(orgID uuid.UUID, roomID *uuid.UUID, status string) ([]models.MealCard, error) {
	args := []interface{}{orgID}
	where := []string{"c.org_id = $1"}
	i := 2
	if roomID != nil {
		where = append(where, fmt.Sprintf("c.room_id = $%d", i))
		args = append(args, *roomID)
		i++
	}
	if status != "" {
		where = append(where, fmt.Sprintf("c.status = $%d", i))
		args = append(args, status)
		i++
	}
	rows, err := r.db.Query(fmt.Sprintf(`%s WHERE %s ORDER BY c.created_at DESC`, mealCardSelect, strings.Join(where, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.MealCard{}
	for rows.Next() {
		c, err := scanMealCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *MealCardRepository) Update(id, orgID uuid.UUID, req *models.UpdateCardRequest) error {
	sets := []string{}
	args := []interface{}{}
	i := 1
	add := func(col string, val interface{}) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	if req.RoomID != nil {
		add("room_id", *req.RoomID)
	}
	if req.Role != nil {
		add("role", *req.Role)
	}
	if req.Status != nil {
		add("status", *req.Status)
	}
	if req.ClearAttendee {
		sets = append(sets, "attendee_id = NULL, holder_name = NULL, identification_card = NULL")
	} else if req.AttendeeID != nil {
		add("attendee_id", *req.AttendeeID)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	i++
	args = append(args, id, orgID)
	q := fmt.Sprintf(`UPDATE meal_cards SET %s WHERE id = $%d AND org_id = $%d`, strings.Join(sets, ", "), i, i+1)
	res, err := r.db.Exec(q, args...)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

// SetHolder denormalizes an attendee's name/ID onto the card (used after resolving
// a picked occupant on assign/update).
func (r *MealCardRepository) SetHolder(id, orgID uuid.UUID, attendeeID uuid.UUID, name, idCard string) error {
	res, err := r.db.Exec(`UPDATE meal_cards SET attendee_id=$1, holder_name=$2, identification_card=$3, updated_at=now() WHERE id=$4 AND org_id=$5`,
		attendeeID, nullString(name), nullString(idCard), id, orgID)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

// Replace voids the old card and issues a new active one carrying the same
// room/role/holder, linked via replaced_card_id. Returns the new card.
func (r *MealCardRepository) Replace(oldID, orgID uuid.UUID, newUID string) (*models.MealCard, error) {
	old, err := r.GetByID(oldID, orgID)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`UPDATE meal_cards SET status='replaced', updated_at=now() WHERE id=$1 AND org_id=$2`, oldID, orgID); err != nil {
		return nil, err
	}
	newID := uuid.New()
	now := time.Now()
	if _, err = tx.Exec(`
		INSERT INTO meal_cards
		    (id, org_id, branch_id, card_uid, room_id, role, attendee_id, holder_name,
		     identification_card, status, replaced_card_id, issued_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',$10,$11,$11,$11)`,
		newID, orgID, old.BranchID, newUID, old.RoomID, old.Role, old.AttendeeID,
		nullString(old.HolderName), nullString(old.IdentificationCard), oldID, now,
	); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(newID, orgID)
}

func (r *MealCardRepository) Void(id, orgID uuid.UUID) error {
	res, err := r.db.Exec(`UPDATE meal_cards SET status='void', updated_at=now() WHERE id=$1 AND org_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
