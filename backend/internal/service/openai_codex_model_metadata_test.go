package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAstraUltraCatalogPreservesWorkflowMetadata(t *testing.T) {
	// Ultra is a Codex workflow. Its inference effort must survive catalog sync,
	// aliases and group generation instead of silently falling back to max.
	_, metadata, err := extractUpstreamModelCatalog([]byte(`{"models":[{
		"slug":"gpt-6-astra","supported_reasoning_levels":[{"effort":"high"},{"effort":"ultra"}],
		"multi_agent_reasoning_effort":"high","multi_agent_version":"v2"
	}]}`), false)
	require.NoError(t, err)
	account := Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://relay.example/v1", "model_mapping": map[string]any{"public-astra": "gpt-6-astra"},
	}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1", APIProtocolResponses: "https://relay.example/v1"}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: metadata})
	body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-astra"}, []Account{account}, nil, nil, true)
	require.NoError(t, err)
	model := decodeCodexManifestModels(t, body)[0]
	require.Equal(t, "high", model["multi_agent_reasoning_effort"])
	require.Equal(t, "v2", model["multi_agent_version"])
	require.Equal(t, []string{"high", "ultra"}, effortsFromManifestModel(t, model))

	peer := Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: account.Credentials}
	body, err = buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-astra"}, []Account{account, peer}, nil, nil, true)
	require.NoError(t, err)
	model = decodeCodexManifestModels(t, body)[0]
	require.Nil(t, model["multi_agent_reasoning_effort"], "do not advertise one account's override for all peers")
	require.Nil(t, model["multi_agent_version"])
}

func TestAstraUltraCatalogPreservesExplicitWorkflowOverrides(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://relay.example/v1"}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1", APIProtocolResponses: "https://relay.example/v1"}}
	for _, fields := range []string{
		`"multi_agent_reasoning_effort":"high","multi_agent_version":"v1"`,
		`"multi_agent_reasoning_effort":null,"multi_agent_version":null`,
	} {
		for _, source := range []string{
			`{"models":[{"slug":"gpt-6-astra",` + fields + `}]}`,
			`{"data":[{"id":"gpt-6-astra",` + fields + `}]}`,
		} {
			converted := convertOpenAIModelListToCodexManifestForAccount([]byte(source), account)
			body, err := completeAPIKeyCodexModelsManifestMetadata(converted, true, account)
			require.NoError(t, err)
			model := decodeCodexManifestModels(t, body)[0]
			var expected map[string]any
			require.NoError(t, json.Unmarshal([]byte(`{`+fields+`}`), &expected))
			for key, value := range expected {
				require.Contains(t, model, key)
				require.Equal(t, value, model[key])
			}
		}
	}
}

func TestAstraCodexToolCapabilitiesUseAccountScopeAndSharedDeclarations(t *testing.T) {
	newAccount := func(baseURL string) Account {
		return Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"base_url": baseURL, "model_mapping": map[string]any{"public-astra": "gpt-6-astra"},
		},
			// 第三方 key 的上游地址只认协议映射，与 base_url 指向同一地址。
			ProtocolEndpoints: map[string]string{
				APIProtocolChatCompletions: baseURL,
				APIProtocolResponses:       baseURL,
			},
		}
	}
	official := newAccount("https://api.openai.com/v1")
	custom := newAccount("https://relay.example/v1")
	bridge := newAccount("https://bridge.example/v1")
	// 只配 chat_completions 地址：Responses 入站转成 Chat Completions，由桥接实现工具发现。
	delete(bridge.ProtocolEndpoints, APIProtocolResponses)
	for _, tt := range []struct {
		name     string
		accounts []Account
		search   bool
	}{
		{"official fallback", []Account{official}, true},
		{"custom host has no guessed capability", []Account{custom}, false},
		{"missing peer capability", []Account{official, custom}, false},
		{"implemented chat bridge", []Account{bridge}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-astra"}, tt.accounts, nil, nil, true)
			require.NoError(t, err)
			model := decodeCodexManifestModels(t, body)[0]
			require.Equal(t, "public-astra", model["slug"])
			require.Equal(t, tt.search, model["supports_search_tool"])
			require.Equal(t, false, model["use_responses_lite"])
			if tt.name == "official fallback" {
				require.Equal(t, "freeform", model["apply_patch_tool_type"])
				require.Equal(t, "3000", model["comp_hash"])
			}
		})
	}
	custom.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"gpt-6-astra": {CodexToolCapabilities: map[string]json.RawMessage{
			"supports_search_tool": json.RawMessage("true"), "apply_patch_tool_type": json.RawMessage("null"),
			"comp_hash": json.RawMessage(`"3000"`), "tool_mode": json.RawMessage("null"), "use_responses_lite": json.RawMessage("false"),
		}},
	}})
	body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-astra"}, []Account{official, custom}, nil, nil, true)
	require.NoError(t, err)
	model := decodeCodexManifestModels(t, body)[0]
	require.Equal(t, true, model["supports_search_tool"])
	require.Nil(t, model["apply_patch_tool_type"], "conflicting tool protocols must not be advertised")
	require.Equal(t, "3000", model["comp_hash"])
}

