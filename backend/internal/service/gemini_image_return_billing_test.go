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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 对话入站（messages / chat / responses）与 /v1beta 生图：客户端拿到 markdown 图片，
// ForwardResult.ImageCount = 上游真正回的张数，按张计价 × 张数，渠道成本按承接上的上游按次价。

const (
	imgTestA       = "aW1hZ2UtYQ==" // "image-a"
	imgTestB       = "aW1hZ2UtYg==" // "image-b"
	imgTestThought = "dGhvdWdodA==" // "thought"
	imgTestModel   = "gemini-3.1-flash-image"
)

func imgTestPart(data string) string {
	return `{"inlineData":{"mimeType":"image/png","data":"` + data + `"}}`
}

func imgTestMarkdown(data string) string {
	return "![image](data:image/png;base64," + data + ")"
}

func imgTestCandidate(parts string, finish bool) string {
	out := `{"candidates":[{"content":{"role":"model","parts":[` + parts + `]}`
	if finish {
		out += `,"finishReason":"STOP"`
	}
	out += `}]`
	if finish {
		out += `,"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":1300}`
	}
	return out + `}`
}

// imgTestSSE 把每个 chunk 写成一行 data 事件；wrap=true 时套 v1internal 的 {"response": ...}。
func imgTestSSE(wrap bool, chunks ...string) string {
	var b strings.Builder
	for _, chunk := range chunks {
		if wrap {
			chunk = `{"response":` + chunk + `}`
		}
		_, _ = b.WriteString("data: " + chunk + "\n\n")
	}
	return b.String()
}

// imgTestIncrementalChunks 增量流：文字、图 A、图 B 各自一个 chunk。
func imgTestIncrementalChunks(images ...string) []string {
	chunks := []string{imgTestCandidate(`{"text":"here you go"}`, false)}
	for i, data := range images {
		chunks = append(chunks, imgTestCandidate(imgTestPart(data), i == len(images)-1))
	}
	if len(images) == 0 {
		chunks = append(chunks, imgTestCandidate(`{"text":"."}`, true))
	}
	return chunks
}

func imgTestImages(n int) []string {
	return []string{imgTestA, imgTestB}[:n]
}

func newImgTestContext(path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	return c, rec
}

func sseResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func requireClientImages(t *testing.T, body string, images ...string) {
	t.Helper()
	require.Equal(t, len(images), strings.Count(body, "![image](data:"), "client body: %s", body)
	for _, data := range images {
		require.Contains(t, body, imgTestMarkdown(data))
	}
}

// ---------- Antigravity 成品号 ----------

func runAntigravityImagePath(t *testing.T, path string, stream bool, upstreamBody string, call func(*AntigravityGatewayService, *gin.Context, *Account, []byte) (*ForwardResult, error), body []byte) (*ForwardResult, string, []byte) {
	t.Helper()
	upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{sseResponse(upstreamBody)}}
	svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
	c, rec := newImgTestContext(path, body)
	result, err := call(svc, c, newAntigravityCompatAccount(AccountTypeOAuth), body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, stream, result.Stream)
	return result, rec.Body.String(), upstream.requestBodies[0]
}

