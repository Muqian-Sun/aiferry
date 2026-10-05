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

	temp, ok := h.probeKeyAccount(c, req.APIKey, req.AccountID, req.ProxyID)
	if !ok {
		return
	}
	temp.ProtocolEndpoints = endpoints
	models, err := h.accountTestService.ProbeUpstreamModels(c.Request.Context(), temp)
	if err != nil {
		writeProbeError(c, err, "probe_upstream_models_failed", "Failed to fetch upstream model list")
		return
	}
	response.Success(c, gin.H{"models": models})
}

// ProbeUpstreamProtocolsRequest 建 / 改渠道时「探测协议」：对表单里的一个地址逐个试四个上游协议。
// key 与代理的取法同探测模型（编辑已有渠道不改 key 时传 account_id）。
type ProbeUpstreamProtocolsRequest struct {
	BaseURL   string `json:"base_url" binding:"required"`
	APIKey    string `json:"api_key"`
	AccountID int64  `json:"account_id"`
	ProxyID   *int64 `json:"proxy_id"`
}

// ProbeUpstreamProtocols 返回四个协议各自的探测结果（支持 / 不支持 / 不确定），不写库。
// POST /api/v1/admin/accounts/protocols/probe
func (h *AccountHandler) ProbeUpstreamProtocols(c *gin.Context) {
	var req ProbeUpstreamProtocolsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	temp, ok := h.probeKeyAccount(c, req.APIKey, req.AccountID, req.ProxyID)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	results, err := h.accountTestService.ProbeUpstreamProtocols(ctx, temp, req.BaseURL, func(model string) bool {
		return h.modelCatalog != nil && h.modelCatalog.LookupPricingEntry(ctx, model) != nil
	})
	if err != nil {
		writeProbeError(c, err, "probe_upstream_protocols_failed", "Failed to probe upstream protocols")
		return
	}
	response.Success(c, gin.H{"protocols": results})
}

// probeKeyAccount 用表单里的 key（或已有渠道存着的 key）与代理拼一个临时第三方 key，供两种探测共用。
// proxy_id 是表单里选的代理：有的上游只有走代理才连得上，探测与真实请求走同一条路。
// 出错时已写好响应，返回 false。
func (h *AccountHandler) probeKeyAccount(c *gin.Context, rawAPIKey string, accountID int64, rawProxyID *int64) (*service.Account, bool) {
	apiKey := strings.TrimSpace(rawAPIKey)
	var proxy *service.Proxy
	var proxyID *int64
	if rawProxyID != nil && *rawProxyID > 0 {
		p, err := h.adminService.GetProxy(c.Request.Context(), *rawProxyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return nil, false
		}
		proxy, proxyID = p, rawProxyID
	}
	if apiKey == "" && accountID > 0 {
		account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
		if err != nil {
			response.ErrorFrom(c, err)
			return nil, false
		}
		if !account.IsThirdPartyKey() {
			response.BadRequest(c, "Probing only applies to API key channels")
			return nil, false
		}
		apiKey = strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
		if proxyID == nil {
			proxy, proxyID = account.Proxy, account.ProxyID
		}
	}
	if apiKey == "" {
		response.BadRequest(c, "api_key is required")
		return nil, false
	}
	if h.accountTestService == nil {
		response.InternalError(c, "Account test service is not configured")
		return nil, false
	}
	return &service.Account{
		Type:        service.AccountTypeAPIKey,
		Platform:    service.PlatformOpenAI, // 只是标签：key 的探测只看协议地址
		Credentials: map[string]any{"api_key": apiKey},
		Proxy:       proxy,
		ProxyID:     proxyID,
	}, true
}

// writeProbeError 探测出错的统一响应：配置 / 不支持 400，内部 500，其余当上游错误 502。
func writeProbeError(c *gin.Context, err error, logKey string, fallback string) {
	var syncErr *service.UpstreamModelSyncError
	if errors.As(err, &syncErr) {
		switch syncErr.Kind {
		case service.UpstreamModelSyncErrorConfiguration, service.UpstreamModelSyncErrorUnsupported:
			response.BadRequest(c, syncErr.SafeMessage())
		case service.UpstreamModelSyncErrorInternal:
			response.InternalError(c, syncErr.SafeMessage())
		default:
			slog.Warn(logKey, "kind", syncErr.Kind)
			response.Error(c, http.StatusBadGateway, syncErr.SafeMessage())
		}
		return
	}
	slog.Warn(logKey)
	response.Error(c, http.StatusBadGateway, fallback)
}
