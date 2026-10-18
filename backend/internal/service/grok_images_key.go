package service

import (
	"context"
	"fmt"
	"mime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ForwardGrokImagesAPIKey 用第三方 key 承接 xAI 图片生成 / 编辑：走 OpenAI 图片端点的 key 转发
// （{chat_completions 根}/v1/images/generations|edits，套账号模型映射），请求体按客户端原样转发。
// 计费口径与 ForwardGrokMedia 一致：张数取响应里的图，尺寸档取请求的 size。
func (s *OpenAIGatewayService) ForwardGrokImagesAPIKey(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	parsed, err := grokImagesKeyRequest(endpoint, body, contentType)
	if err != nil {
		return nil, err
	}
	return s.forwardOpenAIImagesAPIKey(ctx, c, account, body, parsed, "")
}

// grokImagesKeyRequest 从 xAI 图片请求取出 OpenAI 图片 key 转发要用的字段。不走 ParseOpenAIImagesRequest：
// xAI 的编辑请求用 image / images 对象，OpenAI 的解析会按 images[].image_url 拒掉。
func grokImagesKeyRequest(endpoint GrokMediaEndpoint, body []byte, contentType string) (*OpenAIImagesRequest, error) {
	var path string
	switch endpoint {
	case GrokMediaEndpointImagesGenerations:
		path = openAIImagesGenerationsEndpoint
	case GrokMediaEndpointImagesEdits:
		path = openAIImagesEditsEndpoint
	default:
		return nil, fmt.Errorf("grok endpoint %s cannot be served by an api key", endpoint)
	}
	info := ParseGrokMediaRequest(contentType, body)
	req := &OpenAIImagesRequest{
		Endpoint:    path,
		ContentType: strings.TrimSpace(contentType),
		Model:       info.Model,
		Prompt:      info.Prompt,
		N:           info.N,
		Size:        info.Size,
		SizeTier:    info.SizeTier,
		Body:        body,
	}
	if mediaType, _, err := mime.ParseMediaType(req.ContentType); err == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		req.Multipart = true
	} else if gjson.ValidBytes(body) {
		req.Stream = gjson.GetBytes(body, "stream").Bool()
		req.ResponseFormat = strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "response_format").String()))
	}
	if req.ResponseFormat == "" {
		// xAI 默认回 url（与成品号路径一致）；只有客户端要 b64_json 时才让 key 路径把 url 下载回填成 b64_json。
		req.ResponseFormat = "url"
	}
	return req, nil
}
