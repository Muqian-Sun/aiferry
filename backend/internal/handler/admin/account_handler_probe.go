package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ProbeUpstreamModelsRequest 建 / 改渠道时「探测模型」：用表单里的协议地址与 key 临时拼一个第三方 key 向上游要模型名单。
// 编辑已有渠道时表单拿不到明文 key，传 account_id 用存着的 key（地址仍以表单为准，可能刚改过）。
// proxy_id 是表单里选的代理：有的上游只有走代理才连得上，探测与真实请求走同一条路。
type ProbeUpstreamModelsRequest struct {
	APIKey            string            `json:"api_key"`
	AccountID         int64             `json:"account_id"`
	ProxyID           *int64            `json:"proxy_id"`
	ProtocolEndpoints map[string]string `json:"protocol_endpoints" binding:"required"`
}

// ProbeUpstreamModels 返回上游模型名单（去重排序），不写库、不补元数据。
// POST /api/v1/admin/accounts/models/probe
func (h *AccountHandler) ProbeUpstreamModels(c *gin.Context) {
	var req ProbeUpstreamModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	endpoints, err := service.NormalizeProtocolEndpoints(req.ProtocolEndpoints)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(endpoints) == 0 {
		response.BadRequest(c, "protocol_endpoints is required")
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	var proxy *service.Proxy
	var proxyID *int64
	if req.ProxyID != nil && *req.ProxyID > 0 {
		p, err := h.adminService.GetProxy(c.Request.Context(), *req.ProxyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		proxy, proxyID = p, req.ProxyID
	}
	if apiKey == "" && req.AccountID > 0 {
		account, err := h.adminService.GetAccount(c.Request.Context(), req.AccountID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if !account.IsThirdPartyKey() {
			response.BadRequest(c, "Model probing only applies to API key channels")
			return
		}
		apiKey = strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
		if proxyID == nil {
			proxy, proxyID = account.Proxy, account.ProxyID
		}
	}
	if apiKey == "" {
		response.BadRequest(c, "api_key is required")
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "Account test service is not configured")
		return
	}

	temp := &service.Account{
		Type:              service.AccountTypeAPIKey,
		Platform:          service.PlatformOpenAI, // 只是标签：key 的探测只看协议地址
		Credentials:       map[string]any{"api_key": apiKey},
		ProtocolEndpoints: endpoints,
		Proxy:             proxy,
		ProxyID:           proxyID,
	}
	models, err := h.accountTestService.ProbeUpstreamModels(c.Request.Context(), temp)
	if err != nil {
		var syncErr *service.UpstreamModelSyncError
		if errors.As(err, &syncErr) {
			switch syncErr.Kind {
			case service.UpstreamModelSyncErrorConfiguration, service.UpstreamModelSyncErrorUnsupported:
				response.BadRequest(c, syncErr.SafeMessage())
			case service.UpstreamModelSyncErrorInternal:
				response.InternalError(c, syncErr.SafeMessage())
			default:
				slog.Warn("probe_upstream_models_failed", "kind", syncErr.Kind)
				response.Error(c, http.StatusBadGateway, syncErr.SafeMessage())
			}
			return
		}
		slog.Warn("probe_upstream_models_failed")
		response.Error(c, http.StatusBadGateway, "Failed to fetch upstream model list")
		return
	}
	response.Success(c, gin.H{"models": models})
}
