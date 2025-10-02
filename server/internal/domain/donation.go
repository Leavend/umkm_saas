package domain

import "time"

// Donation represents a supporter contribution record.
type Donation struct {
	ID        string
	UserID    *string
	AmountInt int64
	Note      string
	CreatedAt time.Time
}
