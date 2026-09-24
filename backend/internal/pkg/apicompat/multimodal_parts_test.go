package apicompat

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 跨协议转换的多模态分片：能映射的映射，不能映射的返回 UnsupportedContentError（400），
// 绝不静默丢弃。

const (
	testPDFBase64   = "JVBERi0xLjQK"
	testPDFDataURI  = "data:application/pdf;base64," + testPDFBase64
	testPNGDataURI  = "data:image/png;base64,iVBORw0KGgo="
	testImageURL    = "https://example.com/cat.png"
	testDocumentURL = "https://example.com/report.pdf"
)

func requireUnsupportedContent(t *testing.T, err error, upstream, partSubstring string) {
	t.Helper()
	require.Error(t, err)
	unsupported, ok := AsUnsupportedContentError(err)
	require.True(t, ok, "expected UnsupportedContentError, got %T: %v", err, err)
	require.Equal(t, upstream, unsupported.Upstream)
	require.Contains(t, unsupported.Part, partSubstring)
	require.Contains(t, err.Error(), "is not supported when this model is served over the "+upstream+" protocol")
}

// ---------------------------------------------------------------------------
// Chat Completions → Responses
// ---------------------------------------------------------------------------

func chatUserPartsToResponsesParts(t *testing.T, content string) ([]ResponsesContentPart, error) {
	t.Helper()
	req := &ChatCompletionsRequest{
		Model:    "gpt-5.6",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(content)}},
	}
	out, err := ChatCompletionsToResponses(req)
	if err != nil {
		return nil, err
	}
	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(out.Input, &items))
	require.Len(t, items, 1)
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	return parts, nil
}

func TestChatToResponses_MultimodalParts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, parts []ResponsesContentPart)
		wantErr string // part substring when an UnsupportedContentError is expected
	}{
		{
			name:    "https image kept with detail",
			content: `[{"type":"image_url","image_url":{"url":"` + testImageURL + `","detail":"high"}}]`,
			check: func(t *testing.T, parts []ResponsesContentPart) {
				require.Equal(t, []ResponsesContentPart{{Type: "input_image", ImageURL: testImageURL, Detail: "high"}}, parts)
			},
		},
		{
			name:    "base64 image kept",
			content: `[{"type":"image_url","image_url":{"url":"` + testPNGDataURI + `"}}]`,
			check: func(t *testing.T, parts []ResponsesContentPart) {
				require.Equal(t, []ResponsesContentPart{{Type: "input_image", ImageURL: testPNGDataURI}}, parts)
			},
		},
		{
			name:    "base64 pdf mapped to input_file",
			content: `[{"type":"file","file":{"filename":"a.pdf","file_data":"` + testPDFDataURI + `"}}]`,
			check: func(t *testing.T, parts []ResponsesContentPart) {
				require.Equal(t, []ResponsesContentPart{{Type: "input_file", Filename: "a.pdf", FileData: testPDFDataURI}}, parts)
			},
		},
		{
			name:    "file_id stays meaningful between OpenAI protocols",
			content: `[{"type":"file","file":{"file_id":"file-abc"}}]`,
			check: func(t *testing.T, parts []ResponsesContentPart) {
				require.Equal(t, []ResponsesContentPart{{Type: "input_file", FileID: "file-abc"}}, parts)
			},
		},
		{
			name:    "input_audio rejected",
			content: `[{"type":"text","text":"transcribe"},{"type":"input_audio","input_audio":{"data":"UklGRg==","format":"wav"}}]`,
			wantErr: "input_audio",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := chatUserPartsToResponsesParts(t, tt.content)
			if tt.wantErr != "" {
				requireUnsupportedContent(t, err, UpstreamProtocolNameResponses, tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, parts)
		})
	}
}

// ---------------------------------------------------------------------------
// Responses → Anthropic (also the tail of Chat → Anthropic / Chat → Gemini)
// ---------------------------------------------------------------------------

func responsesUserPartsToAnthropicBlocks(t *testing.T, parts string) ([]AnthropicContentBlock, error) {
	t.Helper()
	req := &ResponsesRequest{
		Model: "claude-sonnet-4",
		Input: json.RawMessage(`[{"type":"message","role":"user","content":` + parts + `}]`),
	}
	out, err := ResponsesToAnthropicRequest(req)
	if err != nil {
		return nil, err
	}
	require.Len(t, out.Messages, 1)
	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &blocks))
	return blocks, nil
}

