//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// D13：鉴权不再装载分组，也不再按分组状态拦截——绑着已删除分组的 key 照样过，
// 下游拿到的 apiKey.Group 恒为 nil。
func TestAPIKeyAuth_NoGroupLoaded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10, Concurrency: 3}
	groupID := int64(31)
	apiKey := &service.APIKey{
		ID:      100,
		UserID:  user.ID,
		Key:     "grouped-deleted",
		Status:  service.StatusActive,
		User:    user,
		GroupID: &groupID,
		// 投影不再带分组；即便 repo 填了一个已删除的分组，中间件也不看它
		Group: &service.Group{ID: groupID, Hydrated: true, Status: "deleted", IsExclusive: true},
	}
	apiKeyRepo := &stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	}

	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, nil, cfg)
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	var seen *service.APIKey
	var rejected bool
	router.GET("/t", func(c *gin.Context) {
		seen, _ = GetAPIKeyFromContext(c)
		_, rejected = GetIngressRejectReason(c)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", apiKey.Key)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, seen)
	require.Equal(t, &groupID, seen.GroupID, "group_id 列值仍带给池选择（7b-2 前）")
	require.Nil(t, seen.Group, "鉴权后 apiKey.Group 恒为 nil")
	require.False(t, rejected, "不得记任何入站拒绝")
}
