package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
)

const accountTestSuppressCompletionContextKey = "account_test_suppress_completion"

// testKeyProtocolEndpointConnection 按第三方 key 配置的协议地址选探针。
//
// 协议顺序与 PrimaryUpstreamProtocol 一致：chat_completions 是 OpenAI 网关族的主
// 地址（图片、向量等扩展端点也挂在它下面），其次 responses，再次 anthropic、gemini
// 两个协议专用根。配了 chat_completions 的 key 顺带验证它的 anthropic / responses
// 地址——这三个地址都会被真实请求按入站协议用到。
//
// 例外：管理员选了 gemini-* 模型且 key 配了 gemini 地址时直接测 Gemini 端点。模型
// 决定协议族，同一个探针载荷不可能同时是合法的 Gemini 与 Anthropic 请求。
func (s *AccountTestService) testKeyProtocolEndpointConnection(c *gin.Context, account *Account, modelID string, prompt string, mode string) error {
	geminiEndpoint := account.ProtocolEndpoint(APIProtocolGemini)
	if geminiEndpoint != "" && strings.HasPrefix(strings.TrimSpace(modelID), "gemini-") {
		return s.testGeminiAccountConnection(c, account, modelID, prompt)
	}
	// 主地址协议与 PrimaryUpstreamBaseURL 同一取址顺序，不另抄一份。
	switch account.PrimaryUpstreamProtocol() {
	case APIProtocolChatCompletions:
		return s.testKeyOpenAIFamilyConnection(c, account, modelID, prompt, mode)
	case APIProtocolResponses:
		return s.testOpenAIAccountConnection(c, account, modelID, prompt, normalizeAccountTestMode(mode))
	case APIProtocolAnthropic:
		return s.testKeyAnthropicConnection(c, account, modelID)
	case APIProtocolGemini:
		return s.testGeminiAccountConnection(c, account, modelID, prompt)
	default:
		return s.sendErrorAndEnd(c, MissingProtocolEndpointError(account, strings.Join(UpstreamProtocols(), " / ")).Error())
	}
}

// testKeyOpenAIFamilyConnection 处理配了 chat_completions 地址的 key。
//
// compact 探针（原生 remote compaction v2）与图片生成（/v1/images/generations）是
// OpenAI 协议层面的专用线，由 testOpenAIAccountConnection 自己按协议取址，不走逐个
// 协议端点的探测。
func (s *AccountTestService) testKeyOpenAIFamilyConnection(c *gin.Context, account *Account, modelID string, prompt string, mode string) error {
	mode = normalizeAccountTestMode(mode)
	testModelID := strings.TrimSpace(modelID)
	if testModelID == "" {
		testModelID = openai.DefaultTestModel
	}
	if mode == AccountTestModeCompact || isOpenAIImageModel(account.GetMappedModel(testModelID)) {
		return s.testOpenAIAccountConnection(c, account, modelID, prompt, mode)
	}
	return s.testKeyConfiguredOpenAIEndpoints(c, account, modelID, prompt)
}

