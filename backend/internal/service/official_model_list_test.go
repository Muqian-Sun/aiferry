//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 官方模型名单（2026-10-06）：联网的 LiteLLM 公开价格表，只认官方厂商的条目，按去掉厂商前缀后的精确 ID 比对。

const officialModelListFixture = `{
  "sample_spec": {"litellm_provider": "openai", "input_cost_per_token": 0},
  "gpt-5.3-codex": {"litellm_provider": "openai", "mode": "responses", "input_cost_per_token": 1.75e-06, "output_cost_per_token": 1.4e-05},
  "zai/glm-4.6": {"litellm_provider": "zai", "mode": "chat", "input_cost_per_token": 6e-07, "output_cost_per_token": 2.2e-06},
  "gemini-2.5-pro": {"litellm_provider": "gemini", "mode": "chat", "input_cost_per_token": 1.25e-06, "output_cost_per_token": 1e-05},
  "gemini/gemini-2.5-pro": {"litellm_provider": "gemini", "mode": "chat", "input_cost_per_token": 9e-06, "output_cost_per_token": 9e-05},
  "xai/grok-4.3": {"litellm_provider": "xai", "mode": "chat", "input_cost_per_token": 3e-06, "output_cost_per_token": 1.5e-05},
  "openrouter/anthropic/claude-sonnet-4.5": {"litellm_provider": "openrouter", "mode": "chat", "input_cost_per_token": 3e-06, "output_cost_per_token": 1.5e-05},
  "azure/gpt-5": {"litellm_provider": "azure", "mode": "chat", "input_cost_per_token": 1.25e-06, "output_cost_per_token": 1e-05},
  "low/1024-x-1024/gpt-image-1.5": {"litellm_provider": "openai", "mode": "image_generation", "output_cost_per_image": 0.01},
  "deepseek-chat": {"litellm_provider": "deepseek", "mode": "chat"},
  "dashscope/qwen3-max": {"litellm_provider": "dashscope", "mode": "chat",
    "tiered_pricing": [{"input_cost_per_token": 1.2e-06, "output_cost_per_token": 6e-06, "range": [0, 32000]}]}
}`

func TestParseOfficialModelList(t *testing.T) {
	index, err := parseOfficialModelList([]byte(officialModelListFixture))
	require.NoError(t, err)

	codex, ok := index["gpt-5.3-codex"]
	require.True(t, ok)
	require.True(t, codex.Priced)
	require.Equal(t, "openai", codex.Entry.Vendor)
	require.NotNil(t, codex.Entry.InputPrice)
	require.InDelta(t, 1.75e-06, *codex.Entry.InputPrice, 1e-12, "官方价带进条目，建目录时预填")

	glm, ok := index["glm-4.6"]
	require.True(t, ok, "去掉和提供方同名的前缀")
	require.Equal(t, "zhipu", glm.Entry.Vendor, "智谱在目录里叫 zhipu")
	require.Equal(t, "glm-4.6", glm.Entry.ModelID)

	gemini := index["gemini-2.5-pro"]
	require.InDelta(t, 1.25e-06, *gemini.Entry.InputPrice, 1e-12, "同一个 ID 有带前缀与不带前缀两条时用不带前缀的")

	require.Contains(t, index, "grok-4.3")
	for _, unpriced := range []string{"deepseek-chat", "qwen3-max"} {
		model, ok := index[unpriced]
		require.True(t, ok, "%s：读不出单一价格也是官方 ID", unpriced)
		require.False(t, model.Priced, unpriced)
		require.Nil(t, model.Entry.InputPrice, unpriced)
		require.NotEmpty(t, model.Entry.Vendor, unpriced)
	}
	for _, notOfficial := range []string{"claude-sonnet-4.5", "anthropic/claude-sonnet-4.5", "gpt-5", "gpt-image-1.5", "sample_spec"} {
		require.NotContains(t, index, notOfficial, "%s：转售方的条目、非提供方前缀、文档条目都不收", notOfficial)
	}
}

// officialListServer 计数的假名单服务；status 非 200 时回错误。
type officialListServer struct {
	hits   atomic.Int32
	status atomic.Int32
}

func newOfficialListServer(t *testing.T) (*officialListServer, *httptest.Server) {
	t.Helper()
	s := &officialListServer{}
	s.status.Store(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.hits.Add(1)
		if code := int(s.status.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		_, _ = w.Write([]byte(officialModelListFixture))
	}))
	t.Cleanup(srv.Close)
	return s, srv
}

func newTestOfficialModelList(url string, clock *time.Time) *officialModelList {
	l := newOfficialModelList()
	l.url = url
	l.now = func() time.Time { return *clock }
	return l
}

