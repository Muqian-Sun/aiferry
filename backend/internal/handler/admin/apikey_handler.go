package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminAPIKeyHandler handles admin API key management
type AdminAPIKeyHandler struct {
	adminService service.AdminService
}

// NewAdminAPIKeyHandler creates a new admin API key handler
func NewAdminAPIKeyHandler(adminService service.AdminService) *AdminAPIKeyHandler {
	return &AdminAPIKeyHandler{
		adminService: adminService,
	}
}

// AdminUpdateAPIKeyRequest represents the admin-managed fields of an API key.
// 分组绑定已删（7b-3b-3），只剩限速用量重置。
type AdminUpdateAPIKeyRequest struct {
	ResetRateLimitUsage *bool `json:"reset_rate_limit_usage"` // true=重置 5h/1d/7d 限速用量
}

// UpdateKey handles updating an API key's admin-managed fields.
// PUT /api/v1/admin/api-keys/:id
func (h *AdminAPIKeyHandler) UpdateKey(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	var req AdminUpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ResetRateLimitUsage == nil || !*req.ResetRateLimitUsage {
		response.BadRequest(c, "reset_rate_limit_usage is required")
		return
	}

	apiKey, err := h.adminService.AdminResetAPIKeyRateLimitUsage(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"api_key": dto.APIKeyFromService(apiKey)})
}
