package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Queries struct {
	db DBTX
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type CreateImageJobParams struct {
	UserID      *string
	Provider    string
	Model       string
	Quantity    int32
	AspectRatio *string
	Prompt      []byte
	SourceAsset []byte
}

func (q *Queries) CreateImageJob(ctx context.Context, arg CreateImageJobParams) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, `
INSERT INTO image_jobs (user_id, provider, model, status, quantity, aspect_ratio, prompt, source_asset)
VALUES ($1, $2, $3, 'QUEUED', $4, $5, $6, $7)
RETURNING id
`, arg.UserID, arg.Provider, arg.Model, arg.Quantity, arg.AspectRatio, arg.Prompt, arg.SourceAsset)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

func (q *Queries) StartImageJob(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, `
UPDATE image_jobs
SET status = 'RUNNING', updated_at = now()
WHERE id = $1
`, id)
	return err
}

type CompleteImageJobParams struct {
	ID     uuid.UUID
	Output []byte
}

func (q *Queries) CompleteImageJob(ctx context.Context, arg CompleteImageJobParams) error {
	_, err := q.db.Exec(ctx, `
UPDATE image_jobs
SET status = 'SUCCEEDED', output = $2, updated_at = now()
WHERE id = $1
`, arg.ID, arg.Output)
	return err
}

type FailImageJobParams struct {
	ID    uuid.UUID
	Error string
}

func (q *Queries) FailImageJob(ctx context.Context, arg FailImageJobParams) error {
	_, err := q.db.Exec(ctx, `
UPDATE image_jobs
SET status = 'FAILED', error = $2, updated_at = now()
WHERE id = $1
`, arg.ID, arg.Error)
	return err
}

type ImageJob struct {
	ID          uuid.UUID
	UserID      sql.NullString
	Provider    string
	Model       string
	Status      string
	Quantity    int32
	AspectRatio sql.NullString
	Prompt      []byte
	SourceAsset []byte
	Output      []byte
	Error       sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (q *Queries) GetImageJob(ctx context.Context, id uuid.UUID) (ImageJob, error) {
	row := q.db.QueryRow(ctx, `
SELECT id, user_id, provider, model, status, quantity, aspect_ratio, prompt, source_asset, output, error, created_at, updated_at
FROM image_jobs
WHERE id = $1
`, id)
	var job ImageJob
	err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.Provider,
		&job.Model,
		&job.Status,
		&job.Quantity,
		&job.AspectRatio,
		&job.Prompt,
		&job.SourceAsset,
		&job.Output,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	return job, err
}

type ListImageJobsByUserParams struct {
	UserID string
	Limit  int32
	Offset int32
}

func (q *Queries) ListImageJobsByUser(ctx context.Context, arg ListImageJobsByUserParams) ([]ImageJob, error) {
	rows, err := q.db.Query(ctx, `
SELECT id, user_id, provider, model, status, quantity, aspect_ratio, prompt, source_asset, output, error, created_at, updated_at
FROM image_jobs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`, arg.UserID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []ImageJob
	for rows.Next() {
		var job ImageJob
		if err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.Provider,
			&job.Model,
			&job.Status,
			&job.Quantity,
			&job.AspectRatio,
			&job.Prompt,
			&job.SourceAsset,
			&job.Output,
			&job.Error,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

type StatsSummaryRow struct {
	Total       int64
	Succeeded   int64
	Failed      int64
	SuccessRate sql.NullFloat64
}

func (q *Queries) StatsSummary(ctx context.Context) (StatsSummaryRow, error) {
	row := q.db.QueryRow(ctx, `
WITH agg AS (
  SELECT
    count(*) AS total,
    count(*) FILTER (WHERE status = 'SUCCEEDED') AS succeeded,
    count(*) FILTER (WHERE status = 'FAILED') AS failed
  FROM image_jobs
)
SELECT total, succeeded, failed,
       ROUND(100.0 * succeeded / NULLIF(total, 0), 2) AS success_rate
FROM agg
`)
	var summary StatsSummaryRow
	err := row.Scan(&summary.Total, &summary.Succeeded, &summary.Failed, &summary.SuccessRate)
	return summary, err
}
