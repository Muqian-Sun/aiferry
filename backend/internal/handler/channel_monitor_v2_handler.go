package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelMonitorV2Handler 用户站「服务状态」页的两个读数，对未登录访客公开，只回 dto.ServiceStatus* 白名单。
type ChannelMonitorV2Handler struct {
	service *service.ChannelMonitorV2Service
}

func NewChannelMonitorV2Handler(svc *service.ChannelMonitorV2Service) *ChannelMonitorV2Handler {
	return &ChannelMonitorV2Handler{service: svc}
}

// Snapshot 全站整体：可用率、首字延迟与逐段趋势。
func (h *ChannelMonitorV2Handler) Snapshot(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	result, err := h.service.Snapshot(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ServiceStatusSnapshotFromService(result))
}

// Matrix 上架目录里每个模型一行，带逐段数据。
func (h *ChannelMonitorV2Handler) Matrix(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	result, err := h.service.Matrix(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ServiceStatusMatrixFromService(result))
}

func (h *ChannelMonitorV2Handler) parseFilter(c *gin.Context) (service.ChannelMonitorV2Filter, bool) {
	filter, err := h.service.ParseFilter(c.Query("range"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return service.ChannelMonitorV2Filter{}, false
	}
	return filter, true
}
