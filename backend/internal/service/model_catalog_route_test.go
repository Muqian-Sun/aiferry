//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestCatalogRoutePlatform(t *testing.T) {
	cases := []struct {
		name  string
		entry ModelCatalogEntry
		want  string
	}{
		{"explicit route_platform wins over vendor", ModelCatalogEntry{ModelID: "claude-x", Vendor: "anthropic", RoutePlatform: PlatformOpenAI}, PlatformOpenAI},
		{"vendor map", ModelCatalogEntry{ModelID: "whatever", Vendor: "xai"}, PlatformGrok},
		{"vendor prefix vertex_ai", ModelCatalogEntry{ModelID: "whatever", Vendor: "vertex_ai-language-models"}, PlatformGemini},
		{"vendor text-completion-openai", ModelCatalogEntry{ModelID: "whatever", Vendor: "text-completion-openai"}, PlatformOpenAI},
		{"unknown vendor falls back to model detection", ModelCatalogEntry{ModelID: "claude-sonnet-4", Vendor: "volcengine"}, PlatformAnthropic},
		{"empty vendor falls back to model detection", ModelCatalogEntry{ModelID: "gemini-2.5-pro"}, PlatformGemini},
		{"protocols: anthropic wins", ModelCatalogEntry{ModelID: "team/best", Protocols: []string{ModelCatalogProtocolChatCompletions, ModelCatalogProtocolAnthropic}}, PlatformAnthropic},
		{"protocols: gemini only", ModelCatalogEntry{ModelID: "team/best", Protocols: []string{ModelCatalogProtocolGemini}}, PlatformGemini},
		{"protocols: gemini plus chat goes openai", ModelCatalogEntry{ModelID: "team/best", Protocols: []string{ModelCatalogProtocolGemini, ModelCatalogProtocolChatCompletions}}, PlatformOpenAI},
		{"nothing known defaults to openai", ModelCatalogEntry{ModelID: "team/best"}, PlatformOpenAI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := tc.entry
			require.Equal(t, tc.want, CatalogRoutePlatform(&entry))
		})
	}
	require.Equal(t, PlatformOpenAI, CatalogRoutePlatform(nil))
}

func TestModelCatalogService_ResolveRoute(t *testing.T) {
	repo := &stubModelCatalogRepo{entries: []ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", Vendor: "anthropic", Status: ModelCatalogStatusListed,
			Aliases: []ModelCatalogAlias{{ID: 10, EntryID: 1, Alias: "sonnet-latest"}}},
		{ID: 2, ModelID: "gpt-5.6", Vendor: "openai", Status: ModelCatalogStatusUnlisted},
	}}
	svc := NewModelCatalogService(repo, nil, ModelCatalogSeedInput{})
	ctx := context.Background()

	route, ok := svc.ResolveRoute(ctx, " claude-sonnet-4 ")
	require.True(t, ok)
	require.Equal(t, int64(1), route.EntryID)
	require.Equal(t, "claude-sonnet-4", route.CanonicalModel)
	require.Equal(t, "claude-sonnet-4", route.RequestedModel)
	require.Equal(t, PlatformAnthropic, route.Platform)
	require.NotNil(t, route.Entry)

	route, ok = svc.ResolveRoute(ctx, "sonnet-latest")
	require.True(t, ok, "alias resolves to the entry")
	require.Equal(t, "claude-sonnet-4", route.CanonicalModel)
	require.Equal(t, "sonnet-latest", route.RequestedModel)

	_, ok = svc.ResolveRoute(ctx, "gpt-5.6")
	require.False(t, ok, "unlisted entry must not be admitted")

	_, ok = svc.ResolveRoute(ctx, "nope")
	require.False(t, ok)

	listed := svc.ListListedEntries(ctx)
	require.Len(t, listed, 1)
	require.Equal(t, "claude-sonnet-4", listed[0].ModelID)

	require.Equal(t, []string{"claude-sonnet-4", "sonnet-latest"},
		FilterListedModelIDs(ctx, svc, []string{"claude-sonnet-4", "gpt-5.6", "sonnet-latest", "nope"}))
	require.True(t, IsListedModel(ctx, svc, "sonnet-latest"))
	require.False(t, IsListedModel(ctx, svc, "gpt-5.6"))
}

