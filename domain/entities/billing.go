package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

// SubscriptionStatus represents the status of a subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusTrialing SubscriptionStatus = "trialing"
	SubscriptionStatusPaused   SubscriptionStatus = "paused"
)

func (s SubscriptionStatus) String() string {
	return string(s)
}

// UsageMetric represents a tracked usage dimension.
type UsageMetric string

const (
	UsageMetricDomains        UsageMetric = "domains"
	UsageMetricEmailsReceived UsageMetric = "emails_received"
	UsageMetricEmailsSent     UsageMetric = "emails_sent"
	UsageMetricStorageMB      UsageMetric = "storage_mb"
)

func (m UsageMetric) String() string {
	return string(m)
}

// Plan represents a billing plan tier.
type Plan struct {
	ID             uuid.UUID `json:"id"               db:"id"`
	Name           string    `json:"name"             db:"name"`
	DisplayName    string    `json:"display_name"     db:"display_name"`
	Description    string    `json:"description"      db:"description"`
	StripePriceID  string    `json:"stripe_price_id"  db:"stripe_price_id"`
	PriceCents     int       `json:"price_cents"      db:"price_cents"`
	Currency       string    `json:"currency"         db:"currency"`
	Interval       string    `json:"interval"         db:"interval"`
	DomainLimit    int       `json:"domain_limit"     db:"domain_limit"`
	EmailLimit     int       `json:"email_limit"      db:"email_limit"`
	StorageLimitMB int       `json:"storage_limit_mb" db:"storage_limit_mb"`
	SendLimitDaily int       `json:"send_limit_daily" db:"send_limit_daily"`
	Features       []byte    `json:"features"         db:"features"`
	IsActive       bool      `json:"is_active"        db:"is_active"`
	SortOrder      int       `json:"sort_order"       db:"sort_order"`
	CreatedAt      time.Time `json:"created_at"       db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"       db:"updated_at"`
}

// IsUnlimited returns true when the limit value signals no cap (-1).
func IsUnlimited(limit int) bool {
	return limit == -1
}

// Subscription represents a user's active subscription to a plan.
type Subscription struct {
	ID                   uuid.UUID          `json:"id"                      db:"id"`
	UserID               uuid.UUID          `json:"user_id"                 db:"user_id"`
	PlanID               uuid.UUID          `json:"plan_id"                 db:"plan_id"`
	StripeSubscriptionID string             `json:"stripe_subscription_id"  db:"stripe_subscription_id"`
	StripeCustomerID     string             `json:"stripe_customer_id"      db:"stripe_customer_id"`
	Status               SubscriptionStatus `json:"status"                  db:"status"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start"    db:"current_period_start"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end"      db:"current_period_end"`
	CancelAtPeriodEnd    bool               `json:"cancel_at_period_end"    db:"cancel_at_period_end"`
	CanceledAt           *time.Time         `json:"canceled_at"             db:"canceled_at"`
	TrialEnd             *time.Time         `json:"trial_end"               db:"trial_end"`
	CreatedAt            time.Time          `json:"created_at"              db:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"              db:"updated_at"`

	// Populated via JOIN when needed.
	Plan *Plan `json:"plan,omitempty" db:"-"`
}

func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusTrialing
}

// UsageRecord tracks consumption of a specific metric for a billing period.
type UsageRecord struct {
	ID             uuid.UUID   `json:"id"              db:"id"`
	UserID         uuid.UUID   `json:"user_id"         db:"user_id"`
	SubscriptionID *uuid.UUID  `json:"subscription_id" db:"subscription_id"`
	Metric         UsageMetric `json:"metric"          db:"metric"`
	Value          int64       `json:"value"           db:"value"`
	PeriodStart    time.Time   `json:"period_start"    db:"period_start"`
	PeriodEnd      time.Time   `json:"period_end"      db:"period_end"`
	CreatedAt      time.Time   `json:"created_at"      db:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"      db:"updated_at"`
}
