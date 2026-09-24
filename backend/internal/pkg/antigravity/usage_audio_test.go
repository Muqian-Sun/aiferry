//go:build unit

package antigravity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Gemini 的 AUDIO 模态计在 promptTokenCount / candidatesTokenCount 之内：两条转换路径都把它交给计费
// （输入侧扣掉缓存命中的音频），但不写进返回给客户端的 Claude usage。
func TestGeminiUsageMapping_AudioModalityForBillingOnly(t *testing.T) {
	const geminiBody = `{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],` +
		`"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":300,"cachedContentTokenCount":200,` +
		`"promptTokensDetails":[{"modality":"TEXT","tokenCount":400},{"modality":"AUDIO","tokenCount":600}],` +
		`"cacheTokensDetails":[{"modality":"AUDIO","tokenCount":150}],` +
		`"candidatesTokensDetails":[{"modality":"AUDIO","tokenCount":120},{"modality":"TEXT","tokenCount":180}]}}`

	t.Run("non-stream", func(t *testing.T) {
		body, usage, err := TransformGeminiToClaude([]byte(geminiBody), "gemini-2.5-flash")
		require.NoError(t, err)
		require.NotNil(t, usage)
		require.Equal(t, 800, usage.InputTokens)
		require.Equal(t, 450, usage.AudioInputTokens)
		require.Equal(t, 120, usage.AudioOutputTokens)
		require.NotContains(t, string(body), "audio_")
	})

	t.Run("stream", func(t *testing.T) {
		p := NewStreamingProcessor("gemini-2.5-flash")
		out := p.ProcessLine(`data: {"response":` + geminiBody + `}`)
		require.NotContains(t, string(out), "audio_")
		final, usage := p.Finish()
		require.NotContains(t, string(final), "audio_")
		require.NotNil(t, usage)
		require.Equal(t, 450, usage.AudioInputTokens)
		require.Equal(t, 120, usage.AudioOutputTokens)
	})
}

func TestGeminiUsageMetadata_AudioInputClampedToUncachedInput(t *testing.T) {
	m := &GeminiUsageMetadata{
		PromptTokenCount:        500,
		CachedContentTokenCount: 400,
		PromptTokensDetails:     []GeminiTokenDetail{{Modality: "AUDIO", TokenCount: 450}},
	}
	require.Equal(t, 100, m.AudioInputTokens())
	require.Zero(t, m.AudioOutputTokens())
}
