package admin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type cleanupRepoStub struct {
	mu         sync.Mutex
	created    []*service.UsageCleanupTask
	listTasks  []service.UsageCleanupTask
	listResult *pagination.PaginationResult
	listErr    error
	statusByID map[int64]string
	countValue int64
	counted    []service.UsageCleanupFilters
}

func (s *cleanupRepoStub) CreateTask(ctx context.Context, task *service.UsageCleanupTask) error {
	if task == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if task.ID == 0 {
		task.ID = int64(len(s.created) + 1)
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	task.UpdatedAt = task.CreatedAt
	clone := *task
	s.created = append(s.created, &clone)
	return nil
}

func (s *cleanupRepoStub) ListTasks(ctx context.Context, params pagination.PaginationParams) ([]service.UsageCleanupTask, *pagination.PaginationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listTasks, s.listResult, s.listErr
}

func (s *cleanupRepoStub) ClaimNextPendingTask(ctx context.Context, staleRunningAfterSeconds int64) (*service.UsageCleanupTask, error) {
	return nil, nil
}

func (s *cleanupRepoStub) GetTaskStatus(ctx context.Context, taskID int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statusByID == nil {
		return "", sql.ErrNoRows
	}
	status, ok := s.statusByID[taskID]
	if !ok {
		return "", sql.ErrNoRows
	}
	return status, nil
}

func (s *cleanupRepoStub) UpdateTaskProgress(ctx context.Context, taskID int64, deletedRows int64) error {
	return nil
}

func (s *cleanupRepoStub) CancelTask(ctx context.Context, taskID int64, canceledBy int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statusByID == nil {
		s.statusByID = map[int64]string{}
	}
	status := s.statusByID[taskID]
	if status != service.UsageCleanupStatusPending && status != service.UsageCleanupStatusRunning {
		return false, nil
	}
	s.statusByID[taskID] = service.UsageCleanupStatusCanceled
	return true, nil
}

func (s *cleanupRepoStub) MarkTaskSucceeded(ctx context.Context, taskID int64, deletedRows int64) error {
	return nil
}

func (s *cleanupRepoStub) MarkTaskFailed(ctx context.Context, taskID int64, deletedRows int64, errorMsg string) error {
	return nil
}

func (s *cleanupRepoStub) DeleteUsageLogsBatch(ctx context.Context, filters service.UsageCleanupFilters, limit int) (int64, error) {
	return 0, nil
}

func (s *cleanupRepoStub) CountUsageLogs(ctx context.Context, filters service.UsageCleanupFilters) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counted = append(s.counted, filters)
	return s.countValue, nil
}

var _ service.UsageCleanupRepository = (*cleanupRepoStub)(nil)

func setupCleanupRouter(cleanupService *service.UsageCleanupService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if userID > 0 {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
			c.Next()
		})
	}

	handler := NewUsageHandler(nil, nil, nil, cleanupService)
	router.POST("/api/v1/admin/usage/cleanup-tasks", handler.CreateCleanupTask)
	router.POST("/api/v1/admin/usage/cleanup-tasks/preview", handler.PreviewCleanupTask)
	router.GET("/api/v1/admin/usage/cleanup-tasks", handler.ListCleanupTasks)
	router.POST("/api/v1/admin/usage/cleanup-tasks/:id/cancel", handler.CancelCleanupTask)
	return router
}

