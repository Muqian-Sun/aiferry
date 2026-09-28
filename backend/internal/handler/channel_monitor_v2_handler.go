package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ChannelMonitorV2Handler struct {
	service *service.ChannelMonitorV2Service
}

func NewChannelMonitorV2Handler(svc *service.ChannelMonitorV2Service) *ChannelMonitorV2Handler {
	return &ChannelMonitorV2Handler{service: svc}
}

// channelMonitorV2IsAdmin is true when the request already passed admin auth
// (shared Dimensions/Errors handlers serve both user and admin route groups).
func channelMonitorV2IsAdmin(c *gin.Context) bool {
	role, ok := middleware.GetUserRoleFromContext(c)
	return ok && role == service.RoleAdmin
}

func (h *ChannelMonitorV2Handler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ChannelMonitorV2Handler) UpdateConfig(c *gin.Context) {
	var input service.ChannelMonitorV2Config
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid channel monitor v2 config")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "user not found in context")
		return
	}
	updated, err := h.service.UpdateConfig(c.Request.Context(), input, input.Version, subject.UserID)
	if err != nil {
		if errors.Is(err, service.ErrChannelMonitorV2ConfigConflict) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, service.ErrChannelMonitorV2InvalidConfig) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ChannelMonitorV2Handler) Dimensions(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	admin := channelMonitorV2IsAdmin(c)
	result, err := h.service.Dimensions(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 管理员与用户共用这个 handler：用户站只拿模型名（服务状态页的白名单结构）。
	if !admin {
		response.Success(c, dto.ServiceStatusDimensionsFromService(result))
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) Snapshot(c *gin.Context)      { h.snapshot(c, false) }
func (h *ChannelMonitorV2Handler) AdminSnapshot(c *gin.Context) { h.snapshot(c, true) }
func (h *ChannelMonitorV2Handler) Models(c *gin.Context)        { h.models(c, false) }
func (h *ChannelMonitorV2Handler) AdminModels(c *gin.Context)   { h.models(c, true) }
func (h *ChannelMonitorV2Handler) Matrix(c *gin.Context)        { h.matrix(c, false) }
func (h *ChannelMonitorV2Handler) AdminMatrix(c *gin.Context)   { h.matrix(c, true) }
func (h *ChannelMonitorV2Handler) Users(c *gin.Context)         { h.users(c, false) }
func (h *ChannelMonitorV2Handler) AdminUsers(c *gin.Context)    { h.users(c, true) }

func (h *ChannelMonitorV2Handler) snapshot(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	result, err := h.service.Snapshot(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin {
		response.Success(c, dto.ServiceStatusSnapshotFromService(result))
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) models(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	result, err := h.service.Models(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin {
		response.Success(c, dto.ServiceStatusModelsFromService(result))
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) matrix(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	groupBy, err := service.ParseChannelMonitorV2GroupBy(c.Query("group_by"), admin)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.service.Matrix(c.Request.Context(), filter, groupBy, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin {
		response.Success(c, dto.ServiceStatusMatrixFromService(result))
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) Errors(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	admin := channelMonitorV2IsAdmin(c)
	result, err := h.service.ErrorsForViewer(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin {
		response.Success(c, dto.ServiceStatusErrorsFromService(result))
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) users(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	subject, exists := middleware.GetAuthSubjectFromContext(c)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "user not found in context")
		return
	}
	result, err := h.service.Users(c.Request.Context(), filter, subject.UserID, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin {
		response.Success(c, dto.ServiceStatusSelfFromService(result))
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) parseFilter(c *gin.Context) (service.ChannelMonitorV2Filter, bool) {
	// 上游渠道只对管理员可见：普通用户传的 platform 筛选直接丢弃（不报错，
	// 免得老链接 400），管理员保持原有的平台多选。
	platforms := []string{}
	if channelMonitorV2IsAdmin(c) {
		platforms = queryList(c, "platform")
	}
	filter, err := h.service.ParseFilter(c.Query("range"), platforms, queryList(c, "model"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return service.ChannelMonitorV2Filter{}, false
	}
	return filter, true
}

func queryList(c *gin.Context, key string) []string {
	values := c.QueryArray(key)
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}