func TestResponsesToAnthropic_MultimodalParts(t *testing.T) {
	plainText := base64.StdEncoding.EncodeToString([]byte("hello notes"))
	tests := []struct {
		name    string
		parts   string
		want    []AnthropicContentBlock
		wantErr string
	}{
		{
			name:  "https image mapped to url source",
			parts: `[{"type":"input_image","image_url":"` + testImageURL + `","detail":"low"}]`,
			want:  []AnthropicContentBlock{{Type: "image", Source: &AnthropicImageSource{Type: "url", URL: testImageURL}}},
		},
		{
			name:  "base64 image unchanged",
			parts: `[{"type":"input_image","image_url":"` + testPNGDataURI + `"}]`,
			want:  []AnthropicContentBlock{{Type: "image", Source: &AnthropicImageSource{Type: "base64", MediaType: "image/png", Data: "iVBORw0KGgo="}}},
		},
		{
			name:  "base64 pdf mapped to document",
			parts: `[{"type":"input_file","filename":"report.pdf","file_data":"` + testPDFDataURI + `"}]`,
			want: []AnthropicContentBlock{{
				Type:   "document",
				Source: &AnthropicImageSource{Type: "base64", MediaType: "application/pdf", Data: testPDFBase64},
				Title:  "report.pdf",
			}},
		},
		{
			name:  "raw base64 pdf recognised by filename",
			parts: `[{"type":"input_file","filename":"report.PDF","file_data":"` + testPDFBase64 + `"}]`,
			want: []AnthropicContentBlock{{
				Type:   "document",
				Source: &AnthropicImageSource{Type: "base64", MediaType: "application/pdf", Data: testPDFBase64},
				Title:  "report.PDF",
			}},
		},
		{
			name:  "pdf url mapped to document url source",
			parts: `[{"type":"input_file","file_url":"` + testDocumentURL + `"}]`,
			want:  []AnthropicContentBlock{{Type: "document", Source: &AnthropicImageSource{Type: "url", URL: testDocumentURL}}},
		},
		{
			name:  "plain text file mapped to text document",
			parts: `[{"type":"input_file","filename":"notes.txt","file_data":"data:text/plain;base64,` + plainText + `"}]`,
			want: []AnthropicContentBlock{{
				Type:   "document",
				Source: &AnthropicImageSource{Type: "text", MediaType: "text/plain", Data: "hello notes"},
				Title:  "notes.txt",
			}},
		},
		{name: "input_file file_id rejected", parts: `[{"type":"input_file","file_id":"file-abc"}]`, wantErr: "file referenced by file_id"},
		{name: "input_image file_id rejected", parts: `[{"type":"input_image","file_id":"file-abc"}]`, wantErr: "image referenced by file_id"},
		{name: "input_audio rejected", parts: `[{"type":"input_audio","input_audio":{"data":"UklGRg==","format":"wav"}}]`, wantErr: "input_audio"},
		{name: "non-pdf file rejected", parts: `[{"type":"input_file","filename":"a.zip","file_data":"data:application/zip;base64,UEsDBA=="}]`, wantErr: `media type "application/zip"`},
		{name: "non-http image url rejected", parts: `[{"type":"input_image","image_url":"gs://bucket/cat.png"}]`, wantErr: "neither http(s) nor a base64 data URI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := responsesUserPartsToAnthropicBlocks(t, tt.parts)
			if tt.wantErr != "" {
				requireUnsupportedContent(t, err, UpstreamProtocolNameAnthropic, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, blocks)
		})
	}
}

// 历史行为保持：空 base64 图片与空 file 分片仍被跳过（整条 user 消息为空时丢弃），不报错。
func TestResponsesToAnthropic_EmptyMediaPartsKeepHistoricalDrop(t *testing.T) {
	messages := responsesToAnthropicMessages(t, `[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"},{"type":"input_image","image_url":""},{"type":"input_file"}]}
	]`)
	require.Len(t, messages, 1)
	blocks := parseContentBlocks(messages[0].Content)
	require.Equal(t, []AnthropicContentBlock{{Type: "text", Text: "hi"}}, blocks)
}

func TestResponsesToAnthropic_ToolOutputMediaMapped(t *testing.T) {
	messages := responsesToAnthropicMessages(t, `[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"render"}]},
		{"type":"function_call","call_id":"call_1","name":"render","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":[
			{"type":"input_text","text":"done"},
			{"type":"input_image","image_url":"`+testImageURL+`"},
			{"type":"input_file","filename":"out.pdf","file_data":"`+testPDFDataURI+`"}
		]}
	]`)
	requireAnthropicMessagesAreSendable(t, messages)

	var toolResult *AnthropicContentBlock
	for _, m := range messages {
		for _, b := range parseContentBlocks(m.Content) {
			if b.Type == "tool_result" {
				block := b
				toolResult = &block
			}
		}
	}
	require.NotNil(t, toolResult)
	var inner []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(toolResult.Content, &inner))
	require.Len(t, inner, 3)
	require.Equal(t, AnthropicImageSource{Type: "url", URL: testImageURL}, *inner[1].Source)
	require.Equal(t, "document", inner[2].Type)
	require.Equal(t, "application/pdf", inner[2].Source.MediaType)
}