func TestAntigravityChatInboundImagesAreReturnedAndCounted(t *testing.T) {
	type path struct {
		name, url string
		body      func(stream bool) []byte
		call      func(*AntigravityGatewayService, *gin.Context, *Account, []byte) (*ForwardResult, error)
	}
	messages := path{"messages", "/v1/messages",
		func(stream bool) []byte {
			return []byte(`{"model":"` + imgTestModel + `","max_tokens":256,"stream":` + boolJSON(stream) + `,"messages":[{"role":"user","content":"draw a cat"}]}`)
		},
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.Forward(context.Background(), c, a, b, false)
		}}
	chat := path{"chat completions", "/v1/chat/completions",
		func(stream bool) []byte {
			return []byte(`{"model":"` + imgTestModel + `","stream":` + boolJSON(stream) + `,"messages":[{"role":"user","content":"draw a cat"}]}`)
		},
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.ForwardAsChatCompletions(context.Background(), c, a, b, nil)
		}}
	responses := path{"responses", "/v1/responses",
		func(stream bool) []byte {
			return []byte(`{"model":"` + imgTestModel + `","stream":` + boolJSON(stream) + `,"input":"draw a cat"}`)
		},
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.ForwardAsResponses(context.Background(), c, a, b, nil)
		}}

	for _, p := range []path{messages, chat, responses} {
		for _, stream := range []bool{false, true} {
			for _, n := range []int{1, 2} {
				t.Run(p.name+"/"+streamName(stream)+"/"+imgCountName(n), func(t *testing.T) {
					images := imgTestImages(n)
					result, body, _ := runAntigravityImagePath(t, p.url, stream, imgTestSSE(true, imgTestIncrementalChunks(images...)...), p.call, p.body(stream))
					if p.name == "responses" && stream {
						// Responses 流的 delta / done / completed 各带一遍全文，按收尾事件看最终内容。
						body = imgTestResponsesCompletedEvent(t, body)
					}
					requireClientImages(t, body, images...)
					require.Equal(t, n, result.ImageCount)
				})
			}
		}
	}
}

func TestAntigravityGenerateContentCountsObservedImages(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, n := range []int{1, 2} {
			t.Run(streamName(stream)+"/"+imgCountName(n), func(t *testing.T) {
				images := imgTestImages(n)
				body := []byte(`{"contents":[{"role":"user","parts":[{"text":"draw a cat"}]}],"generationConfig":{"responseModalities":["TEXT","IMAGE"],"imageConfig":{"imageSize":"1K"}}}`)
				result, clientBody, _ := runAntigravityImagePath(t, "/v1beta/models/"+imgTestModel+":generateContent", stream,
					imgTestSSE(true, imgTestIncrementalChunks(images...)...),
					func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
						action := "generateContent"
						if stream {
							action = "streamGenerateContent"
						}
						return svc.ForwardGemini(context.Background(), c, a, imgTestModel, action, stream, b, false)
					}, body)
				// 原生透传回 inlineData，不改写成 markdown。
				require.Equal(t, n, strings.Count(clientBody, `"inlineData"`), clientBody)
				require.Equal(t, n, result.ImageCount, "observed, not the hardcoded 1")
				require.Equal(t, ImageBillingSize1K, result.ImageSize)
			})
		}
	}
}

// 思考过程里的草图（thought=true）不是成品图，不计费。
func TestAntigravityThoughtImagesAreNotBilled(t *testing.T) {
	upstream := imgTestSSE(true,
		imgTestCandidate(`{"inlineData":{"mimeType":"image/png","data":"`+imgTestThought+`"},"thought":true}`, false),
		imgTestCandidate(imgTestPart(imgTestA), true),
	)
	body := []byte(`{"model":"` + imgTestModel + `","max_tokens":256,"messages":[{"role":"user","content":"draw"}]}`)
	result, _, _ := runAntigravityImagePath(t, "/v1/messages", false, upstream,
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.Forward(context.Background(), c, a, b, false)
		}, body)
	require.Equal(t, 1, result.ImageCount)
}

// ---------- Gemini 兼容层（AI Studio / OAuth / 配了 gemini 地址的第三方 key） ----------

func newGeminiImageKeyAccount() *Account {
	return &Account{
		ID: 3758, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1,
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://relay.example.com"},
		Credentials:       map[string]any{"api_key": "gemini-key", "model_mapping": map[string]any{"nana-banana": "nana-banana"}},
	}
}

func runGeminiCompatImagePath(t *testing.T, path string, upstream *http.Response, call func(*GeminiMessagesCompatService, *gin.Context, *Account, []byte) (*ForwardResult, error), body []byte) (*ForwardResult, string) {
	t.Helper()
	svc := &GeminiMessagesCompatService{httpUpstream: &geminiCompatHTTPUpstreamStub{response: upstream}, cfg: &config.Config{}}
	c, rec := newImgTestContext(path, body)
	result, err := call(svc, c, newGeminiImageKeyAccount(), body)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result, rec.Body.String()
}

