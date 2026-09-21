package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"

	"github.com/gin-gonic/gin"
)

// 网关准入中间件共用的「客户端写了哪个模型」提取与错误写出：目录准入与分组白名单
// 看的是同一份客户端模型名，错误格式也按同一套入口协议规则选。

// requestModelCandidates 收集该请求全部可被下游解析器绑定到的模型值：路由参数
// （Gemini 原生 URL）单值；带体方法读请求体取候选集（含重复键与大小写变体，
// 调用方必须逐一校验）；都没有时取查询参数（Grok Realtime 升级请求）。
// 返回 done=false 表示请求体读取失败，响应已写出并 Abort。
func requestModelCandidates(c *gin.Context) ([]string, bool) {
	if model := requestModelFromParams(c); model != "" {
		return []string{model}, true
	}
	var models []string
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		candidates, done := requestModelsFromBody(c)
		if !done {
			return nil, false
		}
		models = candidates
	}
	if len(models) == 0 {
		if model := strings.TrimSpace(c.Query("model")); model != "" {
			models = []string{model}
		}
	}
	return models, true
}

// isResponsesWebSocketRoute 判断当前请求是否命中 OpenAI Responses WebSocket
// 入口。只有这三条路由的模型在升级后的帧里，交给 handler 逐帧校验；其他路由
// 即使携带 WebSocket Upgrade 头也必须经过本中间件校验。
func isResponsesWebSocketRoute(c *gin.Context) bool {
	if c.Request == nil || c.Request.Method != http.MethodGet {
		return false
	}
	switch c.FullPath() {
	case "/v1/responses", "/responses", "/backend-api/codex/responses":
		return true
	}
	return false
}

// requestModelsFromBody 读取请求体并提取客户端模型名，随后把请求体
// 回填（PrereadBody），保证后续 handler 零拷贝重读。读取失败按现有合成中间件
// 的方式返回 400/413（返回 false 表示已写出响应并 Abort）。
//
// 下游同时存在 gjson（首个、大小写敏感）、encoding/json 绑定（末值、大小写
// 不敏感）与 multipart 表单（首/末字段）三类解析器，这里返回「任一解析器可能
// 绑定到的全部模型值」，调用方必须逐一校验，任一未命中即拒绝。
func requestModelsFromBody(c *gin.Context) ([]string, bool) {
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		status := http.StatusBadRequest
		message := "Failed to read request body"
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			status = http.StatusRequestEntityTooLarge
			message = "Request body is too large"
		}
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
		c.Abort()
		return nil, false
	}
	requestmodel.ResetRequestBody(c.Request, body)
	return requestmodel.FromBodyCandidates(c.FullPath(), c.GetHeader("Content-Type"), body), true
}

// requestModelFromParams 从路由参数提取模型名：Gemini 原生 URL 的
// `model` / `modelAction`（去掉 `:action` 后缀）。提取不到返回空串，
// 由调用方决定是否继续读请求体或查询参数。
func requestModelFromParams(c *gin.Context) string {
	if model := strings.TrimSpace(c.Param("model")); model != "" {
		return model
	}
	if modelAction := strings.TrimSpace(c.Param("modelAction")); modelAction != "" {
		modelAction = strings.TrimPrefix(modelAction, "/")
		if idx := strings.LastIndex(modelAction, ":"); idx >= 0 {
			return strings.TrimSpace(modelAction[:idx])
		}
		return modelAction
	}
	return ""
}

// gatewayModelErrorWriter 按入口协议选择错误格式：
// Gemini 原生（/v1beta、/antigravity/v1beta）用 Google 格式；
// Messages 入口（含根路径别名与 /antigravity/v1）用 Anthropic 格式；
// 其余（OpenAI 兼容入口）用 OpenAI 格式。
func gatewayModelErrorWriter(c *gin.Context) GatewayErrorWriter {
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	switch {
	case strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/antigravity/v1beta"):
		return GoogleErrorWriter
	case strings.Contains(path, "/messages"):
		return AnthropicErrorWriter
	default:
		return OpenAIErrorWriter
	}
}
