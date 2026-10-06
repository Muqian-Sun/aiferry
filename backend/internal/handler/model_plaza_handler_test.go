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
	search := 0.01 // 播种时按厂商公开价写进目录的搜索价
	return []service.ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", DisplayName: "Sonnet 4", Vendor: "anthropic", Status: service.ModelCatalogStatusListed, InputPrice: &price,
			SearchPricePerCall: &search},
		{ID: 2, ModelID: "gpt-5.6", DisplayName: "GPT-5.6", Vendor: "openai", Status: service.ModelCatalogStatusListed, InputPrice: &price,
			AudioInputPrice: &audio, SearchPricePerCall: &search,
			TimePricing: &service.TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []service.TimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 1.5}}}},
	}
}

// plazaUserStub 登录访问者：7 号单独设了售价折扣 0.5，8 号不打折；别的 id 查不到
type plazaUserStub struct{}

func (plazaUserStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	custom := 0.5
	switch id {
	case 7:
		return &service.User{ID: 7, RateMultiplier: &custom}, nil
	case 8:
		return &service.User{ID: 8}, nil
	}
	return nil, service.ErrUserNotFound
}

func newPlazaHandlerForTest(values map[string]string) *ModelPlazaHandler {
	return NewModelPlazaHandler(
		service.NewModelPlazaService(plazaCatalogStub{}, nil),
		service.NewSettingService(plazaSettingRepoStub{values: values}, &config.Config{}),
		plazaUserStub{},
	)
}

// getPlaza 以 userID 登录（0 = 未登录）请求广场
func getPlaza(t *testing.T, h *ModelPlazaHandler, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
	if userID > 0 {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	}
	h.Get(c)
	return w
}

// firstInputPrice 广场第一个模型（claude-sonnet-4，官方输入价 1e-6）的输入价
func firstInputPrice(t *testing.T, body []byte) float64 {
	t.Helper()
	var envelope struct {
		Data struct {
			Models []struct {
				Pricing struct {
					InputPrice *float64 `json:"input_price"`
				} `json:"pricing"`
			} `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.NotEmpty(t, envelope.Data.Models)
	require.NotNil(t, envelope.Data.Models[0].Pricing.InputPrice)
	return *envelope.Data.Models[0].Pricing.InputPrice
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
			Description string            `json:"description"`
			Models      []json.RawMessage `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, service.ModelPlazaDescription, envelope.Data.Description)
	// 官方价与倍率都不出接口：只给售价（2026-10-04 D1）
	require.NotContains(t, w.Body.String(), "rate_multiplier")
	require.Len(t, envelope.Data.Models, 2)
	require.NotContains(t, w.Body.String(), `"groups"`, "the plaza is flat: no groups")

	var gpt map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[1], &gpt))
	require.JSONEq(t, `"gpt-5.6"`, string(gpt["model_id"]))
	require.JSONEq(t, `"GPT-5.6"`, string(gpt["display_name"]))
	require.JSONEq(t, `"openai"`, string(gpt["vendor"]))
	require.JSONEq(t, `"token"`, string(gpt["billing_mode"]))
	_, hasAliases := gpt["aliases"]
	require.False(t, hasAliases, "目录不存别名，广场不出别名字段")
	var pricing struct {
		InputPrice      *float64 `json:"input_price"`
		AudioInputPrice *float64 `json:"audio_input_price"`
	}
	require.NoError(t, json.Unmarshal(gpt["pricing"], &pricing))
	require.NotNil(t, pricing.InputPrice)
	require.InDelta(t, 1e-6/15, *pricing.InputPrice, 1e-18, "未登录给新用户的售价：官方价 × 1/15")
	// 计费项列全：音频价也带出来，同样是售价
	require.NotNil(t, pricing.AudioInputPrice)
	require.InDelta(t, 3e-6/15, *pricing.AudioInputPrice, 1e-18)

	// 联网搜索按次价（按原价收，不乘倍率）：条目上的价（计费只认目录）；X 帖子 / 主页价只有 xAI 有
	require.Contains(t, string(gpt["pricing"]), `"search_price_per_call":0.01`)
	require.NotContains(t, string(gpt["pricing"]), "x_post_price")
	var sonnetPricing map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[0], &sonnetPricing))
	require.Contains(t, string(sonnetPricing["pricing"]), `"search_price_per_call":0.01`)
	require.NotContains(t, string(sonnetPricing["pricing"]), "x_user_price")
	require.JSONEq(t, `{"timezone":"Asia/Shanghai","weekdays_only":true,"periods":[{"start_time":"09:00","end_time":"18:00","multiplier":1.5}]}`, string(gpt["time_pricing"]))

	var sonnet map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data.Models[0], &sonnet))
	_, hasTimePricing := sonnet["time_pricing"]
	require.False(t, hasTimePricing)
}