func TestGeminiCompatChatInboundImagesAreReturnedAndCounted(t *testing.T) {
	forwardMessages := func(svc *GeminiMessagesCompatService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
		return svc.Forward(context.Background(), c, a, b)
	}
	forwardChat := func(svc *GeminiMessagesCompatService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
		return svc.ForwardAsChatCompletions(context.Background(), c, a, b)
	}
	for _, inbound := range []struct {
		name, path string
		body       func(stream bool) []byte
		call       func(*GeminiMessagesCompatService, *gin.Context, *Account, []byte) (*ForwardResult, error)
	}{
		{"messages", "/v1/messages", func(stream bool) []byte {
			return []byte(`{"model":"` + imgTestModel + `","max_tokens":256,"stream":` + boolJSON(stream) + `,"messages":[{"role":"user","content":"draw a cat"}]}`)
		}, forwardMessages},
		{"chat completions", "/v1/chat/completions", func(stream bool) []byte {
			return []byte(`{"model":"` + imgTestModel + `","stream":` + boolJSON(stream) + `,"messages":[{"role":"user","content":"draw a cat"}]}`)
		}, forwardChat},
	} {
		for _, n := range []int{1, 2} {
			images := imgTestImages(n)
			t.Run(inbound.name+"/non-stream/"+imgCountName(n), func(t *testing.T) {
				parts := []string{`{"text":"here you go"}`}
				for _, data := range images {
					parts = append(parts, imgTestPart(data))
				}
				result, body := runGeminiCompatImagePath(t, inbound.path, jsonResponse(imgTestCandidate(strings.Join(parts, ","), true)), inbound.call, inbound.body(false))
				requireClientImages(t, body, images...)
				require.Equal(t, n, result.ImageCount)
			})
			t.Run(inbound.name+"/stream/"+imgCountName(n), func(t *testing.T) {
				result, body := runGeminiCompatImagePath(t, inbound.path, sseResponse(imgTestSSE(false, imgTestIncrementalChunks(images...)...)), inbound.call, inbound.body(true))
				requireClientImages(t, body, images...)
				require.Equal(t, n, result.ImageCount)
			})
		}
		// 累积式上游：后一个 chunk 把前面的图整段重发，客户端只收一次、只计一次。
		t.Run(inbound.name+"/stream/cumulative", func(t *testing.T) {
			upstream := imgTestSSE(false,
				imgTestCandidate(imgTestPart(imgTestA), false),
				imgTestCandidate(imgTestPart(imgTestA)+","+imgTestPart(imgTestB), true),
			)
			result, body := runGeminiCompatImagePath(t, inbound.path, sseResponse(upstream), inbound.call, inbound.body(true))
			requireClientImages(t, body, imgTestA, imgTestB)
			require.Equal(t, 2, result.ImageCount)
		})
	}
}

// Code Assist（OAuth + project_id）的非流式请求上游按流式取再聚合：多图分散在不同 chunk 时也要全部回给客户端。
func TestGeminiCodeAssistCollectedResponseKeepsEveryImage(t *testing.T) {
	account := &Account{
		ID: 3759, Platform: PlatformGemini, Type: AccountTypeOAuth, Concurrency: 1,
		Credentials: map[string]any{"access_token": "ya29.test", "project_id": "project-3759"},
	}
	for _, inbound := range []struct {
		name, path string
		body       []byte
		call       func(*GeminiMessagesCompatService, *gin.Context, []byte) (*ForwardResult, error)
	}{
		{"messages", "/v1/messages", []byte(`{"model":"` + imgTestModel + `","max_tokens":256,"messages":[{"role":"user","content":"draw"}]}`),
			func(svc *GeminiMessagesCompatService, c *gin.Context, b []byte) (*ForwardResult, error) {
				return svc.Forward(context.Background(), c, account, b)
			}},
		{"chat completions", "/v1/chat/completions", []byte(`{"model":"` + imgTestModel + `","messages":[{"role":"user","content":"draw"}]}`),
			func(svc *GeminiMessagesCompatService, c *gin.Context, b []byte) (*ForwardResult, error) {
				return svc.ForwardAsChatCompletions(context.Background(), c, account, b)
			}},
	} {
		t.Run(inbound.name, func(t *testing.T) {
			upstream := sseResponse(imgTestSSE(true, imgTestIncrementalChunks(imgTestA, imgTestB)...))
			svc := &GeminiMessagesCompatService{tokenProvider: &GeminiTokenProvider{}, httpUpstream: &geminiCompatHTTPUpstreamStub{response: upstream}, cfg: &config.Config{}}
			c, rec := newImgTestContext(inbound.path, inbound.body)
			result, err := inbound.call(svc, c, inbound.body)
			require.NoError(t, err)
			requireClientImages(t, rec.Body.String(), imgTestA, imgTestB)
			require.Equal(t, 2, result.ImageCount)
		})
	}
}

