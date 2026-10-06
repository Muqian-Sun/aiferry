package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 通义（DashScope）OpenAI 兼容接口的显式缓存：写入数在 prompt_tokens_details.cache_creation_input_tokens，
// 计在 prompt_tokens 里——要认出来，按缓存写价收，不能混在普通输入里按输入价收。
func TestOpenAIUsageFromGJSON_DashScopeCacheCreation(t *testing.T) {
	usage, ok := openAIUsageFromGJSON(gjson.Parse(`{"prompt_tokens":10000,"completion_tokens":100,"total_tokens":10100,
		"prompt_tokens_details":{"cached_tokens":5000,"cache_creation_input_tokens":2000,"cache_type":"ephemeral"}}`))
	require.True(t, ok)
	require.Equal(t, 10000, usage.InputTokens)
	require.Equal(t, 5000, usage.CacheReadInputTokens)
	require.Equal(t, 2000, usage.CacheCreationInputTokens)
}
