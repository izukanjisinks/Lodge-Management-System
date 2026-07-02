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

type ClientRepository struct {
	db *sql.DB
}

func NewClientRepository() *ClientRepository {
	return &ClientRepository{db: database.DB}
}

// ─── Individual ───────────────────────────────────────────────────────────────

func (r *ClientRepository) CreateIndividual(c *models.IndividualClient, orgID uuid.UUID) error {
	query := `
		INSERT INTO individual_profiles
		    (id, full_name, email, phone, id_passport_number, nationality, status, notes, org_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	c.ID = uuid.New()
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := r.db.Exec(query,
		c.ID, c.FullName, c.Email, c.Phone, c.IDPassportNumber,
		c.Nationality, c.Status, c.Notes, orgID, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// FindOrCreateIndividualInTx upserts an individual profile within an existing
// transaction, keyed on the org-scoped (id_passport_number, org_id) unique
// constraint. If the person already exists their contact fields are refreshed
// (a booker's phone/email may have changed since last stay); otherwise a new
// profile is inserted. Used by booking approval to build the client registry
// without creating duplicates for repeat guests.
//
// idNumber is required — attendees without an ID number are skipped by the
// caller, since ID number is the dedup key.
func (r *ClientRepository) FindOrCreateIndividualInTx(tx *sql.Tx, orgID uuid.UUID, c *models.IndividualClient) error {
	now := time.Now()
	return tx.QueryRow(`
		INSERT INTO individual_profiles
		    (id, org_id, full_name, email, phone, id_passport_number, nationality, status, notes, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'active', $7, $8, $8)
		ON CONFLICT (id_passport_number, org_id) DO UPDATE SET
		    full_name   = EXCLUDED.full_name,
		    email       = COALESCE(NULLIF(EXCLUDED.email, ''), individual_profiles.email),
		    phone       = COALESCE(NULLIF(EXCLUDED.phone, ''), individual_profiles.phone),
		    nationality = COALESCE(NULLIF(EXCLUDED.nationality, ''), individual_profiles.nationality),
		    updated_at  = EXCLUDED.updated_at
		RETURNING id`,
		orgID, c.FullName, c.Email, c.Phone, c.IDPassportNumber, c.Nationality, c.Notes, now,
	).Scan(&c.ID)
}

// GetIndividualByUserID returns the individual profile linked to a user account.
func (r *ClientRepository) GetIndividualByUserID(userID uuid.UUID) (*models.IndividualClient, error) {
	query := `
		SELECT id, full_name, email, phone, id_passport_number, nationality, status, notes, created_at, updated_at
		FROM individual_profiles
		WHERE user_id = $1`
	return r.scanIndividual(r.db.QueryRow(query, userID))
}

// UpdateIndividualByUserID updates the profile fields a guest is allowed to change.
func (r *ClientRepository) UpdateIndividualByUserID(userID uuid.UUID, c *models.IndividualClient) error {
	query := `
		UPDATE individual_profiles
		SET full_name=$1, phone=$2, id_passport_number=$3, nationality=$4, updated_at=$5
		WHERE user_id=$6`
	c.UpdatedAt = time.Now()
	res, err := r.db.Exec(query, c.FullName, c.Phone, c.IDPassportNumber, c.Nationality, c.UpdatedAt, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("guest profile not found")
	}
	return nil
}

func (r *ClientRepository) GetIndividualByID(id uuid.UUID, orgID uuid.UUID) (*models.IndividualClient, error) {
	query := `
		SELECT id, full_name, email, phone, id_passport_number, nationality, status, notes, created_at, updated_at
		FROM individual_profiles
		WHERE id = $1 AND org_id = $2`

	return r.scanIndividual(r.db.QueryRow(query, id, orgID))
}

func (r *ClientRepository) ListIndividual(orgID uuid.UUID, search, status string, page, pageSize int) ([]models.IndividualClient, int, error) {
	where, args, i := r.buildClientWhere(orgID, search, status)

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM individual_profiles WHERE %s`, where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	query := fmt.Sprintf(`
		SELECT id, full_name, email, phone, id_passport_number, nationality, status, notes, created_at, updated_at
		FROM individual_profiles
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, i, i+1)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var clients []models.IndividualClient
	for rows.Next() {
		c, err := r.scanIndividual(rows)
		if err != nil {
			return nil, 0, err
		}
		clients = append(clients, *c)
	}
	return clients, total, rows.Err()
}

func (r *ClientRepository) UpdateIndividual(c *models.IndividualClient, orgID uuid.UUID) error {
	query := `
		UPDATE individual_profiles
		SET full_name=$1, email=$2, phone=$3, id_passport_number=$4,
		    nationality=$5, status=$6, notes=$7, updated_at=$8
		WHERE id=$9 AND org_id=$10`

	c.UpdatedAt = time.Now()
	_, err := r.db.Exec(query,
		c.FullName, c.Email, c.Phone, c.IDPassportNumber,
		c.Nationality, c.Status, c.Notes, c.UpdatedAt, c.ID, orgID,
	)
	return err
}

func (r *ClientRepository) DeleteIndividual(id uuid.UUID, orgID uuid.UUID) error {
	query := `DELETE FROM individual_profiles WHERE id=$1 AND org_id=$2`

	res, err := r.db.Exec(query, id, orgID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("individual client not found")
	}
	return nil
}

// ─── Corporate ────────────────────────────────────────────────────────────────
//
// The Corporate Clients view is served from cor_company_details — the table
// corporate bookings populate + dedup on (org_id, reg_number, tpin). The contact
// person/email/phone are enriched from a representative cor_profiles row (the most
// recently updated profile for the company). CompanyRegNumber maps to reg_number.

// corporateSelect is the shared projection: company details joined to a
// representative contact profile. Callers append their own WHERE + ORDER/LIMIT.
const corporateSelect = `
	SELECT c.id, c.company_name,
	       COALESCE(NULLIF(TRIM(p.first_name || ' ' || p.last_name), ''), '') AS contact_person,
	       COALESCE(p.email, '') AS email,
	       COALESCE(p.phone, '') AS phone,
	       COALESCE(c.reg_number, '') AS company_reg_number,
	       COALESCE(c.tpin, '')       AS tpin,
	       COALESCE(c.industry, '')   AS industry,
	       COALESCE(c.country, '')    AS country,
	       c.status, c.created_at, c.updated_at
	FROM cor_company_details c
	LEFT JOIN LATERAL (
	    SELECT first_name, last_name, email, phone
	    FROM cor_profiles p
	    WHERE p.company_id = c.id
	    ORDER BY p.updated_at DESC
	    LIMIT 1
	) p ON TRUE`

// CreateCorporate inserts a company into cor_company_details. Contact-person fields
// on the request are ignored here (contacts live on cor_profiles and are created
// through the booking chain); the staff-created company is a bare shell that
// bookings later enrich.
func (r *ClientRepository) CreateCorporate(c *models.CorporateClient, orgID uuid.UUID) error {
	return r.db.QueryRow(`
		INSERT INTO cor_company_details (org_id, company_name, tpin, reg_number, industry, country, status)
		VALUES ($1,$2,$3,$4,$5,$6,COALESCE(NULLIF($7,''),'active'))
		RETURNING id, created_at, updated_at`,
		orgID, c.CompanyName, c.TPIN, c.CompanyRegNumber, c.Industry, c.Country, c.Status,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *ClientRepository) GetCorporateByID(id uuid.UUID, orgID uuid.UUID) (*models.CorporateClient, error) {
	return r.scanCorporate(r.db.QueryRow(corporateSelect+`
		WHERE c.id = $1 AND c.org_id = $2`, id, orgID))
}

func (r *ClientRepository) ListCorporate(orgID uuid.UUID, search, status string, page, pageSize int) ([]models.CorporateClient, int, error) {
	where, args, i := r.buildCorporateWhere(orgID, search, status)

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM cor_company_details c WHERE %s`, where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	query := fmt.Sprintf(`%s WHERE %s ORDER BY c.company_name ASC LIMIT $%d OFFSET $%d`,
		corporateSelect, where, i, i+1)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var clients []models.CorporateClient
	for rows.Next() {
		c, err := r.scanCorporate(rows)
		if err != nil {
			return nil, 0, err
		}
		clients = append(clients, *c)
	}
	return clients, total, rows.Err()
}

// UpdateCorporate updates the company-level fields on cor_company_details. Contact
// fields aren't written here (they belong to cor_profiles).
func (r *ClientRepository) UpdateCorporate(c *models.CorporateClient, orgID uuid.UUID) error {
	res, err := r.db.Exec(`
		UPDATE cor_company_details
		SET company_name=$1, tpin=$2, reg_number=$3, industry=$4, country=$5,
		    status=COALESCE(NULLIF($6,''), status), updated_at=NOW()
		WHERE id=$7 AND org_id=$8`,
		c.CompanyName, c.TPIN, c.CompanyRegNumber, c.Industry, c.Country, c.Status, c.ID, orgID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("corporate client not found")
	}
	return nil
}

func (r *ClientRepository) DeleteCorporate(id uuid.UUID, orgID uuid.UUID) error {
	res, err := r.db.Exec(`DELETE FROM cor_company_details WHERE id=$1 AND org_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("corporate client not found")
	}
	return nil
}

// LookupIndividualByIDNumber returns the individual client matching an exact
// NRC or passport number within the org — used by the booking dialog pre-flight.
func (r *ClientRepository) LookupIndividualByIDNumber(orgID uuid.UUID, idNumber string) (*models.IndividualClient, error) {
	return r.scanIndividual(r.db.QueryRow(`
		SELECT id, full_name, email, phone, id_passport_number, nationality, status, notes, created_at, updated_at
		FROM individual_profiles
		WHERE org_id = $1 AND id_passport_number = $2`, orgID, idNumber))
}

// SearchCorporate returns corporate clients matching a search term against
// company name, reg number or TPIN — used by the booking dialog pre-flight.
func (r *ClientRepository) SearchCorporate(orgID uuid.UUID, search string, limit int) ([]models.CorporateClient, error) {
	rows, err := r.db.Query(corporateSelect+`
		WHERE c.org_id = $1
		  AND (c.company_name ILIKE $2 OR c.reg_number ILIKE $2 OR c.tpin ILIKE $2)
		ORDER BY c.company_name ASC
		LIMIT $3`, orgID, "%"+search+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clients := []models.CorporateClient{}
	for rows.Next() {
		c, err := r.scanCorporate(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, *c)
	}
	return clients, rows.Err()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildClientWhere builds the individual_profiles WHERE clause with optional
// search (name / email / ID number) and status filters.
func (r *ClientRepository) buildClientWhere(orgID uuid.UUID, search, status string) (string, []interface{}, int) {
	args := []interface{}{orgID}
	conditions := []string{"org_id = $1"}
	i := 2

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(email ILIKE $%d OR full_name ILIKE $%d OR id_passport_number ILIKE $%d)", i, i, i))
		args = append(args, "%"+search+"%")
		i++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", i))
		args = append(args, status)
		i++
	}

	return strings.Join(conditions, " AND "), args, i
}

// buildCorporateWhere builds the cor_company_details WHERE clause (aliased "c")
// with optional search (company name / reg number / TPIN) and status filters.
func (r *ClientRepository) buildCorporateWhere(orgID uuid.UUID, search, status string) (string, []interface{}, int) {
	args := []interface{}{orgID}
	conditions := []string{"c.org_id = $1"}
	i := 2

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(c.company_name ILIKE $%d OR c.reg_number ILIKE $%d OR c.tpin ILIKE $%d)", i, i, i))
		args = append(args, "%"+search+"%")
		i++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("c.status = $%d", i))
		args = append(args, status)
		i++
	}

	return strings.Join(conditions, " AND "), args, i
}

func (r *ClientRepository) scanIndividual(row rowScanner) (*models.IndividualClient, error) {
	var c models.IndividualClient
	var idPassport, nationality, notes sql.NullString

	err := row.Scan(
		&c.ID, &c.FullName, &c.Email, &c.Phone, &idPassport,
		&nationality, &c.Status, &notes, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if idPassport.Valid {
		c.IDPassportNumber = idPassport.String
	}
	if nationality.Valid {
		c.Nationality = nationality.String
	}
	if notes.Valid {
		c.Notes = notes.String
	}
	return &c, nil
}

// scanCorporate scans a row from corporateSelect. Column order:
// id, company_name, contact_person, email, phone, company_reg_number,
// tpin, industry, country, status, created_at, updated_at.
func (r *ClientRepository) scanCorporate(row rowScanner) (*models.CorporateClient, error) {
	var c models.CorporateClient
	err := row.Scan(
		&c.ID, &c.CompanyName, &c.ContactPerson, &c.Email, &c.Phone,
		&c.CompanyRegNumber, &c.TPIN, &c.Industry, &c.Country, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
