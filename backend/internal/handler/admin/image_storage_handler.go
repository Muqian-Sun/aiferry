package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ImageStorageHandler 异步生图结果对象存储（S3 兼容）的后台配置接口。
type ImageStorageHandler struct {
	imageStorage *service.ImageStorageSettingService
}

func NewImageStorageHandler(imageStorage *service.ImageStorageSettingService) *ImageStorageHandler {
	return &ImageStorageHandler{imageStorage: imageStorage}
}

// GetConfig GET /api/v1/admin/image-storage
func (h *ImageStorageHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()
	cfg, err := h.imageStorage.Get(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"config":            cfg,
		"secret_configured": h.imageStorage.SecretConfigured(ctx),
	})
}

// UpdateConfig PUT /api/v1/admin/image-storage
func (h *ImageStorageHandler) UpdateConfig(c *gin.Context) {
	var req service.ImageStorageSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.imageStorage.Update(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// TestConnection POST /api/v1/admin/image-storage/test
func (h *ImageStorageHandler) TestConnection(c *gin.Context) {
	var req service.ImageStorageSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.imageStorage.TestConnection(c.Request.Context(), req); err != nil {
		response.Success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	response.Success(c, gin.H{"ok": true, "message": "connection successful"})
}
