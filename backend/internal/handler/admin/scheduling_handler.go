package admin

import (
	"strconv"

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

// Traces 请求级调度轨迹，新的在前。
// GET /api/v1/admin/scheduling/traces?window=&entry_id=&protocol=&outcome=&account_id=&page=&page_size=
// outcome：success / failed / no_channel / canceled / switched（换过渠道）。
func (h *SchedulingHandler) Traces(c *gin.Context) {
	filter := service.SchedulingTraceFilter{
		CatalogEntryID:  queryInt64(c, "entry_id"),
		InboundProtocol: c.Query("protocol"),
		Outcome:         c.Query("outcome"),
		AccountID:       queryInt64(c, "account_id"),
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.service.ListTraces(c.Request.Context(), c.DefaultQuery("window", "1h"), filter, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func queryInt64(c *gin.Context, key string) int64 {
	value, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return value
}
