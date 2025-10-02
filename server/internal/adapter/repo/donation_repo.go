package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/domain"
)

// DonationRepositoryPG implements DonationRepository using PostgreSQL.
type DonationRepositoryPG struct {
	pool *pgxpool.Pool
}

// NewDonationRepository creates a new donation repo.
func NewDonationRepository(pool *pgxpool.Pool) *DonationRepositoryPG {
	return &DonationRepositoryPG{pool: pool}
}

// Create inserts a new donation record.
func (r *DonationRepositoryPG) Create(ctx context.Context, donation *domain.Donation) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO donations (id, user_id, amount_int, note)
VALUES ($1, $2, $3, $4);
`, donation.ID, donation.UserID, donation.AmountInt, donation.Note)
	return err
}

// ListRecent returns recent donations limited by the input value.
func (r *DonationRepositoryPG) ListRecent(ctx context.Context, limit int) ([]domain.Donation, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, amount_int, note, created_at
FROM donations
ORDER BY created_at DESC
LIMIT $1;
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Donation
	for rows.Next() {
		var donation domain.Donation
		if err := rows.Scan(&donation.ID, &donation.UserID, &donation.AmountInt, &donation.Note, &donation.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, donation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
