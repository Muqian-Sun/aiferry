package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// Anthropic Messages → Gemini generateContent 的多模态分片映射（策略见 apicompat 的
// UnsupportedContentError）：Gemini 只能内联 base64 数据（fileData 需要 File API / GCS
// URI，网关不代为下载任意 URL），所以 base64 图片 / PDF 映射为 inlineData，纯文本文档
// 映射为 text；URL 来源、Anthropic file_id 与非 PDF 文档一律 400 拒收，不再静默丢弃，
// 也不再把整个块（含 base64 PDF）JSON 序列化塞进 text 分片。

func unsupportedGeminiContent(part, hint string) error {
	return &apicompat.UnsupportedContentError{Part: part, Upstream: apicompat.UpstreamProtocolNameGemini, Hint: hint}
}

func geminiInlineDataPart(mimeType, data string) map[string]any {
	return map[string]any{
		"inlineData": map[string]any{
			"mimeType": mimeType,
			"data":     data,
		},
	}
}

// claudeImageBlockToGeminiPart 把 Anthropic image 块映射为 Gemini 分片。
// 返回 (nil, nil) 表示没有可转发的内容（历史行为：缺 source / 缺数据的图片被跳过）。
func claudeImageBlockToGeminiPart(block map[string]any) (map[string]any, error) {
	src, ok := block["source"].(map[string]any)
	if !ok {
		return nil, nil
	}
	srcType, _ := src["type"].(string)
	switch srcType {
	case "base64":
		mediaType, _ := src["media_type"].(string)
		data, _ := src["data"].(string)
		if mediaType == "" || data == "" {
			return nil, nil
		}
		return geminiInlineDataPart(mediaType, data), nil
	case "url":
		return nil, unsupportedGeminiContent("image with a URL source", apicompat.HintNoURLFetch)
	case "file":
		return nil, unsupportedGeminiContent("image with an Anthropic file_id source", apicompat.HintFileIDNotPortable)
	default:
		return nil, nil
	}
}

// claudeDocumentBlockToGeminiPart 把 Anthropic document 块映射为 Gemini 分片：
// base64 PDF → inlineData(application/pdf)；text / 纯文本 content → text；其余拒收。
// 返回 (nil, nil) 表示没有可转发的内容。
func claudeDocumentBlockToGeminiPart(block map[string]any) (map[string]any, error) {
	src, ok := block["source"].(map[string]any)
	if !ok {
		return nil, nil
	}
	srcType, _ := src["type"].(string)
	switch srcType {
	case "base64":
		data, _ := src["data"].(string)
		if data == "" {
			return nil, nil
		}
		mediaType, _ := src["media_type"].(string)
		mediaType = apicompat.NormalizeMediaType(mediaType)
		if mediaType != "" && mediaType != apicompat.MediaTypePDF {
			return nil, unsupportedGeminiContent(apicompat.DescribeFileMediaType("document", mediaType), apicompat.HintPDFOnly)
		}
		return geminiInlineDataPart(apicompat.MediaTypePDF, data), nil
	case "text":
		data, _ := src["data"].(string)
		if data == "" {
			return nil, nil
		}
		return map[string]any{"text": data}, nil
	case "content":
		raw, err := json.Marshal(src["content"])
		if err != nil {
			return nil, unsupportedGeminiContent("document with a non-text content source", "")
		}
		text, ok := apicompat.AnthropicDocumentContentText(raw)
		if !ok {
			return nil, unsupportedGeminiContent("document with a non-text content source", "")
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return map[string]any{"text": text}, nil
	case "url":
		return nil, unsupportedGeminiContent("document with a URL source", apicompat.HintNoURLFetch)
	case "file":
		return nil, unsupportedGeminiContent("document with an Anthropic file_id source", apicompat.HintFileIDNotPortable)
	default:
		return nil, unsupportedGeminiContent(fmt.Sprintf("document with source type %q", srcType), "")
	}
}
