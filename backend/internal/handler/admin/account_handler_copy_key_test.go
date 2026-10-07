//go:build unit

package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 复制渠道：copy_key_from_account_id 原样交给服务层（key 留空时由后端从源渠道取）。
func TestAccountHandlerCreatePassesCopyKeySource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newStubAdminService()
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts", handler.Create)

	body := `{"name":"fenno 2","type":"apikey","credentials":{},"protocol_endpoints":{"chat_completions":"https://relay.example.com"},"copy_key_from_account_id":42}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, svc.createdAccounts, 1)
	require.Equal(t, int64(42), svc.createdAccounts[0].CopyKeyFromAccountID)
}
