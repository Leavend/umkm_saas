package domain

import "context"

// UserRepository defines access methods for users.
type UserRepository interface {
	UpsertByGoogleSub(ctx context.Context, user *User) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByGoogleSub(ctx context.Context, sub string) (*User, error)
	GetDailyUsage(ctx context.Context, userID string) (int, error)
}

// JobRepository defines persistence for job entities.
type JobRepository interface {
	Create(ctx context.Context, job *Job) error
	UpdateStatus(ctx context.Context, jobID string, status JobStatus, errMsg *string, resultJSON []byte) error
	GetByID(ctx context.Context, jobID string) (*Job, error)
}

// AssetRepository handles persistence for generated assets.
type AssetRepository interface {
	ListByJobID(ctx context.Context, jobID string) ([]Asset, error)
	SaveAll(ctx context.Context, jobID string, assets []Asset) error
}

// PromptTemplateRepository defines optional template retrieval for random prompts.
type PromptTemplateRepository interface {
	ListRandom(ctx context.Context, limit int) ([]PromptContract, error)
}

// DonationRepository handles donation persistence.
type DonationRepository interface {
	Create(ctx context.Context, donation *Donation) error
	ListRecent(ctx context.Context, limit int) ([]Donation, error)
}

// AnalyticsRepository updates metrics counters.
type AnalyticsRepository interface {
	IncrementCounters(ctx context.Context, day string, counters map[string]int) error
	GetSummary(ctx context.Context) (*AnalyticsDaily, error)
}
