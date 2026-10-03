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
	audio := 3e-6
	return []service.ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", DisplayName: "Sonnet 4", Vendor: "anthropic", Status: service.ModelCatalogStatusListed, InputPrice: &price},
		{ID: 2, ModelID: "gpt-5.6", DisplayName: "GPT-5.6", Vendor: "openai", Status: service.ModelCatalogStatusListed, InputPrice: &price,
			AudioInputPrice: &audio,
			Aliases:         []service.ModelCatalogAlias{{ID: 10, EntryID: 2, Alias: "gpt-5.6-sol"}},
			TimePricing:     &service.TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []service.TimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 1.5}}}},
	}
}

func newPlazaHandlerForTest(values map[string]string) *ModelPlazaHandler {
	return NewModelPlazaHandler(
		service.NewModelPlazaService(plazaCatalogStub{}, nil),
		service.NewSettingService(plazaSettingRepoStub{values: values}, &config.Config{}),
	)
}

func TestModelPlazaHandler_ReturnsListedCatalogModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 价格说明由代码决定（service.ModelPlazaDescription），库里旧值不生效
	h := newPlazaHandlerForTest(map[string]string{
		"model_plaza_description": "stale",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)

	h.Get(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Data struct {
			Description           string            `json:"description"`
			DefaultRateMultiplier float64           `json:"default_rate_multiplier"`
			Models                []json.RawMessage `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, service.ModelPlazaDescription, envelope.Data.Description)
	// 接口给官方价 + 新用户默认倍率（官方价的 1/15），未登录的展示价 = 官方价 × 它
	require.InDelta(t, 1.0/15, envelope.Data.DefaultRateMultiplier, 1e-15)
	require.Len(t, envelope.Data.Models, 2)
	require.NotContains(t, w.Body.String(), `"groups"`, "the plaza is flat: no groups")

	var gpt map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[1], &gpt))
	require.JSONEq(t, `"gpt-5.6"`, string(gpt["model_id"]))
	require.JSONEq(t, `"GPT-5.6"`, string(gpt["display_name"]))
	require.JSONEq(t, `"openai"`, string(gpt["vendor"]))
	require.JSONEq(t, `"token"`, string(gpt["billing_mode"]))
	require.JSONEq(t, `["gpt-5.6-sol"]`, string(gpt["aliases"]))
	var pricing struct {
		InputPrice      *float64 `json:"input_price"`
		AudioInputPrice *float64 `json:"audio_input_price"`
	}
	require.NoError(t, json.Unmarshal(gpt["pricing"], &pricing))
	require.NotNil(t, pricing.InputPrice)
	require.InDelta(t, 1e-6, *pricing.InputPrice, 1e-18, "广场接口给目录官方价，展示时再乘访问者倍率")
	// 计费项列全：音频价也带出来
	require.NotNil(t, pricing.AudioInputPrice)
	require.InDelta(t, 3e-6, *pricing.AudioInputPrice, 1e-18)

	// 联网搜索价（官方价，不乘倍率）：有官方搜索工具的厂商都给，条目没配按厂商公开价；X 帖子 / 主页价只有 xAI 有
	require.Contains(t, string(gpt["pricing"]), `"search_price_per_call":0.01`)
	require.NotContains(t, string(gpt["pricing"]), "x_post_price")
	var sonnetPricing map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[0], &sonnetPricing))
	require.Contains(t, string(sonnetPricing["pricing"]), `"search_price_per_call":0.01`)
	require.NotContains(t, string(sonnetPricing["pricing"]), "x_user_price")
	require.JSONEq(t, `{"timezone":"Asia/Shanghai","weekdays_only":true,"periods":[{"start_time":"09:00","end_time":"18:00","multiplier":1.5}]}`, string(gpt["time_pricing"]))

	var sonnet map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[0], &sonnet))
	require.JSONEq(t, `[]`, string(sonnet["aliases"]), "no aliases is an empty array, not null")
	_, hasTimePricing := sonnet["time_pricing"]
	require.False(t, hasTimePricing)
}

// 模型广场没有开关：未登录直接拿到完整目录，登录与否结果一致。
func TestModelPlazaHandler_AnonymousSeesFullCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newPlazaHandlerForTest(map[string]string{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
	h.Get(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	anonymous := w.Body.String()

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	h.Get(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, anonymous, w.Body.String())
	require.Contains(t, anonymous, `"model_id":"gpt-5.6"`)
}

type plazaPricingStub struct{ entry *service.ModelCatalogEntry }

func (s plazaPricingStub) LookupPricingEntry(_ context.Context, model string) *service.ModelCatalogEntry {
	if s.entry != nil && model == s.entry.ModelID {
		return s.entry
	}
	return nil
}

// Claude Code 配非 Anthropic 模型时那次搜索请求的计费项：给官方价（token 由前端 × 访问者倍率），不提代执行的模型。
func TestModelPlazaHandler_ClaudeCodeWebSearchBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	input, output := 1e-6, 5e-6
	haiku := &service.ModelCatalogEntry{ID: 9, ModelID: service.WebSearchDelegateModel, Vendor: "anthropic", Status: service.ModelCatalogStatusUnlisted, InputPrice: &input, OutputPrice: &output}
	get := func(pricing service.ModelCatalogPricingSource) string {
		h := NewModelPlazaHandler(service.NewModelPlazaService(plazaCatalogStub{}, pricing), service.NewSettingService(plazaSettingRepoStub{}, &config.Config{}))
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
		h.Get(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return w.Body.String()
	}

	body := get(plazaPricingStub{entry: haiku})
	var envelope struct {
		Data struct {
			ClaudeCodeWebSearch *struct {
				InputPrice         *float64 `json:"input_price"`
				OutputPrice        *float64 `json:"output_price"`
				SearchPricePerCall float64  `json:"search_price_per_call"`
			} `json:"claude_code_web_search"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &envelope))
	require.NotNil(t, envelope.Data.ClaudeCodeWebSearch)
	require.InDelta(t, 1e-6, *envelope.Data.ClaudeCodeWebSearch.InputPrice, 1e-18)
	require.InDelta(t, 5e-6, *envelope.Data.ClaudeCodeWebSearch.OutputPrice, 1e-18)
	require.InDelta(t, 0.01, envelope.Data.ClaudeCodeWebSearch.SearchPricePerCall, 1e-12, "Anthropic 每次 web 搜索公开价")
	require.NotContains(t, body, "haiku", "广场不提代执行的模型")

	require.NotContains(t, get(plazaPricingStub{}), "claude_code_web_search", "目录里没有代执行模型时不给")
}
