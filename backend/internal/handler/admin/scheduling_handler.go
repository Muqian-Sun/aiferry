package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SchedulingHandler 管理站「调度」页。
type SchedulingHandler struct {
	service *service.SchedulingOverviewService
}

// NewSchedulingHandler 创建调度页处理器。
func NewSchedulingHandler(svc *service.SchedulingOverviewService) *SchedulingHandler {
	return &SchedulingHandler{service: svc}
}

// Overview 每个模型的调度线：渠道此刻的调度顺序或不被选的原因，加上时间窗内的请求数。
// GET /api/v1/admin/scheduling/overview?window=15m|1h|24h（默认 1h）
func (h *SchedulingHandler) Overview(c *gin.Context) {
	overview, err := h.service.Overview(c.Request.Context(), c.DefaultQuery("window", "1h"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}
