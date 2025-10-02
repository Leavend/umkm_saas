package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/domain"
)

// AssetRepositoryPG implements domain.AssetRepository using PostgreSQL.
type AssetRepositoryPG struct {
	pool *pgxpool.Pool
}

// NewAssetRepository constructs a new asset repository instance.
func NewAssetRepository(pool *pgxpool.Pool) *AssetRepositoryPG {
	return &AssetRepositoryPG{pool: pool}
}

// ListByJobID returns all assets belonging to the job.
func (r *AssetRepositoryPG) ListByJobID(ctx context.Context, jobID string) ([]domain.Asset, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, job_id, kind, url, width, height, checksum, bytes, created_at
FROM assets
WHERE job_id = $1
ORDER BY created_at ASC;
`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []domain.Asset
	for rows.Next() {
		var asset domain.Asset
		if err := rows.Scan(&asset.ID, &asset.JobID, &asset.Kind, &asset.URL, &asset.Width, &asset.Height, &asset.Checksum, &asset.Bytes, &asset.CreatedAt); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return assets, nil
}

// SaveAll persists a list of assets.
func (r *AssetRepositoryPG) SaveAll(ctx context.Context, jobID string, assets []domain.Asset) error {
	if len(assets) == 0 {
		return nil
	}

	query := `
INSERT INTO assets (id, job_id, kind, url, width, height, checksum, bytes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
`

	for _, asset := range assets {
		a := asset
		if _, err := r.pool.Exec(ctx, query, a.ID, jobID, a.Kind, a.URL, a.Width, a.Height, a.Checksum, a.Bytes); err != nil {
			return err
		}
	}

	return nil
}
