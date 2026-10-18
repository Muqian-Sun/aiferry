//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newAccountImageTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)
	return c, rec
}

// 第三方 key 测 grok-imagine-image*：与网关同一个 {chat_completions 根}/v1/images/generations，套模型映射。
func TestKeyConnectionTestGrokImageModelUsesImagesEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name, response, wantImage string
	}{
		{"b64_json", `{"data":[{"b64_json":"aW1hZ2UtYQ=="}]}`, "data:image/png;base64,aW1hZ2UtYQ=="},
		{"url only", `{"data":[{"url":"https://imgen.x.ai/a.png"}]}`, "https://imgen.x.ai/a.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newAccountImageTestContext()
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(tc.response))}}
			svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{
				ID: 61, Name: "relay", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials:           map[string]any{"api_key": "relay-key"},
				CatalogUpstreamModels: map[string]string{"grok-imagine-image": "grok-imagine-image-pro"},
				ProtocolEndpoints:     map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"},
			}

			require.NoError(t, svc.testKeyProtocolEndpointConnection(c, account, "grok-imagine-image", "", ""))
			require.Equal(t, "https://relay.example.com/v1/images/generations", upstream.lastReq.URL.String())
			require.Equal(t, "grok-imagine-image-pro", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Contains(t, rec.Body.String(), `"type":"image"`)
			require.Contains(t, rec.Body.String(), tc.wantImage)
			require.Contains(t, rec.Body.String(), `"success":true`)
		})
	}
}

// Antigravity 测生图模型：请求要求出图（TEXT+IMAGE），回了图才算成功。
func TestAntigravityTestConnectionImageModelRequestsAnImage(t *testing.T) {
	withImage := "data: " + `{"response":` + imgTestCandidate(`{"text":"here"},`+imgTestPart(imgTestA), true) + `}` + "\n\n"
	textOnly := "data: " + `{"response":` + imgTestCandidate(`{"text":"."}`, true) + `}` + "\n\n"

	t.Run("image returned", func(t *testing.T) {
		upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{sseResponse(withImage)}}
		svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)

		result, err := svc.TestConnection(context.Background(), newAntigravityCompatAccount(AccountTypeOAuth), imgTestModel)
		require.NoError(t, err)
		require.Equal(t, []TestConnectionImage{{MimeType: "image/png", Data: imgTestA}}, result.Images)

		sent := upstream.requestBodies[0]
		require.Equal(t, `["TEXT","IMAGE"]`, gjson.GetBytes(sent, "request.generationConfig.responseModalities").Raw)
		require.False(t, gjson.GetBytes(sent, "request.generationConfig.maxOutputTokens").Exists(), "a 1-token budget can never yield an image")
		require.Contains(t, gjson.GetBytes(sent, "request.systemInstruction.parts.0.text").String(), "Antigravity")

		c, rec := newAccountImageTestContext()
		accountTest := &AccountTestService{antigravityGatewayService: newAntigravityCompatService(
			config.GatewayConfig{MaxLineSize: defaultMaxLineSize},
			&queuedHTTPUpstreamStub{responses: []*http.Response{sseResponse(withImage)}})}
		require.NoError(t, accountTest.testAntigravityAccountConnection(c, newAntigravityCompatAccount(AccountTypeOAuth), imgTestModel))
		require.Contains(t, rec.Body.String(), imgTestMarkdownDataURL(imgTestA))
		require.Contains(t, rec.Body.String(), `"success":true`)
	})

	t.Run("no image fails the test", func(t *testing.T) {
		upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{sseResponse(textOnly)}}
		svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)

		_, err := svc.TestConnection(context.Background(), newAntigravityCompatAccount(AccountTypeOAuth), imgTestModel)
		require.ErrorContains(t, err, "returned no image")
	})

	t.Run("text model keeps the minimal probe", func(t *testing.T) {
		upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{sseResponse(textOnly)}}
		svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)

		result, err := svc.TestConnection(context.Background(), newAntigravityCompatAccount(AccountTypeOAuth), "gemini-3-flash")
		require.NoError(t, err)
		require.Empty(t, result.Images)
		require.Equal(t, int64(1), gjson.GetBytes(upstream.requestBodies[0], "request.generationConfig.maxOutputTokens").Int())
	})
}

func imgTestMarkdownDataURL(data string) string {
	return "data:image/png;base64," + data
}
