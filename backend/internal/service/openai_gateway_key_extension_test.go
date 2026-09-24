//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 扩展端点（Grok 媒体/语音、图片、count_tokens）对第三方 key 不看平台标签：地址取
// KeyUpstreamProtocols 入站为空时的协议地址（chat_completions 根地址），厂商特化看地址。

type recordingRealtimeDialer struct {
	url string
}

func (d *recordingRealtimeDialer) Dial(_ context.Context, wsURL string, _ http.Header, _ string) (openAIWSClientConn, int, http.Header, error) {
	d.url = wsURL
	return nil, 0, nil, errors.New("stop after dial capture")
}

func TestGrokMediaAndVoiceURLsForKeysIgnoreLabel(t *testing.T) {
	key := keyProtocolTestAccount(PlatformOpenAI, map[string]string{
		APIProtocolChatCompletions: xaiOfficialTestBaseURL,
		APIProtocolResponses:       "https://api.x.ai/responses-only-root/v1",
	})
	require.Equal(t, xaiOfficialTestBaseURL, key.GetGrokMediaBaseURL())
	require.Equal(t, xaiOfficialTestBaseURL, key.GetGrokBaseURL())

	mediaURL, err := buildGrokMediaURL(key, nil, GrokMediaEndpointImagesGenerations, "")
	require.NoError(t, err)
	require.Equal(t, xaiOfficialTestBaseURL+"/images/generations", mediaURL)

	voiceURL, err := buildGrokVoiceURL(key, nil, "tts")
	require.NoError(t, err)
	require.Equal(t, xaiOfficialTestBaseURL+"/tts", voiceURL)

	// anthropic 标签：缺地址报错里的协议必须是扩展端点实际要的 chat_completions，而不是标签的默认协议。
	responsesOnly := keyProtocolTestAccount(PlatformAnthropic, map[string]string{APIProtocolResponses: xaiOfficialTestBaseURL})
	require.Empty(t, responsesOnly.GetGrokMediaBaseURL(), "media is an extension endpoint on the chat completions root, not the responses address")
	_, err = buildGrokMediaURL(responsesOnly, nil, GrokMediaEndpointImagesGenerations, "")
	require.Equal(t, "MISSING_PROTOCOL_ENDPOINT", infraerrors.Reason(err))
	require.Contains(t, err.Error(), APIProtocolChatCompletions)
	_, err = buildGrokVoiceURL(responsesOnly, nil, "tts")
	require.Equal(t, "MISSING_PROTOCOL_ENDPOINT", infraerrors.Reason(err))
	require.Contains(t, err.Error(), APIProtocolChatCompletions)
}

