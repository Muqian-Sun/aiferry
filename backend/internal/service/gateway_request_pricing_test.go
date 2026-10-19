//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithGatewayRequestPricingMarksOnlyExplicitRequests(t *testing.T) {
	ctx, pricingAt := WithGatewayRequestPricing(context.Background())

	got, ok := gatewayRequestPricingAtFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, pricingAt, got)
	require.Equal(t, pricingAt, GatewayRequestPricingAtFromContext(ctx))
	require.True(t, GatewayRequestPricingAtFromContext(context.Background()).IsZero())
}
