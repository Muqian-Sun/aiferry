package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelMonitorV2Handler 用户站「服务状态」页的两个读数（对未登录访客公开，只回 dto.ServiceStatus* 白名单）
// 与管理站「渠道状态」页的读数（挂在管理员路由下）。
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

// AdminChannels 管理站渠道状态：按渠道看可用率、首字延迟、缓存命中率与请求数；?model= 只看某个上架模型。
func (h *ChannelMonitorV2Handler) AdminChannels(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	result, err := h.service.Channels(c.Request.Context(), filter, c.Query("model"))
	if err != nil {
		if errors.Is(err, service.ErrChannelMonitorV2InvalidModel) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminChannelStatusFromService(result))
}

func (h *ChannelMonitorV2Handler) parseFilter(c *gin.Context) (service.ChannelMonitorV2Filter, bool) {
	filter, err := h.service.ParseFilter(c.Query("range"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return service.ChannelMonitorV2Filter{}, false
	}
	return filter, true
}
