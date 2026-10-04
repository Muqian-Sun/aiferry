//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newTestModelsRouter 渠道测试的模型下拉：账号从 adminSvc 取，承接关系从目录仓储取。
func newTestModelsRouter(adminSvc service.AdminService, catalog *catalogRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	catalogSvc := service.NewModelCatalogService(catalog, nil, service.ModelCatalogSeedInput{})
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, catalogSvc)
	router.GET("/api/v1/admin/accounts/:id/models", handler.GetAvailableModels)
	return router
}

func getTestModels(t *testing.T, router *gin.Engine, accountID string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/"+accountID+"/models", nil))
	return rec.Code, rec.Body.String()
}

// 下拉只列这个渠道承接的目录模型：值是目录标识（承接上改了上游名也不换），显示名用目录展示名、没填用标识；
// 别的渠道承接的、目录里没人承接的都不列。
func TestAccountHandlerGetAvailableModels_ListsCatalogEntriesBoundToTheAccount(t *testing.T) {
	adminSvc := &availableModelsAdminService{
		stubAdminService: newStubAdminService(),
		account: service.Account{
			ID: 5, Name: "fenno · Messages", Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive,
		},
	}
	catalog := &catalogRepoStub{
		entries: []service.ModelCatalogEntry{
			{ID: 1, ModelID: "gpt-5.5", DisplayName: "GPT-5.5", Status: service.ModelCatalogStatusListed},
			{ID: 2, ModelID: "deepseek-v4.1-flash", DisplayName: "", Status: service.ModelCatalogStatusUnlisted},
			{ID: 3, ModelID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", Status: service.ModelCatalogStatusListed},
			{ID: 4, ModelID: "gpt-5.6-sol", DisplayName: "GPT-5.6 Sol", Status: service.ModelCatalogStatusListed},
		},
		bindings: map[int64][]service.ModelCatalogBinding{
			1: {{EntryID: 1, AccountID: 5}},
			2: {{EntryID: 2, AccountID: 5, UpstreamModel: "deepseek-v4-1-flash"}},
			3: {{EntryID: 3, AccountID: 6}},
		},
	}

	code, body := getTestModels(t, newTestModelsRouter(adminSvc, catalog), "5")

	require.Equal(t, http.StatusOK, code, body)
	var resp struct {
		Data []AccountTestModel `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &resp))
	require.Equal(t, []AccountTestModel{
		{ID: "gpt-5.5", DisplayName: "GPT-5.5"},
		{ID: "deepseek-v4.1-flash", DisplayName: "deepseek-v4.1-flash"},
	}, resp.Data)
}

// 没有承接时不论哪类渠道都返回空数组：不再按平台回退到写死的 Claude / OpenAI / Gemini / xAI 模型表。
func TestAccountHandlerGetAvailableModels_NoBindingsReturnsEmptyListForEveryAccountKind(t *testing.T) {
	accounts := map[string]service.Account{
		"anthropic oauth": {Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth},
		"openai oauth":    {Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth},
		"gemini oauth":    {Platform: service.PlatformGemini, Type: service.AccountTypeOAuth, Credentials: map[string]any{"oauth_type": "google_one"}},
		"grok oauth":      {Platform: service.PlatformGrok, Type: service.AccountTypeOAuth},
		"antigravity":     {Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth},
		"relay key": {
			Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
			Credentials:       map[string]any{"api_key": "sk"},
			ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://relay.example/v1"},
		},
	}
	for name, account := range accounts {
		t.Run(name, func(t *testing.T) {
			account.ID = 9
			account.Name = name
			account.Status = service.StatusActive
			adminSvc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: account}
			catalog := &catalogRepoStub{
				entries:  []service.ModelCatalogEntry{{ID: 1, ModelID: "gpt-5.5"}},
				bindings: map[int64][]service.ModelCatalogBinding{1: {{EntryID: 1, AccountID: 10}}},
			}

			code, body := getTestModels(t, newTestModelsRouter(adminSvc, catalog), "9")

			require.Equal(t, http.StatusOK, code, body)
			require.JSONEq(t, `{"code":0,"message":"success","data":[]}`, body)
		})
	}
}

// missingAccountAdminService 任何账号都查不到。
type missingAccountAdminService struct {
	*stubAdminService
}

func (missingAccountAdminService) GetAccount(context.Context, int64) (*service.Account, error) {
	return nil, service.ErrAccountNotFound
}

// 渠道不存在是 404，不是「没有承接」的空列表。
func TestAccountHandlerGetAvailableModels_UnknownAccountIsNotFound(t *testing.T) {
	catalog := &catalogRepoStub{
		entries:  []service.ModelCatalogEntry{{ID: 1, ModelID: "gpt-5.5"}},
		bindings: map[int64][]service.ModelCatalogBinding{1: {{EntryID: 1, AccountID: 404}}},
	}

	code, body := getTestModels(t, newTestModelsRouter(missingAccountAdminService{newStubAdminService()}, catalog), "404")

	require.Equal(t, http.StatusNotFound, code, body)
}
