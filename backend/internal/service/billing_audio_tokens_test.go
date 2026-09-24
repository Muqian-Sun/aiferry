package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

// 音频 token 计在输入 / 输出总数里：从文本 token 中剥出来按音频价计（费用并入 InputCost / OutputCost，
// AudioInputCost / AudioOutputCost 只做明细）；没配音频价时回退文本价，与改动前同价。

const (
	audioTestModel       = "gpt-audio-test"
	audioTestInputPrice  = 2.5e-6
	audioTestOutputPrice = 10e-6
	audioTestAudioInput  = 40e-6
	audioTestAudioOutput = 80e-6
)

func audioCatalogResolverForTest(withAudioPrices bool) (*BillingService, *ModelPricingResolver) {
	input := audioTestInputPrice
	output := audioTestOutputPrice
	entry := catalogEntryFromCard(audioTestModel, ModelCatalogManagedByAdmin, PricingCard{
		BillingMode: BillingModeToken,
		InputPrice:  &input,
		OutputPrice: &output,
	})
	if withAudioPrices {
		audioInput := audioTestAudioInput
		audioOutput := audioTestAudioOutput
		entry.AudioInputPrice = &audioInput
		entry.AudioOutputPrice = &audioOutput
	}
	entry.ID = 1
	bs := &BillingService{cfg: &config.Config{}, fallbackPrices: map[string]*ModelPricing{}}
	catalog, _ := newTestModelCatalogService(entry)
	return bs, NewModelPricingResolver(catalog, bs)
}

func recordAudioUsageForTest(t *testing.T, withAudioPrices bool, usage OpenAIUsage) *UsageLog {
	t.Helper()
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	bs, resolver := audioCatalogResolverForTest(withAudioPrices)
	svc.billingService = bs
	svc.resolver = resolver

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_audio_tokens",
			Model:     audioTestModel,
			Usage:     usage,
			Duration:  time.Second,
		},
		APIKey:  &APIKey{ID: 11501},
		User:    &User{ID: 21501, RateMultiplier: 1},
		Account: &Account{ID: 31501},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	return usageRepo.lastLog
}

func TestOpenAIRecordUsage_AudioTokensBilledAtAudioPrices(t *testing.T) {
	// 1000 输入里 400 是音频，500 输出里 200 是音频（上游 usage 已解析成 Audio*Tokens）。
	log := recordAudioUsageForTest(t, true, OpenAIUsage{
		InputTokens: 1000, OutputTokens: 500,
		AudioInputTokens: 400, AudioOutputTokens: 200,
	})

	// 输入：600 × $2.5/MTok + 400 × $40/MTok = $0.0015 + $0.016；输出：300 × $10/MTok + 200 × $80/MTok = $0.003 + $0.016。
	require.InDelta(t, 0.0175, log.InputCost, 1e-12)
	require.InDelta(t, 0.019, log.OutputCost, 1e-12)
	require.InDelta(t, 0.0365, log.TotalCost, 1e-12)
	require.Equal(t, 1000, log.InputTokens)
	require.Equal(t, 500, log.OutputTokens)
}

func TestOpenAIRecordUsage_AudioWithoutAudioPriceFallsBackToTextPrice(t *testing.T) {
	log := recordAudioUsageForTest(t, false, OpenAIUsage{
		InputTokens: 1000, OutputTokens: 500,
		AudioInputTokens: 400, AudioOutputTokens: 200,
	})

	// 没配音频价：音频 token 仍按文本价（与改动前同价，不是 0）。
	require.InDelta(t, 1000*audioTestInputPrice, log.InputCost, 1e-12)
	require.InDelta(t, 500*audioTestOutputPrice, log.OutputCost, 1e-12)
	require.InDelta(t, 0.0075, log.TotalCost, 1e-12)
}

func TestOpenAIRecordUsage_AudioInputOnlyCountsUncachedTokens(t *testing.T) {
	// 1000 输入里 300 命中缓存；音频 400 超过未命中缓存的 700 时截到 700。
	log := recordAudioUsageForTest(t, true, OpenAIUsage{
		InputTokens: 1000, CacheReadInputTokens: 300, OutputTokens: 0,
		AudioInputTokens: 800,
	})
	require.InDelta(t, 700*audioTestAudioInput, log.InputCost, 1e-12)
}

