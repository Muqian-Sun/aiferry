//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// catalogRepoStub 是管理端测试用的内存仓储，复刻服务层依赖的接口。
type catalogRepoStub struct {
	mu       sync.Mutex
	entries  []service.ModelCatalogEntry
	seed     service.ModelCatalogSeedResult
	seedErr  error
	bindings map[int64][]service.ModelCatalogBinding
}

func (r *catalogRepoStub) ListEntries(context.Context) ([]service.ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]service.ModelCatalogEntry, len(r.entries))
	copy(out, r.entries)
	for i := range out {
		out[i].Bindings = append([]service.ModelCatalogBinding(nil), r.bindings[out[i].ID]...)
	}
	return out, nil
}

// GetEntryByID 像真实仓储一样把绑定挂上（hydrateEntry）。
func (r *catalogRepoStub) GetEntryByID(_ context.Context, id int64) (*service.ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == id {
			cp := r.entries[i]
			cp.Bindings = append([]service.ModelCatalogBinding(nil), r.bindings[id]...)
			return &cp, nil
		}
	}
	return nil, service.ErrModelCatalogEntryNotFound
}

func (r *catalogRepoStub) GetEntryByModelID(_ context.Context, modelID string) (*service.ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := service.NormalizeModelCatalogKey(modelID)
	for i := range r.entries {
		if service.NormalizeModelCatalogKey(r.entries[i].ModelID) == key {
			cp := r.entries[i]
			return &cp, nil
		}
	}
	return nil, service.ErrModelCatalogEntryNotFound
}

func (r *catalogRepoStub) CreateEntry(_ context.Context, entry *service.ModelCatalogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := service.NormalizeModelCatalogKey(entry.ModelID)
	for i := range r.entries {
		if service.NormalizeModelCatalogKey(r.entries[i].ModelID) == key {
			return service.ErrModelCatalogEntryExists
		}
	}
	entry.ID = int64(len(r.entries) + 1)
	r.entries = append(r.entries, *entry)
	return nil
}

func (r *catalogRepoStub) UpdateEntry(_ context.Context, entry *service.ModelCatalogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == entry.ID {
			r.entries[i] = *entry
			return nil
		}
	}
	return service.ErrModelCatalogEntryNotFound
}

func (r *catalogRepoStub) DeleteEntry(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == id {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return service.ErrModelCatalogEntryNotFound
}

func (r *catalogRepoStub) CreateAlias(_ context.Context, alias *service.ModelCatalogAlias) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := service.NormalizeModelCatalogKey(alias.Alias)
	for i := range r.entries {
		for _, existing := range r.entries[i].Aliases {
			if service.NormalizeModelCatalogKey(existing.Alias) == key {
				return service.ErrModelCatalogAliasExists
			}
		}
	}
	for i := range r.entries {
		if r.entries[i].ID == alias.EntryID {
			alias.ID = int64(len(r.entries[i].Aliases) + 1)
			r.entries[i].Aliases = append(r.entries[i].Aliases, *alias)
			return nil
		}
	}
	return service.ErrModelCatalogEntryNotFound
}

func (r *catalogRepoStub) UpdateAlias(_ context.Context, alias *service.ModelCatalogAlias) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		for j := range r.entries[i].Aliases {
			if r.entries[i].Aliases[j].ID == alias.ID {
				r.entries[i].Aliases[j] = *alias
				return nil
			}
		}
	}
	return service.ErrModelCatalogAliasNotFound
}

func (r *catalogRepoStub) DeleteAlias(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		for j := range r.entries[i].Aliases {
			if r.entries[i].Aliases[j].ID == id {
				r.entries[i].Aliases = append(r.entries[i].Aliases[:j], r.entries[i].Aliases[j+1:]...)
				return nil
			}
		}
	}
	return service.ErrModelCatalogAliasNotFound
}

func (r *catalogRepoStub) InsertOrRefreshSeedEntries(context.Context, []service.ModelCatalogEntry) (service.ModelCatalogSeedResult, error) {
	if r.seedErr != nil {
		return service.ModelCatalogSeedResult{}, r.seedErr
	}
	return r.seed, nil
}

func (r *catalogRepoStub) ListBindingsByEntry(_ context.Context, entryID int64) ([]service.ModelCatalogBinding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.ModelCatalogBinding(nil), r.bindings[entryID]...), nil
}

func (r *catalogRepoStub) ReplaceBindings(_ context.Context, entryID int64, bindings []service.ModelCatalogBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bindings == nil {
		r.bindings = make(map[int64][]service.ModelCatalogBinding)
	}
	r.bindings[entryID] = append([]service.ModelCatalogBinding(nil), bindings...)
	return nil
}

