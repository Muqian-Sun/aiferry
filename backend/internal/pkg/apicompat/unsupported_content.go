package apicompat

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"unicode/utf8"
)

// 跨协议转换的多模态分片策略：目标协议能忠实表达的分片照映射（URL 图片、base64 / URL
// PDF 等）；表达不了的分片一律返回 UnsupportedContentError，由网关以 400
// invalid_request_error 拒收——绝不静默丢弃。静默丢弃的后果是请求照常计费、模型回答
// "我看不到图片"，客户端无从得知问题出在网关。
//
// 上游协议的对外名称（出现在错误消息里，客户端据此判断是哪条转换链不支持）。
const (
	UpstreamProtocolNameAnthropic       = "Anthropic Messages"
	UpstreamProtocolNameResponses       = "OpenAI Responses"
	UpstreamProtocolNameChatCompletions = "OpenAI Chat Completions"
	UpstreamProtocolNameGemini          = "Gemini"
)

// UnsupportedContentError 表示客户端请求里的某个内容分片无法在承接该请求的上游协议上
// 忠实表达。它是客户端错误（HTTP 400 invalid_request_error），不是上游 / 账号故障：
// 调用方不得据此换号、标记账号异常或计入账号健康。转换发生在发出任何上游请求之前。
type UnsupportedContentError struct {
	// Part 是客户端视角的分片描述（名词短语），如 "input_audio content"、
	// "image with a URL source"。
	Part string
	// Upstream 是上游协议的对外名称（UpstreamProtocolName* 常量）。
	Upstream string
	// Hint 是可选的补救建议。
	Hint string
}

// Error 形如 "input_audio content is not supported when this model is served over
// the Anthropic Messages protocol[; <hint>]"，直接作为 400 的 error.message 返回客户端。
func (e *UnsupportedContentError) Error() string {
	if e == nil {
		return ""
	}
	upstream := strings.TrimSpace(e.Upstream)
	if upstream == "" {
		upstream = "upstream"
	}
	msg := fmt.Sprintf("%s is not supported when this model is served over the %s protocol", e.Part, upstream)
	if hint := strings.TrimSpace(e.Hint); hint != "" {
		msg += "; " + hint
	}
	return msg
}

// AsUnsupportedContentError 在错误链上找 UnsupportedContentError。
func AsUnsupportedContentError(err error) (*UnsupportedContentError, bool) {
	var target *UnsupportedContentError
	if err == nil || !errors.As(err, &target) || target == nil {
		return nil, false
	}
	return target, true
}

// RetargetUnsupportedContentError 把错误链上的 UnsupportedContentError 改写为最终上游协议
// 的名称后返回（其余错误原样返回）。
//
// 链式转换（如 Chat → Responses → Anthropic）的中间表示只是网关内部实现：中间一步拒收
// 的分片，最终上游同样表达不了（中间表示至少与最终上游一样丰富），但错误消息必须指向
// 客户端真正打到的上游协议，而不是网关内部的中转格式。
func RetargetUnsupportedContentError(err error, upstream string) error {
	unsupported, ok := AsUnsupportedContentError(err)
	if !ok {
		return err
	}
	retargeted := *unsupported
	retargeted.Upstream = upstream
	return &retargeted
}

// 分片描述尽量与入站协议无关：Chat 的 file / image_url 与 Responses 的 input_file /
// input_image 经链式转换后落到同一个报错点，措辞要让两边的客户端都看得懂。
const (
	partImageFileID = "image referenced by file_id"
	partFileID      = "file referenced by file_id"
	partFileURL     = "file referenced by URL (file_url)"
)

func newUnsupportedContentError(part, upstream, hint string) *UnsupportedContentError {
	return &UnsupportedContentError{Part: part, Upstream: upstream, Hint: hint}
}

// 常用补救建议（service 层的 Gemini / Antigravity 转换复用同一套措辞）。
const (
	HintInlineBase64      = "send it inline as base64 data instead"
	HintFileIDNotPortable = "file IDs are only valid on the provider that issued them; send the file inline as base64 data instead"
	HintPDFOnly           = "only PDF documents (application/pdf) can be forwarded"
	HintNoURLFetch        = "Gemini cannot fetch arbitrary URLs; send it inline as base64 data instead"
)

// ---------------------------------------------------------------------------
// data URI / URL helpers shared by the converters
// ---------------------------------------------------------------------------

// isHTTPURL 报告 s 是否为 http(s) 绝对 URL（大小写不敏感）。
func isHTTPURL(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

// parseBase64DataURI 解析 data:<media_type>;base64,<data>。不是 base64 data URI 时 ok=false。
func parseBase64DataURI(dataURI string) (mediaType, data string, ok bool) {
	rest, found := strings.CutPrefix(dataURI, "data:")
	if !found {
		return "", "", false
	}
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return "", "", false
	}
	mediaType = rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	data, found = strings.CutPrefix(rest, "base64,")
	if !found {
		return "", "", false
	}
	return mediaType, data, true
}

// base64DataURI 组装 data:<media_type>;base64,<data>。
func base64DataURI(mediaType, data string) string {
	return "data:" + mediaType + ";base64," + data
}

// NormalizeMediaType 小写并去掉参数（"Application/PDF; charset=x" → "application/pdf"）。
func NormalizeMediaType(mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if parsed, _, err := mime.ParseMediaType(mediaType); err == nil {
		return parsed
	}
	if idx := strings.Index(mediaType, ";"); idx >= 0 {
		return strings.TrimSpace(mediaType[:idx])
	}
	return mediaType
}

// MediaTypePDF 是 PDF 的媒体类型（Anthropic base64 document 唯一允许的类型）。
const MediaTypePDF = "application/pdf"

// inlineFileData 是 OpenAI file_data（data URI 或裸 base64）解析后的结果。
type inlineFileData struct {
	MediaType string
	Data      string
}

// parseOpenAIFileData 解析 OpenAI Chat file.file_data / Responses input_file.file_data。
// 官方形态是 data URI；裸 base64 时按文件名扩展名推断类型（推断不出为空串）。
func parseOpenAIFileData(fileData, filename string) inlineFileData {
	if mediaType, data, ok := parseBase64DataURI(fileData); ok {
		return inlineFileData{MediaType: NormalizeMediaType(mediaType), Data: data}
	}
	return inlineFileData{MediaType: mediaTypeFromFilename(filename), Data: fileData}
}

func mediaTypeFromFilename(filename string) string {
	ext := strings.ToLower(path.Ext(strings.TrimSpace(filename)))
	switch ext {
	case ".pdf":
		return MediaTypePDF
	case ".txt":
		return "text/plain"
	default:
		return ""
	}
}

// decodeBase64Text 把 base64 文本载荷解码成 UTF-8 字符串；不是合法 base64 / UTF-8 时 ok=false。
func decodeBase64Text(data string) (string, bool) {
	trimmed := strings.TrimSpace(data)
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(trimmed)
		if err != nil {
			return "", false
		}
	}
	if !utf8.Valid(decoded) {
		return "", false
	}
	return string(decoded), true
}

// DescribeFileMediaType 生成 `document with media type "image/tiff"` 形式的分片描述。
func DescribeFileMediaType(part, mediaType string) string {
	if strings.TrimSpace(mediaType) == "" {
		return part + " with an unrecognized media type"
	}
	return fmt.Sprintf("%s with media type %q", part, mediaType)
}

// defaultDocumentFilename 是 PDF 转 OpenAI file 分片时缺省的文件名（OpenAI 要求 file_data
// 带 filename）。
const defaultDocumentFilename = "document.pdf"
