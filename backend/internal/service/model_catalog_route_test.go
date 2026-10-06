//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogVendorPlatform(t *testing.T) {
	cases := []struct {
		name  string
		entry ModelCatalogEntry
		want  string
	}{
		{"vendor map anthropic", ModelCatalogEntry{ModelID: "claude-x", Vendor: "anthropic"}, PlatformAnthropic},
		{"vendor map bedrock", ModelCatalogEntry{ModelID: "claude-x", Vendor: "bedrock"}, PlatformAnthropic},
		{"vendor map openai", ModelCatalogEntry{ModelID: "whatever", Vendor: "openai"}, PlatformOpenAI},
		{"vendor map xai", ModelCatalogEntry{ModelID: "whatever", Vendor: "xai"}, PlatformGrok},
		{"vendor map moonshot", ModelCatalogEntry{ModelID: "whatever", Vendor: "moonshot"}, PlatformKimi},
		{"vendor prefix vertex_ai", ModelCatalogEntry{ModelID: "whatever", Vendor: "vertex_ai-language-models"}, PlatformGemini},
		{"vendor prefix azure", ModelCatalogEntry{ModelID: "whatever", Vendor: "azure_ai"}, PlatformOpenAI},
		{"vendor text-completion-openai", ModelCatalogEntry{ModelID: "whatever", Vendor: "text-completion-openai"}, PlatformOpenAI},
		{"case and whitespace are normalized", ModelCatalogEntry{ModelID: "whatever", Vendor: " XAI "}, PlatformGrok},
		{"unknown vendor is not guessed from the model name", ModelCatalogEntry{ModelID: "claude-sonnet-4", Vendor: "volcengine"}, ""},
		{"empty vendor is not guessed from the model name", ModelCatalogEntry{ModelID: "gemini-2.5-pro"}, ""},
		{"protocols do not imply a vendor", ModelCatalogEntry{ModelID: "team/best", Protocols: []string{ModelCatalogProtocolAnthropic}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := tc.entry
			require.Equal(t, tc.want, CatalogVendorPlatform(&entry))
		})
	}
	require.Equal(t, "", CatalogVendorPlatform(nil))
}

func TestRequestVendorPlatform(t *testing.T) {
	routed := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: 1, Entry: &ModelCatalogEntry{ID: 1, ModelID: "grok-4", Vendor: "xai"}})
	platform, ok := RequestVendorPlatform(routed)
	require.True(t, ok)
	require.Equal(t, PlatformGrok, platform, "a catalog route answers with the entry's vendor")

	unknownVendor := WithCatalogRoute(context.Background(),
		CatalogRoute{EntryID: 2, Entry: &ModelCatalogEntry{ID: 2, ModelID: "team/best", Vendor: "custom"}})
	_, ok = RequestVendorPlatform(unknownVendor)
	require.False(t, ok, "a routed request with an unknown vendor has no vendor platform")

	_, ok = RequestVendorPlatform(context.Background())
	require.False(t, ok, "without a route there is no vendor platform (composite groups are gone)")
}

func TestModelCatalogService_ResolveRoute(t *testing.T) {
	repo := &stubModelCatalogRepo{entries: []ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", Vendor: "anthropic", Status: ModelCatalogStatusListed},
		{ID: 2, ModelID: "gpt-5.6", Vendor: "openai", Status: ModelCatalogStatusUnlisted},
	}}
	svc := NewModelCatalogService(repo, nil, ModelCatalogSeedInput{})
	ctx := context.Background()

	route, ok := svc.ResolveRoute(ctx, " claude-sonnet-4 ")
	require.True(t, ok)
	require.Equal(t, int64(1), route.EntryID)
	require.Equal(t, "claude-sonnet-4", route.CanonicalModel)
	require.Equal(t, "claude-sonnet-4", route.RequestedModel)
	require.NotNil(t, route.Entry)
	require.Equal(t, "anthropic", route.Entry.Vendor)

	// 用户只能请求目录模型标识（D4）：别的名字、大小写变体、claude 的点号写法都不认
	for _, name := range []string{"sonnet-latest", "Claude-Sonnet-4", "claude.sonnet.4"} {
		_, ok = svc.ResolveRoute(ctx, name)
		require.False(t, ok, name)
	}

	_, ok = svc.ResolveRoute(ctx, "gpt-5.6")
	require.False(t, ok, "unlisted entry must not be admitted")

	_, ok = svc.ResolveRoute(ctx, "nope")
	require.False(t, ok)

	listed := svc.ListListedEntries(ctx)
	require.Len(t, listed, 1)
	require.Equal(t, "claude-sonnet-4", listed[0].ModelID)

	require.Equal(t, []string{"claude-sonnet-4"},
		FilterListedModelIDs(ctx, svc, []string{"claude-sonnet-4", "gpt-5.6", "sonnet-latest", "nope"}))
	require.True(t, IsListedModel(ctx, svc, "claude-sonnet-4"))
	require.False(t, IsListedModel(ctx, svc, "sonnet-latest"))
	require.False(t, IsListedModel(ctx, svc, "gpt-5.6"))
}