// 模型广场没有开关，未登录也能看完整目录；价格是访问者自己的售价：
// 未登录与跟默认倍率的用户 = 官方价 × 1/15，单独设了倍率的用户 = 官方价 × 他的倍率。
func TestModelPlazaHandler_PricesAreTheViewersSalePrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newPlazaHandlerForTest(map[string]string{})

	anonymous := getPlaza(t, h, 0)
	require.Equal(t, http.StatusOK, anonymous.Code, anonymous.Body.String())
	require.Contains(t, anonymous.Body.String(), `"model_id":"gpt-5.6"`)
	require.InDelta(t, 1e-6/15, firstInputPrice(t, anonymous.Body.Bytes()), 1e-18)

	// 售价（没单独定 = 官方价 × 1/15）× 折扣 0.5
	custom := getPlaza(t, h, 7)
	require.Equal(t, http.StatusOK, custom.Code, custom.Body.String())
	require.InDelta(t, 1e-6/15*0.5, firstInputPrice(t, custom.Body.Bytes()), 1e-18)

	byDefault := getPlaza(t, h, 8)
	require.Equal(t, http.StatusOK, byDefault.Code, byDefault.Body.String())
	require.JSONEq(t, anonymous.Body.String(), byDefault.Body.String())

	// 登录了却查不到用户：报错，不拿默认价顶上
	missing := getPlaza(t, h, 99)
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
}

type plazaPricingStub struct{ entry *service.ModelCatalogEntry }

func (s plazaPricingStub) LookupPricingEntry(_ context.Context, model string) *service.ModelCatalogEntry {
	if s.entry != nil && model == s.entry.ModelID {
		return s.entry
	}
	return nil
}

// Claude Code 配非 Anthropic 模型时那次搜索请求的计费项：token 给售价，每次搜索按原价，不提代执行的模型。
func TestModelPlazaHandler_ClaudeCodeWebSearchBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	input, output, search := 1e-6, 5e-6, 0.01
	haiku := &service.ModelCatalogEntry{ID: 9, ModelID: service.WebSearchDelegateModel, Vendor: "anthropic", Status: service.ModelCatalogStatusUnlisted, InputPrice: &input, OutputPrice: &output,
		SearchPricePerCall: &search}
	get := func(pricing service.ModelCatalogPricingSource) string {
		h := NewModelPlazaHandler(service.NewModelPlazaService(plazaCatalogStub{}, pricing), service.NewSettingService(plazaSettingRepoStub{}, &config.Config{}), plazaUserStub{})
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
	require.InDelta(t, 1e-6/15, *envelope.Data.ClaudeCodeWebSearch.InputPrice, 1e-18, "token 价是售价")
	require.InDelta(t, 5e-6/15, *envelope.Data.ClaudeCodeWebSearch.OutputPrice, 1e-18)
	require.InDelta(t, 0.01, envelope.Data.ClaudeCodeWebSearch.SearchPricePerCall, 1e-12, "每次搜索按目录条目上的原价，不乘倍率")
	require.NotContains(t, body, "haiku", "广场不提代执行的模型")

	require.NotContains(t, get(plazaPricingStub{}), "claude_code_web_search", "目录里没有代执行模型时不给")
}