func TestAstraCodexToolCapabilitiesPreserveLiveNullAndFalse(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://api.openai.com/v1"}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com/v1", APIProtocolResponses: "https://api.openai.com/v1"}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"gpt-6-astra": {CodexToolCapabilities: map[string]json.RawMessage{
			"supports_search_tool": json.RawMessage("true"), "apply_patch_tool_type": json.RawMessage(`"freeform"`),
			"comp_hash": json.RawMessage(`"3000"`), "tool_mode": json.RawMessage(`"code_mode_only"`),
		}},
	}})
	for _, source := range []string{
		`{"models":[{"slug":"gpt-6-astra","supports_search_tool":false,"apply_patch_tool_type":null,"tool_mode":null}]}`,
		`{"data":[{"id":"gpt-6-astra","supports_search_tool":false,"apply_patch_tool_type":null,"tool_mode":null}]}`,
	} {
		converted := convertOpenAIModelListToCodexManifestForAccount([]byte(source), account)
		body, err := applySyncedAPIKeyCodexModelMetadata(converted, account, source[2:6] == "data")
		require.NoError(t, err)
		body, err = completeAPIKeyCodexModelsManifestMetadata(body, true, account)
		require.NoError(t, err)
		model := decodeCodexManifestModels(t, body)[0]
		require.Equal(t, false, model["supports_search_tool"])
		require.Nil(t, model["apply_patch_tool_type"])
		require.Nil(t, model["tool_mode"])
		require.Equal(t, "3000", model["comp_hash"])
	}
	fields := make(map[string]json.RawMessage)
	applyCodexToolCapabilities(fields, map[string]json.RawMessage{
		"supports_search_tool": json.RawMessage(`"true"`), "comp_hash": json.RawMessage(`{"unexpected":true}`),
		"model_messages": json.RawMessage(`"do not persist prompts"`),
	}, true)
	require.Empty(t, fields)
}

func TestBuildCodexModelsManifestForGroupAdvertisesSearchOnlyForChatBridgeRoutes(t *testing.T) {
	t.Parallel()

	newAccount := func(id int64, nativeResponses bool) Account {
		account := Account{
			ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":      "https://provider.example/v1",
				"model_mapping": map[string]any{"company-coding-model": "company-coding-model"},
			},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://provider.example/v1"},
		}
		if nativeResponses {
			account.ProtocolEndpoints[APIProtocolResponses] = "https://provider.example/v1"
		}
		return account
	}

	for _, tc := range []struct {
		name     string
		accounts []Account
		want     bool
	}{
		{name: "chat bridge", accounts: []Account{newAccount(80, false)}, want: true},
		{name: "native responses", accounts: []Account{newAccount(81, true)}, want: false},
		{name: "mixed routes", accounts: []Account{newAccount(82, false), newAccount(83, true)}, want: false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := buildCodexModelsManifestForAccounts(
				PlatformOpenAI, []string{"company-coding-model"}, tc.accounts, nil, nil, true,
			)
			require.NoError(t, err)
			models := decodeCodexManifestModels(t, body)
			require.Len(t, models, 1)
			require.Equal(t, tc.want, models[0]["supports_search_tool"])
		})
	}
}

func TestCompleteAPIKeyCodexManifestSearchCapabilityPreservesUpstreamAndFailsClosed(t *testing.T) {
	t.Parallel()

	nativeAccount := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://provider.example/v1",
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://provider.example/v1", APIProtocolResponses: "https://provider.example/v1"},
	}
	body, err := completeAPIKeyCodexModelsManifestMetadata([]byte(`{"models":[
		{"slug":"explicit","supports_search_tool":true},
		{"slug":"missing"}
	]}`), true, nativeAccount)
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Equal(t, true, models[0]["supports_search_tool"])
	require.Equal(t, false, models[1]["supports_search_tool"])

	chatAccount := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://provider.example/v1",
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://provider.example/v1"},
	}
	body, err = completeAPIKeyCodexModelsManifestMetadata(
		[]byte(`{"models":[
			{"slug":"explicit-disabled","supports_search_tool":false},
			{"slug":"generated"}
		]}`), true, chatAccount,
	)
	require.NoError(t, err)
	models = decodeCodexManifestModels(t, body)
	require.Equal(t, false, models[0]["supports_search_tool"])
	require.Equal(t, true, models[1]["supports_search_tool"])
}

