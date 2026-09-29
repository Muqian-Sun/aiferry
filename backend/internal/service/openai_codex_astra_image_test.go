package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Astra 旧快照（只写了 text）的修正只对 OpenAI 成品号：指向 api.openai.com 的 key 按中转处理，
// 上游元数据说了算（2026-09-29 海外四家不再有官方 key）。
func TestAccountCodexModelSupportsImageInput_AstraStaleSnapshotCorrectedForSubscriptionOnly(t *testing.T) {
	t.Parallel()

	staleAstra := UpstreamModelMetadataSnapshot{
		Models: map[string]UpstreamModelMetadata{
			"gpt-6-astra": {ID: "gpt-6-astra", InputModalities: []string{"text"}},
		},
	}
	subscription := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	subscription.SetUpstreamModelMetadataSnapshot(staleAstra)
	require.True(t, accountCodexModelSupportsImageInput(subscription, "gpt-6-astra"))

	keyOnOfficialHost := &Account{
		ID:                2,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Credentials:       map[string]any{"api_key": "sk-test"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com"},
	}
	keyOnOfficialHost.SetUpstreamModelMetadataSnapshot(staleAstra)
	require.False(t, accountCodexModelSupportsImageInput(keyOnOfficialHost, "gpt-6-astra"))
}

func TestBuildCodexModelsManifestForGroupPreservesAstraTextOnlyMetadataForKeyOnOfficialHost(t *testing.T) {
	t.Parallel()

	account := Account{
		ID:                1,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Credentials:       map[string]any{"api_key": "sk-test"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com"},
	}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{
		Models: map[string]UpstreamModelMetadata{
			"gpt-6-astra": {ID: "gpt-6-astra", InputModalities: []string{"text"}},
		},
	})

	svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{accounts: []Account{account}}}

	body, err := buildCodexManifestFromCatalogForTest(svc, "gpt-6-astra")
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, []any{"text"}, models[0]["input_modalities"])
}

func TestBuildCodexModelsManifestForGroupPreservesCompatibleAstraTextOnlyMetadata(t *testing.T) {
	t.Parallel()

	account := Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://relay.example.test/v1"},
		ProtocolEndpoints: map[string]string{
			APIProtocolChatCompletions: "https://relay.example.test/v1",
		},
	}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{
		Models: map[string]UpstreamModelMetadata{
			"gpt-6-astra": {ID: "gpt-6-astra", InputModalities: []string{"text"}},
		},
	})

	svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{accounts: []Account{account}}}

	body, err := buildCodexManifestFromCatalogForTest(svc, "gpt-6-astra")
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, []any{"text"}, models[0]["input_modalities"])
}
