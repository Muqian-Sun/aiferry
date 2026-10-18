//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// xAI 图片经第三方 key 承接：调度门、转发地址与模型映射、计费与渠道成本。视频与其它 xAI 原生端点仍只认 Grok 成品号。

func grokImageKey(id int64, endpoints map[string]string) *Account {
	return &Account{
		ID: id, Name: "relay", Platform: PlatformGrok, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials:           map[string]any{"api_key": "relay-key"},
		CatalogUpstreamModels: map[string]string{"grok-imagine-image": "grok-imagine-image-pro"},
		ProtocolEndpoints:     endpoints,
	}
}

func TestGrokImageGenerationAdmission(t *testing.T) {
	chatKey := grokImageKey(1, map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"})
	responsesOnlyKey := grokImageKey(2, map[string]string{APIProtocolResponses: "https://relay.example.com/v1"})
	cnVendorKey := grokImageKey(3, map[string]string{APIProtocolChatCompletions: DefaultDeepseekBaseURL})
	eligibleOAuth := &Account{ID: 4, Platform: PlatformGrok, Type: AccountTypeOAuth,
		Extra: map[string]any{grokBillingExtraKey: &xai.BillingSummary{StatusCode: http.StatusOK, Partial: true}}}
	unobservedOAuth := &Account{ID: 5, Platform: PlatformGrok, Type: AccountTypeOAuth}
	forbiddenOAuth := &Account{ID: 6, Platform: PlatformGrok, Type: AccountTypeOAuth,
		Extra: map[string]any{grokBillingExtraKey: &xai.BillingSummary{StatusCode: http.StatusForbidden}}}
	openAIOAuth := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	image := OpenAIEndpointCapabilityGrokImageGeneration
	video := OpenAIEndpointCapabilityGrokMediaGeneration
	cases := []struct {
		name       string
		account    *Account
		capability OpenAIEndpointCapability
		want       bool
	}{
		{"key with chat_completions address serves images", chatKey, image, true},
		{"responses-only key cannot serve images", responsesOnlyKey, image, false},
		{"key on a CN vendor has no /v1/images", cnVendorKey, image, false},
		{"eligible grok oauth serves images", eligibleOAuth, image, true},
		{"unobserved grok oauth is probed at request time", unobservedOAuth, image, true},
		{"forbidden grok oauth is rejected", forbiddenOAuth, image, false},
		{"openai oauth is not an xai image account", openAIOAuth, image, false},
		{"key still cannot create videos", chatKey, video, false},
		{"grok oauth still creates videos", eligibleOAuth, video, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.account.SupportsOpenAIEndpointCapability(tc.capability))
			admitted, reason := SelectOptions{Capability: tc.capability, Transport: OpenAIUpstreamTransportHTTPSSE}.admits(nil, nil, tc.account)
			require.Equal(t, tc.want, admitted, reason)
		})
	}

	// 334b9e86a 收口的 xAI 原生端点（语音 / 实时语音 / 原生搜索 / 视频任务查询）按 Platform 选成品号，key 照旧被拒。
	admitted, reason := SelectOptions{Capability: OpenAIEndpointCapabilityChatCompletions, Transport: OpenAIUpstreamTransportHTTPSSE, Platform: PlatformGrok}.admits(nil, nil, chatKey)
	require.False(t, admitted)
	require.Equal(t, "platform_mismatch", reason)
	admitted, reason = SelectOptions{Transport: OpenAIUpstreamTransportHTTPSSE, OnlyAccountID: chatKey.ID, Platform: PlatformGrok}.admits(nil, nil, chatKey)
	require.False(t, admitted)
	require.Equal(t, "platform_mismatch", reason)
}

func newGrokImageKeyForwardContext(path, contentType string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	return c, rec
}