// Scenario: the same public alias may target different models on one platform when complete snapshots can be intersected.
func TestBuildCodexModelsManifestForGroupIntersectsDifferentMappedTargetsWithoutLeakingAlias(t *testing.T) {
	t.Parallel()

	const groupID int64 = 739
	reasoning := true
	newAccount := func(id int64, target, displayName, description string, levels, modalities []string, contextWindow int64) Account {
		account := Account{
			ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":      "https://provider.example/v1",
				"model_mapping": map[string]any{"my-coder": target},
			},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://provider.example/v1", APIProtocolResponses: "https://provider.example/v1"},
		}
		account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
			target: {
				ID: target, DisplayName: displayName, Description: description, Reasoning: &reasoning,
				SupportedReasoningLevels: levels,
				InputModalities:          modalities,
				ContextWindow:            contextWindow,
			},
		}})
		return account
	}
	openAIAccount := newAccount(
		31,
		"gpt-5.6-sol",
		"GPT-5.6 Sol",
		"OpenAI upstream model",
		[]string{"low", "medium", "high", "xhigh"},
		[]string{"text", "image"},
		272_000,
	)
	arkAccount := newAccount(
		32,
		"glm-5.3",
		"GLM 5.3",
		"Ark upstream model",
		[]string{"low", "medium", "high"},
		[]string{"text"},
		1_000_000,
	)

	for _, accounts := range [][]Account{{openAIAccount, arkAccount}, {arkAccount, openAIAccount}} {
		svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{
			groupID: accounts,
		}}}
		body, err := buildCodexManifestFromCatalogForTest(svc, "my-coder")
		require.NoError(t, err)
		models := decodeCodexManifestModels(t, body)
		require.Len(t, models, 1)
		require.Equal(t, "my-coder", models[0]["slug"])
		require.Equal(t, "my-coder", models[0]["display_name"])
		require.Equal(t, "Custom model routed through Sub2API.", models[0]["description"])
		require.Equal(t, []string{"low", "medium", "high"}, effortsFromManifestModel(t, models[0]))
		require.Equal(t, []any{"text"}, models[0]["input_modalities"])
		require.EqualValues(t, 272_000, models[0]["context_window"])
	}
}

// Scenario: accounts that leave the current scheduling pool only because of
// transient state still participate in capability intersection.
func TestBuildCodexModelsManifestForGroupIntersectsTransientlyUnschedulableMappedAccounts(t *testing.T) {
	t.Parallel()

	const groupID int64 = 741
	schedulable := newCodexCatalogMappedAccount(
		41,
		"gpt-5.6-sol",
		"GPT-5.6 Sol",
		[]string{"low", "medium", "high", "xhigh"},
		[]string{"text", "image"},
		1_000_000,
		true,
		nil,
	)
	transientlyUnschedulable := newCodexCatalogMappedAccount(
		42,
		"glm-5.3",
		"GLM 5.3",
		[]string{"low", "medium", "high"},
		[]string{"text"},
		272_000,
		true,
		map[string]any{"exclusive-model": "exclusive-upstream"},
	)
	svc := &GatewayService{accountRepo: splitCodexModelsAccountRepo{
		schedulable: map[int64][]Account{groupID: {schedulable}},
		catalog:     map[int64][]Account{groupID: {schedulable, transientlyUnschedulable}},
	}}

	body, err := buildCodexManifestFromCatalogForTest(svc, "my-coder")
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, "my-coder", models[0]["slug"])
	require.Equal(t, "my-coder", models[0]["display_name"])
	require.Equal(t, []string{"low", "medium", "high"}, effortsFromManifestModel(t, models[0]))
	require.Equal(t, []any{"text"}, models[0]["input_modalities"])
	require.EqualValues(t, 272_000, models[0]["context_window"])
}

