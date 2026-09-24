package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

// Anthropic Messages → Gemini：base64 图片 / PDF 内联，纯文本文档转 text；URL / file_id /
// 非 PDF 文档返回 UnsupportedContentError（400），不再静默丢弃，也不再把整个块 JSON
// 序列化进 text 分片。
func TestConvertClaudeMessagesToGemini_MultimodalParts(t *testing.T) {
	const pdfBase64 = "JVBERi0xLjQK"
	tests := []struct {
		name      string
		blocks    string
		wantParts []map[string]any
		wantErr   string
	}{
		{
			name:   "base64 image inlined (unchanged)",
			blocks: `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}]`,
			wantParts: []map[string]any{
				{"inlineData": map[string]any{"mimeType": "image/png", "data": "iVBORw0KGgo="}},
			},
		},
		{
			name:   "base64 pdf inlined as application/pdf",
			blocks: `[{"type":"text","text":"summarise"},{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"` + pdfBase64 + `"}}]`,
			wantParts: []map[string]any{
				{"text": "summarise"},
				{"inlineData": map[string]any{"mimeType": "application/pdf", "data": pdfBase64}},
			},
		},
		{
			name:      "text document becomes a text part",
			blocks:    `[{"type":"document","source":{"type":"text","media_type":"text/plain","data":"plain body"}}]`,
			wantParts: []map[string]any{{"text": "plain body"}},
		},
		{name: "https image rejected", blocks: `[{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}}]`, wantErr: "image with a URL source"},
		{name: "pdf url rejected", blocks: `[{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}}]`, wantErr: "document with a URL source"},
		{name: "image file_id rejected", blocks: `[{"type":"image","source":{"type":"file","file_id":"file_01"}}]`, wantErr: "image with an Anthropic file_id source"},
		{name: "document file_id rejected", blocks: `[{"type":"document","source":{"type":"file","file_id":"file_01"}}]`, wantErr: "document with an Anthropic file_id source"},
		{name: "non-pdf base64 document rejected", blocks: `[{"type":"document","source":{"type":"base64","media_type":"image/tiff","data":"AAAA"}}]`, wantErr: `media type "image/tiff"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"gemini-2.5-flash","max_tokens":64,"messages":[{"role":"user","content":` + tt.blocks + `}]}`)
			out, err := convertClaudeMessagesToGeminiGenerateContent(body)
			if tt.wantErr != "" {
				unsupported, ok := apicompat.AsUnsupportedContentError(err)
				require.True(t, ok, "expected UnsupportedContentError, got %v", err)
				require.Equal(t, apicompat.UpstreamProtocolNameGemini, unsupported.Upstream)
				require.Contains(t, unsupported.Part, tt.wantErr)
				return
			}
			require.NoError(t, err)
			var req struct {
				Contents []struct {
					Parts []map[string]any `json:"parts"`
				} `json:"contents"`
			}
			require.NoError(t, json.Unmarshal(out, &req))
			require.Len(t, req.Contents, 1)
			require.Equal(t, tt.wantParts, req.Contents[0].Parts)
			require.NotContains(t, string(out), `\"type\":\"document\"`, "document 块不能再被 JSON 序列化进 text 分片")
		})
	}
}
