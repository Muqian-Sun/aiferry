package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestAccountWriteEntriesRejectLegacyUpstreamType 固定：历史类型 upstream 已被迁移并入
// apikey，所有能写入账号类型的管理端入口都必须拒绝它，而不是原样落库成一个没有任何
// 代码路径认领的类型。每个入口先用只差类型的 apikey 请求做对照，证明拒绝来自类型校验。
func TestAccountWriteEntriesRejectLegacyUpstreamType(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   func(accountType string) string
		mount  func(*gin.Engine, *AccountHandler)
		called func(*stubAdminService) bool
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/accounts",
			body: func(accountType string) string {
				return `{"name":"relay","platform":"antigravity","type":"` + accountType + `","credentials":{"api_key":"sk-test"}}`
			},
			mount:  func(router *gin.Engine, handler *AccountHandler) { router.POST("/accounts", handler.Create) },
			called: func(stub *stubAdminService) bool { return len(stub.createdAccounts) > 0 },
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/accounts/1",
			body:   func(accountType string) string { return `{"type":"` + accountType + `"}` },
			mount:  func(router *gin.Engine, handler *AccountHandler) { router.PUT("/accounts/:id", handler.Update) },
			called: func(stub *stubAdminService) bool { return stub.updateAccountCalls > 0 },
		},
		{
			name:   "batch create",
			method: http.MethodPost,
			path:   "/accounts/batch",
			body: func(accountType string) string {
				return `{"accounts":[{"name":"relay","platform":"antigravity","type":"` + accountType + `","credentials":{"api_key":"sk-test"}}]}`
			},
			mount:  func(router *gin.Engine, handler *AccountHandler) { router.POST("/accounts/batch", handler.BatchCreate) },
			called: func(stub *stubAdminService) bool { return len(stub.createdAccounts) > 0 },
		},
	}

	serve := func(t *testing.T, mount func(*gin.Engine, *AccountHandler), method, path, body string) (*httptest.ResponseRecorder, *stubAdminService) {
		t.Helper()
		gin.SetMode(gin.TestMode)
		stub := newStubAdminService()
		handler := NewAccountHandler(stub, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := gin.New()
		mount(router, handler)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		return recorder, stub
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accepted, acceptedStub := serve(t, tt.mount, tt.method, tt.path, tt.body("apikey"))
			require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
			require.True(t, tt.called(acceptedStub))

			rejected, rejectedStub := serve(t, tt.mount, tt.method, tt.path, tt.body("upstream"))
			require.Equal(t, http.StatusBadRequest, rejected.Code, rejected.Body.String())
			require.Contains(t, rejected.Body.String(), "Field validation for 'Type' failed on the 'oneof' tag")
			require.False(t, tt.called(rejectedStub))
		})
	}
}

// TestImportDataRejectsLegacyUpstreamType 覆盖导入入口：旧实例导出的数据里可能带着
// upstream 类型，导入时应逐条报错而不是建出无人认领的账号。
func TestImportDataRejectsLegacyUpstreamType(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	payload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":        "relay",
					"platform":    "antigravity",
					"type":        "upstream",
					"credentials": map[string]any{"api_key": "sk-test"},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Data DataImportResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Data.AccountCreated)
	require.Equal(t, 1, response.Data.AccountFailed)
	require.Len(t, response.Data.Errors, 1)
	require.Equal(t, "account type is invalid: upstream", response.Data.Errors[0].Message)
	require.Empty(t, adminSvc.createdAccounts)
}