// 零张图：模型名是已知生图模型按 1 张计（/v1beta 既有口径），别名按 0（落 token 路径）。
func TestGeminiImageZeroObservedFollowsV1BetaRule(t *testing.T) {
	textOnly := imgTestCandidate(`{"text":"I cannot draw that."}`, true)
	forward := func(svc *GeminiMessagesCompatService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
		return svc.Forward(context.Background(), c, a, b)
	}
	result, _ := runGeminiCompatImagePath(t, "/v1/messages", jsonResponse(textOnly), forward,
		[]byte(`{"model":"`+imgTestModel+`","max_tokens":64,"messages":[{"role":"user","content":"draw"}]}`))
	require.Equal(t, 1, result.ImageCount)

	result, _ = runGeminiCompatImagePath(t, "/v1/messages", jsonResponse(textOnly), forward,
		[]byte(`{"model":"nana-banana","max_tokens":64,"messages":[{"role":"user","content":"draw"}]}`))
	require.Equal(t, 0, result.ImageCount)

	antigravity, _, _ := runAntigravityImagePath(t, "/v1/messages", false, imgTestSSE(true, textOnly),
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.Forward(context.Background(), c, a, b, false)
		}, []byte(`{"model":"`+imgTestModel+`","max_tokens":64,"messages":[{"role":"user","content":"draw"}]}`))
	require.Equal(t, 1, antigravity.ImageCount)
}

// ---------- 计费 ----------

func imgBillingEntry(tiers bool) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ID: 1, ModelID: imgTestModel, BillingMode: BillingModeImage, Status: ModelCatalogStatusListed,
		ManagedBy: ModelCatalogManagedByAdmin, InputPrice: upstreamCostPtr(3e-7), OutputPrice: upstreamCostPtr(2.5e-6),
		PerRequestPrice: upstreamCostPtr(0.039),
	}
	if tiers {
		entry.PerRequestPrice = upstreamCostPtr(0.06)
		entry.Intervals = []PricingInterval{
			{TierLabel: ImageBillingSize1K, PerRequestPrice: upstreamCostPtr(0.06), SortOrder: 0},
			{TierLabel: ImageBillingSize2K, PerRequestPrice: upstreamCostPtr(0.08), SortOrder: 1},
		}
	}
	return entry
}

func recordImgUsage(t *testing.T, entry ModelCatalogEntry, accountID int64, result *ForwardResult) *UsageLog {
	t.Helper()
	usageRepo := &openAIRecordUsageBestEffortLogRepoStub{}
	svc, apiKey := newGatewayRecordUsageServiceWithResolverForTest(usageRepo)
	svc.resolver = newUpstreamCostTestResolver(t, svc.billingService, entry, mediaTestBinding(1, accountID, 0.02))
	result.RequestID = "img-billing-" + time.Now().Format(time.RFC3339Nano)
	result.Duration = time.Second
	require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: result, APIKey: apiKey,
		User:    &User{ID: 1, RateMultiplier: customRate(1)},
		Account: &Account{ID: accountID, Platform: PlatformAntigravity},
	}))
	require.NotNil(t, usageRepo.lastLog)
	return usageRepo.lastLog
}

