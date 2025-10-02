package domain

import "time"

// UserRole enumerates supported roles.
type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

// UserPlan enumerates billing plans.
type UserPlan string

const (
	UserPlanFree UserPlan = "free"
	UserPlanPro  UserPlan = "pro"
)

// User represents an authenticated account within the platform.
type User struct {
	ID         string
	GoogleSub  string
	Email      string
	Name       string
	Picture    string
	Locale     string
	Role       UserRole
	Plan       UserPlan
	QuotaDaily int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	QuotaUsed  int // derived field for remaining quota calculations
}

// IsFree reports whether the user is using the free plan.
func (u User) IsFree() bool {
	return u.Plan == UserPlanFree
}
