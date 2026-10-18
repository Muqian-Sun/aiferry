package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// geminiImageOutputCounterKey 是请求级内联图片计数器挂在 gin.Context 上的键。
const geminiImageOutputCounterKey = "gemini_image_output_counter"

// geminiInlineImageTally 按图片数据去重地数一次响应里 Gemini 回吐的内联图片，
// 计费（geminiImageOutputCounter）与兼容层回图（流式只发新出现的图）共用同一口径：
//   - 同一份数据在单个 payload 里出现 n 次算 n 张；
//   - 跨 payload 取每份数据出现次数的最大值。累积式 SSE 会把已发过的图整段重发
//     （本文件同目录的 computeGeminiTextDelta 就是为此存在的），逐 chunk 累加会重复计费；
//     增量式流（Antigravity / Gemini 官方）多张图分散在不同 chunk 里，每张都要算。
type geminiInlineImageTally struct {
	maxPerDigest map[[sha256.Size]byte]int
	total        int
}

// geminiInlineImagePayload 是一个 payload 内各份图片数据已出现的次数。
type geminiInlineImagePayload map[[sha256.Size]byte]int

// admit 登记 payload 里的一张图，报告它是不是此前没数过的新图。
func (t *geminiInlineImageTally) admit(payload geminiInlineImagePayload, data string) bool {
	digest := sha256.Sum256([]byte(data))
	payload[digest]++
	if payload[digest] <= t.maxPerDigest[digest] {
		return false
	}
	if t.maxPerDigest == nil {
		t.maxPerDigest = make(map[[sha256.Size]byte]int)
	}
	t.maxPerDigest[digest] = payload[digest]
	t.total++
	return true
}

// geminiImageOutputCounter 记录一次转发里 Gemini 上游真正回吐的内联图片数量。
type geminiImageOutputCounter struct {
	tally geminiInlineImageTally
}

// beginGeminiImageOutputObservation 在每次 Forward 开头重置计数器。
// failover 会拿同一个 gin.Context 重跑转发，不重置就会把上一个账号的图数带进来。
func beginGeminiImageOutputObservation(c *gin.Context) *geminiImageOutputCounter {
	if c == nil {
		return nil
	}
	counter := &geminiImageOutputCounter{}
	c.Set(geminiImageOutputCounterKey, counter)
	return counter
}

func geminiImageOutputCounterFromContext(c *gin.Context) *geminiImageOutputCounter {
	if c == nil {
		return nil
	}
	value, ok := c.Get(geminiImageOutputCounterKey)
	if !ok {
		return nil
	}
	counter, _ := value.(*geminiImageOutputCounter)
	return counter
}

// observeGeminiImageOutputs 观测一段上游响应（整份或单个 chunk，裸 Gemini 响应或
// v1internal 的 {"response": ...} 封装都认）里的内联图片。
func observeGeminiImageOutputs(c *gin.Context, payload []byte) {
	counter := geminiImageOutputCounterFromContext(c)
	if counter == nil {
		return
	}
	seen := geminiInlineImagePayload{}
	forEachGeminiInlineImageOutput(payload, func(_, data string) {
		counter.tally.admit(seen, data)
	})
}

func observedGeminiImageOutputs(c *gin.Context) int {
	counter := geminiImageOutputCounterFromContext(c)
	if counter == nil {
		return 0
	}
	return counter.tally.total
}

// resolveGeminiImageCount 决定本次请求按几张图计费，所有 Gemini 出图入口（原生 /v1beta、
// 对话入站的 messages / chat / responses，Antigravity 与 Gemini 兼容层）同一条规则：
//
//  1. 上游真正返回了内联图片：按观测到的张数计。走 GeminiMessagesCompatService 的账号多是
//     API Key + 自定义模型映射，客户端请求名和上游模型名都可能是站长自取的别名
//     （issue #5358 里的 nana-banana-2），模型名白名单判不出。
//  2. 一张图都没数到（模型拒绝出图 / 只回了文字，或上游用 fileData 引用回图）：请求名或映射后的
//     上游名是已知生图模型时按 1 张计，否则 0（落 token 路径；条目是按张计价时 token 路径仍按
//     1 次按次价计）。这是改动前 /v1beta 的既有口径，保证不回退。
func resolveGeminiImageCount(c *gin.Context, originalModel, mappedModel string) int {
	if observed := observedGeminiImageOutputs(c); observed > 0 {
		return observed
	}
	if isImageGenerationModel(originalModel) || isImageGenerationModel(mappedModel) {
		return 1
	}
	return 0
}