func TestGeminiImageBillingUsesObservedCount(t *testing.T) {
	for _, n := range []int{1, 2} {
		t.Run("antigravity messages/"+imgCountName(n), func(t *testing.T) {
			body := []byte(`{"model":"` + imgTestModel + `","max_tokens":256,"messages":[{"role":"user","content":"draw"}]}`)
			result, _, _ := runAntigravityImagePath(t, "/v1/messages", false, imgTestSSE(true, imgTestIncrementalChunks(imgTestImages(n)...)...),
				func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
					return svc.Forward(context.Background(), c, a, b, false)
				}, body)
			log := recordImgUsage(t, imgBillingEntry(false), 3757, result)
			require.Equal(t, n, log.ImageCount)
			require.Equal(t, string(BillingModeImage), *log.BillingMode)
			require.InDelta(t, float64(n)*0.039, log.TotalCost, 1e-12, "count × per_request_price")
			require.InDelta(t, float64(n)*0.039, log.ActualCost, 1e-12)
			require.InDelta(t, float64(n)*0.02, log.AccountCost, 1e-12, "count × binding per-request price")
		})
		t.Run("gemini compat chat/"+imgCountName(n), func(t *testing.T) {
			result, _ := runGeminiCompatImagePath(t, "/v1/chat/completions", sseResponse(imgTestSSE(false, imgTestIncrementalChunks(imgTestImages(n)...)...)),
				func(svc *GeminiMessagesCompatService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
					return svc.ForwardAsChatCompletions(context.Background(), c, a, b)
				}, []byte(`{"model":"`+imgTestModel+`","stream":true,"messages":[{"role":"user","content":"draw"}]}`))
			log := recordImgUsage(t, imgBillingEntry(false), 3758, result)
			require.InDelta(t, float64(n)*0.039, log.TotalCost, 1e-12)
			require.InDelta(t, float64(n)*0.02, log.AccountCost, 1e-12)
		})
	}
}

// 条目有 1K / 2K 分档、/v1beta 请求带 imageConfig.imageSize：按请求的档计；对话入站拿不到尺寸，按默认档（2K）。
func TestGeminiImageBillingSizeTiers(t *testing.T) {
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"draw"}]}],"generationConfig":{"imageConfig":{"imageSize":"1K"}}}`)
	result, _, _ := runAntigravityImagePath(t, "/v1beta/models/"+imgTestModel+":generateContent", false,
		imgTestSSE(true, imgTestIncrementalChunks(imgTestA, imgTestB)...),
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.ForwardGemini(context.Background(), c, a, imgTestModel, "generateContent", false, b, false)
		}, body)
	log := recordImgUsage(t, imgBillingEntry(true), 3757, result)
	require.InDelta(t, 2*0.06, log.TotalCost, 1e-12, "1K tier")
	require.InDelta(t, 2*0.02, log.AccountCost, 1e-12, "upstream base price on the base tier")

	chat, _, _ := runAntigravityImagePath(t, "/v1/messages", false, imgTestSSE(true, imgTestIncrementalChunks(imgTestA)...),
		func(svc *AntigravityGatewayService, c *gin.Context, a *Account, b []byte) (*ForwardResult, error) {
			return svc.Forward(context.Background(), c, a, b, false)
		}, []byte(`{"model":"`+imgTestModel+`","max_tokens":256,"messages":[{"role":"user","content":"draw"}]}`))
	log = recordImgUsage(t, imgBillingEntry(true), 3757, chat)
	require.InDelta(t, 0.08, log.TotalCost, 1e-12, "no size on chat inbound: default 2K tier")
	require.InDelta(t, 0.02*0.08/0.06, log.AccountCost, 1e-12)
}

func imgTestResponsesCompletedEvent(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"response.completed"`) {
			return line
		}
	}
	require.Fail(t, "no response.completed event", body)
	return ""
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func streamName(stream bool) string {
	if stream {
		return "stream"
	}
	return "non-stream"
}

func imgCountName(n int) string {
	if n == 1 {
		return "1 image"
	}
	return "2 images"
}
