package domain

import "time"

// AnalyticsDaily stores aggregated metrics for a specific day.
type AnalyticsDaily struct {
	Day             time.Time
	Visitors        int
	OnlineUsers     int
	AIRequests      int
	Last24h         int
	VideosGenerated int
	ImagesGenerated int
	RequestSuccess  int
	RequestFail     int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