// countGeminiInlineImageOutputs 统计单个 Gemini 响应 JSON 里的内联图片 part。
func countGeminiInlineImageOutputs(payload []byte) int {
	count := 0
	forEachGeminiInlineImageOutput(payload, func(_, _ string) { count++ })
	return count
}

// forEachGeminiInlineImageOutput 逐个交出响应里可计费的内联图片。
func forEachGeminiInlineImageOutput(payload []byte, fn func(mimeType, data string)) {
	// 对话流量绝大多数 chunk 没有图：先按字节筛掉，免得每个 chunk 都做一次 JSON 校验。
	if !bytes.Contains(payload, []byte("inline")) || !gjson.ValidBytes(payload) {
		return
	}
	candidates := gjson.GetBytes(payload, "candidates")
	if !candidates.Exists() {
		candidates = gjson.GetBytes(payload, "response.candidates")
	}
	candidates.ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			if mimeType, data, ok := geminiPartInlineImage(part); ok {
				fn(mimeType, data)
			}
			return true
		})
		return true
	})
}

// geminiPartInlineImage 取 part 里的内联图片。Gemini REST 回 camelCase 的 inlineData，
// 官方 SDK 与部分中转会回 snake_case 的 inline_data，两种都要认。
func geminiPartInlineImage(part gjson.Result) (mimeType, data string, ok bool) {
	if part.Get("thought").Bool() {
		return "", "", false
	}
	inline := part.Get("inlineData")
	if !inline.Exists() {
		inline = part.Get("inline_data")
	}
	if !inline.Exists() {
		return "", "", false
	}
	mime := inline.Get("mimeType")
	if !mime.Exists() {
		mime = inline.Get("mime_type")
	}
	data = inline.Get("data").String()
	if !isGeminiInlineImageOutput(mime.String(), data) {
		return "", "", false
	}
	return mime.String(), data, true
}

// geminiInlineImagePart 是 geminiPartInlineImage 的 map 版本，供已经反序列化 part 的转换层用。
func geminiInlineImagePart(part map[string]any) (mimeType, data string, ok bool) {
	if thought, _ := part["thought"].(bool); thought {
		return "", "", false
	}
	inline, found := part["inlineData"].(map[string]any)
	if !found {
		inline, found = part["inline_data"].(map[string]any)
	}
	if !found {
		return "", "", false
	}
	mimeType, found = inline["mimeType"].(string)
	if !found {
		mimeType, _ = inline["mime_type"].(string)
	}
	data, _ = inline["data"].(string)
	if !isGeminiInlineImageOutput(mimeType, data) {
		return "", "", false
	}
	return mimeType, data, true
}

// isGeminiInlineImageOutput 只认数据是合法 base64 的图片 part：空壳 part、音频等非图片附件不计费也不回图，
// 非法数据拼进 markdown 会破坏回包；思考过程里的草图（thought=true）由调用方先排除。
func isGeminiInlineImageOutput(mimeType, data string) bool {
	if !isGeminiInlineImageMIMEType(strings.ToLower(strings.TrimSpace(mimeType))) || data == "" {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(data)
	return err == nil
}

// geminiInlineImageMarkdown 是对话入站回图的统一格式，与 Antigravity 转换器（pkg/antigravity）写出的一致。
func geminiInlineImageMarkdown(mimeType, data string) string {
	return fmt.Sprintf("![image](data:%s;base64,%s)", mimeType, data)
}