// testKeyConfiguredOpenAIEndpoints verifies the Chat Completions endpoint of a
// key plus its Anthropic and Responses endpoints when configured.
func (s *AccountTestService) testKeyConfiguredOpenAIEndpoints(c *gin.Context, account *Account, modelID string, prompt string) error {
	testModelID := strings.TrimSpace(modelID)
	if testModelID == "" {
		testModelID = openai.DefaultTestModel
	}
	testModelID = account.GetMappedModel(testModelID)

	authToken := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if authToken == "" {
		return s.sendErrorAndEnd(c, "No API key available")
	}

	// The existing Chat probe owns the SSE lifecycle. Suppress intermediate
	// completion events until every native adaptive endpoint has passed.
	c.Set(accountTestSuppressCompletionContextKey, true)
	defer c.Set(accountTestSuppressCompletionContextKey, false)
	if err := s.testKeyChatCompletionsConnection(c, account, modelID, prompt); err != nil {
		return err
	}

	if account.ProtocolEndpoint(APIProtocolAnthropic) != "" {
		if err := s.testKeySecondaryAnthropicConnection(c, account, testModelID, authToken); err != nil {
			return err
		}
	}

	if account.ProtocolEndpoint(APIProtocolResponses) != "" {
		if err := s.testKeySecondaryResponsesConnection(c, account, testModelID, authToken); err != nil {
			return err
		}
	}

	c.Set(accountTestSuppressCompletionContextKey, false)
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) testKeySecondaryAnthropicConnection(c *gin.Context, account *Account, testModelID string, authToken string) error {
	ctx := c.Request.Context()
	baseURL, err := s.validateUpstreamBaseURL(account.ProtocolEndpoint(APIProtocolAnthropic))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid adaptive Anthropic base URL: %s", err.Error()))
	}
	// 与真实转发同一个拼接口径：地址带不带 /v1 都得到 {base}/v1/messages。
	apiURL := joinUpstreamEndpointURL(baseURL, "/v1/messages")

	payload, err := createTestPayload(testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create adaptive Anthropic test payload")
	}
	payloadBytes, _ := json.Marshal(payload)

	s.sendEvent(c, TestEvent{Type: "status", Text: "正在通过原生 /v1/messages 测试自适应 Anthropic 端点"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create adaptive Anthropic request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("anthropic-version", "2023-06-01")
	for key, value := range claude.DefaultHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("anthropic-beta", claude.APIKeyBetaHeader)
	// Ollama Cloud Anthropic 兼容端点按 adaptive 实际选用的 Anthropic
	// base_url 强制 Bearer，其余保持 extra/default 行为。
	setAnthropicAPIKeyAuthHeader(req.Header, account, authToken, account.ProtocolEndpoint(APIProtocolAnthropic))
	account.ApplyHeaderOverrides(req.Header)
	applyOpenCodeSessionHeader(c, account, apiURL, req.Header, payloadBytes)

	resp, err := s.doKeyProbeRequest(req, account)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Adaptive Anthropic endpoint request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Adaptive Anthropic endpoint returned %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == http.StatusUnauthorized && s.accountRepo != nil {
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, errMsg)
	}

	if err := s.processKeyAnthropicProbeStream(c, resp.Body); err != nil {
		return err
	}
	s.sendEvent(c, TestEvent{Type: "status", Text: "已通过原生 /v1/messages 验证"})
	return nil
}

func (s *AccountTestService) processKeyAnthropicProbeStream(c *gin.Context, body io.Reader) error {
	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return s.sendErrorAndEnd(c, "Adaptive Anthropic stream ended before message_stop")
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Adaptive Anthropic stream read error: %s", err.Error()))
		}

		line = strings.TrimSpace(line)
		if line == "" || !sseDataPrefix.MatchString(line) {
			continue
		}
		jsonStr := sseDataPrefix.ReplaceAllString(line, "")
		if jsonStr == "[DONE]" {
			return nil
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}
		switch eventType, _ := data["type"].(string); eventType {
		case "content_block_delta":
			if delta, ok := data["delta"].(map[string]any); ok {
				if text, ok := delta["text"].(string); ok && text != "" {
					s.sendEvent(c, TestEvent{Type: "content", Text: text})
				}
			}
		case "message_stop":
			return nil
		case "error":
			errorMsg := "Unknown error"
			if errData, ok := data["error"].(map[string]any); ok {
				if message, ok := errData["message"].(string); ok && message != "" {
					errorMsg = message
				}
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Adaptive Anthropic endpoint error: %s", errorMsg))
		}
	}
}

func (s *AccountTestService) testKeySecondaryResponsesConnection(c *gin.Context, account *Account, testModelID string, authToken string) error {
	ctx := c.Request.Context()
	baseURL, err := s.validateUpstreamBaseURL(account.ProtocolEndpoint(APIProtocolResponses))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid adaptive Responses base URL: %s", err.Error()))
	}
	apiURL := buildOpenAIResponsesURLForVendor(account.Vendor(), baseURL)

	payload := createOpenAITestPayload(testModelID, false)
	// DeepSeek / Kimi native Responses endpoints are stateless and do not need
	// the OpenAI probe's synthetic instructions.
	delete(payload, "instructions")
	payloadBytes, _ := json.Marshal(payload)
	payloadBytes = normalizeDeepSeekResponsesRequestBody(account, payloadBytes)

	s.sendEvent(c, TestEvent{Type: "status", Text: "正在通过原生 /responses 测试自适应 Responses 端点"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create adaptive Responses request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+authToken)
	applyOpenAICodexProbeHeaders(req.Header)
	account.ApplyHeaderOverrides(req.Header)
	applyOpenCodeSessionHeader(c, account, apiURL, req.Header, payloadBytes)

	resp, err := s.doKeyProbeRequest(req, account)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Adaptive Responses endpoint request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Adaptive Responses endpoint returned %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == http.StatusUnauthorized && s.accountRepo != nil {
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, errMsg)
	}

	if err := s.processOpenAIStream(c, resp.Body); err != nil {
		return err
	}
	s.sendEvent(c, TestEvent{Type: "status", Text: "已通过原生 /responses 验证"})
	return nil
}

