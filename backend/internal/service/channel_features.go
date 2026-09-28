package service

// 渠道级行为（2026-09-28 P5：渠道表单里通用 / 第三方 key 共用的配置写进代码，渠道上不再能改）。
// 库里旧行上的这些键不再读取（开发阶段没有历史数据，不写迁移）。要改就改这里、重新发版。
//
// 同一批一起收掉、不需要常量的（读取处直接删掉了条件或分支）：
//   - 过期自动暂停：设了过期时间就到期停调，不再看渠道开关；
//   - 负载因子：调度负载一律按并发数算；
//   - 渠道自动停调阈值覆盖：只走全站表 accountSchedulingThresholds（gateway_features.go）；
//   - 限额重置方式：日 / 周限额一律滚动窗口（从本周期第一笔用量起算 24 小时 / 7 天），没有固定时间重置；
//   - 请求头覆写开关：覆写表里有条目就生效；
//   - 自定义错误码、临时不可调度规则：删掉，上游错误一律按默认策略处理；
//   - Ollama Cloud 自动刷新用量：配了浏览器 Cookie 就定时刷新（全站开关见 ollamaCloudUsageEnabled）。

// PoolModeRetryCount 池模式渠道遇到可重试状态码时，在同一渠道上重试几次：3（原默认值；池模式开关本身仍按渠道）。
const PoolModeRetryCount = 3

// poolModeRetryStatusCodes 池模式下触发同渠道重试的上游状态码：401 / 403 / 429（原默认值）。
var poolModeRetryStatusCodes = []int{401, 403, 429}

// AccountQuotaNotifyRemainingPercent 渠道额度提醒：设了日 / 周 / 总限额的渠道，剩余额度降到该维度限额的 20%
// （即用到 80%）时给管理员发一次邮件；全站门是配了 SMTP。80% 是拍的，按实际收到提醒的时机再调。
const AccountQuotaNotifyRemainingPercent = 20.0

// upstreamRequestIDHeaders 记录上游请求标识时依次查的响应头（大小写不敏感），取第一个非空值写进
// usage_logs.upstream_request_id：Anthropic、通用 / 中转、One API 系、Google、xAI、sub2api 中转。
var upstreamRequestIDHeaders = []string{
	"request-id",
	"x-request-id",
	"x-oneapi-request-id",
	"x-goog-request-id",
	"xai-request-id",
	"x-client-request-id",
}