func TestResponsesToAnthropic_ToolOutputFileIDRejected(t *testing.T) {
	req := &ResponsesRequest{Model: "claude-sonnet-4", Input: json.RawMessage(`[
		{"type":"function_call","call_id":"call_1","name":"read","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_file","file_id":"file-abc"}]}
	]`)}
	_, err := ResponsesToAnthropicRequest(req)
	requireUnsupportedContent(t, err, UpstreamProtocolNameAnthropic, "file referenced by file_id")
}

// Chat → Responses → Anthropic 链（/v1/chat/completions 打到 Anthropic / Gemini 资源）端到端。
func TestChatToAnthropicChain_MultimodalParts(t *testing.T) {
	chat := &ChatCompletionsRequest{Model: "claude-sonnet-4", Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`[
		{"type":"text","text":"compare"},
		{"type":"image_url","image_url":{"url":"` + testImageURL + `"}},
		{"type":"file","file":{"filename":"a.pdf","file_data":"` + testPDFDataURI + `"}}
	]`)}}}
	responsesReq, err := ChatCompletionsToResponses(chat)
	require.NoError(t, err)
	anthropicReq, err := ResponsesToAnthropicRequest(responsesReq)
	require.NoError(t, err)
	require.Len(t, anthropicReq.Messages, 1)
	blocks := parseContentBlocks(anthropicReq.Messages[0].Content)
	require.Len(t, blocks, 3)
	require.Equal(t, &AnthropicImageSource{Type: "url", URL: testImageURL}, blocks[1].Source)
	require.Equal(t, "document", blocks[2].Type)
	require.Equal(t, &AnthropicImageSource{Type: "base64", MediaType: "application/pdf", Data: testPDFBase64}, blocks[2].Source)

	chat.Messages[0].Content = json.RawMessage(`[{"type":"file","file":{"file_id":"file-abc"}}]`)
	responsesReq, err = ChatCompletionsToResponses(chat)
	require.NoError(t, err)
	_, err = ResponsesToAnthropicRequest(responsesReq)
	requireUnsupportedContent(t, err, UpstreamProtocolNameAnthropic, "file referenced by file_id")
}

// ---------------------------------------------------------------------------
// Anthropic → Responses
// ---------------------------------------------------------------------------

func anthropicUserBlocksToResponsesParts(t *testing.T, blocks string) ([]ResponsesInputItem, error) {
	t.Helper()
	req := &AnthropicRequest{
		Model:     "gpt-5.6",
		MaxTokens: 256,
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(blocks)}},
	}
	out, err := AnthropicToResponses(req)
	if err != nil {
		return nil, err
	}
	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(out.Input, &items))
	return items, nil
}

func singleUserMessageParts(t *testing.T, items []ResponsesInputItem) []ResponsesContentPart {
	t.Helper()
	require.Len(t, items, 1)
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	return parts
}

