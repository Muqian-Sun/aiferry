//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Claude Code 版本上下限写在代码里（service.MinClaudeCodeVersion / MaxClaudeCodeVersion，都为空 = 不限）：
// 多旧多新的 Claude Code 客户端都放行。
func TestCheckClaudeCodeVersion_CodeBoundsAllowAnyVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GatewayHandler{}
	for _, version := range []string{"0.0.1", "2.1.81", "99.0.0"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx := service.SetClaudeCodeVersion(service.SetClaudeCodeClient(context.Background(), true), version)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
		require.True(t, h.checkClaudeCodeVersion(c), version)
	}
}
