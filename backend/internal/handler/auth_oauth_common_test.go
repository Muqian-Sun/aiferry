package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSanitizeFrontendRedirectPath(t *testing.T) {
	require.Equal(t, "/dashboard", sanitizeFrontendRedirectPath("/dashboard"))
	require.Equal(t, "/dashboard", sanitizeFrontendRedirectPath(" /dashboard "))
	require.Equal(t, "", sanitizeFrontendRedirectPath("dashboard"))
	require.Equal(t, "", sanitizeFrontendRedirectPath("//evil.com"))
	require.Equal(t, "", sanitizeFrontendRedirectPath("https://evil.com"))
	require.Equal(t, "", sanitizeFrontendRedirectPath("/\nfoo"))

	long := "/" + strings.Repeat("a", oauthMaxRedirectLen)
	require.Equal(t, "", sanitizeFrontendRedirectPath(long))
}

func TestSingleLineStripsWhitespace(t *testing.T) {
	require.Equal(t, "hello world", singleLine("hello\r\nworld"))
	require.Equal(t, "", singleLine("\n\t\r"))
}

func TestPrepareOAuthBindAccessTokenCookieSetsHttpOnlyCookie(t *testing.T) {
	handler, client := newOAuthPendingFlowTestHandler(t, false)
	t.Cleanup(func() { _ = client.Close() })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/bind-token", nil)
	req.Header.Set("Authorization", "Bearer access-token-value")
	c.Request = req

	handler.PrepareOAuthBindAccessTokenCookie(c)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	accessTokenCookie := findCookie(recorder.Result().Cookies(), oauthBindAccessTokenCookieName)
	require.NotNil(t, accessTokenCookie)
	require.Equal(t, oauthBindAccessTokenCookiePath, accessTokenCookie.Path)
	require.Equal(t, oauthCookieMaxAgeSec, accessTokenCookie.MaxAge)
	require.True(t, accessTokenCookie.HttpOnly)
	require.Equal(t, url.QueryEscape("access-token-value"), accessTokenCookie.Value)
}
