package repository

import (
	"database/sql"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type RoleRepository struct {
	db *sql.DB
}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{db: database.DB}
}

func (r *RoleRepository) GetRoleByName(name string) (*models.Role, error) {
	var role models.Role
	err := r.db.QueryRow(
		`SELECT role_id, name, description, created_at, updated_at FROM roles WHERE name = $1`, name,
	).Scan(&role.RoleID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetAllRoles returns one row per role name for the given org. Roles exist in two
// scopes — the org's own rows and a shared NULL-org set — so we deduplicate by name,
// preferring the org-scoped row (org_id = $1) over the shared one (org_id IS NULL).
func (r *RoleRepository) GetAllRoles(orgID uuid.UUID) ([]models.Role, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT ON (name) role_id, name, description, created_at, updated_at
		FROM roles
		WHERE org_id = $1 OR org_id IS NULL
		ORDER BY name, (org_id IS NULL)`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.RoleID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}
