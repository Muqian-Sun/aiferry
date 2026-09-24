package antigravity

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// Claude → v1internal（Gemini 形态）的多模态分片映射，策略同 apicompat.UnsupportedContentError：
// 上游只能内联 base64 数据（Gemini fileData 需要 File API / GCS URI，网关不代为下载任意
// URL），base64 图片 / PDF 映射为 inlineData，纯文本文档映射为 text；URL 来源、Anthropic
// file_id 与非 PDF 文档一律返回 UnsupportedContentError（HTTP 400），不再静默丢弃。

func unsupportedContent(part, hint string) error {
	return &apicompat.UnsupportedContentError{Part: part, Upstream: apicompat.UpstreamProtocolNameGemini, Hint: hint}
}

// claudeImageBlockToGeminiPart 映射 image 块。返回 (nil, nil) 表示没有可转发的内容。
// base64 来源保持历史行为原样内联。
func claudeImageBlockToGeminiPart(block ContentBlock) (*GeminiPart, error) {
	src := block.Source
	if src == nil {
		return nil, nil
	}
	switch src.Type {
	case "base64":
		return &GeminiPart{InlineData: &GeminiInlineData{MimeType: src.MediaType, Data: src.Data}}, nil
	case "url":
		return nil, unsupportedContent("image with a URL source", apicompat.HintNoURLFetch)
	case "file":
		return nil, unsupportedContent("image with an Anthropic file_id source", apicompat.HintFileIDNotPortable)
	default:
		return nil, nil
	}
}

// claudeDocumentBlockToGeminiPart 映射 document 块：base64 PDF → inlineData(application/pdf)，
// text / 纯文本 content → text，其余拒收。返回 (nil, nil) 表示没有可转发的内容。
func claudeDocumentBlockToGeminiPart(block ContentBlock) (*GeminiPart, error) {
	src := block.Source
	if src == nil {
		return nil, nil
	}
	switch src.Type {
	case "base64":
		if src.Data == "" {
			return nil, nil
		}
		mediaType := apicompat.NormalizeMediaType(src.MediaType)
		if mediaType != "" && mediaType != apicompat.MediaTypePDF {
			return nil, unsupportedContent(apicompat.DescribeFileMediaType("document", mediaType), apicompat.HintPDFOnly)
		}
		return &GeminiPart{InlineData: &GeminiInlineData{MimeType: apicompat.MediaTypePDF, Data: src.Data}}, nil
	case "text":
		if strings.TrimSpace(src.Data) == "" {
			return nil, nil
		}
		return &GeminiPart{Text: src.Data}, nil
	case "content":
		text, ok := apicompat.AnthropicDocumentContentText(src.Content)
		if !ok {
			return nil, unsupportedContent("document with a non-text content source", "")
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return &GeminiPart{Text: text}, nil
	case "url":
		return nil, unsupportedContent("document with a URL source", apicompat.HintNoURLFetch)
	case "file":
		return nil, unsupportedContent("document with an Anthropic file_id source", apicompat.HintFileIDNotPortable)
	default:
		return nil, unsupportedContent(fmt.Sprintf("document with source type %q", src.Type), "")
	}
}
