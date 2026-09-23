package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func contentModerationStatus(decision *service.ContentModerationDecision) int {
	if decision == nil || decision.StatusCode < 400 || decision.StatusCode > 599 {
		return http.StatusForbidden
	}
	return decision.StatusCode
}

func contentModerationErrorCode(decision *service.ContentModerationDecision) string {
	return "content_policy_violation"
}

func runContentModeration(c *gin.Context, reqLog *zap.Logger, svc *service.ContentModerationService, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *service.ContentModerationDecision {
	if svc == nil || c == nil || c.Request == nil {
		return nil
	}
	input := buildContentModerationInput(c, apiKey, subject, protocol, model, body)
	if reqLog != nil {
		reqLog.Info("content_moderation.gateway_check_start",
			zap.String("request_id", input.RequestID),
			zap.Int64("user_id", input.UserID),
			zap.Int64("api_key_id", input.APIKeyID),
			zap.String("api_key_name", input.APIKeyName),
			zap.String("endpoint", input.Endpoint),
			zap.String("provider", input.Provider),
			zap.String("protocol", input.Protocol),
			zap.String("model", input.Model),
			zap.Int("body_bytes", len(body)),
		)
	}
	decision, err := svc.Check(c.Request.Context(), input)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("content_moderation.check_failed", zap.Error(err))
		}
		return nil
	}
	if reqLog != nil && decision != nil {
		reqLog.Info("content_moderation.gateway_check_done",
			zap.String("request_id", input.RequestID),
			zap.Bool("allowed", decision.Allowed),
			zap.Bool("blocked", decision.Blocked),
			zap.Bool("flagged", decision.Flagged),
			zap.String("action", decision.Action),
			zap.Int("status_code", decision.StatusCode),
			zap.String("highest_category", decision.HighestCategory),
			zap.Float64("highest_score", decision.HighestScore),
		)
	}
	return decision
}

func buildContentModerationInput(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) service.ContentModerationCheckInput {
	input := service.ContentModerationCheckInput{
		RequestID: contentModerationRequestID(c.Request.Context()),
		UserID:    subject.UserID,
		Endpoint:  GetInboundEndpoint(c),
		Provider:  contentModerationProvider(c),
		Model:     clientRequestedModel(c, model),
		Protocol:  protocol,
		Body:      body,
	}
	if resolvedPlatform, ok := service.RequestVendorPlatform(c.Request.Context()); ok {
		input.Provider = resolvedPlatform
	}
	if forcedPlatform, ok := middleware2.GetForcePlatformFromContext(c); ok {
		input.Provider = strings.TrimSpace(forcedPlatform)
	}
	if apiKey != nil {
		input.APIKeyID = apiKey.ID
		input.APIKeyName = apiKey.Name
		if apiKey.User != nil {
			input.UserEmail = apiKey.User.Email
		}
	}
	if input.Endpoint == "" && c.Request != nil && c.Request.URL != nil {
		input.Endpoint = c.Request.URL.Path
	}
	return input
}

// clientRequestedModel 客户端原始请求的公开模型名（目录路由挂在 ctx 上的 RequestedModel，可能是别名）；
// 没有路由时用调用方传入的模型。
func clientRequestedModel(c *gin.Context, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if c == nil || c.Request == nil {
		return fallback
	}
	if model, ok := service.RequestedPublicModelFromContext(c.Request.Context()); ok {
		return model
	}
	return fallback
}

// contentModerationProvider 审计输入里的厂商：目录路由的条目厂商；无路由（无模型端点）为空。
func contentModerationProvider(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	platform, _ := service.RequestVendorPlatform(c.Request.Context())
	return strings.TrimSpace(platform)
}

func contentModerationRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(ctxkey.RequestID).(string); ok {
		return strings.TrimSpace(requestID)
	}
	return ""
}