func TestUsageHandlerCreateCleanupTaskUnauthorized(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 0)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskUnavailable(t *testing.T) {
	router := setupCleanupRouter(nil, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskBindError(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 88)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewBufferString("{bad-json"))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskMissingRange(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 88)

	payload := map[string]any{
		"start_date": "2024-01-01",
		"timezone":   "UTC",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskInvalidDate(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 88)

	payload := map[string]any{
		"start_date": "2024-13-01",
		"end_date":   "2024-01-02",
		"timezone":   "UTC",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskInvalidEndDate(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 88)

	payload := map[string]any{
		"start_date": "2024-01-01",
		"end_date":   "2024-02-40",
		"timezone":   "UTC",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskInvalidRequestType(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 88)

	payload := map[string]any{
		"start_date":   "2024-01-01",
		"end_date":     "2024-01-02",
		"timezone":     "UTC",
		"request_type": "invalid",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUsageHandlerCreateCleanupTaskRequestTypePriority(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 99)

	payload := map[string]any{
		"start_date":   "2024-01-01",
		"end_date":     "2024-01-02",
		"timezone":     "UTC",
		"request_type": "ws_v2",
		"stream":       false,
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.created, 1)
	created := repo.created[0]
	require.NotNil(t, created.Filters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *created.Filters.RequestType)
	require.Nil(t, created.Filters.Stream)
}

func TestUsageHandlerCreateCleanupTaskWithLegacyStream(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 99)

	payload := map[string]any{
		"start_date": "2024-01-01",
		"end_date":   "2024-01-02",
		"timezone":   "UTC",
		"stream":     true,
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.created, 1)
	created := repo.created[0]
	require.Nil(t, created.Filters.RequestType)
	require.NotNil(t, created.Filters.Stream)
	require.True(t, *created.Filters.Stream)
}

func TestUsageHandlerCreateCleanupTaskSuccess(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 99)

	payload := map[string]any{
		"start_date": " 2024-01-01 ",
		"end_date":   "2024-01-02",
		"timezone":   "UTC",
		"model":      "gpt-4",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.created, 1)
	created := repo.created[0]
	require.Equal(t, int64(99), created.CreatedBy)
	require.NotNil(t, created.Filters.Model)
	require.Equal(t, "gpt-4", *created.Filters.Model)

	// 按天：与列表同一个解析，半开区间，结束日次日零点
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	require.True(t, created.Filters.StartTime.Equal(start))
	require.True(t, created.Filters.EndTime.Equal(end))
}

func postCleanupJSON(t *testing.T, router *gin.Engine, path string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// 用量页「近 24 小时」：按精确时刻删，不按自然日扩成两整天；页面上的每个筛选（含用户）都带上（2026-10-04 D8）
func TestUsageHandlerCreateCleanupTaskExactWindowWithAllPageFilters(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	router := setupCleanupRouter(service.NewUsageCleanupService(repo, nil, nil, cfg), 99)

	recorder := postCleanupJSON(t, router, "/api/v1/admin/usage/cleanup-tasks", map[string]any{
		"start_time":              "2024-01-01T14:06:00Z",
		"end_time":                "2024-01-02T14:06:00Z",
		"user_id":                 7,
		"api_key_id":              8,
		"account_id":              9,
		"model":                   "claude-sonnet-4-6",
		"native_compaction_v2":    true,
		"billing_type":            0,
		"billing_mode":            "token",
		"upstream_model_mismatch": false,
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.created, 1)
	filters := repo.created[0].Filters
	require.True(t, filters.StartTime.Equal(time.Date(2024, 1, 1, 14, 6, 0, 0, time.UTC)))
	require.True(t, filters.EndTime.Equal(time.Date(2024, 1, 2, 14, 6, 0, 0, time.UTC)))
	require.Equal(t, int64(7), *filters.UserID)
	require.Equal(t, int64(8), *filters.APIKeyID)
	require.Equal(t, int64(9), *filters.AccountID)
	require.Equal(t, "claude-sonnet-4-6", *filters.Model)
	require.True(t, *filters.NativeCompactionV2)
	require.Equal(t, int8(0), *filters.BillingType)
	require.Equal(t, "token", *filters.BillingMode)
	require.False(t, *filters.UpstreamModelMismatch)
}

// 预览数的就是随后要删的：同一个请求体，预览带去数的条件与建任务存下的条件完全相同
func TestUsageHandlerPreviewCleanupTaskCountsWhatCreateDeletes(t *testing.T) {
	repo := &cleanupRepoStub{countValue: 128}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	router := setupCleanupRouter(service.NewUsageCleanupService(repo, nil, nil, cfg), 99)
	payload := map[string]any{
		"start_date":   "2024-01-01",
		"end_date":     "2024-01-07",
		"timezone":     "Asia/Shanghai",
		"user_id":      7,
		"model":        " gpt-5 ",
		"request_type": "stream",
		"stream":       true,
		"billing_mode": "token",
	}

	recorder := postCleanupJSON(t, router, "/api/v1/admin/usage/cleanup-tasks/preview", payload)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, int64(128), resp.Data.Count)

	recorder = postCleanupJSON(t, router, "/api/v1/admin/usage/cleanup-tasks", payload)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.counted, 1)
	require.Len(t, repo.created, 1)
	require.Equal(t, repo.counted[0], repo.created[0].Filters)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	require.True(t, repo.counted[0].StartTime.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, shanghai)))
	require.True(t, repo.counted[0].EndTime.Equal(time.Date(2024, 1, 8, 0, 0, 0, 0, shanghai)))
}

