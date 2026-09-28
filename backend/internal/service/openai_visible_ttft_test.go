package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIVisibleOutputClassification(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		eventType string
		want      bool
	}{
		{name: "keepalive", data: `{"type":"keepalive"}`, want: false},
		{name: "created", data: `{"type":"response.created"}`, want: false},
		{name: "empty output item", data: `{"type":"response.output_item.added","item":{"id":"item_test","type":"reasoning","summary":[]}}`, want: false},
		{name: "empty delta", data: `{"type":"response.output_text.delta","delta":""}`, want: false},
		{name: "text delta", data: `{"type":"response.output_text.delta","delta":"test output"}`, want: true},
		{name: "tool arguments", data: `{"type":"response.function_call_arguments.delta","delta":"{}"}`, want: true},
		{name: "partial image", data: `{"type":"response.image_generation_call.partial_image","partial_image_b64":"dGVzdA=="}`, want: true},
		{name: "completed image item", data: `{"type":"response.output_item.done","item":{"id":"item_test","type":"image_generation_call","result":"dGVzdA=="}}`, want: true},
		{name: "empty completed", data: `{"type":"response.completed","response":{"id":"resp_test","output":[]}}`, want: false},
		{name: "completed with output usage only", data: `{"type":"response.completed","response":{"id":"resp_test","usage":{"input_tokens":1,"output_tokens":2}}}`, want: false},
		{name: "completed with text", data: `{"type":"response.completed","response":{"id":"resp_test","output":[{"type":"message","content":[{"type":"output_text","text":"test output"}]}]}}`, want: true},
		{name: "done marker", data: `[DONE]`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, openAIStreamDataStartsVisibleOutput(tt.data, tt.eventType))
		})
	}
}

// 首 token 口径是代码常量 OpenAITTFTMode（visible）：跳过 preamble 与空 reasoning 条目，
// 等到 120ms 后才出现的可见输出才计首 token。
func TestOpenAIResponsesTTFTStartsAtVisibleOutput(t *testing.T) {
	result := runSyntheticVisibleTTFTStream(t, 120*time.Millisecond, 0,
		`{"type":"response.output_text.delta","delta":"test output"}`)
	require.NotNil(t, result.firstTokenMs)
	require.GreaterOrEqual(t, *result.firstTokenMs, 100)
}

func TestOpenAIResponsesTTFTStartsAtCompletedImage(t *testing.T) {
	result := runSyntheticVisibleTTFTStream(t, 120*time.Millisecond, 0,
		`{"type":"response.output_item.done","item":{"id":"item_test","type":"image_generation_call","result":"dGVzdA=="}}`)
	require.NotNil(t, result.firstTokenMs)
	require.GreaterOrEqual(t, *result.firstTokenMs, 100)
}

func TestOpenAINativeMetadataDoesNotDisarmFirstOutputTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		MaxLineSize:                     defaultMaxLineSize,
		OpenAIFirstOutputTimeoutSeconds: 1,
	}}}
	reader, writer := io.Pipe()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer func() { _ = writer.Close() }()
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"item_test\",\"type\":\"reasoning\",\"summary\":[]}}\n\n")
		time.Sleep(1200 * time.Millisecond)
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: reader}
	account := &Account{ID: 1, Name: "account_test", Platform: PlatformOpenAI}

	_, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "test-model", "test-model")
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.SafeToFailoverAfterWrite)
	require.Empty(t, recorder.Body.String())
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("synthetic upstream writer did not exit")
	}
}

// 两种首 token 口径对同一事件的判定（运行时只用 OpenAITTFTMode 那一种；另一种留着是为了改常量时有据可查）：
// semantic 跳过 preamble 后第一个事件就算，visible 要等到有可见内容。
func TestOpenAIStreamDataStartsTTFTByMode(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		forceOutput bool
		semantic    bool
		visible     bool
	}{
		{name: "created is preamble", data: `{"type":"response.created"}`, semantic: false, visible: false},
		{name: "empty reasoning item", data: `{"type":"response.output_item.added","item":{"id":"item_test","type":"reasoning","summary":[]}}`, semantic: true, visible: false},
		{name: "usage-only completed", data: `{"type":"response.completed","response":{"id":"resp_test","usage":{"input_tokens":1,"output_tokens":2}}}`, semantic: true, visible: false},
		{name: "text delta", data: `{"type":"response.output_text.delta","delta":"test output"}`, semantic: true, visible: true},
		{name: "forced failed event", data: `{"type":"response.failed","response":{"id":"resp_test"}}`, forceOutput: true, semantic: true, visible: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.semantic, openAIStreamDataStartsTTFT(tt.data, "", tt.forceOutput, OpenAITTFTModeSemantic), "semantic")
			require.Equal(t, tt.visible, openAIStreamDataStartsTTFT(tt.data, "", tt.forceOutput, OpenAITTFTModeVisible), "visible")
		})
	}
}

func runSyntheticVisibleTTFTStream(t *testing.T, visibleDelay time.Duration, timeoutSeconds int, visibleEvent string) *openaiStreamingResult {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		MaxLineSize:                     defaultMaxLineSize,
		OpenAIFirstOutputTimeoutSeconds: timeoutSeconds,
	}}}
	reader, writer := io.Pipe()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer func() { _ = writer.Close() }()
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"item_test\",\"type\":\"reasoning\",\"summary\":[]}}\n\n")
		time.Sleep(visibleDelay)
		_, _ = io.WriteString(writer, "data: "+visibleEvent+"\n\n")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: reader}
	account := &Account{ID: 1, Name: "account_test", Platform: PlatformOpenAI}
	started := time.Now()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, account, started, "test-model", "test-model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, recorder.Body.String(), `"type":"response.output_item.added"`)
	require.Contains(t, recorder.Body.String(), visibleEvent)
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("synthetic upstream writer did not exit")
	}
	return result
}
