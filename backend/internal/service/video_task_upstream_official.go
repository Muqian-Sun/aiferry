package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 官方视频厂商（Seedance / 可灵 / MiniMax）适配器的公共部分：key 的 video 地址就是厂商官方根地址，
// 认证都是 Authorization: Bearer <API Key>；成片是厂商给的限时签名地址，下载不带凭据。
// 请求先按上游模型校验并补齐默认时长 / 分辨率（PrepareRequest），超出模型支持范围的一律 400，不收敛：
// 收敛会让实际生成与计费的时长 / 分辨率和客户端要的不一样。

// officialVideoModelSpec 一个上游模型支持的分辨率与时长（取自各家官方文档，见各适配器文件头）。
type officialVideoModelSpec struct {
	resolutions       []string
	defaultResolution string
	// durations 非空时只能取这几个值（可灵 2.6 / 2.5 Turbo 只有 5 / 10 秒），否则取 [minDuration, maxDuration]。
	durations       []int
	minDuration     int
	maxDuration     int
	defaultDuration int
}

// apply 补齐默认值并校验分辨率与时长。
func (spec officialVideoModelSpec) apply(req VideoTaskRequest, model string) (VideoTaskRequest, error) {
	if req.Resolution == "" {
		req.Resolution = spec.defaultResolution
	}
	if !slices.Contains(spec.resolutions, req.Resolution) {
		return req, newVideoTaskInvalidRequest("resolution %s is not supported by %s (supported: %s)", req.Resolution, model, strings.Join(spec.resolutions, ", "))
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = spec.defaultDuration
	}
	if len(spec.durations) > 0 {
		if !slices.Contains(spec.durations, req.DurationSeconds) {
			return req, newVideoTaskInvalidRequest("duration must be one of %s seconds for %s", joinInts(spec.durations), model)
		}
		return req, nil
	}
	if req.DurationSeconds < spec.minDuration || req.DurationSeconds > spec.maxDuration {
		return req, newVideoTaskInvalidRequest("duration must be between %d and %d seconds for %s", spec.minDuration, spec.maxDuration, model)
	}
	return req, nil
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}

// videoImageCounts 按角色数输入图。
type videoImageCounts struct{ first, last, reference int }

func countVideoImages(req VideoTaskRequest) videoImageCounts {
	var counts videoImageCounts
	for _, image := range req.Images {
		switch image.Role {
		case VideoInputImageFirstFrame:
			counts.first++
		case VideoInputImageLastFrame:
			counts.last++
		case VideoInputImageReference:
			counts.reference++
		}
	}
	return counts
}

// validateFrameImages 首帧 / 尾帧各最多一张；首尾帧与参考图不能混用（三家文档都这样约束）。
func validateFrameImages(counts videoImageCounts) error {
	if counts.first > 1 || counts.last > 1 {
		return newVideoTaskInvalidRequest("at most one first_frame and one last_frame image are supported")
	}
	if counts.reference > 0 && counts.first+counts.last > 0 {
		return newVideoTaskInvalidRequest("first/last frame images cannot be combined with reference images")
	}
	return nil
}

func validateVideoAspectRatio(ratio string, allowed []string, vendor string) error {
	if ratio == "" || slices.Contains(allowed, ratio) {
		return nil
	}
	return newVideoTaskInvalidRequest("aspect ratio %s is not supported by %s (supported: %s)", ratio, vendor, strings.Join(allowed, ", "))
}

// officialVideoModelKey 规范化的模型名：小写、点换成连字符（seedance-2.0-fast 与 doubao-seedance-2-0-fast-260128 同一套写法）。
func officialVideoModelKey(model string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(model)), ".", "-")
}

// mergeVideoExtraDeep 把 Extra 并进上游请求体：规范字段已写的键不覆盖；两边都是对象时逐层合并
// （可灵的 settings / options 是嵌套对象，透传 {"settings":{"multi_shot":false}} 要并进去）。
func mergeVideoExtraDeep(body map[string]any, extra map[string]any) {
	for key, value := range extra {
		existing, exists := body[key]
		if !exists {
			body[key] = value
			continue
		}
		existingMap, ok1 := existing.(map[string]any)
		valueMap, ok2 := value.(map[string]any)
		if ok1 && ok2 {
			mergeVideoExtraDeep(existingMap, valueMap)
		}
	}
}

// officialVideoBaseURL key 的 video 地址（厂商官方根地址），过出站地址校验。
func (s *OpenAIGatewayService) officialVideoBaseURL(account *Account) (string, error) {
	baseURL, err := ResolveUpstreamBaseURL(account, account.ProtocolEndpoint(APIProtocolVideo), APIProtocolVideo, "")
	if err != nil {
		return "", err
	}
	return s.validateUpstreamBaseURL(baseURL)
}

// doOfficialVideoJSON 向官方视频 API 发一个 JSON 请求（带 key 与账号级请求头），读回 2xx 响应体。
func (s *OpenAIGatewayService) doOfficialVideoJSON(ctx context.Context, account *Account, method, endpoint string, query url.Values, payload any) ([]byte, error) {
	root, err := s.officialVideoBaseURL(account)
	if err != nil {
		return nil, err
	}
	target := joinUpstreamEndpointURL(root, endpoint)
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	httpReq, err := newVideoUpstreamJSONRequest(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), method, target, payload)
	if err != nil {
		return nil, err
	}
	if err := applyVideoKeyHeaders(httpReq, account); err != nil {
		return nil, err
	}
	body, _, err := s.doVideoUpstreamJSON(httpReq, account)
	return body, err
}

// openPublicVideoContent 下载厂商给的签名成片地址：不带任何凭据与账号请求头，只允许公网 http(s) 目的地
// （地址来自上游响应，不可信），不跟随重定向。
func (s *OpenAIGatewayService) openPublicVideoContent(ctx context.Context, c *gin.Context, account *Account, rawURL string) (*http.Response, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errVideoTaskNoVideoURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid video url %q", safeUpstreamURL(rawURL))
	}
	downloadURL, err := s.validateOutboundURL(parsed.String())
	if err != nil {
		return nil, fmt.Errorf("invalid video url: %w", err)
	}
	if err := rejectPrivateImageHost(downloadURL); err != nil {
		return nil, err
	}
	contentCtx := WithHTTPUpstreamPublicHostsOnly(WithHTTPUpstreamRedirectsDisabled(ctx))
	req, err := http.NewRequestWithContext(contentCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	copyVideoRangeHeader(c, req)
	return s.doVideoContentRequest(req, account)
}
