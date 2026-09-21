//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsUpstreamModelRestrictedByChannel_CompactMappingMatchesForwardPath(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping":         map[string]any{"gpt-5.4-channel": "gpt-5.4-account"},
			"compact_model_mapping": map[string]any{"gpt-5.4-account": "gpt-5.4-compact"},
		},
	}
	tests := []struct {
		name                   string
		allowedUpstreamModel   string
		useCompactModelMapping bool
	}{
		{
			name:                   "legacy compact applies compact mapping after channel and account mapping",
			allowedUpstreamModel:   "gpt-5.4-compact",
			useCompactModelMapping: true,
		},
		{
			name:                   "native v2 stops after channel and account mapping",
			allowedUpstreamModel:   "gpt-5.4-account",
			useCompactModelMapping: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			channelSvc := newTestChannelService(makeStandardRepo(Channel{
				ID:                 1,
				Status:             StatusActive,
				GroupIDs:           []int64{10},
				RestrictModels:     true,
				BillingModelSource: BillingModelSourceUpstream,
				ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, Models: []string{tt.allowedUpstreamModel}},
				},
				ModelMapping: map[string]map[string]string{
					PlatformOpenAI: {"gpt-5.4": "gpt-5.4-channel"},
				},
			}, map[int64]string{10: PlatformOpenAI}))
			svc := &GatewayService{channelService: channelSvc}
			mapping := channelSvc.ResolveChannelMapping(context.Background(), 10, "gpt-5.4")
			require.True(t, mapping.Mapped)
			require.Equal(t, "gpt-5.4-channel", mapping.MappedModel)

			ctx := WithOpenAIForwardModel(
				context.Background(),
				mapping.MappedModel,
				tt.useCompactModelMapping,
			)
			require.False(t, svc.isUpstreamModelRestrictedByChannel(
				ctx, 10, account, "gpt-5.4",
			))
			require.True(t, svc.isUpstreamModelRestrictedByChannel(
				context.Background(), 10, account, "gpt-5.4",
			), "without the forward-model context the restriction check follows a different chain")
		})
	}
}

func TestIsUpstreamModelRestrictedByChannel_PassthroughMatchesForwardPath(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-5.4-channel": "gpt-5.4-account"},
			"compact_model_mapping": map[string]any{
				"gpt-5.4-channel": "gpt-5.4-compact",
			},
		},
		Extra:             map[string]any{"openai_passthrough": true},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	tests := []struct {
		name                   string
		allowedUpstreamModel   string
		useCompactModelMapping bool
	}{
		{
			name:                   "native v2 keeps channel-mapped model and ignores normal account mapping",
			allowedUpstreamModel:   "gpt-5.4-channel",
			useCompactModelMapping: false,
		},
		{
			name:                   "legacy compact applies compact mapping to channel-mapped model",
			allowedUpstreamModel:   "gpt-5.4-compact",
			useCompactModelMapping: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			channelSvc := newTestChannelService(makeStandardRepo(Channel{
				ID:                 1,
				Status:             StatusActive,
				GroupIDs:           []int64{10},
				RestrictModels:     true,
				BillingModelSource: BillingModelSourceUpstream,
				ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, Models: []string{tt.allowedUpstreamModel}},
				},
				ModelMapping: map[string]map[string]string{
					PlatformOpenAI: {"gpt-5.4": "gpt-5.4-channel"},
				},
			}, map[int64]string{10: PlatformOpenAI}))
			svc := &GatewayService{channelService: channelSvc}
			mapping := channelSvc.ResolveChannelMapping(context.Background(), 10, "gpt-5.4")
			require.True(t, mapping.Mapped)
			require.Equal(t, "gpt-5.4-channel", mapping.MappedModel)

			ctx := WithOpenAIForwardModel(
				context.Background(),
				mapping.MappedModel,
				tt.useCompactModelMapping,
			)
			require.False(t, svc.isUpstreamModelRestrictedByChannel(
				ctx, 10, account, "gpt-5.4",
			))
		})
	}
}

func TestIsUpstreamModelRestrictedByChannel_PassthroughFlagWithRawChatFallbackMatchesForwardPath(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-5.4-channel": "gpt-5.4-account"},
			"compact_model_mapping": map[string]any{
				"gpt-5.4-account": "gpt-5.4-compact",
			},
		},
		Extra: map[string]any{
			"openai_passthrough": true,
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com"},
	}

	for _, useCompactModelMapping := range []bool{false, true} {
		useCompactModelMapping := useCompactModelMapping
		name := "native v2"
		if useCompactModelMapping {
			name = "legacy compact"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			channelSvc := newTestChannelService(makeStandardRepo(Channel{
				ID:                 1,
				Status:             StatusActive,
				GroupIDs:           []int64{10},
				RestrictModels:     true,
				BillingModelSource: BillingModelSourceUpstream,
				ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, Models: []string{"gpt-5.4-account"}},
				},
				ModelMapping: map[string]map[string]string{
					PlatformOpenAI: {"gpt-5.4": "gpt-5.4-channel"},
				},
			}, map[int64]string{10: PlatformOpenAI}))
			svc := &GatewayService{channelService: channelSvc}
			ctx := WithOpenAIForwardModel(
				context.Background(),
				"gpt-5.4-channel",
				useCompactModelMapping,
			)

			require.False(t, svc.isUpstreamModelRestrictedByChannel(
				ctx, 10, account, "gpt-5.4",
			))
		})
	}
}

func TestIsUpstreamModelRestrictedByChannel_ForwardModelContextMatchesNormalForwardPath(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-5.4-channel": "gpt-5.4-account"},
		},
		Extra: map[string]any{
			"openai_passthrough": true,
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com"},
	}
	channelSvc := newTestChannelService(makeStandardRepo(Channel{
		ID:                 1,
		Status:             StatusActive,
		GroupIDs:           []int64{10},
		RestrictModels:     true,
		BillingModelSource: BillingModelSourceUpstream,
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformOpenAI, Models: []string{"gpt-5.4-account"}},
		},
		ModelMapping: map[string]map[string]string{
			PlatformOpenAI: {"gpt-5.4": "gpt-5.4-channel"},
		},
	}, map[int64]string{10: PlatformOpenAI}))
	svc := &GatewayService{channelService: channelSvc}
	ctx := WithOpenAIForwardModel(context.Background(), "gpt-5.4-channel", false)

	require.False(t, svc.isUpstreamModelRestrictedByChannel(
		ctx, 10, account, "gpt-5.4",
	))
}
