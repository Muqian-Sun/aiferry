//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type schedulingTraceRepoStub struct {
	mu       sync.Mutex
	inserted []service.SchedulingTrace
}

func (s *schedulingTraceRepoStub) InsertSchedulingTraces(_ context.Context, traces []service.SchedulingTrace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inserted = append(s.inserted, traces...)
	return nil
}

func (s *schedulingTraceRepoStub) DeleteSchedulingTracesBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *schedulingTraceRepoStub) ListSchedulingTraces(context.Context, service.SchedulingTraceFilter) ([]service.SchedulingTrace, int64, error) {
	return nil, 0, nil
}

func (s *schedulingTraceRepoStub) CountSchedulingTraces(context.Context, time.Time) ([]service.SchedulingTraceCount, error) {
	return nil, nil
}

// 真实的 Messages 处理走一遍：中间件合成的轨迹带上请求、用户、条目、协议与调度器记下的挑选；
// 计 token 的路径同样会选渠道，但不记。
func TestSchedulingTraceMiddleware_RecordsGatewayRequest(t *testing.T) {
	const entryID = 7
	key := keyRouteAccount(1103, service.PlatformOpenAI,
		map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "gemini-2.5-flash")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})
	repo := &schedulingTraceRepoStub{}
	writer := service.NewSchedulingTraceWriter(repo)

	prepare := func(c *gin.Context) {
		entry := &service.ModelCatalogEntry{ID: entryID, ModelID: "gemini-2.5-flash", Vendor: "gemini", Status: service.ModelCatalogStatusListed}
		ctx := service.WithCatalogRoute(c.Request.Context(), service.CatalogRoute{EntryID: entryID, CanonicalModel: entry.ModelID, RequestedModel: entry.ModelID, Entry: entry})
		ctx = service.WithInboundProtocol(ctx, service.APIProtocolAnthropic)
		ctx = context.WithValue(ctx, ctxkey.RequestID, "req-trace-1")
		c.Request = c.Request.WithContext(ctx)
		apiKey := &service.APIKey{ID: 3101, UserID: 4101, Status: service.StatusActive, User: &service.User{ID: 4101, Concurrency: 10, Balance: 100}}
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
		c.Next()
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/v1/messages", prepare, SchedulingTraceMiddleware(writer), hs.handler.Messages)
	engine.POST("/v1/messages/count_tokens", prepare, SchedulingTraceMiddleware(writer), hs.handler.Messages)

	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	require.Len(t, hs.geminiUpstream.recorded(), 2, "两条路径都真的选了渠道并转发")
	writer.Stop()

	require.Len(t, repo.inserted, 1, "计 token 的请求不记")
	trace := repo.inserted[0]
	require.Equal(t, "req-trace-1", trace.RequestID)
	require.Equal(t, int64(4101), *trace.UserID)
	require.Equal(t, int64(3101), *trace.APIKeyID)
	require.Equal(t, int64(entryID), *trace.CatalogEntryID)
	require.Equal(t, "gemini-2.5-flash", trace.Model)
	require.Equal(t, service.APIProtocolAnthropic, trace.InboundProtocol)
	require.Equal(t, service.SchedulingTraceOutcomeSuccess, trace.Outcome)
	require.Equal(t, http.StatusOK, trace.StatusCode)
	require.Equal(t, key.ID, *trace.FinalAccountID)
	require.Zero(t, trace.SwitchCount)
	require.Len(t, trace.Picks, 1)
	require.Equal(t, key.ID, trace.Picks[0].AccountID)
	require.Empty(t, trace.Attempts)
}