func TestUsageHandlerPreviewCleanupTaskRejectsBadRange(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	router := setupCleanupRouter(service.NewUsageCleanupService(repo, nil, nil, cfg), 99)

	for name, payload := range map[string]map[string]any{
		"mixed forms": {"start_time": "2024-01-01T00:00:00Z", "start_date": "2024-01-01", "end_date": "2024-01-02"},
		"open end":    {"start_time": "2024-01-01T00:00:00Z"},
		"reversed":    {"start_time": "2024-01-02T00:00:00Z", "end_time": "2024-01-01T00:00:00Z"},
		"too long":    {"start_date": "2024-01-01", "end_date": "2024-02-05", "timezone": "UTC"},
	} {
		recorder := postCleanupJSON(t, router, "/api/v1/admin/usage/cleanup-tasks/preview", payload)
		require.Equal(t, http.StatusBadRequest, recorder.Code, name)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Empty(t, repo.counted)
}

func TestUsageHandlerListCleanupTasksUnavailable(t *testing.T) {
	router := setupCleanupRouter(nil, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage/cleanup-tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestUsageHandlerListCleanupTasksSuccess(t *testing.T) {
	repo := &cleanupRepoStub{}
	repo.listTasks = []service.UsageCleanupTask{
		{
			ID:        7,
			Status:    service.UsageCleanupStatusSucceeded,
			CreatedBy: 4,
		},
	}
	repo.listResult = &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage/cleanup-tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []dto.UsageCleanupTask `json:"items"`
			Total int64                  `json:"total"`
			Page  int                    `json:"page"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, int64(7), resp.Data.Items[0].ID)
	require.Equal(t, int64(1), resp.Data.Total)
	require.Equal(t, 1, resp.Data.Page)
}

func TestUsageHandlerListCleanupTasksError(t *testing.T) {
	repo := &cleanupRepoStub{listErr: errors.New("boom")}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true, MaxRangeDays: 31}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage/cleanup-tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestUsageHandlerCancelCleanupTaskUnauthorized(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 0)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks/1/cancel", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsageHandlerCancelCleanupTaskNotFound(t *testing.T) {
	repo := &cleanupRepoStub{}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks/999/cancel", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUsageHandlerCancelCleanupTaskConflict(t *testing.T) {
	repo := &cleanupRepoStub{statusByID: map[int64]string{2: service.UsageCleanupStatusSucceeded}}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks/2/cancel", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestUsageHandlerCancelCleanupTaskSuccess(t *testing.T) {
	repo := &cleanupRepoStub{statusByID: map[int64]string{3: service.UsageCleanupStatusPending}}
	cfg := &config.Config{UsageCleanup: config.UsageCleanupConfig{Enabled: true}}
	cleanupService := service.NewUsageCleanupService(repo, nil, nil, cfg)
	router := setupCleanupRouter(cleanupService, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks/3/cancel", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
