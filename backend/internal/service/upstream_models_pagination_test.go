//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 模型列表翻页（2026-10-06：建渠道时探测名单不全）：Anthropic /v1/models 默认一页 20 个、Gemini 默认 50 个，
// 按上一页的分页标记接着取，全部合起来。

func pagedModelsService(bodies ...string) (*AccountTestService, *httpUpstreamRecorder) {
	upstream := &httpUpstreamRecorder{}
	for _, body := range bodies {
		upstream.responses = append(upstream.responses, &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		})
	}
	return &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}, upstream
}

func anthropicModelsPage(t *testing.T, ids []string, hasMore bool) string {
	t.Helper()
	data := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		data = append(data, map[string]string{"id": id, "type": "model"})
	}
	page := map[string]any{"data": data, "has_more": hasMore}
	if len(ids) > 0 {
		page["first_id"], page["last_id"] = ids[0], ids[len(ids)-1]
	}
	raw, err := json.Marshal(page)
	require.NoError(t, err)
	return string(raw)
}

func modelIDs(prefix string, from, to int) []string {
	out := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, fmt.Sprintf("%s-%02d", prefix, i))
	}
	return out
}

func TestUpstreamModelList_FollowsAnthropicPagination(t *testing.T) {
	first, second := modelIDs("claude", 0, 20), modelIDs("claude", 20, 25)
	svc, upstream := pagedModelsService(anthropicModelsPage(t, first, true), anthropicModelsPage(t, second, false))
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://relay.example"})

	models, err := svc.ProbeUpstreamModels(context.Background(), account)
	require.NoError(t, err)
	require.Len(t, models, 25, "both pages")
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://relay.example/v1/models", upstream.requests[0].URL.String(), "first request unchanged")
	next := upstream.requests[1].URL.Query()
	require.Equal(t, "claude-19", next.Get("after_id"))
	require.Equal(t, "1000", next.Get("limit"))
	require.Equal(t, "sk-key", upstream.requests[1].Header.Get("x-api-key"), "next page keeps auth headers")
}

func TestUpstreamModelList_FollowsGeminiPagination(t *testing.T) {
	svc, upstream := pagedModelsService(
		`{"models":[{"name":"models/gemini-a"},{"name":"models/gemini-b"}],"nextPageToken":"tok-1"}`,
		`{"models":[{"name":"models/gemini-c"}]}`,
	)
	account := keyModelSyncAccount(PlatformGemini, map[string]string{APIProtocolGemini: "https://relay.example"})

	models, err := svc.ProbeUpstreamModels(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []string{"gemini-a", "gemini-b", "gemini-c"}, models)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "tok-1", upstream.requests[1].URL.Query().Get("pageToken"))
	require.Equal(t, "1000", upstream.requests[1].URL.Query().Get("pageSize"))
}

// 「同步上游模型」从多页合起来的原文里抽元数据：后面几页的模型也在。
func TestUpstreamModelList_MergedBodyCoversAllPages(t *testing.T) {
	svc, _ := pagedModelsService(anthropicModelsPage(t, []string{"claude-a"}, true), anthropicModelsPage(t, []string{"claude-b"}, false))
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://relay.example"})

	models, body, err := svc.fetchUpstreamModelList(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-a", "claude-b"}, models)
	ids, err := extractUpstreamModelIDs(body)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-a", "claude-b"}, ids)
}

// 分页标记不前进（has_more 却给同一个 last_id / 同一个 pageToken）按失败报，不死循环、不静默截断。
func TestUpstreamModelList_StuckPaginationFails(t *testing.T) {
	page := anthropicModelsPage(t, []string{"claude-a"}, true)
	svc, upstream := pagedModelsService(page, page)
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://relay.example"})

	_, err := svc.ProbeUpstreamModels(context.Background(), account)
	require.ErrorContains(t, err, "pagination")
	require.Len(t, upstream.requests, 2)

	svc, _ = pagedModelsService(`{"models":[{"name":"models/a"}],"nextPageToken":"t"}`, `{"models":[{"name":"models/b"}],"nextPageToken":"t"}`)
	_, err = svc.ProbeUpstreamModels(context.Background(), keyModelSyncAccount(PlatformGemini, map[string]string{APIProtocolGemini: "https://relay.example"}))
	require.ErrorContains(t, err, "pagination")
}

// 翻页有上限：超过 maxUpstreamModelPages 页按失败报。
func TestUpstreamModelList_TooManyPagesFails(t *testing.T) {
	bodies := make([]string, 0, maxUpstreamModelPages+1)
	for i := 0; i <= maxUpstreamModelPages; i++ {
		bodies = append(bodies, anthropicModelsPage(t, []string{fmt.Sprintf("m-%03d", i)}, true))
	}
	svc, upstream := pagedModelsService(bodies...)
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://relay.example"})

	_, err := svc.ProbeUpstreamModels(context.Background(), account)
	require.ErrorContains(t, err, "too many pages")
	require.Len(t, upstream.requests, maxUpstreamModelPages)
}

// 不分页的 OpenAI 兼容列表：一次请求，原文原样返回。
func TestUpstreamModelList_UnpagedListIsSingleRequest(t *testing.T) {
	body := `{"object":"list","data":[{"id":"gpt-5.5"},{"id":"gpt-6-astra"}]}`
	svc, upstream := pagedModelsService(body)
	account := keyModelSyncAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"})

	models, raw, err := svc.fetchUpstreamModelList(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.5", "gpt-6-astra"}, models)
	require.Equal(t, body, string(raw))
	require.Len(t, upstream.requests, 1)
}