func TestAnthropicToResponses_MultimodalParts(t *testing.T) {
	tests := []struct {
		name    string
		blocks  string
		want    []ResponsesContentPart
		wantErr string
	}{
		{
			name:   "url image mapped",
			blocks: `[{"type":"image","source":{"type":"url","url":"` + testImageURL + `"}}]`,
			want:   []ResponsesContentPart{{Type: "input_image", ImageURL: testImageURL}},
		},
		{
			name:   "base64 image unchanged",
			blocks: `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}]`,
			want:   []ResponsesContentPart{{Type: "input_image", ImageURL: testPNGDataURI}},
		},
		{
			name:   "base64 pdf mapped to input_file",
			blocks: `[{"type":"document","title":"q3.pdf","source":{"type":"base64","media_type":"application/pdf","data":"` + testPDFBase64 + `"}}]`,
			want:   []ResponsesContentPart{{Type: "input_file", Filename: "q3.pdf", FileData: testPDFDataURI}},
		},
		{
			name:   "untitled pdf gets a default filename",
			blocks: `[{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"` + testPDFBase64 + `"}}]`,
			want:   []ResponsesContentPart{{Type: "input_file", Filename: "document.pdf", FileData: testPDFDataURI}},
		},
		{
			name:   "pdf url mapped to input_file file_url",
			blocks: `[{"type":"document","source":{"type":"url","url":"` + testDocumentURL + `"}}]`,
			want:   []ResponsesContentPart{{Type: "input_file", FileURL: testDocumentURL}},
		},
		{
			name:   "text document mapped to input_text",
			blocks: `[{"type":"document","source":{"type":"text","media_type":"text/plain","data":"plain body"}}]`,
			want:   []ResponsesContentPart{{Type: "input_text", Text: "plain body"}},
		},
		{
			name:   "text-only content document mapped to input_text",
			blocks: `[{"type":"document","source":{"type":"content","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}}]`,
			want:   []ResponsesContentPart{{Type: "input_text", Text: "a\n\nb"}},
		},
		{name: "image file source rejected", blocks: `[{"type":"image","source":{"type":"file","file_id":"file_01"}}]`, wantErr: "image with an Anthropic file_id source"},
		{name: "document file source rejected", blocks: `[{"type":"document","source":{"type":"file","file_id":"file_01"}}]`, wantErr: "document with an Anthropic file_id source"},
		{name: "non-pdf base64 document rejected", blocks: `[{"type":"document","source":{"type":"base64","media_type":"image/tiff","data":"AAAA"}}]`, wantErr: `media type "image/tiff"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := anthropicUserBlocksToResponsesParts(t, tt.blocks)
			if tt.wantErr != "" {
				requireUnsupportedContent(t, err, UpstreamProtocolNameResponses, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, singleUserMessageParts(t, items))
		})
	}
}

func TestAnthropicToResponses_ToolResultMediaLifted(t *testing.T) {
	items, err := anthropicUserBlocksToResponsesParts(t, `[{"type":"tool_result","tool_use_id":"toolu_1","content":[
		{"type":"text","text":"see attachments"},
		{"type":"image","source":{"type":"url","url":"`+testImageURL+`"}},
		{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"`+testPDFBase64+`"}},
		{"type":"document","source":{"type":"text","media_type":"text/plain","data":"inline notes"}}
	]}]`)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "function_call_output", items[0].Type)
	require.Equal(t, "see attachments\n\ninline notes", items[0].Output)
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[1].Content, &parts))
	require.Equal(t, []ResponsesContentPart{
		{Type: "input_image", ImageURL: testImageURL},
		{Type: "input_file", Filename: "document.pdf", FileData: testPDFDataURI},
	}, parts)
}

// ---------------------------------------------------------------------------
// Anthropic → Chat Completions (direct bridge)
// ---------------------------------------------------------------------------

func TestAnthropicToChat_MultimodalParts(t *testing.T) {
	tests := []struct {
		name    string
		blocks  string
		want    string // expected user message content JSON
		wantErr string
	}{
		{
			name:   "url image mapped",
			blocks: `[{"type":"text","text":"what"},{"type":"image","source":{"type":"url","url":"` + testImageURL + `"}}]`,
			want:   `[{"type":"text","text":"what"},{"type":"image_url","image_url":{"url":"` + testImageURL + `"}}]`,
		},
		{
			name:   "base64 pdf mapped to file",
			blocks: `[{"type":"document","title":"q3.pdf","source":{"type":"base64","media_type":"application/pdf","data":"` + testPDFBase64 + `"}}]`,
			want:   `[{"type":"file","file":{"filename":"q3.pdf","file_data":"` + testPDFDataURI + `"}}]`,
		},
		{
			name:   "text document folds into string content",
			blocks: `[{"type":"text","text":"summarise"},{"type":"document","source":{"type":"text","media_type":"text/plain","data":"body"}}]`,
			want:   `"summarise\n\nbody"`,
		},
		{name: "pdf url rejected", blocks: `[{"type":"document","source":{"type":"url","url":"` + testDocumentURL + `"}}]`, wantErr: "document with a URL source"},
		{name: "image file source rejected", blocks: `[{"type":"image","source":{"type":"file","file_id":"file_01"}}]`, wantErr: "image with an Anthropic file_id source"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := AnthropicToChatCompletionsRequest(&AnthropicRequest{
				Model:     "gpt-5.6",
				MaxTokens: 256,
				Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(tt.blocks)}},
			})
			if tt.wantErr != "" {
				requireUnsupportedContent(t, err, UpstreamProtocolNameChatCompletions, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, out.Messages, 1)
			require.JSONEq(t, tt.want, string(out.Messages[0].Content))
		})
	}
}

func TestAnthropicToChat_ToolResultPDFLifted(t *testing.T) {
	out, err := AnthropicToChatCompletionsRequest(&AnthropicRequest{
		Model:     "gpt-5.6",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_1","name":"fetch","input":{}}]`)},
			{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"` + testPDFBase64 + `"}}]}]`)},
		},
	})
	require.NoError(t, err)
	last := out.Messages[len(out.Messages)-1]
	require.Equal(t, "user", last.Role)
	require.JSONEq(t, `[{"type":"file","file":{"filename":"document.pdf","file_data":"`+testPDFDataURI+`"}}]`, string(last.Content))
}

// ---------------------------------------------------------------------------
// Responses → Chat Completions
// ---------------------------------------------------------------------------

func TestResponsesToChat_MultimodalParts(t *testing.T) {
	tests := []struct {
		name    string
		parts   string
		want    string
		wantErr string
	}{
		{
			name:  "https image kept with detail",
			parts: `[{"type":"input_image","image_url":"` + testImageURL + `","detail":"high"}]`,
			want:  `[{"type":"image_url","image_url":{"url":"` + testImageURL + `","detail":"high"}}]`,
		},
		{
			name:  "base64 pdf mapped to file",
			parts: `[{"type":"input_text","text":"read"},{"type":"input_file","filename":"a.pdf","file_data":"` + testPDFDataURI + `"}]`,
			want:  `[{"type":"text","text":"read"},{"type":"file","file":{"filename":"a.pdf","file_data":"` + testPDFDataURI + `"}}]`,
		},
		{
			name:  "file_id kept between OpenAI protocols",
			parts: `[{"type":"input_file","file_id":"file-abc"}]`,
			want:  `[{"type":"file","file":{"file_id":"file-abc"}}]`,
		},
		{
			name:  "input_audio mapped",
			parts: `[{"type":"input_audio","input_audio":{"data":"UklGRg==","format":"wav"}}]`,
			want:  `[{"type":"input_audio","input_audio":{"data":"UklGRg==","format":"wav"}}]`,
		},
		{name: "pdf url rejected", parts: `[{"type":"input_file","file_url":"` + testDocumentURL + `"}]`, wantErr: "file referenced by URL (file_url)"},
		{name: "image file_id rejected", parts: `[{"type":"input_image","file_id":"file-abc"}]`, wantErr: "image referenced by file_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
				Model: "gpt-5.6",
				Input: json.RawMessage(`[{"type":"message","role":"user","content":` + tt.parts + `}]`),
			})
			if tt.wantErr != "" {
				requireUnsupportedContent(t, err, UpstreamProtocolNameChatCompletions, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, out.Messages, 1)
			require.JSONEq(t, tt.want, string(out.Messages[0].Content))
		})
	}
}

// ---------------------------------------------------------------------------
// Wire shape / error helpers
// ---------------------------------------------------------------------------

func TestAnthropicImageSourceWireShape(t *testing.T) {
	base64Source, err := json.Marshal(AnthropicImageSource{Type: "base64", MediaType: "image/png", Data: "AAAA"})
	require.NoError(t, err)
	require.Equal(t, `{"type":"base64","media_type":"image/png","data":"AAAA"}`, string(base64Source), "base64 形态必须逐字节不变")

	urlSource, err := json.Marshal(AnthropicImageSource{Type: "url", URL: testImageURL})
	require.NoError(t, err)
	require.Equal(t, `{"type":"url","url":"`+testImageURL+`"}`, string(urlSource), "url 来源不能带多余的 media_type/data 键")

	block, err := json.Marshal(AnthropicContentBlock{Type: "document", Title: "a.pdf", Source: &AnthropicImageSource{Type: "url", URL: testDocumentURL}})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"document","title":"a.pdf","source":{"type":"url","url":"`+testDocumentURL+`"}}`, string(block))
}

func TestUnsupportedContentErrorHelpers(t *testing.T) {
	base := &UnsupportedContentError{Part: "input_audio content", Upstream: UpstreamProtocolNameResponses}
	require.Equal(t, "input_audio content is not supported when this model is served over the OpenAI Responses protocol", base.Error())

	wrapped := fmt.Errorf("convert chat completions to responses: %w", base)
	got, ok := AsUnsupportedContentError(wrapped)
	require.True(t, ok)
	require.Same(t, base, got)

	retargeted := RetargetUnsupportedContentError(wrapped, UpstreamProtocolNameGemini)
	require.True(t, strings.HasSuffix(retargeted.Error(), "served over the Gemini protocol"))
	require.Equal(t, UpstreamProtocolNameResponses, base.Upstream, "retarget must not mutate the original error")

	plain := errors.New("parse responses input")
	require.Equal(t, plain, RetargetUnsupportedContentError(plain, UpstreamProtocolNameGemini))
	_, ok = AsUnsupportedContentError(plain)
	require.False(t, ok)
}
