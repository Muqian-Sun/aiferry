//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 扩展端点（图片、count_tokens）对第三方 key 不看平台标签：地址取 KeyUpstreamProtocols 入站为空时的
// 协议地址（chat_completions 根地址），厂商特化看地址。xAI 媒体 / 语音是 Grok 成品号专属的厂商端点，
// 第三方 key 一律按中转、不承接（指向 api.x.ai 的 key 也一样，2026-09-29）。

type recordingRealtimeDialer struct {
	url string
}

func (d *recordingRealtimeDialer) Dial(_ context.Context, wsURL string, _ http.Header, _ string) (openAIWSClientConn, int, http.Header, error) {
	d.url = wsURL
	return nil, 0, nil, errors.New("stop after dial capture")
}

// xAI 媒体 / 语音 / 实时语音只对 Grok 成品号：第三方 key 不论标签、不论地址是不是 api.x.ai 都拒绝，
// 一次上游请求都不发（2026-09-29 海外四家不再有官方 key）。
func TestForwardGrokMediaAndVoiceRejectThirdPartyKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, key := range map[string]*Account{
		"openai label on api.x.ai": keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: xaiOfficialTestBaseURL}),
		"grok label on api.x.ai":   keyProtocolTestAccount(PlatformGrok, map[string]string{APIProtocolChatCompletions: xaiOfficialTestBaseURL}),
	} {
		t.Run(name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{err: errors.New("must not be called")}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := grokMediaContentTestContext(http.MethodGet, "https://gateway.example/v1/videos/task-1/content", nil)
			_, err := svc.ForwardGrokMedia(context.Background(), c, key, GrokMediaEndpointVideoContent, "task-1", nil, "")
			require.ErrorContains(t, err, "not supported for grok media")

			recorder := httptest.NewRecorder()
			voiceCtx, _ := gin.CreateTestContext(recorder)
			voiceCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/tts", bytes.NewReader([]byte(`{}`)))
			_, err = svc.ForwardGrokVoice(context.Background(), voiceCtx, key, "tts", []byte(`{}`), "application/json")
			require.ErrorContains(t, err, "not supported for grok voice")
			require.Empty(t, upstream.requests)

			dialer := &recordingRealtimeDialer{}
			svc = &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: dialer}
			require.ErrorContains(t, svc.ProbeGrokRealtime(context.Background(), key, "token", ""), "not supported for grok realtime")
			_, err = svc.OpenGrokRealtime(context.Background(), key, "token", "")
			require.ErrorContains(t, err, "grok realtime account is required")
			require.Empty(t, dialer.url, "no realtime dial for third-party keys")
		})
	}

	t.Run("realtime proxy", func(t *testing.T) {
		key := keyProtocolTestAccount(PlatformGrok, map[string]string{APIProtocolChatCompletions: xaiOfficialTestBaseURL})
		proxyDialer := &recordingRealtimeDialer{}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: proxyDialer}
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
			require.ErrorContains(t, proxyErr, "not supported for grok realtime")
		case <-time.After(3 * time.Second):
			t.Fatal("ProxyGrokRealtime did not return")
		}
		require.Empty(t, proxyDialer.url, "no realtime dial for third-party keys")
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

// input_tokens 是 OpenAI 成品号才发上游：第三方 key 一律本地估算，responses 地址是 api.openai.com 也一样
// （2026-09-29 海外四家不再有官方 key）。
func TestShouldEstimateOpenAIInputTokensLocallyForKeysIgnoresLabel(t *testing.T) {
	require.True(t, shouldEstimateOpenAIInputTokensLocally(keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolResponses: "https://api.openai.com"})),
		"a key on api.openai.com is a relay and estimates locally")
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