func TestWithCatalogRoute(t *testing.T) {
	route := CatalogRoute{EntryID: 7, CanonicalModel: "gpt-5.6", RequestedModel: "gpt-5.6-sol", Entry: &ModelCatalogEntry{ID: 7, ModelID: "gpt-5.6", Vendor: "openai"}}
	ctx := WithCatalogRoute(context.Background(), route)

	got, ok := CatalogRouteFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, route, got)

	requested, ok := RequestedPublicModelFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "gpt-5.6-sol", requested, "客户端原始模型名随路由挂上（用量 / 审计记录用）")

	_, ok = CatalogRouteFromContext(context.Background())
	require.False(t, ok)
}

func TestCatalogBindingServes(t *testing.T) {
	chatOnlyKey := &Account{ID: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://cc.example.com"}}
	responsesOnlyKey := &Account{ID: 2, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
		ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://r.example.com"}}
	anthropicKey := &Account{ID: 3, Type: AccountTypeAPIKey, Platform: PlatformAnthropic,
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"}}
	geminiKey := &Account{ID: 4, Type: AccountTypeAPIKey, Platform: PlatformGemini,
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://g.example.com"}}
	noAddressKey := &Account{ID: 5, Type: AccountTypeAPIKey, Platform: PlatformOpenAI}
	anthropicOAuth := &Account{ID: 6, Type: AccountTypeOAuth, Platform: PlatformAnthropic}
	antigravity := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformAntigravity}
	geminiOAuth := &Account{ID: 8, Type: AccountTypeOAuth, Platform: PlatformGemini}
	openAIOAuth := &Account{ID: 9, Type: AccountTypeOAuth, Platform: PlatformOpenAI}

	serves := func(anthropic, chat, responses, gemini bool) map[string]bool {
		return map[string]bool{
			APIProtocolAnthropic: anthropic, APIProtocolChatCompletions: chat,
			APIProtocolResponses: responses, APIProtocolGemini: gemini,
		}
	}
	cases := []struct {
		name    string
		account *Account
		want    map[string]bool
	}{
		{"chat-only key converts message / response, never gemini", chatOnlyKey, serves(true, true, true, false)},
		{"responses-only key converts message / completion, never gemini", responsesOnlyKey, serves(true, true, true, false)},
		{"anthropic key converts completion / response, never gemini", anthropicKey, serves(true, true, true, false)},
		{"gemini key serves message / completion / generate, never response", geminiKey, serves(true, true, false, true)},
		{"key without any address serves nothing", noAddressKey, serves(false, false, false, false)},
		{"anthropic subscription", anthropicOAuth, serves(true, true, true, false)},
		{"antigravity subscription serves all four", antigravity, serves(true, true, true, true)},
		{"gemini subscription never serves response", geminiOAuth, serves(true, true, false, true)},
		{"openai subscription never serves gemini", openAIOAuth, serves(true, true, true, false)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, CatalogBindingServes(tc.account))
		})
	}
}

// 绑定资格只看协议转换注册表，与条目厂商无关：responses-only key 能绑 anthropic 厂商的条目。
func TestAccountServesCatalogEntry(t *testing.T) {
	responsesOnlyKey := &Account{ID: 2, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
		ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://r.example.com"}}
	geminiOAuth := &Account{ID: 8, Type: AccountTypeOAuth, Platform: PlatformGemini}
	noAddressKey := &Account{ID: 5, Type: AccountTypeAPIKey, Platform: PlatformOpenAI}
	anthropicEntry := &ModelCatalogEntry{ModelID: "claude-sonnet-4", Vendor: "anthropic"}
	grokEntry := &ModelCatalogEntry{ModelID: "grok-4", Vendor: "xai"}

	require.NoError(t, AccountServesCatalogEntry(anthropicEntry, responsesOnlyKey))
	require.NoError(t, AccountServesCatalogEntry(grokEntry, geminiOAuth))

	err := AccountServesCatalogEntry(anthropicEntry, noAddressKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CATALOG_BINDING_UNSERVABLE")
	require.Contains(t, err.Error(), "claude-sonnet-4")

	require.Error(t, AccountServesCatalogEntry(nil, responsesOnlyKey))
	require.Error(t, AccountServesCatalogEntry(anthropicEntry, nil))
}

type stubCatalogBindingAccounts map[int64]*Account

func (m stubCatalogBindingAccounts) GetAccount(_ context.Context, id int64) (*Account, error) {
	if account, ok := m[id]; ok {
		return account, nil
	}
	return nil, ErrAccountNotFound
}

// 经扩展端点调用的条目只看 openai / grok 厂商；图片 / 视频计费或价格文件标成生图 / 向量的才算。
func TestCatalogEntryServedByExtensionEndpoints(t *testing.T) {
	cases := []struct {
		name          string
		entry         ModelCatalogEntry
		priceFileMode string
		want          bool
	}{
		{"openai token-priced image model", ModelCatalogEntry{ModelID: "gpt-image-2", Vendor: "openai"}, "image_generation", true},
		{"openai embedding model", ModelCatalogEntry{ModelID: "text-embedding-3-small", Vendor: "openai"}, "embedding", true},
		{"azure image model", ModelCatalogEntry{ModelID: "gpt-image-1", Vendor: "azure"}, "image_generation", true},
		{"grok per-image model", ModelCatalogEntry{ModelID: "grok-imagine-image", Vendor: "xai", BillingMode: BillingModeImage}, "", true},
		{"grok video model", ModelCatalogEntry{ModelID: "grok-imagine-video", Vendor: "xai", BillingMode: BillingModeVideo}, "", true},
		{"openai chat model", ModelCatalogEntry{ModelID: "gpt-5.5", Vendor: "openai"}, "chat", false},
		{"openai responses model", ModelCatalogEntry{ModelID: "gpt-5.5-pro", Vendor: "openai"}, "responses", false},
		{"gemini image model is called via chat", ModelCatalogEntry{ModelID: "gemini-2.5-flash-image", Vendor: "gemini", BillingMode: BillingModeImage}, "image_generation", false},
		{"no vendor", ModelCatalogEntry{ModelID: "doubao-embedding-vision"}, "embedding", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, CatalogEntryServedByExtensionEndpoints(&tc.entry, tc.priceFileMode))
		})
	}
}

func TestSchedulingScopeID(t *testing.T) {
	routed := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: 7})
	require.Equal(t, int64(7), SchedulingScopeID(routed), "catalog route scopes by entry")
	require.Equal(t, int64(0), SchedulingScopeID(context.Background()), "no route: one shared scope")
}

