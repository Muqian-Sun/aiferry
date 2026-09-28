package service

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpenAIImageIntentHintTestContext(transport OpenAIClientTransport) *gin.Context {
	c := &gin.Context{}
	SetOpenAIClientTransport(c, transport)
	return c
}

func countingOpenAIImageIntentClassifier(calls *atomic.Int64) openAIImageIntentClassifier {
	return func(endpoint string, requestedModel string, body []byte) bool {
		calls.Add(1)
		return IsImageGenerationIntent(endpoint, requestedModel, body)
	}
}

func TestResolveOpenAIImageIntentHintCachesTrueAndFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body []byte
		want bool
	}{
		{name: "true", body: []byte(`{"model":"gpt-5.4","tools":[{"type":"image_generation"}]}`), want: true},
		{name: "false is known", body: []byte(`{"model":"gpt-5.4","input":"write code"}`), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newOpenAIImageIntentHintTestContext(OpenAIClientTransportHTTP)
			var calls atomic.Int64
			classify := countingOpenAIImageIntentClassifier(&calls)

			require.Equal(t, tt.want, resolveOpenAIImageIntentHint(c, "gpt-5.4", tt.body, classify))
			require.Equal(t, tt.want, resolveOpenAIImageIntentHint(c, "gpt-5.4", tt.body, classify))
			require.Equal(t, int64(1), calls.Load())
			cached, known := getOpenAIImageIntentHint(c)
			require.True(t, known)
			require.Equal(t, tt.want, cached)
		})
	}
}

func TestResolveOpenAIImageIntentHintUsesHandlerSeed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, seeded := range []bool{false, true} {
		c := newOpenAIImageIntentHintTestContext(OpenAIClientTransportHTTP)
		SetOpenAIImageIntentHint(c, seeded)
		var calls atomic.Int64

		got := resolveOpenAIImageIntentHint(c, "gpt-5.4", []byte(`{"model":"gpt-5.4"}`), countingOpenAIImageIntentClassifier(&calls))

		require.Equal(t, seeded, got)
		require.Zero(t, calls.Load())
	}
}

func TestResolveOpenAIImageIntentHintExcludesWebSocketAndUnknownTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, transport := range []OpenAIClientTransport{OpenAIClientTransportWS, OpenAIClientTransportUnknown} {
		c := newOpenAIImageIntentHintTestContext(transport)
		var calls atomic.Int64
		classify := countingOpenAIImageIntentClassifier(&calls)
		body := []byte(`{"model":"gpt-5.4","input":"write code"}`)

		require.False(t, resolveOpenAIImageIntentHint(c, "gpt-5.4", body, classify))
		require.False(t, resolveOpenAIImageIntentHint(c, "gpt-5.4", body, classify))
		require.Equal(t, int64(2), calls.Load())
		_, known := getOpenAIImageIntentHint(c)
		require.False(t, known)
	}
}

func TestResolveOpenAIImageIntentHintConcurrentRequestsAreIsolated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const requests = 32
	var calls atomic.Int64
	classify := countingOpenAIImageIntentClassifier(&calls)
	var wg sync.WaitGroup
	results := make([][2]bool, requests)

	for i := range requests {
		wg.Add(1)
		go func(index int, image bool) {
			defer wg.Done()
			c := newOpenAIImageIntentHintTestContext(OpenAIClientTransportHTTP)
			body := []byte(`{"model":"gpt-5.4","input":"write code"}`)
			if image {
				body = []byte(`{"model":"gpt-5.4","tools":[{"type":"image_generation"}]}`)
			}
			results[index][0] = resolveOpenAIImageIntentHint(c, "gpt-5.4", body, classify)
			results[index][1] = resolveOpenAIImageIntentHint(c, "gpt-5.4", body, classify)
		}(i, i%2 == 0)
	}
	wg.Wait()
	for i, result := range results {
		require.Equal(t, i%2 == 0, result[0])
		require.Equal(t, result[0], result[1])
	}
	require.Equal(t, int64(requests), calls.Load())
}