func (r *catalogRepoStub) ListEntryIDsByAccount(_ context.Context, accountID int64) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []int64
	for entryID, bindings := range r.bindings {
		for _, binding := range bindings {
			if binding.AccountID == accountID {
				ids = append(ids, entryID)
				break
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (r *catalogRepoStub) ReplaceAccountBindings(_ context.Context, accountID int64, entryIDs []int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bindings == nil {
		r.bindings = make(map[int64][]service.ModelCatalogBinding)
	}
	for entryID, bindings := range r.bindings {
		kept := bindings[:0:0]
		for _, binding := range bindings {
			if binding.AccountID != accountID {
				kept = append(kept, binding)
			}
		}
		r.bindings[entryID] = kept
	}
	for _, entryID := range entryIDs {
		r.bindings[entryID] = append(r.bindings[entryID], service.ModelCatalogBinding{EntryID: entryID, AccountID: accountID})
	}
	return nil
}

// catalogAccountsStub 按 ID 取账号；不在表里的账号视为不存在。
type catalogAccountsStub map[int64]*service.Account

func (m catalogAccountsStub) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if account, ok := m[id]; ok {
		return account, nil
	}
	return nil, service.ErrAccountNotFound
}

func newCatalogHandler(repo *catalogRepoStub) *ModelCatalogHandler {
	return newCatalogHandlerWithAccounts(repo, catalogAccountsStub{})
}

func newCatalogHandlerWithAccounts(repo *catalogRepoStub, accounts catalogAccountsStub) *ModelCatalogHandler {
	svc := service.NewModelCatalogService(repo, nil, service.ModelCatalogSeedInput{})
	return NewModelCatalogHandler(svc, accounts)
}

func newCatalogRouter(h *ModelCatalogHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/entries", h.ListEntries)
	r.GET("/entries/:id", h.GetEntry)
	r.POST("/entries", h.CreateEntry)
	r.PUT("/entries/:id", h.UpdateEntry)
	r.DELETE("/entries/:id", h.DeleteEntry)
	r.GET("/entries/:id/bindings", h.ListBindings)
	r.PUT("/entries/:id/bindings", h.ReplaceBindings)
	r.GET("/entries/:id/diagnosis", h.Diagnose)
	r.POST("/aliases", h.CreateAlias)
	r.PUT("/aliases/:id", h.UpdateAlias)
	r.DELETE("/aliases/:id", h.DeleteAlias)
	r.POST("/seed", h.Seed)
	r.GET("/price-lookup", h.PriceLookup)
	r.GET("/accounts/:id/catalog-entries", h.ListAccountEntries)
	r.PUT("/accounts/:id/catalog-entries", h.ReplaceAccountEntries)
	return r
}

func decodeCatalogResponse(t *testing.T, rec *httptest.ResponseRecorder) response.Response {
	t.Helper()
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope
}

func TestModelCatalogHandler_ListEntries(t *testing.T) {
	h := newCatalogHandler(&catalogRepoStub{entries: []service.ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", Vendor: "anthropic", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed},
		{ID: 2, ModelID: "gemini-embedding", Vendor: "vertex_ai-embedding-models", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusUnlisted},
		{ID: 3, ModelID: "mystery", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusUnlisted},
		{ID: 4, ModelID: "gpt-image-2", Vendor: "openai", BillingMode: service.BillingModeImage, Status: service.ModelCatalogStatusListed},
	}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/entries", nil)
	newCatalogRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	envelope := decodeCatalogResponse(t, rec)
	require.Equal(t, 0, envelope.Code)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var entries []ModelCatalogEntryView
	require.NoError(t, json.Unmarshal(raw, &entries))
	require.Len(t, entries, 4)
	require.Equal(t, "claude-sonnet-4", entries[0].ModelID)
	// 渠道表单按厂商族分组：厂商族与条目字段平铺在同一层
	require.Equal(t, service.PlatformAnthropic, entries[0].VendorPlatform)
	require.Equal(t, service.PlatformGemini, entries[1].VendorPlatform, "vertex_ai-* 按前缀归 gemini")
	require.Empty(t, entries[2].VendorPlatform, "没有厂商就没有厂商族")
	require.Contains(t, string(raw), `"vendor_platform":"anthropic"`)
	// 渠道表单默认只勾对话模型：生图 / 视频 / 向量走扩展端点，另有承接条件
	require.False(t, entries[0].ExtensionEndpoints)
	require.True(t, entries[3].ExtensionEndpoints, "OpenAI 按图计费的条目走扩展端点")
	require.Contains(t, string(raw), `"model_id":"claude-sonnet-4"`)
}

func TestModelCatalogHandler_GetEntry(t *testing.T) {
	h := newCatalogHandler(&catalogRepoStub{entries: []service.ModelCatalogEntry{
		{ID: 7, ModelID: "m", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed},
	}})
	router := newCatalogRouter(h)

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/7", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		envelope := decodeCatalogResponse(t, rec)
		raw, err := json.Marshal(envelope.Data)
		require.NoError(t, err)
		var entry service.ModelCatalogEntry
		require.NoError(t, json.Unmarshal(raw, &entry))
		require.Equal(t, int64(7), entry.ID)
	})

	t.Run("not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/99", nil))
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/abc", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("zero id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/0", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestModelCatalogHandler_CreateEntry(t *testing.T) {
	repo := &catalogRepoStub{}
	h := newCatalogHandler(repo)
	router := newCatalogRouter(h)

	t.Run("ok marks as admin", func(t *testing.T) {
		body := `{"model_id":"claude-sonnet-4","input_price":0.000003}`
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/entries", bytes.NewBufferString(body)))
		require.Equal(t, http.StatusOK, rec.Code)
		envelope := decodeCatalogResponse(t, rec)
		raw, err := json.Marshal(envelope.Data)
		require.NoError(t, err)
		var entry service.ModelCatalogEntry
		require.NoError(t, json.Unmarshal(raw, &entry))
		require.Equal(t, service.ModelCatalogManagedByAdmin, entry.ManagedBy)
		require.Equal(t, int64(1), entry.ID)
		require.Equal(t, service.ModelCatalogManagedByAdmin, repo.entries[0].ManagedBy)
	})

	t.Run("missing model_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/entries", bytes.NewBufferString(`{"input_price":1}`)))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("duplicate", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/entries", bytes.NewBufferString(`{"model_id":"claude-sonnet-4","input_price":0.000003}`)))
		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestModelCatalogHandler_UpdateEntry(t *testing.T) {
	price := 3e-6
	repo := &catalogRepoStub{entries: []service.ModelCatalogEntry{{
		ID: 3, ModelID: "m", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
		InputPrice: &price,
	}}}
	h := newCatalogHandler(repo)
	router := newCatalogRouter(h)

	t.Run("ok", func(t *testing.T) {
		body := `{"model_id":"m","status":"unlisted","input_price":0.000004}`
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3", bytes.NewBufferString(body)))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, service.ModelCatalogStatusUnlisted, repo.entries[0].Status)
		require.Equal(t, service.ModelCatalogManagedByAdmin, repo.entries[0].ManagedBy)
		require.InDelta(t, 4e-6, *repo.entries[0].InputPrice, 1e-12)
	})

	t.Run("not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/99", bytes.NewBufferString(`{"model_id":"x","input_price":1}`)))
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("audio prices round-trip", func(t *testing.T) {
		body := `{"model_id":"m","status":"unlisted","input_price":0.000004,"audio_input_price":0.00004,"audio_output_price":0.00008}`
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3", bytes.NewBufferString(body)))
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, repo.entries[0].AudioInputPrice)
		require.InDelta(t, 4e-5, *repo.entries[0].AudioInputPrice, 1e-12)
		require.NotNil(t, repo.entries[0].AudioOutputPrice)
		require.InDelta(t, 8e-5, *repo.entries[0].AudioOutputPrice, 1e-12)

		envelope := decodeCatalogResponse(t, rec)
		raw, err := json.Marshal(envelope.Data)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"audio_input_price":0.00004`)
		require.Contains(t, string(raw), `"audio_output_price":0.00008`)
	})
}

func TestModelCatalogHandler_DeleteEntry(t *testing.T) {
	repo := &catalogRepoStub{entries: []service.ModelCatalogEntry{{
		ID: 5, ModelID: "m", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed,
	}}}
	h := newCatalogHandler(repo)
	router := newCatalogRouter(h)

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/entries/5", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, repo.entries)
	})

	t.Run("not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/entries/5", nil))
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestModelCatalogHandler_AliasCRUD(t *testing.T) {
	repo := &catalogRepoStub{entries: []service.ModelCatalogEntry{{
		ID: 1, ModelID: "m", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed,
	}}}
	h := newCatalogHandler(repo)
	router := newCatalogRouter(h)

	t.Run("create", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/aliases", bytes.NewBufferString(`{"alias":"nick","entry_id":1}`)))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, repo.entries[0].Aliases, 1)
		require.Equal(t, service.ModelCatalogAliasSourceManual, repo.entries[0].Aliases[0].Source)
	})

	t.Run("duplicate", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/aliases", bytes.NewBufferString(`{"alias":"NICK","entry_id":1}`)))
		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("update", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/aliases/1", bytes.NewBufferString(`{"alias":"renamed","entry_id":1}`)))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "renamed", repo.entries[0].Aliases[0].Alias)
	})

	t.Run("delete", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/aliases/1", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, repo.entries[0].Aliases)
	})

	t.Run("bare wildcard rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/aliases", bytes.NewBufferString(`{"alias":"*","entry_id":1}`)))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestModelCatalogHandler_Seed(t *testing.T) {
	repo := &catalogRepoStub{seed: service.ModelCatalogSeedResult{Inserted: 3, Refreshed: 1, SkippedAdmin: 2, CandidateModels: 6}}
	h := newCatalogHandler(repo)
	rec := httptest.NewRecorder()
	newCatalogRouter(h).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/seed", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	envelope := decodeCatalogResponse(t, rec)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var result service.ModelCatalogSeedResult
	require.NoError(t, json.Unmarshal(raw, &result))
	require.Equal(t, 3, result.Inserted)
	require.Equal(t, 1, result.Refreshed)
	require.Equal(t, 2, result.SkippedAdmin)
}

func TestModelCatalogHandler_SeedError(t *testing.T) {
	repo := &catalogRepoStub{seedErr: errors.New("db down")}
	h := newCatalogHandler(repo)
	rec := httptest.NewRecorder()
	newCatalogRouter(h).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/seed", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestModelCatalogHandler_Bindings(t *testing.T) {
	price := 3e-6
	repo := &catalogRepoStub{entries: []service.ModelCatalogEntry{{
		ID: 3, ModelID: "claude-sonnet-4", Vendor: "anthropic", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin, InputPrice: &price,
	}}}
	accounts := catalogAccountsStub{
		1: {ID: 1, Name: "oauth-a", Type: service.AccountTypeOAuth, Platform: service.PlatformAnthropic, Status: service.StatusActive},
		2: {ID: 2, Name: "relay-key", Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI, Status: service.StatusActive,
			ProtocolEndpoints: map[string]string{service.APIProtocolAnthropic: "https://relay.example.com"}},
		// 没配任何上游地址的 key：注册表里没有它能承接的入站协议。
		3: {ID: 3, Name: "no-address", Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	}
	router := newCatalogRouter(newCatalogHandlerWithAccounts(repo, accounts))

	decodeBindings := func(t *testing.T, rec *httptest.ResponseRecorder) []ModelCatalogBindingResponse {
		t.Helper()
		envelope := decodeCatalogResponse(t, rec)
		raw, err := json.Marshal(envelope.Data)
		require.NoError(t, err)
		var out []ModelCatalogBindingResponse
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}

	t.Run("empty at first", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/3/bindings", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, decodeBindings(t, rec))
	})

	t.Run("replace returns bindings with account summaries", func(t *testing.T) {
		body := `{"bindings":[{"account_id":2,"priority":5},{"account_id":1}]}`
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3/bindings", bytes.NewBufferString(body)))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodeBindings(t, rec)
		require.Len(t, got, 2)
		require.Equal(t, int64(2), got[0].AccountID)
		require.Equal(t, 5, *got[0].Priority)
		require.Equal(t, "relay-key", got[0].Account.Name)
		require.Equal(t, "", got[0].Account.Vendor, "generic relay has no official vendor")
		require.Equal(t, int64(1), got[1].AccountID)
		require.Nil(t, got[1].Priority)
		require.Equal(t, service.PlatformAnthropic, got[1].Account.Vendor)

		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/3", nil))
		require.Contains(t, rec.Body.String(), `"bindings":[`, "entry payload carries bindings for the list column")
	})

	t.Run("unservable account is rejected as a whole", func(t *testing.T) {
		body := `{"bindings":[{"account_id":1},{"account_id":3}]}`
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3/bindings", bytes.NewBufferString(body)))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "CATALOG_BINDING_UNSERVABLE")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/3/bindings", nil))
		require.Len(t, decodeBindings(t, rec), 2, "previous bindings are untouched")
	})

	t.Run("unknown account and entry", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3/bindings", bytes.NewBufferString(`{"bindings":[{"account_id":99}]}`)))
		require.Equal(t, http.StatusNotFound, rec.Code)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/42/bindings", bytes.NewBufferString(`{"bindings":[]}`)))
		require.Equal(t, http.StatusNotFound, rec.Code)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3/bindings", bytes.NewBufferString(`{"bindings":[{"priority":1}]}`)))
		require.Equal(t, http.StatusBadRequest, rec.Code, "account_id is required")
	})

}

func TestModelCatalogHandler_Diagnose(t *testing.T) {
	price := 3e-6
	repo := &catalogRepoStub{entries: []service.ModelCatalogEntry{{
		ID: 3, ModelID: "claude-sonnet-4", Vendor: "anthropic",
		BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed,
		ManagedBy: service.ModelCatalogManagedByAdmin, InputPrice: &price,
	}}}
	accounts := catalogAccountsStub{
		1: {ID: 1, Name: "chat-only", Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true,
			ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://cc.example.com"}},
		2: {ID: 2, Name: "openai-oauth-disabled", Type: service.AccountTypeOAuth, Platform: service.PlatformOpenAI, Status: service.StatusDisabled, Schedulable: true},
	}
	router := newCatalogRouter(newCatalogHandlerWithAccounts(repo, accounts))

	priority := 4
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/entries/3/bindings", bytes.NewBufferString(`{"bindings":[{"account_id":1,"priority":4},{"account_id":2}]}`)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/3/diagnosis", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	envelope := decodeCatalogResponse(t, rec)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var got ModelCatalogDiagnosisResponse
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, int64(3), got.EntryID)
	require.Len(t, got.Accounts, 2)

	require.Equal(t, "chat-only", got.Accounts[0].Name)
	require.Equal(t, &priority, got.Accounts[0].Priority)
	require.True(t, got.Accounts[0].Schedulable)
	require.Empty(t, got.Accounts[0].BlockedReason)
	require.Equal(t, map[string]bool{
		service.APIProtocolAnthropic: true, service.APIProtocolChatCompletions: true,
		service.APIProtocolResponses: true, service.APIProtocolGemini: false,
	}, got.Accounts[0].Serves, "messages / responses convert to the chat address; nothing converts to gemini")

	require.Equal(t, "openai-oauth-disabled", got.Accounts[1].Name)
	require.False(t, got.Accounts[1].Schedulable)
	require.Equal(t, "disabled", got.Accounts[1].BlockedReason)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/entries/42/diagnosis", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func catalogPrice(v float64) *float64 { return &v }

// 渠道表单里直接勾选承接的模型（2026-09-25）：按渠道读写整份条目列表，校验走服务层。
func TestModelCatalogHandler_AccountCatalogEntries(t *testing.T) {
	repo := &catalogRepoStub{entries: []service.ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4", Vendor: "anthropic", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed, InputPrice: catalogPrice(1e-6)},
		{ID: 2, ModelID: "claude-opus-4", Vendor: "anthropic", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed, InputPrice: catalogPrice(1e-6)},
	}}
	h := newCatalogHandlerWithAccounts(repo, catalogAccountsStub{
		5: {ID: 5, Type: service.AccountTypeAPIKey, Platform: service.PlatformAnthropic, ProtocolEndpoints: map[string]string{service.APIProtocolAnthropic: "https://relay.example.com"}},
		6: {ID: 6, Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI},
	})
	router := newCatalogRouter(h)
	entryIDs := func(rec *httptest.ResponseRecorder) []int64 {
		envelope := decodeCatalogResponse(t, rec)
		raw, err := json.Marshal(envelope.Data)
		require.NoError(t, err)
		var body struct {
			EntryIDs []int64 `json:"entry_ids"`
		}
		require.NoError(t, json.Unmarshal(raw, &body))
		return body.EntryIDs
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/accounts/5/catalog-entries", strings.NewReader(`{"entry_ids":[2,1]}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{1, 2}, entryIDs(rec))

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/accounts/5/catalog-entries", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{1, 2}, entryIDs(rec))

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/accounts/6/catalog-entries", strings.NewReader(`{"entry_ids":[1]}`)))
	require.Equal(t, http.StatusBadRequest, rec.Code, "没配地址的 key 承接不了")

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/accounts/99/catalog-entries", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// 「添加模型」按模型 ID 带价：没有价格服务时恒为 found=false；缺 model_id 报 400。
func TestModelCatalogHandler_PriceLookup(t *testing.T) {
	router := newCatalogRouter(newCatalogHandler(&catalogRepoStub{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/price-lookup?model_id=claude-sonnet-4", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	envelope := decodeCatalogResponse(t, rec)
	require.Equal(t, map[string]any{"found": false}, envelope.Data)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/price-lookup", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