// 无路由的池是全部资源：绑没绑分组都在池里（7a 起），粘性命中不能再按分组把账号判成不在池里。
func TestAccountInSchedulingScope(t *testing.T) {
	bound := &Account{ID: 1, CatalogEntryIDs: []int64{7}}
	unbound := &Account{ID: 2, CatalogEntryIDs: []int64{8}}
	ungrouped := &Account{ID: 3}
	routed := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: 7})

	require.True(t, accountInSchedulingScope(routed, bound))
	require.False(t, accountInSchedulingScope(routed, unbound), "catalog route: only bound accounts are in scope")
	require.False(t, accountInSchedulingScope(routed, ungrouped))

	require.True(t, accountInSchedulingScope(context.Background(), bound), "grouped accounts are in the all-resources pool")
	require.True(t, accountInSchedulingScope(context.Background(), ungrouped))
	require.False(t, accountInSchedulingScope(routed, nil))
}

func TestResolveCatalogRouteForCandidates(t *testing.T) {
	repo := &stubModelCatalogRepo{entries: []ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", Vendor: "anthropic", Status: ModelCatalogStatusListed},
		{ID: 2, ModelID: "gpt-5.6", Vendor: "openai", Status: ModelCatalogStatusListed},
		{ID: 3, ModelID: "hidden", Vendor: "openai", Status: ModelCatalogStatusUnlisted},
	}}
	svc := NewModelCatalogService(repo, nil, ModelCatalogSeedInput{})
	ctx := context.Background()

	route, blocked, ok := ResolveCatalogRouteForCandidates(ctx, svc, []string{"claude-sonnet-4", "claude-sonnet-4"})
	require.True(t, ok, "repeated keys with the same catalog model ID resolve to one entry")
	require.Empty(t, blocked)
	require.Equal(t, int64(1), route.EntryID)

	_, blocked, ok = ResolveCatalogRouteForCandidates(ctx, svc, []string{"claude-sonnet-4", "sonnet-latest"})
	require.False(t, ok, "a name outside the catalog is not a catalog model ID (D4)")
	require.Equal(t, "sonnet-latest", blocked)

	_, blocked, ok = ResolveCatalogRouteForCandidates(ctx, svc, []string{"claude-sonnet-4", "gpt-5.6"})
	require.False(t, ok, "candidates resolving to different entries are rejected")
	require.Equal(t, "gpt-5.6", blocked)

	_, blocked, ok = ResolveCatalogRouteForCandidates(ctx, svc, []string{"gpt-5.6", "hidden"})
	require.False(t, ok)
	require.Equal(t, "hidden", blocked)

	_, blocked, ok = ResolveCatalogRouteForCandidates(ctx, svc, nil)
	require.False(t, ok, "no candidates means no route")
	require.Empty(t, blocked)
}