// Scenario: a persistently disabled account cannot narrow the advertised contract.
func TestBuildCodexModelsManifestForGroupIgnoresPersistentlyDisabledMappedAccounts(t *testing.T) {
	t.Parallel()

	const groupID int64 = 742
	remaining := newCodexCatalogMappedAccount(
		41,
		"gpt-5.6-sol",
		"GPT-5.6 Sol",
		[]string{"low", "medium", "high", "xhigh"},
		[]string{"text", "image"},
		1_000_000,
		true,
		nil,
	)
	disabled := newCodexCatalogMappedAccount(
		42,
		"glm-5.3",
		"GLM 5.3",
		[]string{"low", "medium", "high"},
		[]string{"text"},
		272_000,
		false,
		nil,
	)
	svc := &GatewayService{accountRepo: splitCodexModelsAccountRepo{
		schedulable: map[int64][]Account{groupID: {remaining}},
		catalog:     map[int64][]Account{groupID: {remaining}},
		all:         map[int64][]Account{groupID: {remaining, disabled}},
	}}

	body, err := buildCodexManifestFromCatalogForTest(svc, "my-coder")
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, []any{"text", "image"}, models[0]["input_modalities"])
	require.EqualValues(t, 1_000_000, models[0]["context_window"])
}

func TestAstraCodexToolCapabilitiesFollowAPIKeyAlias(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://relay.example/v1", "model_mapping": map[string]any{"my-astra": "gpt-6-astra"},
	}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1", APIProtocolResponses: "https://relay.example/v1"}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"gpt-6-astra": {CodexToolCapabilities: map[string]json.RawMessage{
			"supports_search_tool": json.RawMessage("true"), "apply_patch_tool_type": json.RawMessage(`"freeform"`),
			"use_responses_lite": json.RawMessage("false"),
		}},
	}})
	for _, source := range []string{`{"data":[{"id":"my-astra"}]}`, `{"models":[{"slug":"my-astra"}]}`} {
		converted := convertOpenAIModelListToCodexManifestForAccount([]byte(source), account)
		body, err := completeAPIKeyCodexModelsManifestMetadata(converted, true, account)
		require.NoError(t, err)
		models := decodeCodexManifestModels(t, body)
		require.Len(t, models, 1)
		model := models[0]
		require.Equal(t, "my-astra", model["slug"])
		require.Equal(t, "my-astra", model["display_name"])
		require.Equal(t, true, model["supports_search_tool"])
		require.Equal(t, "freeform", model["apply_patch_tool_type"])
		require.Equal(t, false, model["use_responses_lite"])
	}
}

func TestAstraCodexToolCapabilitiesKeepAPIKeyResponsesLiteGuard(t *testing.T) {
	account := Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://relay.example/v1", "model_mapping": map[string]any{"my-astra": "gpt-6-astra"},
	}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1", APIProtocolResponses: "https://relay.example/v1"}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"gpt-6-astra": {CodexToolCapabilities: map[string]json.RawMessage{"use_responses_lite": json.RawMessage("true")}},
	}})
	body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"my-astra"}, []Account{account}, nil, nil, true)
	require.NoError(t, err)
	require.Equal(t, false, decodeCodexManifestModels(t, body)[0]["use_responses_lite"])
	body, err = adjustAPIKeyCodexModelsManifest([]byte(`{"models":[{"slug":"my-astra","use_responses_lite":true}]}`), &account)
	require.NoError(t, err)
	require.Equal(t, false, decodeCodexManifestModels(t, body)[0]["use_responses_lite"])
}