func TestForwardGrokImagesAPIKeyUsesOpenAIImagesEndpointWithMappedModel(t *testing.T) {
	const xaiResponse = `{"data":[{"url":"https://imgen.x.ai/a.png"},{"url":"https://imgen.x.ai/b.png"}]}`
	cases := []struct {
		name     string
		endpoint GrokMediaEndpoint
		path     string
		body     string
		wantURL  string
	}{
		{"generations", GrokMediaEndpointImagesGenerations, "/v1/images/generations",
			`{"model":"grok-imagine-image","prompt":"a cat","n":2,"size":"1024x1024"}`,
			"https://relay.example.com/v1/images/generations"},
		// xAI 编辑请求的 image 对象原样转发（OpenAI 的 images[].image_url 解析会拒掉它）。
		{"edits", GrokMediaEndpointImagesEdits, "/v1/images/edits",
			`{"model":"grok-imagine-image","prompt":"make it blue","image":{"url":"https://example.com/in.png","type":"image_url"}}`,
			"https://relay.example.com/v1/images/edits"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(xaiResponse))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, rec := newGrokImageKeyForwardContext(tc.path, "application/json", []byte(tc.body))
			account := grokImageKey(11, map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"})

			result, err := svc.ForwardGrokImagesAPIKey(context.Background(), c, account, tc.endpoint, []byte(tc.body), "application/json")
			require.NoError(t, err)
			require.Equal(t, tc.wantURL, upstream.lastReq.URL.String())
			require.Equal(t, "Bearer relay-key", upstream.lastReq.Header.Get("Authorization"))
			require.Equal(t, "grok-imagine-image-pro", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, gjson.Get(tc.body, "image.url").String(), gjson.GetBytes(upstream.lastBody, "image.url").String())
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, 2, strings.Count(rec.Body.String(), "imgen.x.ai"))

			require.Equal(t, "grok-imagine-image", result.Model)
			require.Equal(t, "grok-imagine-image-pro", result.UpstreamModel)
			require.Equal(t, 2, result.ImageCount)
		})
	}
}

// key 路径与成品号同一套计费：xai 条目的按次价（1K / 2K 分档按请求 size）× 张数，渠道成本按承接上的上游按次价。
func TestForwardGrokImagesAPIKeyRecordsUsageAndChannelCost(t *testing.T) {
	entry := ModelCatalogEntry{
		ID: 1, ModelID: "grok-imagine-image", Vendor: "xai", BillingMode: BillingModeImage, Status: ModelCatalogStatusListed,
		ManagedBy: ModelCatalogManagedBySeed, PerRequestPrice: upstreamCostPtr(0.02),
		Intervals: []PricingInterval{
			{TierLabel: ImageBillingSize1K, PerRequestPrice: upstreamCostPtr(0.02), SortOrder: 0},
			{TierLabel: ImageBillingSize2K, PerRequestPrice: upstreamCostPtr(0.04), SortOrder: 1},
		},
	}
	for _, tc := range []struct {
		size                string
		wantTotal, wantCost float64
	}{
		{"1024x1024", 2 * 0.02, 2 * 0.01},
		{"2048x2048", 2 * 0.04, 2 * 0.01 * 0.04 / 0.02},
	} {
		t.Run(tc.size, func(t *testing.T) {
			body := `{"model":"grok-imagine-image","prompt":"a cat","n":2,"size":"` + tc.size + `"}`
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aW1hZ2UtYQ=="},{"b64_json":"aW1hZ2UtYg=="}]}`))}}
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
			svc.httpUpstream = upstream
			svc.resolver = newUpstreamCostTestResolver(t, svc.billingService, entry, mediaTestBinding(1, 11, 0.01))
			c, _ := newGrokImageKeyForwardContext("/v1/images/generations", "application/json", []byte(body))
			account := grokImageKey(11, map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"})

			result, err := svc.ForwardGrokImagesAPIKey(context.Background(), c, account, GrokMediaEndpointImagesGenerations, []byte(body), "application/json")
			require.NoError(t, err)
			result.Duration = time.Second
			require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: result, APIKey: &APIKey{ID: 10}, User: &User{ID: 20, RateMultiplier: officialRate(1)}, Account: account,
			}))
			log := usageRepo.lastLog
			require.NotNil(t, log)
			require.Equal(t, 2, log.ImageCount)
			require.Equal(t, string(BillingModeImage), *log.BillingMode)
			require.InDelta(t, tc.wantTotal, log.TotalCost, 1e-12)
			require.InDelta(t, tc.wantCost, log.AccountCost, 1e-12)
		})
	}
}