func TestOfficialModelList_CachesAndFallsBack(t *testing.T) {
	server, srv := newOfficialListServer(t)
	clock := time.Date(2026, 10, 6, 3, 0, 0, 0, time.UTC)
	list := newTestOfficialModelList(srv.URL, &clock)
	ctx := context.Background()

	_, ok, err := list.lookup(ctx, "GPT-5.3-Codex")
	require.NoError(t, err)
	require.True(t, ok, "不分大小写")
	_, _, _ = list.lookup(ctx, "glm-4.6")
	require.Equal(t, int32(1), server.hits.Load(), "缓存期内只拉一次")

	clock = clock.Add(officialModelListTTL + time.Second)
	server.status.Store(http.StatusBadGateway)
	_, ok, err = list.lookup(ctx, "gpt-5.3-codex")
	require.NoError(t, err)
	require.True(t, ok, "过期后拉取失败：沿用上一份")
	require.Equal(t, int32(2), server.hits.Load())

	_, _, _ = list.lookup(ctx, "gpt-5.3-codex")
	require.Equal(t, int32(2), server.hits.Load(), "失败后 officialModelListRetryAfter 内不再去撞")

	clock = clock.Add(officialModelListRetryAfter + time.Second)
	server.status.Store(http.StatusOK)
	_, _, _ = list.lookup(ctx, "gpt-5.3-codex")
	require.Equal(t, int32(3), server.hits.Load(), "过了重试间隔再拉，成功后换新")
}

func TestOfficialModelList_UnavailableWithoutAnyCopy(t *testing.T) {
	server, srv := newOfficialListServer(t)
	server.status.Store(http.StatusInternalServerError)
	clock := time.Date(2026, 10, 6, 3, 0, 0, 0, time.UTC)
	list := newTestOfficialModelList(srv.URL, &clock)

	_, ok, err := list.lookup(context.Background(), "gpt-5.3-codex")
	require.False(t, ok)
	require.True(t, errors.Is(err, ErrOfficialModelListUnavailable))
}

func TestLookupOfficialModels(t *testing.T) {
	_, srv := newOfficialListServer(t)
	clock := time.Date(2026, 10, 6, 3, 0, 0, 0, time.UTC)
	svc := &ModelCatalogService{officialModels: newTestOfficialModelList(srv.URL, &clock)}

	got := svc.LookupOfficialModels(context.Background(), []string{"gpt-5.3-codex", " glm-4.6 ", "GPT-5.3-CODEX", "gpt-5.3-codex-spark", "zai/glm-4.6", ""})
	require.True(t, got.Available)
	require.Len(t, got.Models, 4, "去空、按小写去重")
	require.True(t, got.Models[0].Official)
	require.True(t, got.Models[0].Priced)
	require.Equal(t, "gpt-5.3-codex", got.Models[0].Entry.ModelID)
	require.True(t, got.Models[1].Official)
	require.Equal(t, "glm-4.6", got.Models[1].ModelID)
	require.Equal(t, "zhipu", got.Models[1].Entry.Vendor)
	require.False(t, got.Models[2].Official, "表里没有的不算官方")
	require.Nil(t, got.Models[2].Entry)
	require.False(t, got.Models[3].Official, "带厂商前缀的写法不是官方 ID，该映射到目录里已有的模型")

	offline := &ModelCatalogService{}
	got = offline.LookupOfficialModels(context.Background(), []string{"gpt-5.3-codex"})
	require.False(t, got.Available, "没有联网名单：报不可用，由管理员判断")
	require.False(t, got.Models[0].Official)
}

// 「添加模型」预填：内置价格资料查不到时用联网名单的官方价。
func TestLookupPriceEntry_FallsBackToOfficialList(t *testing.T) {
	_, srv := newOfficialListServer(t)
	clock := time.Date(2026, 10, 6, 3, 0, 0, 0, time.UTC)
	svc := &ModelCatalogService{officialModels: newTestOfficialModelList(srv.URL, &clock)}

	entry, ok := svc.LookupPriceEntry(context.Background(), "glm-4.6")
	require.True(t, ok)
	require.Equal(t, "glm-4.6", entry.ModelID)
	require.InDelta(t, 6e-07, *entry.InputPrice, 1e-12)

	_, ok = svc.LookupPriceEntry(context.Background(), "gpt-5.3-codex-spark")
	require.False(t, ok)
	_, ok = svc.LookupPriceEntry(context.Background(), "qwen3-max")
	require.False(t, ok, "官方但读不出价格：「添加模型」不预填")
}