func (s *AccountTestService) doKeyProbeRequest(req *http.Request, account *Account) (*http.Response, error) {
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	return s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
}

// testKeyAnthropicConnection verifies the native Anthropic endpoint of a
// third-party key whose request goes over the anthropic protocol. The probe uses
// the anthropic protocol endpoint (same address as real /v1/messages
// forwarding) and the shared API-key auth header. A key without that address
// never reaches here — no官方端点兜底，缺地址是配置错误。
func (s *AccountTestService) testKeyAnthropicConnection(c *gin.Context, account *Account, modelID string) error {
	ctx := c.Request.Context()

	testModelID := strings.TrimSpace(modelID)
	if testModelID == "" {
		testModelID = claude.DefaultTestModel
	}
	testModelID = account.GetMappedModel(testModelID)

	authToken := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if authToken == "" {
		return s.sendErrorAndEnd(c, "No API key available")
	}

	baseURL, err := s.validateUpstreamBaseURL(account.ProtocolEndpoint(APIProtocolAnthropic))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid Anthropic base URL: %s", err.Error()))
	}
	if hint := anthropicProbeBaseURLMisconfigHint(baseURL); hint != "" {
		return s.sendErrorAndEnd(c, hint)
	}
	// 地址带不带 /v1 都得到 {base}/v1/messages。第三方 key 一律按中转探测：不带 Anthropic
	// 官方端点的 ?beta=true 与 anthropic-beta（指向 api.anthropic.com 的 key 也一样）。
	apiURL := joinUpstreamEndpointURL(baseURL, "/v1/messages")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	payload, err := createTestPayload(testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Anthropic test payload")
	}
	payloadBytes, _ := json.Marshal(payload)

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Anthropic test request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("anthropic-version", "2023-06-01")
	for key, value := range claude.DefaultHeaders {
		req.Header.Set(key, value)
	}
	// Ollama Cloud Anthropic 兼容端点按实际 base_url 强制 Bearer，其余保持
	// extra/default 行为。
	setAnthropicAPIKeyAuthHeader(req.Header, account, authToken, account.ProtocolEndpoint(APIProtocolAnthropic))
	account.ApplyHeaderOverrides(req.Header)
	applyOpenCodeSessionHeader(c, account, apiURL, req.Header, payloadBytes)

	resp, err := s.doKeyProbeRequest(req, account)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Anthropic endpoint request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Anthropic endpoint returned %d: %s", resp.StatusCode, string(body))
		if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && s.accountRepo != nil {
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, errMsg)
	}

	return s.processClaudeStream(c, resp.Body)
}

// anthropicProbeBaseURLMisconfigHint reports an actionable error when an
// anthropic-protocol account's base_url still points at an OpenAI-compatible
// endpoint (paas path, chat/completions or responses suffix). The joined
// {base}/v1/messages would 404 with no hint about the actual misconfiguration.
//
// 纯版本后缀（.../v1）不再算误配：拼接口径与转发一致（joinUpstreamEndpointURL 版本
// 感知），https://api.anthropic.com/v1 这类地址能正确拼成 {base}/messages。
func anthropicProbeBaseURLMisconfigHint(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	path := strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	if path == "" {
		return ""
	}
	openAICompatShaped := strings.Contains(path, "/paas/") ||
		strings.HasSuffix(path, "/chat/completions") ||
		strings.HasSuffix(path, "/responses")
	if !openAICompatShaped {
		return ""
	}
	return fmt.Sprintf(
		"the anthropic protocol endpoint (%s) looks like an OpenAI-compatible endpoint; "+
			"requests would hit {base}/v1/messages and 404. Set it to the provider's Anthropic endpoint "+
			"(e.g. https://open.bigmodel.cn/api/anthropic) or configure the address under chat_completions instead.",
		baseURL,
	)
}
