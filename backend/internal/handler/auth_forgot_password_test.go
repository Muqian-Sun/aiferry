//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 部署没配 SERVER_FRONTEND_URL 时找回密码发不出链接：响应要带 reason，前端靠它显示中文原因
func TestForgotPasswordWithoutFrontendURLReportsReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newOAuthCaptchaTestHandler(false)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(`{"email":"user@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ForgotPassword(c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var body struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "PASSWORD_RESET_NOT_CONFIGURED", body.Reason)
}
