package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2QueryListSupportsRepeatedAndCommaValues(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest("GET", "/?platform=openai,grok&platform=anthropic", nil)
	require.Equal(t, []string{"openai", "grok", "anthropic"}, queryList(c, "platform"))
}

func TestChannelMonitorV2GroupByQueryDefaultsAndRejectsInvalid(t *testing.T) {
	groupBy, err := service.ParseChannelMonitorV2GroupBy("", true)
	require.NoError(t, err)
	require.Equal(t, service.ChannelMonitorV2GroupByPlatformModel, groupBy)
	groupBy, err = service.ParseChannelMonitorV2GroupBy("", false)
	require.NoError(t, err)
	require.Equal(t, service.ChannelMonitorV2GroupByModel, groupBy)
	_, err = service.ParseChannelMonitorV2GroupBy("invalid", true)
	require.Error(t, err)
	_, err = service.ParseChannelMonitorV2GroupBy("platform_group", true)
	require.Error(t, err)
}

func TestChannelMonitorV2MatrixHandlerRejectsInvalidGroupBy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/matrix?group_by=invalid", nil)
	h := NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(nil))
	h.Matrix(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

// 上游渠道只对管理员可见：普通用户传的 platform 筛选被丢弃，不报错也不生效。
func TestChannelMonitorV2ParseFilterDropsPlatformForOrdinaryUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(nil))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot?platform=openai&model=gpt-5", nil)
	filter, ok := h.parseFilter(c)
	require.True(t, ok)
	require.Empty(t, filter.Platforms)
	require.Equal(t, []string{"gpt-5"}, filter.Models)
}