// Scenario: an OpenAI group keeps a failover account whose mapping points the
// shared public alias at a different upstream model. Without an explicit routing
// rule the alias target is ambiguous and capabilities must fail closed; with one,
// the operator has declared who serves the alias, so the reasoning slider and
// image input must survive.
func TestCodexAliasFailoverMappingHonorsModelRouting(t *testing.T) {
	newAccount := func(id int64, target string, priority int) Account {
		return Account{
			ID:       id,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Priority: priority,
			Credentials: map[string]any{
				"base_url":      "https://relay.example/v1",
				"model_mapping": map[string]any{"gpt-6-astra": target},
			},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1", APIProtocolResponses: "https://relay.example/v1"},
		}
	}
	// Account 16-alike serves the alias natively; account 1-alike is the failover
	// and can only reach a different upstream model.
	primary := newAccount(16, "gpt-6-astra", 0)
	failover := newAccount(1, "gpt-5.6-sol", 10)
	accounts := []Account{primary, failover}

	// supported_reasoning_levels entries are {effort, description} objects; the
	// slider only cares about the effort names.
	manifestFieldsOf := func(t *testing.T, group *Group) ([]string, any, any) {
		t.Helper()
		body, err := buildCodexModelsManifestForAccounts(
			PlatformOpenAI, []string{"gpt-6-astra"}, accounts, group, nil, true,
		)
		require.NoError(t, err)
		models := decodeCodexManifestModels(t, body)
		require.Len(t, models, 1)
		model := models[0]
		require.Equal(t, "gpt-6-astra", model["slug"])
		var efforts []string
		levels, _ := model["supported_reasoning_levels"].([]any)
		for _, level := range levels {
			entry, ok := level.(map[string]any)
			require.True(t, ok, "reasoning level entry must be an object")
			effort, ok := entry["effort"].(string)
			require.True(t, ok, "reasoning level entry must carry an effort name")
			efforts = append(efforts, effort)
		}
		return efforts, model["default_reasoning_level"], model["input_modalities"]
	}

	t.Run("no routing rule still fails closed", func(t *testing.T) {
		levels, def, modalities := manifestFieldsOf(t, nil)
		require.Empty(t, levels, "ambiguous alias target must not advertise reasoning levels")
		require.Nil(t, def)
		require.Equal(t, []any{"text", "image"}, modalities,
			"image input is resolved per-account independently of routing rules")
	})

	t.Run("routing rule resolves the alias", func(t *testing.T) {
		group := &Group{
			ID:                  2,
			Platform:            PlatformOpenAI,
			ModelRoutingEnabled: true,
			ModelRouting:        map[string][]int64{"gpt-6-astra": {16, 1}},
		}
		levels, def, modalities := manifestFieldsOf(t, group)
		require.Equal(t,
			[]string{"low", "medium", "high", "xhigh", "max", "ultra"},
			levels,
			"routed alias must keep a movable reasoning slider",
		)
		require.Equal(t, "medium", def)
		require.Contains(t, modalities, "image")
	})

	t.Run("routing disabled falls back to failing closed", func(t *testing.T) {
		group := &Group{
			ID:                  2,
			Platform:            PlatformOpenAI,
			ModelRoutingEnabled: false,
			ModelRouting:        map[string][]int64{"gpt-6-astra": {16, 1}},
		}
		levels, _, _ := manifestFieldsOf(t, group)
		require.Empty(t, levels, "a disabled routing rule must not resolve the alias")
	})

	t.Run("routing rule for another alias does not leak", func(t *testing.T) {
		group := &Group{
			ID:                  2,
			Platform:            PlatformOpenAI,
			ModelRoutingEnabled: true,
			ModelRouting:        map[string][]int64{"gpt-5.6-sol": {1}},
		}
		levels, _, _ := manifestFieldsOf(t, group)
		require.Empty(t, levels, "unrelated routing rules must not resolve this alias")
	})
}

// Scenario: mixed groups prefer capability metadata synced for the routed account.

// accountCodexToolCapabilities 不看第三方 key 的平台标签。
func TestAccountCodexToolCapabilities_KeysIgnoreLabel(t *testing.T) {
	t.Run("chat bridge search tool for any label", func(t *testing.T) {
		bridge := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://bridge.example/v1"}}
		require.True(t, shouldForwardOpenAIResponsesViaRawChatCompletions(bridge))
		require.Equal(t, json.RawMessage("true"), accountCodexToolCapabilities(bridge, "gpt-5.1")["supports_search_tool"])
	})

	t.Run("astra official defaults follow openai vendor", func(t *testing.T) {
		official := &Account{Platform: PlatformDeepseek, Type: AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com/v1", APIProtocolResponses: "https://api.openai.com/v1"}}
		require.Equal(t, PlatformOpenAI, official.Vendor())
		capabilities := accountCodexToolCapabilities(official, "gpt-6-astra")
		require.Equal(t, json.RawMessage(`"freeform"`), capabilities["apply_patch_tool_type"])
		require.Equal(t, json.RawMessage("false"), capabilities["use_responses_lite"])
	})

	t.Run("responses lite guard for any key label", func(t *testing.T) {
		key := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example/v1"}}
		key.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
			"gpt-6-astra": {CodexToolCapabilities: map[string]json.RawMessage{"use_responses_lite": json.RawMessage("true")}},
		}})
		require.Equal(t, json.RawMessage("false"), accountCodexToolCapabilities(key, "gpt-6-astra")["use_responses_lite"])
	})
}
