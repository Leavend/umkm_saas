package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/domain"
)

// AnalyticsRepositoryPG implements AnalyticsRepository using PostgreSQL.
type AnalyticsRepositoryPG struct {
	pool *pgxpool.Pool
}

// NewAnalyticsRepository constructs the repository.
func NewAnalyticsRepository(pool *pgxpool.Pool) *AnalyticsRepositoryPG {
	return &AnalyticsRepositoryPG{pool: pool}
}

// IncrementCounters upserts metrics for the provided day.
func (r *AnalyticsRepositoryPG) IncrementCounters(ctx context.Context, day string, counters map[string]int) error {
	query := `
INSERT INTO analytics_daily (
    day, visitors, online_users, ai_requests, last24h, videos_generated, images_generated, request_success, request_fail
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) ON CONFLICT (day) DO UPDATE SET
    visitors = analytics_daily.visitors + EXCLUDED.visitors,
    online_users = analytics_daily.online_users + EXCLUDED.online_users,
    ai_requests = analytics_daily.ai_requests + EXCLUDED.ai_requests,
    last24h = EXCLUDED.last24h,
    videos_generated = analytics_daily.videos_generated + EXCLUDED.videos_generated,
    images_generated = analytics_daily.images_generated + EXCLUDED.images_generated,
    request_success = analytics_daily.request_success + EXCLUDED.request_success,
    request_fail = analytics_daily.request_fail + EXCLUDED.request_fail;
`
	_, err := r.pool.Exec(ctx, query,
		day,
		counters["visitors"],
		counters["online_users"],
		counters["ai_requests"],
		counters["last24h"],
		counters["videos_generated"],
		counters["images_generated"],
		counters["request_success"],
		counters["request_fail"],
	)
	return err
}

// GetSummary returns aggregated stats.
func (r *AnalyticsRepositoryPG) GetSummary(ctx context.Context) (*domain.AnalyticsDaily, error) {
	row := r.pool.QueryRow(ctx, `
SELECT day, visitors, online_users, ai_requests, last24h, videos_generated, images_generated, request_success, request_fail, created_at, updated_at
FROM analytics_daily
ORDER BY day DESC
LIMIT 1;
`)

	var summary domain.AnalyticsDaily
	if err := row.Scan(
		&summary.Day,
		&summary.Visitors,
		&summary.OnlineUsers,
		&summary.AIRequests,
		&summary.Last24h,
		&summary.VideosGenerated,
		&summary.ImagesGenerated,
		&summary.RequestSuccess,
		&summary.RequestFail,
		&summary.CreatedAt,
		&summary.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &summary, nil
}
