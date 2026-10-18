package service

import "strings"

// 视频计费档：与规范分辨率（NormalizeVideoResolution）一一对应，目录条目的视频分档只认这几个（区分大小写）。
const (
	VideoBillingResolution480P  = "480p"
	VideoBillingResolution720P  = "720p"
	VideoBillingResolution768P  = "768p"
	VideoBillingResolution1080P = "1080p"
	VideoBillingResolution2K    = "2k"
	VideoBillingResolution4K    = "4k"
)

// VideoBillingResolutions 全部视频计费档，按清晰度从低到高。
func VideoBillingResolutions() []string {
	return []string{
		VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution768P,
		VideoBillingResolution1080P, VideoBillingResolution2K, VideoBillingResolution4K,
	}
}

// 视频按秒计费。计费时长必须与上游实际消耗对齐，否则用户可通过拉长 duration 套利（提交时长由用户控制）。
// 上限是接入的上游里最长的（Seedance 2.5 官方 4–30 秒），各适配器按模型再收窄；没指定时长按 8 秒计（xAI 的默认）。
const (
	VideoBillingMinDurationSeconds     = 1
	VideoBillingMaxDurationSeconds     = 30
	VideoBillingDefaultDurationSeconds = 8
)

// GrokVideoMaxDurationSeconds xAI 视频 duration 请求参数允许 1–15 秒。
const GrokVideoMaxDurationSeconds = 15

// NormalizeVideoBillingDurationSecondsOrDefault 归一化计费用视频时长：
// 未指定（<=0）按默认 8 秒计，超出区间按边界收敛。
func NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds int) int {
	return clampVideoDurationSecondsOrDefault(durationSeconds, VideoBillingMaxDurationSeconds)
}

// NormalizeGrokVideoDurationSecondsOrDefault 同上，上限按 xAI 的 15 秒（旧 Grok 媒体路径按请求时长计费）。
func NormalizeGrokVideoDurationSecondsOrDefault(durationSeconds int) int {
	return clampVideoDurationSecondsOrDefault(durationSeconds, GrokVideoMaxDurationSeconds)
}

func clampVideoDurationSecondsOrDefault(durationSeconds, maxSeconds int) int {
	if durationSeconds <= 0 {
		return VideoBillingDefaultDurationSeconds
	}
	return min(max(durationSeconds, VideoBillingMinDurationSeconds), maxSeconds)
}

// LookupVideoBillingResolution 归一化分辨率并报告是否为已知档位。
// 配置解析路径必须用它而不是 OrDefault：把无法识别的档位（如拼错的 "1080i"）
// 静默折算成 480p，会让管理员配的高分辨率单价被挂到低分辨率档上。
func LookupVideoBillingResolution(resolution string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480", "480p", "sd":
		return VideoBillingResolution480P, true
	case "720", "720p", "hd":
		return VideoBillingResolution720P, true
	case "768", "768p":
		return VideoBillingResolution768P, true
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		return VideoBillingResolution1080P, true
	case "2k", "1440", "1440p", "qhd":
		return VideoBillingResolution2K, true
	case "4k", "2160", "2160p", "uhd":
		return VideoBillingResolution4K, true
	default:
		return "", false
	}
}

// NormalizeVideoBillingResolutionOrDefault 用于运行时计费：上游回传的分辨率
// 缺失或无法识别时按最低档兜底，保证请求仍可计费。
func NormalizeVideoBillingResolutionOrDefault(resolution string) string {
	if normalized, ok := LookupVideoBillingResolution(resolution); ok {
		return normalized
	}
	return VideoBillingResolution480P
}
