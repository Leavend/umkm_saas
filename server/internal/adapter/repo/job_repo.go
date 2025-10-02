package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/domain"
)

// JobRepositoryPG implements domain.JobRepository.
type JobRepositoryPG struct {
	pool *pgxpool.Pool
}

// NewJobRepository creates a new job repository backed by PostgreSQL.
func NewJobRepository(pool *pgxpool.Pool) *JobRepositoryPG {
	return &JobRepositoryPG{pool: pool}
}

// Create inserts a new job record.
func (r *JobRepositoryPG) Create(ctx context.Context, job *domain.Job) error {
	query := `
INSERT INTO jobs (id, user_id, type, status, prompt_json, result_json, quantity, aspect_ratio, provider, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
`
	_, err := r.pool.Exec(ctx, query,
		job.ID,
		job.UserID,
		job.Type,
		job.Status,
		job.PromptJSON,
		job.ResultJSON,
		job.Quantity,
		job.AspectRatio,
		job.Provider,
		job.ErrorMessage,
	)
	return err
}

// UpdateStatus updates job status and optionally error/result payloads.
func (r *JobRepositoryPG) UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, errMsg *string, resultJSON []byte) error {
	query := `
UPDATE jobs
SET status = $2,
    updated_at = NOW(),
    error_message = COALESCE($3, error_message),
    result_json = COALESCE($4, result_json)
WHERE id = $1;
`
	_, err := r.pool.Exec(ctx, query, jobID, status, errMsg, nullableBytes(resultJSON))
	return err
}

// GetByID fetches a job by its identifier.
func (r *JobRepositoryPG) GetByID(ctx context.Context, jobID string) (*domain.Job, error) {
	query := `
SELECT id, user_id, type, status, prompt_json, result_json, quantity, aspect_ratio, provider, error_message, created_at, updated_at
FROM jobs
WHERE id = $1;
`
	row := r.pool.QueryRow(ctx, query, jobID)
	var job domain.Job
	if err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.Type,
		&job.Status,
		&job.PromptJSON,
		&job.ResultJSON,
		&job.Quantity,
		&job.AspectRatio,
		&job.Provider,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &job, nil
}

func nullableBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}
