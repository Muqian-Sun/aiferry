package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientRequestIDGeneratesAndExposesID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Body.String())
	require.Equal(t, w.Body.String(), w.Header().Get(clientRequestIDHeader))
}

func TestClientRequestIDBoundsExistingContextID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, strings.Repeat("x", 200)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Len(t, w.Body.String(), 36)
	require.NotEqual(t, strings.Repeat("x", maxPersistentRequestIDBytes), w.Body.String())
	require.Equal(t, w.Body.String(), w.Header().Get(clientRequestIDHeader))
}

func TestClientRequestIDPreservesExistingContextID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "existing-client-request-id"))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "existing-client-request-id", w.Body.String())
	require.Equal(t, "existing-client-request-id", w.Header().Get(clientRequestIDHeader))
}

// 客户端没带自己的 ID 时沿用本次的 X-Request-ID：用量记录的请求 ID 取它，用户在用量明细里看到的和响应头一致
// （2026-10-04 D5 / U15：原来另起一个 UUID，用量里的 ID 与 X-Request-ID 对不上）
func TestClientRequestIDReusesRequestIDWhenClientSendsNone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestLogger(), ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	require.NotEmpty(t, w.Header().Get(requestIDHeader))
	require.Equal(t, w.Header().Get(requestIDHeader), w.Body.String())
	require.Equal(t, w.Header().Get(requestIDHeader), w.Header().Get(clientRequestIDHeader))
}
