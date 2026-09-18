//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// catalogRepoStub 是管理端测试用的内存仓储，复刻服务层依赖的接口。
type catalogRepoStub struct {
	mu      sync.Mutex
	entries []service.ModelCatalogEntry
	seed    service.ModelCatalogSeedResult
	seedErr error
}

func (r *catalogRepoStub) ListEntries(context.Context) ([]service.ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]service.ModelCatalogEntry, len(r.entries))
	copy(out, r.entries)
	return out, nil
}

func (r *catalogRepoStub) GetEntryByID(_ context.Context, id int64) (*service.ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == id {
			cp := r.entries[i]
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

func newCatalogHandler(repo *catalogRepoStub) *ModelCatalogHandler {
	svc := service.NewModelCatalogService(repo, nil, service.ModelCatalogSeedInput{})
	return NewModelCatalogHandler(svc)
}

func newCatalogRouter(h *ModelCatalogHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/entries", h.ListEntries)
	r.GET("/entries/:id", h.GetEntry)
	r.POST("/entries", h.CreateEntry)
	r.PUT("/entries/:id", h.UpdateEntry)
	r.DELETE("/entries/:id", h.DeleteEntry)
	r.POST("/aliases", h.CreateAlias)
	r.PUT("/aliases/:id", h.UpdateAlias)
	r.DELETE("/aliases/:id", h.DeleteAlias)
	r.POST("/seed", h.Seed)
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
		{ID: 1, ModelID: "claude-sonnet-4", BillingMode: service.BillingModeToken, Status: service.ModelCatalogStatusListed},
	}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/entries", nil)
	newCatalogRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	envelope := decodeCatalogResponse(t, rec)
	require.Equal(t, 0, envelope.Code)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var entries []service.ModelCatalogEntry
	require.NoError(t, json.Unmarshal(raw, &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "claude-sonnet-4", entries[0].ModelID)
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
