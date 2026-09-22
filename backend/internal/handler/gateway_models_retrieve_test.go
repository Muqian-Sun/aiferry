package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func requestModelForTest(h *GatewayHandler, group *service.Group, modelID, etag string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Request.Header.Set("If-None-Match", etag)
	if modelID != "" {
		c.Params = gin.Params{{Key: "model", Value: modelID}}
	}
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
	h.Models(c)
	return rec
}

func TestRetrieveModelMatchesVisibleCatalogue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini, service.PlatformGrok, service.PlatformComposite} {
		t.Run(platform, func(t *testing.T) {
			group := &service.Group{ID: 71, Platform: platform}
			h := newGatewayModelsHandlerForTest("custom-model", "second-model")
			list := requestModelForTest(h, group, "", "")
			require.Equal(t, http.StatusOK, list.Code, list.Body.String())
			var catalog struct {
				Data []json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(list.Body.Bytes(), &catalog))
			require.Len(t, catalog.Data, 2)
			var model struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(catalog.Data[0], &model))
			retrieved := requestModelForTest(h, group, model.ID, "")
			require.Equal(t, http.StatusOK, retrieved.Code, retrieved.Body.String())
			require.JSONEq(t, string(catalog.Data[0]), retrieved.Body.String())
			require.Equal(t, http.StatusNotFound, requestModelForTest(h, group, "unknown-model", "").Code)
		})
	}
}