func TestComputeTokenBreakdown_AudioCostsAreFoldedIntoInputAndOutput(t *testing.T) {
	bs := &BillingService{cfg: &config.Config{}, fallbackPrices: map[string]*ModelPricing{}}
	pricing := &ModelPricing{
		InputPricePerToken:       audioTestInputPrice,
		OutputPricePerToken:      audioTestOutputPrice,
		AudioInputPricePerToken:  audioTestAudioInput,
		AudioOutputPricePerToken: audioTestAudioOutput,
	}
	bd := bs.computeTokenBreakdown(pricing, UsageTokens{
		InputTokens: 1000, OutputTokens: 500, AudioInputTokens: 400, AudioOutputTokens: 200,
	}, 2, "flex", false)

	// flex 档 0.5 倍同样作用到音频明细。
	require.InDelta(t, 400*audioTestAudioInput*0.5, bd.AudioInputCost, 1e-12)
	require.InDelta(t, 200*audioTestAudioOutput*0.5, bd.AudioOutputCost, 1e-12)
	require.InDelta(t, (600*audioTestInputPrice+400*audioTestAudioInput)*0.5, bd.InputCost, 1e-12)
	require.InDelta(t, (300*audioTestOutputPrice+200*audioTestAudioOutput)*0.5, bd.OutputCost, 1e-12)
	require.InDelta(t, bd.InputCost+bd.OutputCost, bd.TotalCost, 1e-12)
	require.InDelta(t, bd.TotalCost*2, bd.ActualCost, 1e-12)
}

