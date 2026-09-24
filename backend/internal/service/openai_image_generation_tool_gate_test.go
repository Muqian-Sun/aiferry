package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 生图未开放（gateway.image_generation_tool_enabled=false，默认）时 Responses image_generation 工具的把关：
// 非 Codex 客户端 400 且不打上游，Codex 官方客户端剥掉工具照常转发，Codex 桥接不注入；开关打开时不变。

const (
	imageToolGateCodexUA    = "codex_cli_rs/0.98.0"
	imageToolGateGenericUA  = "curl/8.0"
	imageToolGateToolBody   = `{"model":"gpt-5.4","input":"draw a cat","stream":false,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}},{"type":"image_generation","size":"1024x1024"}]}`
	imageToolGateCompletion = `{"id":"resp_gate","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":1}}`
)

func imageToolGateConfig(enabled bool) *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.ImageGenerationToolEnabled = enabled
	return cfg
}

func TestGateOpenAIImageGenerationTool(t *testing.T) {
	tests := []struct {
		name         string
		enabled      bool
		forceCodex   bool
		userAgent    string
		model        string
		body         string
		wantErr      bool
		wantStripped bool
	}{
		{name: "switch off rejects non-codex client", userAgent: imageToolGateGenericUA, body: imageToolGateToolBody, wantErr: true},
		{name: "switch off strips for codex cli", userAgent: imageToolGateCodexUA, body: imageToolGateToolBody, wantStripped: true},
		{name: "switch off strips when force_codex_cli", forceCodex: true, userAgent: imageToolGateGenericUA, body: imageToolGateToolBody, wantStripped: true},
		{name: "switch on leaves request unchanged", enabled: true, userAgent: imageToolGateGenericUA, body: imageToolGateToolBody},
		{name: "no image tool passes", userAgent: imageToolGateGenericUA, body: `{"model":"gpt-5.4","input":"hi","tools":[{"type":"function","name":"lookup"}]}`},
		{name: "image_gen namespace is a client function tool", userAgent: imageToolGateGenericUA, body: `{"model":"gpt-5.4","input":"hi","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}`},
		{name: "responses lite additional_tools carrying the tool is rejected", userAgent: imageToolGateGenericUA, body: `{"model":"gpt-5.4","input":[{"type":"additional_tools","tools":[{"type":"image_generation"}]},{"type":"message","role":"user","content":"draw"}]}`, wantErr: true},
		{name: "tool_choice selecting the tool is rejected", userAgent: imageToolGateGenericUA, body: `{"model":"gpt-5.4","input":"draw","tool_choice":{"type":"image_generation"}}`, wantErr: true},
		{name: "image model request is the image product, not gated here", userAgent: imageToolGateGenericUA, model: "gpt-image-2", body: `{"model":"gpt-image-2","input":"draw","tools":[{"type":"image_generation"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := imageToolGateConfig(tt.enabled)
			cfg.Gateway.ForceCodexCLI = tt.forceCodex
			model := tt.model
			if model == "" {
				model = "gpt-5.4"
			}
			out, stripped, err := GateOpenAIImageGenerationTool(cfg, tt.userAgent, "", model, []byte(tt.body))
			if tt.wantErr {
				require.ErrorIs(t, err, ErrOpenAIImageGenerationToolUnavailable)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantStripped, stripped)
			if !tt.wantStripped {
				require.Equal(t, tt.body, string(out))
				return
			}
			require.False(t, gjson.GetBytes(out, `tools.#(type=="image_generation")`).Exists())
			require.Equal(t, "lookup", gjson.GetBytes(out, `tools.#(type=="function").name`).String())
		})
	}
}

func newImageToolGateForwardFixture(t *testing.T, enabled bool, userAgent string) (*OpenAIGatewayService, *httpUpstreamRecorder, *gin.Context, *httptest.ResponseRecorder, *Account) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(imageToolGateCompletion)),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	svc.cfg.Gateway.ImageGenerationToolEnabled = enabled
	c, rec := newOpenAIImageGenerationControlTestContext(userAgent)
	return svc, upstream, c, rec, newOpenAIImageGenerationControlTestAccount()
}

func TestOpenAIForward_ImageToolSwitchOffRejectsNonCodexWithoutUpstreamCall(t *testing.T) {
	svc, upstream, c, rec, account := newImageToolGateForwardFixture(t, false, imageToolGateGenericUA)

	result, err := svc.Forward(context.Background(), c, account, []byte(imageToolGateToolBody))

	require.ErrorIs(t, err, ErrOpenAIImageGenerationToolUnavailable)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "no upstream call")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Equal(t, OpenAIImageGenerationToolUnavailableMessage, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
	// 客户端请求错误：不是 failover 错误，账号健康熔断不计入。
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	_, _, eligible := classifyOpenAIAPIKeyHealthFailure(err)
	require.False(t, eligible)
}

func TestOpenAIForward_ImageToolSwitchOffStripsForCodexCLI(t *testing.T) {
	svc, upstream, c, _, account := newImageToolGateForwardFixture(t, false, imageToolGateCodexUA)

	result, err := svc.Forward(context.Background(), c, account, []byte(imageToolGateToolBody))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, `tools.#(name=="lookup")`).Exists())
	require.Zero(t, result.ImageCount)
}

func TestOpenAIForward_ImageToolSwitchOffBlocksCodexBridgeEvenWithAccountOverride(t *testing.T) {
	svc, upstream, c, _, account := newImageToolGateForwardFixture(t, false, imageToolGateCodexUA)
	svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = true
	account.Extra = map[string]any{featureKeyCodexImageGenerationBridge: true}

	result, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.4","input":"write code","stream":false}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
	require.NotContains(t, gjson.GetBytes(upstream.lastBody, "instructions").String(), "image_generation")
	require.False(t, svc.isCodexImageGenerationBridgeEnabled(context.Background(), account, nil))
}

