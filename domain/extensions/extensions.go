// Package extensions defines the integration points between the OSS core
// and external/commercial extensions (e.g. quota enforcement, usage metering).
//
// The OSS core depends only on these interfaces and ships no-op defaults.
// Commercial overlays inject their own implementations at the cmd/ layer.
package extensions

import (
	"context"

	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// DomainLimiter gates domain creation. The OSS default allows every request;
// commercial overlays enforce per-plan limits.
type DomainLimiter interface {
	CheckCanCreateDomain(ctx context.Context, userID uuid.UUID) error
}

// UsageTracker records billable usage events. The OSS default is a no-op;
// commercial overlays persist these to billing storage for invoicing/limits.
type UsageTracker interface {
	IncrementUsage(ctx context.Context, userID uuid.UUID, metric entities.UsageMetric, amount int64) error
}

// NoopDomainLimiter allows every CreateDomain call.
type NoopDomainLimiter struct{}

func (NoopDomainLimiter) CheckCanCreateDomain(_ context.Context, _ uuid.UUID) error {
	return nil
}

// NoopUsageTracker discards every increment.
type NoopUsageTracker struct{}

func (NoopUsageTracker) IncrementUsage(_ context.Context, _ uuid.UUID, _ entities.UsageMetric, _ int64) error {
	return nil
}
