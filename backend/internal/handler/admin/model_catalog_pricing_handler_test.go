//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 价格页夹具：条目 1 = gpt-5.5（官方 5 / 30 / 0.5，>272K 一段 10 / 45），渠道 1 按官方价的 3% 承接；
// 条目 2 是生图模型（不上价格页）。渠道 1 = Chat 中转、2 = 没配地址的 key（承接不了）、3 = Claude 成品号。
func newPricingTestRouter(t *testing.T) (*gin.Engine, *catalogRepoStub) {
	t.Helper()
	repo := &catalogRepoStub{
		entries: []service.ModelCatalogEntry{
			{
				ID: 1, ModelID: "gpt-5.5", Vendor: "openai", BillingMode: service.BillingModeToken,
				Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
				InputPrice: catalogPrice(5e-6), OutputPrice: catalogPrice(30e-6), CacheReadPrice: catalogPrice(0.5e-6),
				Intervals: []service.PricingInterval{{MinTokens: 272000, InputPrice: catalogPrice(10e-6), OutputPrice: catalogPrice(45e-6)}},
			},
			{
				ID: 2, ModelID: "gpt-image-2", Vendor: "openai", BillingMode: service.BillingModeImage,
				Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			},
		},
		bindings: map[int64][]service.ModelCatalogBinding{1: {{
			EntryID: 1, AccountID: 1, InputPrice: 0.15e-6, OutputPrice: 0.9e-6, CacheReadPrice: catalogPrice(0.015e-6),
		}}},
	}
	accounts := catalogAccountsStub{
		1: {ID: 1, Name: "fenno · Chat", Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI, Status: service.StatusActive,
			Schedulable: true, Priority: 10, ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.fenno.example:8443/v1"}},
		2: {ID: 2, Name: "no-endpoint", Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true},
		3: {ID: 3, Name: "claude-oauth", Type: service.AccountTypeOAuth, Platform: service.PlatformAnthropic, Status: service.StatusActive, Schedulable: true, Priority: 5},
	}
	h := newCatalogHandlerWithAccounts(repo, accounts)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/pricing", h.PricingOverview)
	r.PUT("/pricing/models/:id", h.SavePricingModel)
	r.PUT("/pricing/channels/:id", h.SavePricingChannel)
	return r, repo
}

func decodePricingData[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	envelope := decodeCatalogResponse(t, rec)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var out T
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func doPricingJSON(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func TestModelCatalogHandler_PricingOverview(t *testing.T) {
	router, _ := newPricingTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pricing", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodePricingData[PricingOverviewResponse](t, rec)

	require.InDelta(t, 1.0/15, got.DefaultSaleRatio, 1e-12)
	require.InDelta(t, 0.3, got.MinMargin, 1e-12)

	require.Len(t, got.Entries, 1, "only token models are on the pricing page")
	entry := got.Entries[0]
	require.Equal(t, "gpt-5.5", entry.ModelID)
	require.Equal(t, catalogPrice(5e-6), entry.InputPrice)
	require.Len(t, entry.Intervals, 1)
	require.Equal(t, []int64{1, 3}, entry.BindableAccountIDs, "the key without an address cannot serve")
	require.Len(t, entry.Bindings, 1)
	require.Equal(t, int64(1), entry.Bindings[0].AccountID)
	require.NotNil(t, entry.Bindings[0].CostRatio)
	require.InDelta(t, 0.03, *entry.Bindings[0].CostRatio, 1e-12)
	require.Contains(t, rec.Body.String(), `"intervals":[]`, "no upstream segments encodes as an empty list")
	require.Nil(t, entry.Bindings[0].PeakCostRatio, "neither side has peak pricing")
	require.Nil(t, entry.Bindings[0].TimePricing)
	require.Nil(t, entry.TimePricing, "the entry has no official peak hours")

	require.Len(t, got.Accounts, 3)
	byID := map[int64]PricingAccountResponse{}
	for _, a := range got.Accounts {
		byID[a.ID] = a
	}
	require.Equal(t, "fenno · Chat", byID[1].Name)
	require.Equal(t, service.APIProtocolChatCompletions, byID[1].Protocol)
	require.Equal(t, "api.fenno.example", byID[1].UpstreamHost, "host without port")
	require.Equal(t, 10, byID[1].Priority)
	require.Empty(t, byID[3].Protocol, "subscription accounts have no upstream protocol")
	require.Empty(t, byID[3].UpstreamHost)
	require.Equal(t, service.PlatformAnthropic, byID[3].Vendor)
}

// 渠道多于一页时翻页读完。
func TestModelCatalogHandler_PricingOverviewReadsAllAccountPages(t *testing.T) {
	old := pricingAccountPageSize
	pricingAccountPageSize = 2
	t.Cleanup(func() { pricingAccountPageSize = old })

	router, _ := newPricingTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pricing", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodePricingData[PricingOverviewResponse](t, rec)
	require.Len(t, got.Accounts, 3)
	require.Equal(t, []int64{1, 3}, got.Entries[0].BindableAccountIDs)
}

func TestModelCatalogHandler_SavePricingModel(t *testing.T) {
	body := func() map[string]any {
		return map[string]any{
			"input_price": 4e-6, "output_price": 24e-6, "cache_read_price": 0.4e-6,
			"intervals": []any{},
			"bindings": []any{
				map[string]any{"account_id": 3, "input_price": 0.2e-6, "output_price": 1.2e-6, "cache_read_price": 0.02e-6},
			},
		}
	}

	t.Run("saves the block", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", body())
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[PricingEntryResponse](t, rec)
		require.Equal(t, catalogPrice(4e-6), got.InputPrice)
		require.Empty(t, got.Intervals, "official segments cleared")
		require.Len(t, got.Bindings, 1, "channel 1 dropped, channel 3 added")
		require.Equal(t, int64(3), got.Bindings[0].AccountID)
		require.InDelta(t, 0.05, *got.Bindings[0].CostRatio, 1e-12)
		require.Equal(t, []int64{1, 3}, got.BindableAccountIDs)

		require.Len(t, repo.bindings[1], 1)
		require.Equal(t, int64(1), repo.bindings[1][0].EntryID, "entry id comes from the path")
	})

	t.Run("upstream model name round-trips", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		b := body()
		b["bindings"].([]any)[0].(map[string]any)["upstream_model"] = " gpt-5.5-relay "
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[PricingEntryResponse](t, rec)
		require.Equal(t, "gpt-5.5-relay", got.Bindings[0].UpstreamModel)
		require.Equal(t, "gpt-5.5-relay", repo.bindings[1][0].UpstreamModel)

		b["bindings"].([]any)[0].(map[string]any)["upstream_model"] = "gpt-*"
		rec = doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "upstream_model")
	})

	t.Run("upstream time pricing round-trips", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		b := body()
		b["bindings"].([]any)[0].(map[string]any)["time_pricing"] = map[string]any{
			"timezone": "Asia/Shanghai", "weekdays_only": true,
			"periods": []any{map[string]any{"start_time": "09:00", "end_time": "18:00", "multiplier": 2}},
		}
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[PricingEntryResponse](t, rec)
		require.NotNil(t, got.Bindings[0].TimePricing)
		require.Len(t, got.Bindings[0].TimePricing.Periods, 1)
		require.InDelta(t, 0.05, *got.Bindings[0].CostRatio, 1e-12, "cost ratio stays off-peak")
		require.NotNil(t, got.Bindings[0].PeakCostRatio)
		require.InDelta(t, 0.1, *got.Bindings[0].PeakCostRatio, 1e-12)
		require.NotNil(t, repo.bindings[1][0].TimePricing)

		b["bindings"].([]any)[0].(map[string]any)["time_pricing"].(map[string]any)["timezone"] = "Mars/Olympus"
		rec = doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "upstream time_pricing")
	})

	t.Run("official time pricing round-trips onto the entry", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		b := body()
		b["time_pricing"] = map[string]any{
			"timezone": "Asia/Shanghai", "weekdays_only": true,
			"periods": []any{map[string]any{"start_time": "09:00", "end_time": "12:00", "multiplier": 2}},
		}
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[PricingEntryResponse](t, rec)
		require.NotNil(t, got.TimePricing)
		require.Len(t, got.TimePricing.Periods, 1)
		require.NotNil(t, repo.entries[0].TimePricing, "stored on the catalog entry")
		require.Nil(t, repo.bindings[1][0].TimePricing, "the binding keeps its own (none)")

		b["time_pricing"].(map[string]any)["periods"] = []any{map[string]any{"start_time": "12:00", "end_time": "09:00", "multiplier": 2}}
		rec = doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "time_pricing")
	})

	// 最高推理倍率三套（muqian 2026-10-07）：顶层 = 官方，sale_prices 里 = 售价，承接上 = 上游；没填的跟官方。
	t.Run("max reasoning multipliers round-trip", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		b := body()
		b["max_reasoning_effort_multiplier"] = 3
		b["sale_prices"] = map[string]any{"max_reasoning_effort_multiplier": 1}
		b["bindings"].([]any)[0].(map[string]any)["max_reasoning_effort_multiplier"] = 2
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[PricingEntryResponse](t, rec)
		require.Equal(t, catalogPrice(3), got.MaxReasoningEffortMultiplier)
		require.Equal(t, catalogPrice(1), got.SalePrices.MaxReasoningEffortMultiplier)
		require.Equal(t, catalogPrice(2), got.Bindings[0].MaxReasoningEffortMultiplier)
		require.Nil(t, got.Bindings[0].PeakCostRatio)
		require.NotNil(t, got.Bindings[0].MaxReasoningCostRatio, "上游 max 档 × 2、售价不加：max 档成本比翻倍")
		require.InDelta(t, 0.1, *got.Bindings[0].MaxReasoningCostRatio, 1e-12)
		require.Equal(t, catalogPrice(3), repo.entries[0].MaxReasoningEffortMultiplier)
		require.Equal(t, catalogPrice(2), repo.bindings[1][0].MaxReasoningEffortMultiplier)

		b["bindings"].([]any)[0].(map[string]any)["max_reasoning_effort_multiplier"] = 0
		rec = doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "upstream max_reasoning_effort_multiplier must be")
	})

	t.Run("upstream output price is required", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		b := body()
		delete(b["bindings"].([]any)[0].(map[string]any), "output_price")
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "upstream input_price and output_price are required")
		require.Equal(t, int64(1), repo.bindings[1][0].AccountID, "nothing written")
	})

	t.Run("validation errors are 400", func(t *testing.T) {
		router, _ := newPricingTestRouter(t)
		b := body()
		delete(b["bindings"].([]any)[0].(map[string]any), "cache_read_price")
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/1", b)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "upstream cache_read_price is required")
	})

	t.Run("unknown model is 404", func(t *testing.T) {
		router, _ := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/models/42", body())
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestModelCatalogHandler_SavePricingChannel(t *testing.T) {
	t.Run("saves the block", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/channels/3", map[string]any{
			"bindings": []any{map[string]any{"entry_id": 1, "upstream_model": "claude-proxy-5.5", "input_price": 0.25e-6, "output_price": 1.5e-6, "cache_read_price": 0.025e-6}},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[[]PricingBindingResponse](t, rec)
		require.Len(t, got, 1)
		require.Equal(t, "claude-proxy-5.5", got[0].UpstreamModel)
		require.Equal(t, int64(3), got[0].AccountID, "account id comes from the path")
		require.InDelta(t, 0.05, *got[0].CostRatio, 1e-12)
		require.Nil(t, got[0].MaxReasoningEffortMultiplier, "not set = follow official")
		require.Nil(t, got[0].MaxReasoningCostRatio, "max effort is no worse than usual")
		require.Len(t, repo.bindings[1], 2, "channel 1 on the same model untouched")
	})

	t.Run("upstream max reasoning multiplier round-trips", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/channels/3", map[string]any{
			"bindings": []any{map[string]any{"entry_id": 1, "input_price": 0.25e-6, "output_price": 1.5e-6, "cache_read_price": 0.025e-6,
				"max_reasoning_effort_multiplier": 1}},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodePricingData[[]PricingBindingResponse](t, rec)
		require.Equal(t, catalogPrice(1), got[0].MaxReasoningEffortMultiplier)
		for _, b := range repo.bindings[1] {
			if b.AccountID == 3 {
				require.Equal(t, catalogPrice(1), b.MaxReasoningEffortMultiplier)
			}
		}
	})

	t.Run("channel that cannot serve is 400", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/channels/2", map[string]any{
			"bindings": []any{map[string]any{"entry_id": 1, "input_price": 0.25e-6, "output_price": 1.5e-6, "cache_read_price": 0.025e-6}},
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Len(t, repo.bindings[1], 1, "nothing written")
	})

	t.Run("bad channel id is 400", func(t *testing.T) {
		router, _ := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/channels/x", map[string]any{"bindings": []any{}})
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("empty list removes all the channel's bindings", func(t *testing.T) {
		router, repo := newPricingTestRouter(t)
		rec := doPricingJSON(router, http.MethodPut, "/pricing/channels/1", map[string]any{"bindings": []any{}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Empty(t, repo.bindings[1])
	})
}

// 联网搜索价：价格页给出厂商公开价作占位；上游搜索价可不填（= 上游不收）；保存后原样带回。
func TestModelCatalogHandler_PricingSearchPrices(t *testing.T) {
	router, repo := newPricingTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pricing", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	overview := decodePricingData[PricingOverviewResponse](t, rec)
	require.Nil(t, overview.Entries[0].SearchPricePerCall)
	require.Equal(t, &PricingSearchDefaults{SearchPricePerCall: 0.01}, overview.Entries[0].SearchDefaults, "OpenAI 只有每次 web 搜索价")

	body := map[string]any{
		"input_price": 5e-6, "output_price": 30e-6, "cache_read_price": 0.5e-6,
		"intervals":             []any{map[string]any{"min_tokens": 272000, "input_price": 10e-6, "output_price": 45e-6}},
		"search_price_per_call": 0.02,
		"bindings": []any{
			map[string]any{"account_id": 1, "input_price": 0.15e-6, "output_price": 0.9e-6, "cache_read_price": 0.015e-6},
		},
	}
	rec = doPricingJSON(router, http.MethodPut, "/pricing/models/1", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Nil(t, repo.bindings[1][0].SearchPricePerCall, "上游搜索价可不填")

	body["bindings"].([]any)[0].(map[string]any)["search_price_per_call"] = 0.015
	rec = doPricingJSON(router, http.MethodPut, "/pricing/models/1", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodePricingData[PricingEntryResponse](t, rec)
	require.Equal(t, catalogPrice(0.02), got.SearchPricePerCall)
	require.Equal(t, catalogPrice(0.015), got.Bindings[0].SearchPricePerCall)
	require.Equal(t, catalogPrice(0.015), repo.bindings[1][0].SearchPricePerCall)

	// 按渠道保存同样带上搜索上游价
	rec = doPricingJSON(router, http.MethodPut, "/pricing/channels/1", map[string]any{
		"bindings": []any{map[string]any{"entry_id": 1, "input_price": 0.15e-6, "output_price": 0.9e-6, "cache_read_price": 0.015e-6, "search_price_per_call": 0.012}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	saved := decodePricingData[[]PricingBindingResponse](t, rec)
	require.Equal(t, catalogPrice(0.012), saved[0].SearchPricePerCall)
}

// 「联网搜索」计费项就是代执行模型这条目录：价格页给它打标记。
func TestModelCatalogHandler_PricingMarksWebSearchDelegate(t *testing.T) {
	router, repo := newPricingTestRouter(t)
	repo.entries = append(repo.entries, service.ModelCatalogEntry{
		ID: 9, ModelID: service.WebSearchDelegateModel, Vendor: "anthropic", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: catalogPrice(1e-6), OutputPrice: catalogPrice(5e-6),
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pricing", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodePricingData[PricingOverviewResponse](t, rec)
	flags := map[string]bool{}
	for _, entry := range got.Entries {
		flags[entry.ModelID] = entry.WebSearchDelegate
	}
	require.Equal(t, map[string]bool{"gpt-5.5": false, service.WebSearchDelegateModel: true}, flags)
}
