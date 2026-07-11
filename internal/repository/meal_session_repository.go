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

type MealSessionRepository struct {
	db *sql.DB
}

func NewMealSessionRepository() *MealSessionRepository {
	return &MealSessionRepository{db: database.DB}
}

const mealSessionSelect = `
	SELECT s.id, s.org_id, s.branch_id, s.meal_period, s.buffet_menu_item_id,
	       COALESCE(mi.name, '') AS buffet_name,
	       to_char(s.start_time, 'HH24:MI'), to_char(s.end_time, 'HH24:MI'),
	       s.days_of_week, s.auto_open_close, s.status, s.grace_period_minutes,
	       s.created_at, s.updated_at
	FROM meal_sessions s
	LEFT JOIN menu_items mi ON mi.id = s.buffet_menu_item_id`

func scanMealSession(row interface{ Scan(...interface{}) error }) (*models.ResidentMealSession, error) {
	var s models.ResidentMealSession
	var branchID uuid.NullUUID
	if err := row.Scan(
		&s.ID, &s.OrgID, &branchID, &s.MealPeriod, &s.BuffetMenuItemID, &s.BuffetName,
		&s.StartTime, &s.EndTime, pq.Array(&s.DaysOfWeek), &s.AutoOpenClose, &s.Status,
		&s.GracePeriodMinutes, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if branchID.Valid {
		s.BranchID = &branchID.UUID
	}
	if s.DaysOfWeek == nil {
		s.DaysOfWeek = []string{}
	}
	return &s, nil
}

func (r *MealSessionRepository) Create(s *models.ResidentMealSession, orgID uuid.UUID, branchID *uuid.UUID) error {
	s.ID = uuid.New()
	now := time.Now()
	err := r.db.QueryRow(`
		INSERT INTO meal_sessions
		    (id, org_id, branch_id, meal_period, buffet_menu_item_id, start_time, end_time,
		     days_of_week, auto_open_close, status, grace_period_minutes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
		RETURNING id`,
		s.ID, orgID, branchID, s.MealPeriod, s.BuffetMenuItemID, s.StartTime, s.EndTime,
		pq.Array(s.DaysOfWeek), s.AutoOpenClose, s.Status, s.GracePeriodMinutes, now,
	).Scan(&s.ID)
	return err
}

func (r *MealSessionRepository) GetByID(id, orgID uuid.UUID) (*models.ResidentMealSession, error) {
	row := r.db.QueryRow(mealSessionSelect+` WHERE s.id = $1 AND s.org_id = $2`, id, orgID)
	return scanMealSession(row)
}

func (r *MealSessionRepository) List(orgID uuid.UUID, branchID *uuid.UUID, status, mealPeriod string, page, pageSize int) ([]models.ResidentMealSession, int, error) {
	args := []interface{}{orgID}
	where := []string{"s.org_id = $1"}
	i := 2
	if branchID != nil {
		where = append(where, fmt.Sprintf("s.branch_id = $%d", i))
		args = append(args, *branchID)
		i++
	}
	if status != "" {
		where = append(where, fmt.Sprintf("s.status = $%d", i))
		args = append(args, status)
		i++
	}
	if mealPeriod != "" {
		where = append(where, fmt.Sprintf("s.meal_period = $%d", i))
		args = append(args, mealPeriod)
		i++
	}
	whereStr := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM meal_sessions s WHERE `+whereStr, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(fmt.Sprintf(`%s WHERE %s ORDER BY s.start_time ASC LIMIT $%d OFFSET $%d`,
		mealSessionSelect, whereStr, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	sessions := []models.ResidentMealSession{}
	for rows.Next() {
		s, err := scanMealSession(rows)
		if err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, total, rows.Err()
}

func (r *MealSessionRepository) Update(id, orgID uuid.UUID, req *models.MealSessionUpdateRequest) error {
	sets := []string{}
	args := []interface{}{}
	i := 1
	add := func(col string, val interface{}) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	if req.MealPeriod != nil {
		add("meal_period", *req.MealPeriod)
	}
	if req.BuffetMenuItemID != nil {
		add("buffet_menu_item_id", *req.BuffetMenuItemID)
	}
	if req.StartTime != nil {
		add("start_time", *req.StartTime)
	}
	if req.EndTime != nil {
		add("end_time", *req.EndTime)
	}
	if req.AutoOpenClose != nil {
		add("auto_open_close", *req.AutoOpenClose)
	}
	if req.DaysOfWeek != nil {
		add("days_of_week", pq.Array(req.DaysOfWeek))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	i++
	args = append(args, id, orgID)
	q := fmt.Sprintf(`UPDATE meal_sessions SET %s WHERE id = $%d AND org_id = $%d`, strings.Join(sets, ", "), i, i+1)
	res, err := r.db.Exec(q, args...)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

func (r *MealSessionRepository) UpdateStatus(id, orgID uuid.UUID, status string) error {
	res, err := r.db.Exec(`UPDATE meal_sessions SET status = $1, updated_at = now() WHERE id = $2 AND org_id = $3`, status, id, orgID)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

func (r *MealSessionRepository) UpdateGracePeriod(id, orgID uuid.UUID, minutes int) error {
	res, err := r.db.Exec(`UPDATE meal_sessions SET grace_period_minutes = $1, updated_at = now() WHERE id = $2 AND org_id = $3`, minutes, id, orgID)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

func (r *MealSessionRepository) Delete(id, orgID uuid.UUID) error {
	res, err := r.db.Exec(`DELETE FROM meal_sessions WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	return affectedOrNotFound(res)
}

func affectedOrNotFound(res sql.Result) error {
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
