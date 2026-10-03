package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// webSearchDelegateUnavailableMessage 搜索请求该交给 Haiku 执行、却执行不了（目录里没有它、或没有可用渠道）时回给客户端的话；
// 不提 Haiku（用户站不出现它）。
const webSearchDelegateUnavailableMessage = "Web search is temporarily unavailable"

var errWebSearchDelegateMissing = errors.New("web search delegate model is not in the catalog")

// delegateClaudeCodeWebSearch Claude Code 配第三方模型时，那次单独的搜索请求交给 Haiku 执行（见
// service/web_search_delegate.go）：只认 Claude Code 的 User-Agent（muqian 2026-10-03）、tools 只有一个
// web_search、请求的模型不是 Anthropic 厂商。命中后改写请求体，并把目录路由换成 Haiku；目录里没有 Haiku 时报错
// （muqian：没有承接就直接报错，不换别的路子）。不命中时 ok=false、原请求不动。
func (h *GatewayHandler) delegateClaudeCodeWebSearch(c *gin.Context, body []byte) (rewritten []byte, ok bool, err error) {
	if !claudeCodeValidator.ValidateUserAgent(c.GetHeader("User-Agent")) || !service.IsWebSearchOnlyRequest(body) {
		return nil, false, nil
	}
	route, routed := service.CatalogRouteFromContext(c.Request.Context())
	if !routed || !service.NeedsWebSearchDelegate(route) {
		return nil, false, nil
	}
	delegate, found := h.gatewayService.WebSearchDelegateRoute(c.Request.Context())
	if !found {
		return nil, false, errWebSearchDelegateMissing
	}
	rewritten, err = service.BuildWebSearchDelegateBody(body)
	if err != nil {
		return nil, false, err
	}
	c.Request = c.Request.WithContext(service.WithWebSearchDelegate(c.Request.Context(), delegate, route.RequestedModel))
	return rewritten, true, nil
}
