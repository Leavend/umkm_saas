package domain

import "time"

// JobType enumerates supported generation job categories.
type JobType string

const (
	JobTypeImageGenerate JobType = "image_generate"
	JobTypeImageEnhance  JobType = "image_enhance"
	JobTypeVideoGenerate JobType = "video_generate"
)

// JobStatus enumerates job lifecycle states.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

// Job encapsulates the lifecycle of image/video generation.
type Job struct {
	ID           string
	UserID       string
	Type         JobType
	Status       JobStatus
	PromptJSON   []byte
	ResultJSON   []byte
	Quantity     int
	AspectRatio  string
	Provider     string
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PromptJSON holds the canonical contract as raw bytes for storage while still exposing structured helpers.
type PromptJSON map[string]any