func TestForwardGrokMediaAndVoiceAcceptKeysWhateverLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: xaiOfficialTestBaseURL})

	t.Run("media", func(t *testing.T) {
		upstream := &grokMediaContentUpstreamStub{
			responses: []*http.Response{grokMediaContentStatusResponse(`{"status":"completed"}`), {
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"video/mp4"}},
				Body:       io.NopCloser(strings.NewReader("video")),
			}},
		}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		c, _ := grokMediaContentTestContext(http.MethodGet, "https://gateway.example/v1/videos/task-1/content", nil)
		_, err := svc.ForwardGrokMedia(context.Background(), c, key, GrokMediaEndpointVideoContent, "task-1", nil, "")
		require.NoError(t, err)
		require.NotEmpty(t, upstream.requests)
		require.Equal(t, xaiOfficialTestBaseURL+"/videos/task-1", upstream.requests[0].URL.String())
	})

	t.Run("voice", func(t *testing.T) {
		upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/tts", bytes.NewReader([]byte(`{}`)))
		_, err := svc.ForwardGrokVoice(context.Background(), c, key, "tts", []byte(`{}`), "application/json")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "not supported")
		require.Len(t, upstream.requests, 1)
		require.Equal(t, xaiOfficialTestBaseURL+"/tts", upstream.lastReq.URL.String())
	})

	t.Run("realtime", func(t *testing.T) {
		probeDialer := &recordingRealtimeDialer{}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: probeDialer}
		err := svc.ProbeGrokRealtime(context.Background(), key, "token", "")
		require.EqualError(t, err, "stop after dial capture")
		require.True(t, strings.HasPrefix(probeDialer.url, "wss://api.x.ai/v1/realtime?"), probeDialer.url)

		openDialer := &recordingRealtimeDialer{}
		svc = &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: openDialer}
		_, err = svc.OpenGrokRealtime(context.Background(), key, "token", "")
		var dialErr *GrokRealtimeDialError
		require.ErrorAs(t, err, &dialErr)
		require.True(t, strings.HasPrefix(openDialer.url, "wss://api.x.ai/v1/realtime?"), openDialer.url)

		proxyDialer := &recordingRealtimeDialer{}
		svc = &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: proxyDialer}
		proxyErrCh := make(chan error, 1)
		wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := coderws.Accept(w, r, nil)
			if err != nil {
				proxyErrCh <- err
				return
			}
			defer func() { _ = conn.CloseNow() }()
			_, proxyErr := svc.ProxyGrokRealtime(r.Context(), nil, conn, key, "token", "")
			proxyErrCh <- proxyErr
		}))
		defer wsServer.Close()
		dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
		clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
		cancelDial()
		require.NoError(t, err)
		defer func() { _ = clientConn.CloseNow() }()
		select {
		case proxyErr := <-proxyErrCh:
			require.ErrorAs(t, proxyErr, &dialErr)
		case <-time.After(3 * time.Second):
			t.Fatal("ProxyGrokRealtime did not return")
		}
		require.True(t, strings.HasPrefix(proxyDialer.url, "wss://api.x.ai/v1/realtime?"), proxyDialer.url)
	})

	t.Run("subscriptions of other platforms stay rejected", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		subscription := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
		_, err := svc.ForwardGrokMedia(context.Background(), nil, subscription, GrokMediaEndpointImagesGenerations, "", nil, "")
		require.ErrorContains(t, err, "not supported for grok media")
		_, err = svc.ForwardGrokVoice(context.Background(), nil, subscription, "tts", nil, "")
		require.ErrorContains(t, err, "not supported for grok voice")
		require.ErrorContains(t, svc.ProbeGrokRealtime(context.Background(), subscription, "", ""), "not supported for grok realtime")
		_, err = svc.OpenGrokRealtime(context.Background(), subscription, "", "")
		require.ErrorContains(t, err, "grok realtime account is required")
	})
}

func TestSupportsOpenAIImageCapabilityForKeysFollowsVendorNotLabel(t *testing.T) {
	relay := featureRelayKey(featureRelayEndpoints())
	vendor := featureZhipuKey(featureZhipuEndpoints())
	requireFeatureFixtures(t, relay, vendor)
	require.True(t, relay.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
	require.False(t, vendor.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
	require.True(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}).SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))
	require.False(t, (&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}).SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))
}

func TestShouldEstimateOpenAIInputTokensLocallyForKeysIgnoresLabel(t *testing.T) {
	require.False(t, shouldEstimateOpenAIInputTokensLocally(keyProtocolTestAccount(PlatformGrok, map[string]string{APIProtocolResponses: "https://api.openai.com"})),
		"a key whose responses address is api.openai.com calls the official input_tokens endpoint whatever its label")
	require.True(t, shouldEstimateOpenAIInputTokensLocally(keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolResponses: "https://relay.example/v1"})))
	require.True(t, shouldEstimateOpenAIInputTokensLocally(keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://api.openai.com"})),
		"without a responses address there is no input_tokens endpoint to call")
	require.True(t, shouldEstimateOpenAIInputTokensLocally(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
	require.False(t, shouldEstimateOpenAIInputTokensLocally(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
}

func TestForwardCountTokensAsAnthropicLocalEstimateFollowsVendorNotLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`)
	count := func(account *Account) (*httpUpstreamRecorder, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
		upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
		_ = svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, body)
		return upstream, recorder
	}

	deepseek := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolAnthropic: "https://api.deepseek.com/anthropic"})
	require.Equal(t, PlatformDeepseek, deepseek.Vendor(), "fixture: api.deepseek.com must be recognised as deepseek")
	upstream, recorder := count(deepseek)
	require.Empty(t, upstream.requests, "vendors without a count_tokens endpoint are estimated locally")
	require.Equal(t, http.StatusOK, recorder.Code)

	relay := featureRelayKey(featureRelayEndpoints())
	upstream, _ = count(relay)
	require.Len(t, upstream.requests, 1, "a kimi label on a relay is not the kimi vendor")
}
