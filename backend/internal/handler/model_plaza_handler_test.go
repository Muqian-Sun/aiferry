//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type plazaSettingRepoStub struct {
	service.SettingRepository
	values map[string]string
}

func (s plazaSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

// plazaCatalogStub 给广场两条上架条目和一条下架条目。
type plazaCatalogStub struct{ listedCatalogStub }

func (s plazaCatalogStub) ListListedEntries(context.Context) []service.ModelCatalogEntry {
	price := 1e-6
	return []service.ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", DisplayName: "Sonnet 4", Vendor: "anthropic", Status: service.ModelCatalogStatusListed, InputPrice: &price},
		{ID: 2, ModelID: "gpt-5.6", DisplayName: "GPT-5.6", Vendor: "openai", Status: service.ModelCatalogStatusListed, InputPrice: &price,
			Aliases:     []service.ModelCatalogAlias{{ID: 10, EntryID: 2, Alias: "gpt-5.6-sol"}},
			TimePricing: &service.ChannelTimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []service.ChannelTimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 1.5}}}},
	}
}

func newPlazaHandlerForTest(values map[string]string) *ModelPlazaHandler {
	return NewModelPlazaHandler(
		service.NewModelPlazaService(plazaCatalogStub{}),
		service.NewSettingService(plazaSettingRepoStub{values: values}, &config.Config{}),
	)
}

func TestModelPlazaHandler_NilSettingServiceFailsClosed404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ModelPlazaHandler{} // settingService == nil → fail-closed
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)

	h.Get(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestModelPlazaHandler_ReturnsListedCatalogModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newPlazaHandlerForTest(map[string]string{
		service.SettingKeyModelPlazaEnabled:     "true",
		service.SettingKeyModelPlazaDescription: "hello",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)

	h.Get(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Data struct {
			Description string            `json:"description"`
			Models      []json.RawMessage `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, "hello", envelope.Data.Description)
	require.Len(t, envelope.Data.Models, 2)
	require.NotContains(t, w.Body.String(), `"groups"`, "the plaza is flat: no groups")

	var gpt map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[1], &gpt))
	require.JSONEq(t, `"gpt-5.6"`, string(gpt["model_id"]))
	require.JSONEq(t, `"GPT-5.6"`, string(gpt["display_name"]))
	require.JSONEq(t, `"openai"`, string(gpt["vendor"]))
	require.JSONEq(t, `"token"`, string(gpt["billing_mode"]))
	require.JSONEq(t, `["gpt-5.6-sol"]`, string(gpt["aliases"]))
	require.Contains(t, string(gpt["pricing"]), `"input_price":0.000001`)
	require.JSONEq(t, `{"timezone":"Asia/Shanghai","weekdays_only":true,"periods":[{"start_time":"09:00","end_time":"18:00","multiplier":1.5}]}`, string(gpt["time_pricing"]))

	var sonnet map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[0], &sonnet))
	require.JSONEq(t, `[]`, string(sonnet["aliases"]), "no aliases is an empty array, not null")
	_, hasTimePricing := sonnet["time_pricing"]
	require.False(t, hasTimePricing)
}

func TestModelPlazaHandler_RequireAuthRejectsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newPlazaHandlerForTest(map[string]string{
		service.SettingKeyModelPlazaEnabled:     "true",
		service.SettingKeyModelPlazaRequireAuth: "true",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
	h.Get(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	h.Get(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