func TestWithCatalogRoute(t *testing.T) {
	route := CatalogRoute{EntryID: 7, CanonicalModel: "gpt-5.6", RequestedModel: "gpt-5.6-sol", Platform: PlatformOpenAI}
	ctx := WithCatalogRoute(context.Background(), route)

	got, ok := CatalogRouteFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, route, got)

	platform, ok := ResolvedTargetPlatformFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, PlatformOpenAI, platform)
	require.Equal(t, "gpt-5.6-sol", ctx.Value(ctxkey.RequestedPublicModel))

	_, ok = CatalogRouteFromContext(context.Background())
	require.False(t, ok)
}

func TestAccountServesCatalogEntry(t *testing.T) {
	chatOnlyKey := &Account{ID: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://cc.example.com"}}
	anthropicKey := &Account{ID: 2, Type: AccountTypeAPIKey, Platform: PlatformAnthropic,
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"}}
	anthropicOAuth := &Account{ID: 3, Type: AccountTypeOAuth, Platform: PlatformAnthropic}
	antigravity := &Account{ID: 4, Type: AccountTypeOAuth, Platform: PlatformAntigravity}
	geminiOAuth := &Account{ID: 5, Type: AccountTypeOAuth, Platform: PlatformGemini}
	grokOAuth := &Account{ID: 6, Type: AccountTypeOAuth, Platform: PlatformGrok}
	openAIOAuth := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformOpenAI}

	anthropicEntry := &ModelCatalogEntry{ModelID: "claude-sonnet-4", Vendor: "anthropic"}
	openAIEntry := &ModelCatalogEntry{ModelID: "claude-sonnet-4", Vendor: "anthropic", RoutePlatform: PlatformOpenAI}
	geminiEntry := &ModelCatalogEntry{ModelID: "gemini-2.5-pro", Vendor: "gemini"}
	grokEntry := &ModelCatalogEntry{ModelID: "grok-4", Vendor: "xai"}

	cases := []struct {
		name    string
		entry   *ModelCatalogEntry
		account *Account
		ok      bool
	}{
		{"chat-only key cannot serve anthropic family", anthropicEntry, chatOnlyKey, false},
		{"same key serves the entry once routed to the openai family", openAIEntry, chatOnlyKey, true},
		{"anthropic-address key serves anthropic family", anthropicEntry, anthropicKey, true},
		{"anthropic-address key also serves openai family via anthropic protocol", openAIEntry, anthropicKey, true},
		{"anthropic oauth serves anthropic family", anthropicEntry, anthropicOAuth, true},
		{"anthropic oauth cannot serve gemini family", geminiEntry, anthropicOAuth, false},
		{"antigravity serves anthropic family", anthropicEntry, antigravity, true},
		{"antigravity serves gemini family", geminiEntry, antigravity, true},
		{"antigravity cannot serve openai family", openAIEntry, antigravity, false},
		{"gemini oauth serves gemini family", geminiEntry, geminiOAuth, true},
		{"gemini oauth cannot serve anthropic family", anthropicEntry, geminiOAuth, false},
		{"grok oauth serves grok family", grokEntry, grokOAuth, true},
		{"grok oauth cannot serve openai family", openAIEntry, grokOAuth, false},
		{"openai oauth serves openai family", openAIEntry, openAIOAuth, true},
		{"openai oauth cannot serve grok family", grokEntry, openAIOAuth, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := AccountServesCatalogEntry(tc.entry, tc.account)
			if tc.ok {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "CATALOG_BINDING_UNSERVABLE")
		})
	}
	require.Error(t, AccountServesCatalogEntry(nil, chatOnlyKey))
	require.Error(t, AccountServesCatalogEntry(anthropicEntry, nil))
}
