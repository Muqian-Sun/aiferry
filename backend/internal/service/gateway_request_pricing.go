package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type gatewayRequestPricingAtCtxKey struct{}

// WithGatewayRequestPricing marks a shared-gateway request as billed (token
// requests and media generation) and freezes the downstream pricing instant for
// its whole lifetime; the profit gate is installed only on marked requests.
// Metadata-only handlers and lookups of existing media tasks deliberately do not
// call this helper.
func WithGatewayRequestPricing(ctx context.Context) (context.Context, time.Time) {
	if ctx == nil {
		ctx = context.Background()
	}
	pricingAt := timezone.Now()
	ctx = context.WithValue(ctx, gatewayRequestPricingAtCtxKey{}, pricingAt)
	return ctx, pricingAt
}

func gatewayRequestPricingAtFromContext(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	pricingAt, ok := ctx.Value(gatewayRequestPricingAtCtxKey{}).(time.Time)
	return pricingAt, ok && !pricingAt.IsZero()
}

// GatewayRequestPricingAtFromContext exposes the frozen instant to
// handlers before they detach asynchronous usage-recording work.
func GatewayRequestPricingAtFromContext(ctx context.Context) time.Time {
	pricingAt, _ := gatewayRequestPricingAtFromContext(ctx)
	return pricingAt
}