func TestExtractOpenAIUsage_ParsesAudioTokenDetails(t *testing.T) {
	t.Run("chat completions", func(t *testing.T) {
		usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{"usage":{"prompt_tokens":1000,"completion_tokens":500,
			"prompt_tokens_details":{"cached_tokens":0,"audio_tokens":400},
			"completion_tokens_details":{"reasoning_tokens":0,"audio_tokens":200}}}`))
		require.True(t, ok)
		require.Equal(t, 400, usage.AudioInputTokens)
		require.Equal(t, 200, usage.AudioOutputTokens)
	})
	t.Run("realtime subtracts cached audio", func(t *testing.T) {
		usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{"usage":{"input_tokens":1000,"output_tokens":300,
			"input_token_details":{"cached_tokens":100,"text_tokens":400,"audio_tokens":600,"cached_tokens_details":{"text_tokens":20,"audio_tokens":80}},
			"output_token_details":{"text_tokens":100,"audio_tokens":200}}}`))
		require.True(t, ok)
		require.Equal(t, 520, usage.AudioInputTokens)
		require.Equal(t, 200, usage.AudioOutputTokens)
	})
	t.Run("responses usage from apicompat", func(t *testing.T) {
		usage := copyOpenAIUsageFromResponsesUsage(&apicompat.ResponsesUsage{
			InputTokens: 50, OutputTokens: 20,
			InputTokensDetails:  &apicompat.ResponsesInputTokensDetails{AudioTokens: 30},
			OutputTokensDetails: &apicompat.ResponsesOutputTokensDetails{AudioTokens: 10},
		})
		require.Equal(t, 30, usage.AudioInputTokens)
		require.Equal(t, 10, usage.AudioOutputTokens)
	})
	t.Run("text-only usage has no audio", func(t *testing.T) {
		usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{"usage":{"input_tokens":10,"output_tokens":5}}`))
		require.True(t, ok)
		require.Zero(t, usage.AudioInputTokens)
		require.Zero(t, usage.AudioOutputTokens)
	})
}

func TestExtractGeminiUsage_AudioModality(t *testing.T) {
	usage := extractGeminiUsage([]byte(`{"usageMetadata":{
		"promptTokenCount":1000,"candidatesTokenCount":300,"cachedContentTokenCount":200,"thoughtsTokenCount":50,
		"promptTokensDetails":[{"modality":"TEXT","tokenCount":400},{"modality":"AUDIO","tokenCount":600}],
		"cacheTokensDetails":[{"modality":"AUDIO","tokenCount":150},{"modality":"TEXT","tokenCount":50}],
		"candidatesTokensDetails":[{"modality":"AUDIO","tokenCount":120},{"modality":"TEXT","tokenCount":180}]
	}}`))
	require.NotNil(t, usage)
	require.Equal(t, 800, usage.InputTokens)
	require.Equal(t, 350, usage.OutputTokens)
	require.Equal(t, 450, usage.AudioInputTokens, "prompt AUDIO minus cached AUDIO")
	require.Equal(t, 120, usage.AudioOutputTokens)
}

func TestGatewayRecordUsageCost_GeminiAudioModalityBilledAtAudioPrice(t *testing.T) {
	// 普通 Gemini 对话模型收音频输入：价格文件带 input_cost_per_audio_token（gemini-2.5-flash：$1/MTok vs 文本 $0.3/MTok）。
	bs := &BillingService{cfg: &config.Config{}, fallbackPrices: map[string]*ModelPricing{}, pricingService: &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gemini-2.5-flash": {
				LiteLLMProvider: "gemini", Mode: "chat",
				InputCostPerToken: 3e-7, OutputCostPerToken: 2.5e-6, InputCostPerAudioToken: 1e-6,
			},
		},
	}}
	svc := &GatewayService{billingService: bs, resolver: NewModelPricingResolver(nil, bs)}
	usage := extractGeminiUsage([]byte(`{"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":100,
		"promptTokensDetails":[{"modality":"TEXT","tokenCount":200},{"modality":"AUDIO","tokenCount":800}]}}`))
	require.NotNil(t, usage)

	cost := svc.calculateRecordUsageCost(context.Background(), &ForwardResult{Model: "gemini-2.5-flash", Usage: *usage},
		&APIKey{}, "gemini-2.5-flash", 1, time.Time{})

	require.NotNil(t, cost)
	// 输入：200 × $0.3/MTok + 800 × $1/MTok = $0.00006 + $0.0008；输出：100 × $2.5/MTok（无音频输出价，文本价）。
	require.InDelta(t, 0.00086, cost.InputCost, 1e-12)
	require.InDelta(t, 0.0008, cost.AudioInputCost, 1e-12)
	require.InDelta(t, 0.00025, cost.OutputCost, 1e-12)
	require.InDelta(t, 0.00111, cost.TotalCost, 1e-12)
}

func TestModelCatalogAudioPrices_SeedApplyAndValidate(t *testing.T) {
	entry := seedEntryFromLiteLLM("gpt-audio-test", &LiteLLMModelPricing{
		LiteLLMProvider: "openai", Mode: "chat",
		InputCostPerToken: audioTestInputPrice, OutputCostPerToken: audioTestOutputPrice,
		InputCostPerAudioToken: audioTestAudioInput, OutputCostPerAudioToken: audioTestAudioOutput,
	})
	require.NotNil(t, entry.AudioInputPrice)
	require.InDelta(t, audioTestAudioInput, *entry.AudioInputPrice, 1e-15)
	require.NotNil(t, entry.AudioOutputPrice)
	require.InDelta(t, audioTestAudioOutput, *entry.AudioOutputPrice, 1e-15)

	noAudio := seedEntryFromLiteLLM("plain", &LiteLLMModelPricing{InputCostPerToken: 1e-6})
	require.Nil(t, noAudio.AudioInputPrice)
	require.Nil(t, noAudio.AudioOutputPrice)

	pricing := &ModelPricing{}
	entry.ApplyToModelPricing(pricing)
	require.InDelta(t, audioTestAudioInput, pricing.AudioInputPricePerToken, 1e-15)
	require.InDelta(t, audioTestAudioOutput, pricing.AudioOutputPricePerToken, 1e-15)

	negative := -1.0
	entry.AudioInputPrice = &negative
	entry.Normalize()
	require.ErrorContains(t, entry.Validate(), "audio_input_price must be >= 0")
}

func TestPricingService_ParsesAudioTokenPrices(t *testing.T) {
	svc := &PricingService{}
	data, err := svc.parsePricingData([]byte(`{"gpt-audio-test":{"litellm_provider":"openai","mode":"chat",
		"input_cost_per_token":2.5e-6,"output_cost_per_token":1e-5,
		"input_cost_per_audio_token":4e-5,"output_cost_per_audio_token":8e-5}}`))
	require.NoError(t, err)
	require.InDelta(t, 4e-5, data["gpt-audio-test"].InputCostPerAudioToken, 1e-15)
	require.InDelta(t, 8e-5, data["gpt-audio-test"].OutputCostPerAudioToken, 1e-15)

	bs := &BillingService{cfg: &config.Config{}, fallbackPrices: map[string]*ModelPricing{}, pricingService: &PricingService{pricingData: data}}
	pricing, err := bs.GetModelPricing("gpt-audio-test")
	require.NoError(t, err)
	require.InDelta(t, 4e-5, pricing.AudioInputPricePerToken, 1e-15)
	require.InDelta(t, 8e-5, pricing.AudioOutputPricePerToken, 1e-15)
}