func TestOpenAIForward_ImageToolSwitchOnForwardsUnchanged(t *testing.T) {
	svc, upstream, c, _, account := newImageToolGateForwardFixture(t, true, imageToolGateGenericUA)

	result, err := svc.Forward(context.Background(), c, account, []byte(imageToolGateToolBody))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.True(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
}

func TestOpenAIForward_ImageToolSwitchOnCodexBridgeStillInjects(t *testing.T) {
	svc, upstream, c, _, account := newImageToolGateForwardFixture(t, true, imageToolGateCodexUA)
	svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = true

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.4","input":"write code","stream":false}`))

	require.NoError(t, err)
	require.True(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
}

// imageToolGateWSHarness 起一个 WS 入站代理：客户端经 clientConn 发 response.create，上游写入落在 captureConn。
type imageToolGateWSHarness struct {
	captureConn *openAIWSCaptureConn
	clientConn  *coderws.Conn
	serverErrCh chan error
}

func newImageToolGateWSHarness(t *testing.T, userAgent string, upstreamEvents ...string) *imageToolGateWSHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	events := make([][]byte, 0, len(upstreamEvents))
	for _, event := range upstreamEvents {
		events = append(events, []byte(event))
	}
	captureConn := &openAIWSCaptureConn{events: events}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSCaptureDialer{conn: captureConn})
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID: 41, Name: "openai-ws-image-gate", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
		Extra:       map[string]any{"openai_oauth_responses_websockets_v2_enabled": true},
	}

	serverErrCh := make(chan error, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			serverErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := r.Clone(r.Context())
		req.Header = req.Header.Clone()
		req.Header.Set("User-Agent", userAgent)
		ginCtx.Request = req
		ginCtx.Set("api_key", &APIKey{ID: 1, UserID: 1})
		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		_, firstMessage, readErr := conn.Read(readCtx)
		cancel()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "test-token", firstMessage, nil)
	}))
	t.Cleanup(wsServer.Close)

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	cancelDial()
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.CloseNow() })
	return &imageToolGateWSHarness{captureConn: captureConn, clientConn: clientConn, serverErrCh: serverErrCh}
}

func (h *imageToolGateWSHarness) write(t *testing.T, payload string) {
	t.Helper()
	writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, h.clientConn.Write(writeCtx, coderws.MessageText, []byte(payload)))
}

func (h *imageToolGateWSHarness) read(t *testing.T) []byte {
	t.Helper()
	readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, message, err := h.clientConn.Read(readCtx)
	require.NoError(t, err)
	return message
}

// WS 入站：后续 turn 带 image_generation 工具（非 Codex 客户端）→ 先回 error 事件、再以 PolicyViolation
// 关连接，该帧不转发上游。
func TestOpenAIWSIngress_ImageToolSwitchOffRejectsFollowUpTurnWithoutForwarding(t *testing.T) {
	h := newImageToolGateWSHarness(t, imageToolGateGenericUA,
		`{"type":"response.completed","response":{"id":"resp_gate_turn1","model":"gpt-5.5","usage":{"input_tokens":1,"output_tokens":1}}}`)

	h.write(t, `{"type":"response.create","model":"gpt-5.5","stream":false,"input":"hello"}`)
	require.Equal(t, "resp_gate_turn1", gjson.GetBytes(h.read(t), "response.id").String())

	h.write(t, `{"type":"response.create","model":"gpt-5.5","stream":false,"previous_response_id":"resp_gate_turn1","input":"draw a cat","tools":[{"type":"image_generation"}]}`)
	event := h.read(t)
	require.Equal(t, "error", gjson.GetBytes(event, "type").String())
	require.Equal(t, "invalid_request_error", gjson.GetBytes(event, "error.type").String())
	require.Equal(t, OpenAIImageGenerationToolUnavailableMessage, gjson.GetBytes(event, "error.message").String())

	select {
	case serverErr := <-h.serverErrCh:
		var closeErr *OpenAIWSClientCloseError
		require.ErrorAs(t, serverErr, &closeErr)
		require.Equal(t, coderws.StatusPolicyViolation, closeErr.StatusCode())
		require.ErrorIs(t, serverErr, ErrOpenAIImageGenerationToolUnavailable)
	case <-time.After(5 * time.Second):
		t.Fatal("等待 ingress websocket 结束超时")
	}
	require.Len(t, h.captureConn.writes, 1, "the rejected turn must not reach upstream")
}

// WS 入站：Codex 官方客户端首帧带 image_generation 工具 → 剥掉后照常转发。
func TestOpenAIWSIngress_ImageToolSwitchOffStripsForCodexCLI(t *testing.T) {
	h := newImageToolGateWSHarness(t, imageToolGateCodexUA,
		`{"type":"response.completed","response":{"id":"resp_gate_codex","model":"gpt-5.5","usage":{"input_tokens":1,"output_tokens":1}}}`)

	h.write(t, `{"type":"response.create","model":"gpt-5.5","stream":false,"input":"draw a cat","tools":[{"type":"function","name":"shell","parameters":{"type":"object"}},{"type":"image_generation"}]}`)
	require.Equal(t, "resp_gate_codex", gjson.GetBytes(h.read(t), "response.id").String())

	require.Len(t, h.captureConn.writes, 1)
	upstreamPayload := requestToJSONString(h.captureConn.writes[0])
	require.False(t, gjson.Get(upstreamPayload, `tools.#(type=="image_generation")`).Exists())
	require.True(t, gjson.Get(upstreamPayload, `tools.#(name=="shell")`).Exists())
}
