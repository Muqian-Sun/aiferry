package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

// Claude → v1internal：base64 图片 / PDF 内联，纯文本文档转 text；URL / file_id / 非 PDF
// 文档返回 UnsupportedContentError（400），不再静默丢弃。
func TestBuildParts_MultimodalParts(t *testing.T) {
	const pdfBase64 = "JVBERi0xLjQK"
	tests := []struct {
		name    string
		content string
		want    []GeminiPart
		wantErr string
	}{
		{
			name:    "base64 image inlined (unchanged)",
			content: `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}]`,
			want:    []GeminiPart{{InlineData: &GeminiInlineData{MimeType: "image/png", Data: "iVBORw0KGgo="}}},
		},
		{
			name:    "base64 pdf inlined as application/pdf",
			content: `[{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"` + pdfBase64 + `"}}]`,
			want:    []GeminiPart{{InlineData: &GeminiInlineData{MimeType: "application/pdf", Data: pdfBase64}}},
		},
		{
			name:    "text document becomes a text part",
			content: `[{"type":"document","source":{"type":"text","media_type":"text/plain","data":"plain body"}}]`,
			want:    []GeminiPart{{Text: "plain body"}},
		},
		{name: "https image rejected", content: `[{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}}]`, wantErr: "image with a URL source"},
		{name: "pdf url rejected", content: `[{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}}]`, wantErr: "document with a URL source"},
		{name: "document file_id rejected", content: `[{"type":"document","source":{"type":"file","file_id":"file_01"}}]`, wantErr: "document with an Anthropic file_id source"},
		{name: "non-pdf base64 document rejected", content: `[{"type":"document","source":{"type":"base64","media_type":"text/html","data":"PGgxPg=="}}]`, wantErr: `media type "text/html"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, _, err := buildParts(json.RawMessage(tt.content), map[string]string{}, true)
			if tt.wantErr != "" {
				unsupported, ok := apicompat.AsUnsupportedContentError(err)
				require.True(t, ok, "expected UnsupportedContentError, got %v", err)
				require.Equal(t, apicompat.UpstreamProtocolNameGemini, unsupported.Upstream)
				require.Contains(t, unsupported.Part, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, parts)
		})
	}
}

// 错误沿 TransformClaudeToGemini 的包装链保留类型，service 层据此写 400 而不是笼统的 "Invalid request"。
func TestTransformClaudeToGemini_UnsupportedContentKeepsType(t *testing.T) {
	req := &ClaudeRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 64,
		Messages: []ClaudeMessage{{
			Role:    "user",
			Content: json.RawMessage(`[{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}}]`),
		}},
	}
	_, err := TransformClaudeToGemini(req, "project-1", "claude-sonnet-4-5")
	_, ok := apicompat.AsUnsupportedContentError(err)
	require.True(t, ok, "expected UnsupportedContentError in the chain, got %v", err)
}
