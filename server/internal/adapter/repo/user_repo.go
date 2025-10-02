package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/domain"
)

// UserRepositoryPG implements domain.UserRepository backed by PostgreSQL.
type UserRepositoryPG struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepositoryPG.
func NewUserRepository(pool *pgxpool.Pool) *UserRepositoryPG {
	return &UserRepositoryPG{pool: pool}
}

// UpsertByGoogleSub inserts or updates a user based on Google sub value.
func (r *UserRepositoryPG) UpsertByGoogleSub(ctx context.Context, user *domain.User) (*domain.User, error) {
	query := `
INSERT INTO users (id, google_sub, email, name, picture, locale, role, plan, quota_daily)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (google_sub) DO UPDATE
SET email = EXCLUDED.email,
    name = EXCLUDED.name,
    picture = EXCLUDED.picture,
    locale = EXCLUDED.locale,
    updated_at = NOW()
RETURNING id, google_sub, email, name, picture, locale, role, plan, quota_daily, created_at, updated_at;
`

	row := r.pool.QueryRow(ctx, query,
		user.ID,
		user.GoogleSub,
		user.Email,
		user.Name,
		user.Picture,
		user.Locale,
		user.Role,
		user.Plan,
		user.QuotaDaily,
	)

	return scanUser(row)
}

// GetByID fetches a user by UUID.
func (r *UserRepositoryPG) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, google_sub, email, name, picture, locale, role, plan, quota_daily, created_at, updated_at FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// GetByGoogleSub fetches a user by Google subject identifier.
func (r *UserRepositoryPG) GetByGoogleSub(ctx context.Context, sub string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, google_sub, email, name, picture, locale, role, plan, quota_daily, created_at, updated_at FROM users WHERE google_sub = $1`, sub)
	return scanUser(row)
}

// GetDailyUsage returns the total quantity generated today by the user.
func (r *UserRepositoryPG) GetDailyUsage(ctx context.Context, userID string) (int, error) {
	row := r.pool.QueryRow(ctx, `
SELECT COALESCE(SUM(quantity), 0)
FROM jobs
WHERE user_id = $1
  AND created_at::date = CURRENT_DATE;
`, userID)

	var total int
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.Picture, &u.Locale, &u.Role, &u.Plan, &u.QuotaDaily, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
