package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type settingHandlerRepoStub struct {
	values      map[string]string
	lastUpdates map[string]string
}

func (s *settingHandlerRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *settingHandlerRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values != nil {
		if value, ok := s.values[key]; ok {
			return value, nil
		}
	}
	return "", nil
}

func (s *settingHandlerRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingHandlerRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingHandlerRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.lastUpdates = make(map[string]string, len(settings))
	for key, value := range settings {
		s.lastUpdates[key] = value
		if s.values == nil {
			s.values = map[string]string{}
		}
		s.values[key] = value
	}
	return nil
}

func (s *settingHandlerRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *settingHandlerRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingHandler_UpdateSettings_PersistsPaymentVisibleMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{},
	}
	svc := service.NewSettingService(repo, &config.Config{})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	body := map[string]any{
		"invitation_code_enabled":               true,
		"payment_visible_method_alipay_source":  "easypay",
		"payment_visible_method_wxpay_source":   "wxpay",
		"payment_visible_method_alipay_enabled": true,
		"payment_visible_method_wxpay_enabled":  false,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.VisibleMethodSourceEasyPayAlipay, repo.values[service.SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, service.VisibleMethodSourceOfficialWechat, repo.values[service.SettingPaymentVisibleMethodWxpaySource])
	require.Equal(t, "true", repo.values[service.SettingPaymentVisibleMethodAlipayEnabled])
	require.Equal(t, "false", repo.values[service.SettingPaymentVisibleMethodWxpayEnabled])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, service.VisibleMethodSourceEasyPayAlipay, data["payment_visible_method_alipay_source"])
	require.Equal(t, service.VisibleMethodSourceOfficialWechat, data["payment_visible_method_wxpay_source"])
	require.Equal(t, true, data["payment_visible_method_alipay_enabled"])
	require.Equal(t, false, data["payment_visible_method_wxpay_enabled"])
}

func TestSettingHandler_UpdateSettings_PreservesLegacyBlankPaymentVisibleMethodSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingPaymentVisibleMethodAlipayEnabled: "true",
			service.SettingPaymentVisibleMethodAlipaySource:  "",
			service.SettingPaymentVisibleMethodWxpayEnabled:  "false",
			service.SettingPaymentVisibleMethodWxpaySource:   "",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	body := map[string]any{
		"invitation_code_enabled": false,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "", repo.values[service.SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, "true", repo.values[service.SettingPaymentVisibleMethodAlipayEnabled])
}

func TestSettingHandler_UpdateSettings_RejectsInvalidPaymentVisibleMethodSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{},
	}
	svc := service.NewSettingService(repo, &config.Config{})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	body := map[string]any{
		"invitation_code_enabled":              true,
		"payment_visible_method_alipay_source": "bogus",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, repo.values, service.SettingPaymentVisibleMethodAlipaySource)
}
