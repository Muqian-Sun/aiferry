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

// 只实现 SearchAPIKeys：返回两把同名密钥，分属两个用户。
type searchAPIKeysRepoStub struct {
	service.APIKeyRepository
}

func (s *searchAPIKeysRepoStub) SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]service.APIKey, error) {
	return []service.APIKey{
		{ID: 11, Name: "default", UserID: 1, User: &service.User{ID: 1, Email: "alice@test.com"}},
		{ID: 12, Name: "default", UserID: 2, User: &service.User{ID: 2, Email: "bob@test.com"}},
	}, nil
}

// 管理站选密钥不显示内部 id，同名密钥靠所属用户邮箱区分：接口必须给出 user_email。
func TestAdminUsageSearchAPIKeys_ReturnsOwnerEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeySvc := service.NewAPIKeyService(&searchAPIKeysRepoStub{}, nil, nil, nil)
	handler := NewUsageHandler(nil, apiKeySvc, nil, nil)
	router := gin.New()
	router.GET("/admin/usage/search-api-keys", handler.SearchAPIKeys)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-api-keys?q=default", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			UserEmail string `json:"user_email"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	require.Equal(t, "alice@test.com", resp.Data[0].UserEmail)
	require.Equal(t, "bob@test.com", resp.Data[1].UserEmail)
}
