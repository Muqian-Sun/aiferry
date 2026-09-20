package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// catalogStub 按名字表解析：值是条目 ID 与网关族；别名映射到同一条目。
type catalogStub struct {
	routes map[string]service.CatalogRoute
}

func (s catalogStub) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	route, ok := s.routes[strings.ToLower(strings.TrimSpace(model))]
	if !ok {
		return service.CatalogRoute{}, false
	}
	route.RequestedModel = strings.TrimSpace(model)
	return route, true
}

func newCatalogStub() catalogStub {
	sonnet := service.CatalogRoute{EntryID: 1, CanonicalModel: "claude-sonnet-4", Platform: service.PlatformAnthropic}
	gpt := service.CatalogRoute{EntryID: 2, CanonicalModel: "gpt-5.6", Platform: service.PlatformOpenAI}
	gemini := service.CatalogRoute{EntryID: 3, CanonicalModel: "gemini-2.5-pro", Platform: service.PlatformGemini}
	return catalogStub{routes: map[string]service.CatalogRoute{
		"claude-sonnet-4": sonnet,
		"sonnet-latest":   sonnet,
		"gpt-5.6":         gpt,
		"gemini-2.5-pro":  gemini,
	}}
}

type catalogAdmissionSeen struct {
	route service.CatalogRoute
	ok    bool
	calls int
}

func newCatalogAdmissionTestRouter(pathPrefix string, requirePlatforms ...string) (*gin.Engine, *catalogAdmissionSeen) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	seen := &catalogAdmissionSeen{}
	router.Use(CatalogAdmission(newCatalogStub(), requirePlatforms...))
	register := func(method, path string) {
		router.Handle(method, path, func(c *gin.Context) {
			seen.calls++
			seen.route, seen.ok = service.CatalogRouteFromContext(c.Request.Context())
			c.Status(http.StatusOK)
		})
	}
	register(http.MethodPost, pathPrefix+"/messages")
	register(http.MethodPost, pathPrefix+"/responses")
	register(http.MethodGet, pathPrefix+"/responses")
	register(http.MethodPost, pathPrefix+"/chat/completions")
	register(http.MethodGet, pathPrefix+"/models")
	register(http.MethodGet, pathPrefix+"/models/:model")
	register(http.MethodPost, pathPrefix+"/models/*modelAction")
	register(http.MethodGet, pathPrefix+"/realtime")
	return router, seen
}

func TestCatalogAdmission_ListedModelPassesWithRoute(t *testing.T) {
	router, seen := newCatalogAdmissionTestRouter("/v1")
	w := doJSON(t, router, http.MethodPost, "/v1/messages", `{"model":"sonnet-latest"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, seen.ok, "route is attached to the request context")
	require.Equal(t, int64(1), seen.route.EntryID)
	require.Equal(t, "claude-sonnet-4", seen.route.CanonicalModel, "alias resolves to the canonical model")
	require.Equal(t, "sonnet-latest", seen.route.RequestedModel)
	require.Equal(t, service.PlatformAnthropic, seen.route.Platform)
}

func TestCatalogAdmission_UnlistedModelIsRejectedPerProtocol(t *testing.T) {
	cases := []struct {
		name, path, body, want string
	}{
		{"anthropic format", "/v1/messages", `{"model":"claude-unknown"}`, `"type":"not_found_error"`},
		{"openai format", "/v1/chat/completions", `{"model":"claude-unknown"}`, `"type":"invalid_request_error"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			var rejected IngressRejectReason
			var limited bool
			router.Use(func(c *gin.Context) {
				c.Next()
				rejected, _ = GetIngressRejectReason(c)
				limited = service.HasOpsClientBusinessLimited(c)
			})
			router.Use(CatalogAdmission(newCatalogStub()))
			router.POST(tc.path, func(c *gin.Context) { c.Status(http.StatusOK) })

			w := doJSON(t, router, http.MethodPost, tc.path, tc.body)
			require.Equal(t, http.StatusNotFound, w.Code)
			require.Contains(t, w.Body.String(), tc.want)
			require.Contains(t, w.Body.String(), `claude-unknown`)
			require.Equal(t, IngressRejectModelNotListed, rejected)
			require.True(t, limited)
		})
	}

	t.Run("google format from path param", func(t *testing.T) {
		router, seen := newCatalogAdmissionTestRouter("/v1beta", service.PlatformGemini)
		w := doJSON(t, router, http.MethodPost, "/v1beta/models/gemini-unknown:generateContent", `{}`)
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Contains(t, w.Body.String(), `"status":"NOT_FOUND"`)
		require.Zero(t, seen.calls)
	})
}

func TestCatalogAdmission_PathParamAndQueryExtraction(t *testing.T) {
	router, seen := newCatalogAdmissionTestRouter("/v1beta", service.PlatformGemini)
	w := doJSON(t, router, http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", `{}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, int64(3), seen.route.EntryID)

	router, seen = newCatalogAdmissionTestRouter("/v1")
	req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=gpt-5.6", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(2), seen.route.EntryID)

	req = httptest.NewRequest(http.MethodGet, "/v1/models/gpt-5.6", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(2), seen.route.EntryID)
}

func TestCatalogAdmission_ModelFreeRequestsPass(t *testing.T) {
	router, seen := newCatalogAdmissionTestRouter("/v1")
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, seen.ok, "no route without a model")

	w = doJSON(t, router, http.MethodPost, "/v1/messages", `{"messages":[]}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, seen.ok)
}

func TestCatalogAdmission_SkipsResponsesWebSocketUpgrade(t *testing.T) {
	router, seen := newCatalogAdmissionTestRouter("/v1")
	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, seen.calls)
	require.False(t, seen.ok)
}

func TestCatalogAdmission_RequirePlatforms(t *testing.T) {
	router, seen := newCatalogAdmissionTestRouter("/v1beta", service.PlatformGemini)
	w := doJSON(t, router, http.MethodPost, "/v1beta/models/gpt-5.6:generateContent", `{}`)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not available on this endpoint")
	require.Zero(t, seen.calls)

	router, seen = newCatalogAdmissionTestRouter("/antigravity/v1", service.PlatformAnthropic, service.PlatformGemini)
	w = doJSON(t, router, http.MethodPost, "/antigravity/v1/messages", `{"model":"claude-sonnet-4"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(1), seen.route.EntryID)
	w = doJSON(t, router, http.MethodPost, "/antigravity/v1/messages", `{"model":"gpt-5.6"}`)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCatalogAdmission_ConflictingCandidatesAreRejected(t *testing.T) {
	router, seen := newCatalogAdmissionTestRouter("/v1")
	// gjson 取首个键、encoding/json 取末值：两个解析器会绑定到不同条目，必须拒绝。
	w := doJSON(t, router, http.MethodPost, "/v1/chat/completions", `{"model":"gpt-5.6","model":"claude-sonnet-4"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Zero(t, seen.calls)

	// 同一条目的别名与本名并存则放行。
	w = doJSON(t, router, http.MethodPost, "/v1/chat/completions", `{"model":"claude-sonnet-4","model":"sonnet-latest"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, int64(1), seen.route.EntryID)
}

func TestCatalogAdmission_BodyIsReplayedToHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CatalogAdmission(newCatalogStub()))
	var got string
	router.POST("/v1/messages", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		got = string(body)
		c.Status(http.StatusOK)
	})
	body := `{"model":"gpt-5.6","messages":[{"role":"user","content":"hi"}]}`
	w := doJSON(t, router, http.MethodPost, "/v1/messages", body)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, body, got)
}
