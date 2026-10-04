//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/require"
)

// Saving settings is a whole-document PUT. A client that sends only the field it
// cares about must not reset everything else: a payload as small as
// `{"risk_control_enabled":true}` used to clear site_name, after which
// getStringOrDefault rendered the empty value as the built-in default and the
// login page silently changed name.

func TestUpdateSettingsPartialPayloadKeepsUnsentKeys(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyProfitMinMargin:              "0.30000000",
		service.SettingKeyOpsQueryModeDefault:          "raw",
		service.SettingKeyOpsMetricsIntervalSeconds:    "120",
		service.SettingKeyOpsRealtimeMonitoringEnabled: "false",
	})

	rec := doUpdateSettings(t, h, map[string]any{"risk_control_enabled": true}, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, "true", repo.values[service.SettingKeyRiskControlEnabled],
		"the field the caller actually sent must be written")

	require.Equal(t, "0.30000000", repo.values[service.SettingKeyProfitMinMargin])
	require.Equal(t, "raw", repo.values[service.SettingKeyOpsQueryModeDefault])
	require.Equal(t, "120", repo.values[service.SettingKeyOpsMetricsIntervalSeconds])
	require.Equal(t, "false", repo.values[service.SettingKeyOpsRealtimeMonitoringEnabled])
}

// A full payload keeps whole-document semantics: fields explicitly set to their
// zero value are still cleared.
func TestUpdateSettingsFullPayloadStillClearsSentEmptyFields(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyProfitMinMargin: "0.30000000",
	})

	rec := doUpdateSettings(t, h, map[string]any{"profit_min_margin": 0}, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, "0.00000000", repo.values[service.SettingKeyProfitMinMargin],
		"an explicitly sent zero value is a deliberate clear, not an omission")
}

// newStepUpSwitchTestHandler 设置保存用的最小 handler（名字沿用旧文件）。
func newStepUpSwitchTestHandler(t *testing.T, stored map[string]string) (*SettingHandler, *settingHandlerRepoStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: stored}
	svc := service.NewSettingService(repo, &config.Config{})
	return NewSettingHandler(svc, nil, nil), repo
}

func doUpdateSettings(t *testing.T, h *SettingHandler, body map[string]any, prepare func(c *gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")
	if prepare != nil {
		prepare(c)
	}

	h.UpdateSettings(c)
	return rec
}

// 设置页每一节单独保存，只写这一节带的键：原来一次保存把 16 个键全写一遍（没带的写回保存前读到的值），
// 两个管理员同时改不同的节，后保存的会把先保存的改回去（2026-10-04 D7）。
func TestUpdateSettingsWritesOnlySentKeys(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyProfitMinMargin:     "0.30000000",
		service.SettingKeyRiskControlEnabled:  "false",
		service.SettingKeyOpsQueryModeDefault: "raw",
	})

	rec := doUpdateSettings(t, h, map[string]any{"profit_min_margin": 0.2}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, map[string]string{service.SettingKeyProfitMinMargin: "0.20000000"}, repo.lastUpdates)

	// 另一节（开关）只写自己的键，不碰刚才那一节
	rec = doUpdateSettings(t, h, map[string]any{"risk_control_enabled": true}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, map[string]string{service.SettingKeyRiskControlEnabled: "true"}, repo.lastUpdates)
	require.Equal(t, "0.20000000", repo.values[service.SettingKeyProfitMinMargin])
}
