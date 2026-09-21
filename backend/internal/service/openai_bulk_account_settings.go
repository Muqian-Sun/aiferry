package service

import (
	"fmt"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type bulkOpenAISettings struct {
	endpointCapabilities bool
}

func (s bulkOpenAISettings) any() bool {
	return s.endpointCapabilities
}

func normalizeBulkOpenAISettings(input *BulkUpdateAccountsInput) (bulkOpenAISettings, error) {
	var settings bulkOpenAISettings
	if input == nil {
		return settings, nil
	}

	if raw, exists := input.Credentials[openAIEndpointCapabilitiesCredentialKey]; exists {
		settings.endpointCapabilities = true
		capabilities, err := normalizeBulkOpenAIEndpointCapabilities(raw)
		if err != nil {
			return settings, err
		}
		input.Credentials[openAIEndpointCapabilitiesCredentialKey] = capabilities
	}

	return settings, nil
}

func normalizeBulkOpenAIEndpointCapabilities(raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}

	values := make([]string, 0, 2)
	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, invalidBulkOpenAIEndpointCapabilities()
			}
			values = append(values, value)
		}
	case []string:
		values = append(values, typed...)
	default:
		return nil, invalidBulkOpenAIEndpointCapabilities()
	}

	selected := make(map[string]bool, 2)
	for _, value := range values {
		switch OpenAIEndpointCapability(value) {
		case OpenAIEndpointCapabilityChatCompletions, OpenAIEndpointCapabilityEmbeddings:
			selected[value] = true
		default:
			return nil, invalidBulkOpenAIEndpointCapabilities()
		}
	}
	if len(selected) == 0 {
		return nil, invalidBulkOpenAIEndpointCapabilities()
	}

	includeChat := selected[string(OpenAIEndpointCapabilityChatCompletions)]
	if includeChat && selected[string(OpenAIEndpointCapabilityEmbeddings)] {
		return nil, nil
	}
	if includeChat {
		return []string{string(OpenAIEndpointCapabilityChatCompletions)}, nil
	}
	return []string{string(OpenAIEndpointCapabilityEmbeddings)}, nil
}

func invalidBulkOpenAIEndpointCapabilities() error {
	return infraerrors.BadRequest(
		"OPENAI_ENDPOINT_CAPABILITIES_INVALID",
		"openai_capabilities must contain chat_completions, embeddings, or both",
	)
}

func validateBulkOpenAISettingsTargets(
	input *BulkUpdateAccountsInput,
	settings bulkOpenAISettings,
	targetsByID map[int64]*Account,
) error {
	if input == nil || !settings.any() {
		return nil
	}
	for _, accountID := range input.AccountIDs {
		account, ok := targetsByID[accountID]
		if !ok || account == nil {
			return invalidBulkOpenAITarget(accountID, "account does not exist")
		}
		if settings.endpointCapabilities {
			if !account.IsThirdPartyKey() || !account.HasOpenAIProtocolEndpoint() {
				return invalidBulkOpenAITarget(accountID, "endpoint capabilities require a key with an OpenAI-protocol endpoint")
			}
		}
	}
	return nil
}

func invalidBulkOpenAITarget(accountID int64, message string) error {
	return infraerrors.BadRequest(
		"OPENAI_BULK_TARGET_INVALID",
		fmt.Sprintf("account %d: %s", accountID, message),
	).WithMetadata(map[string]string{"account_id": strconv.FormatInt(accountID, 10)})
}
