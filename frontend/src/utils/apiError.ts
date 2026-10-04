/**
 * Centralized API error message extraction
 *
 * The API client interceptor (api/client.ts) rejects with a plain object:
 * { status, code, message, reason?, error?, metadata? } — 字段都在顶层，没有 axios 的 response.data。
 * 曾有调用方读 error.response.data.detail，永远是 undefined，报错就静默丢了；这里不再兼容那种形状，
 * 单测也要按真实形状造错误，免得 spec 绿而线上取不到。
 *
 * 页面上的报错文案只来自前端（2026-10-04 muqian）：后端返回的 message / error 是写给 API 调用方的英文，
 * 一律不上屏。按 reason（或字符串 code）查中文：先查调用处给的命名空间，再查全局 apiErrors；
 * 都查不到就用调用处给的兜底（如「保存代理失败」）。
 */

interface ApiErrorLike {
  status?: number
  code?: number | string
  message?: string
  error?: string
  reason?: string
  metadata?: Record<string, unknown>
}

/**
 * Extract the error code from an API error object.
 *
 * Prefers the string `reason` (e.g. "PAYMENT_PROVIDER_MISCONFIGURED") over the
 * numeric HTTP `code`, because reason is granular enough to drive i18n lookup
 * while HTTP code is not.
 */
export function extractApiErrorCode(err: unknown): string | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  const code = e.reason ?? e.code
  return code != null ? String(code) : undefined
}

/**
 * Extract metadata (interpolation params) from an API error object.
 * Backend errors carry `metadata` with template variables that fill i18n placeholders.
 */
export function extractApiErrorMetadata(err: unknown): Record<string, unknown> | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  return e.metadata
}

type TranslateFn = (key: string, params?: Record<string, unknown>) => string
type TranslateWithExistsFn = TranslateFn & { te?: (key: string) => boolean }

/**
 * Translate a value via i18n if a matching key exists, otherwise return the original.
 * Example: "certSerial" → t('admin.settings.payment.field_certSerial') → "证书序列号".
 */
function tryTranslate(t: TranslateFn, key: string, fallback: string): string {
  const translated = t(key)
  if (translated === key) return fallback
  const te = (t as TranslateWithExistsFn).te
  if (te && !te(key)) return fallback
  return translated
}

/**
 * Replace raw config field names in metadata (e.g. "certSerial") with their
 * localized UI labels (e.g. "证书序列号"), using the provider-config field i18n namespace.
 * Handles both single `key` and `/`-joined `keys` patterns used by wxpay errors.
 */
function localizeMetadata(metadata: Record<string, unknown>, t: TranslateFn): Record<string, unknown> {
  const out: Record<string, unknown> = { ...metadata }
  if (typeof out.key === 'string') {
    out.key = tryTranslate(t, `admin.settings.payment.field_${out.key}`, out.key)
  }
  if (typeof out.keys === 'string') {
    out.keys = out.keys
      .split('/')
      .map(k => tryTranslate(t, `admin.settings.payment.field_${k}`, k))
      .join(' / ')
  }
  return out
}

const RATE_LIMITED_MESSAGE_KEY = 'errors.tooManyRequests'

/** 按 reason 查全局文案（apiErrors.<REASON>）；由 i18n 初始化时注册，避免这里直接依赖 vue-i18n 实例 */
type ReasonTranslator = (reason: string, params: Record<string, unknown>) => string | null
let reasonTranslator: ReasonTranslator | null = null

export function setApiErrorReasonTranslator(fn: ReasonTranslator | null): void {
  reasonTranslator = fn
}

/** 拦截器造的接口错误对象（不是前端自己 new 的 Error） */
function isApiErrorObject(err: unknown): err is ApiErrorLike {
  return !!err && typeof err === 'object' && !(err instanceof Error) && typeof (err as ApiErrorLike).status === 'number'
}

/** 断网：拦截器给的 status 0、没有任何错误码；message 是前端本地化好的 */
function isNetworkError(err: ApiErrorLike): boolean {
  return err.status === 0 && err.code == null && err.reason == null
}

function translateReasonGlobally(err: unknown, t?: TranslateFn): string | null {
  const code = extractApiErrorCode(err)
  if (!code || !reasonTranslator) return null
  const raw = extractApiErrorMetadata(err) ?? {}
  return reasonTranslator(code, t ? localizeMetadata(raw, t) : raw)
}

/** 请求被限流（HTTP 429）。拦截器把状态码放在顶层 status 上。 */
export function isRateLimitedError(err: unknown): boolean {
  return !!err && typeof err === 'object' && (err as ApiErrorLike).status === 429
}

/**
 * Extract a localized error message from an API error by looking up
 * `<namespace>.<REASON>` in i18n and substituting metadata as placeholders.
 *
 * Config-field names in metadata (`key` / `keys`) are automatically translated
 * to their UI labels before substitution, so error messages read like
 * "缺少必填项：证书序列号" instead of "缺少必填项：certSerial".
 *
 * @param err      - The caught error
 * @param t        - Vue i18n translate function
 * @param namespace- i18n key prefix, e.g. "payment.errors"
 * @param fallback - Fallback key or plain string if no localized mapping exists
 */
export function extractI18nErrorMessage(
  err: unknown,
  t: TranslateFn,
  namespace: string,
  fallback: string,
): string {
  const code = extractApiErrorCode(err)
  if (code) {
    const key = `${namespace}.${code}`
    const rawMetadata = extractApiErrorMetadata(err) ?? {}
    const metadata = localizeMetadata(rawMetadata, t)
    const translated = t(key, metadata)
    // Vue i18n returns the key itself when missing; detect that and fall back.
    if (translated !== key) return translated
    // If the framework exposes `te`, use it to double-check.
    const te = (t as TranslateWithExistsFn).te
    if (te && te(key)) return translated
  }
  const global = translateReasonGlobally(err, t)
  if (global) return global
  // 限流中间件的 429 不带 reason，只有英文的 { error, message }：按状态码兜底成统一的中文提示。
  // 带 reason 且本命名空间有映射的 429（如 VERIFY_CODE_TOO_FREQUENT）上面已经返回了。
  if (isRateLimitedError(err)) {
    const translated = t(RATE_LIMITED_MESSAGE_KEY)
    if (translated !== RATE_LIMITED_MESSAGE_KEY) return translated
  }
  return extractApiErrorMessage(err, fallback)
}

/**
 * Extract a displayable error message from an API error.
 *
 * 接口错误不返回后端的 message：按错误码查中文（i18nMap → 全局 apiErrors），查不到用 fallback。
 * 只有两类直接用 message：断网（拦截器本地化过）和前端自己 throw 的 Error（文案由前端写）。
 *
 * @param err - The caught error (unknown type)
 * @param fallback - 调用处的中文兜底，写清是哪个动作失败（如 t('admin.proxies.failedToCreate')）
 * @param i18nMap - Optional map of error codes to i18n translated strings
 */
export function extractApiErrorMessage(
  err: unknown,
  fallback = 'Unknown error',
  i18nMap?: Record<string, string>,
): string {
  if (!err) return fallback

  // Try i18n mapping by error code first
  if (i18nMap) {
    const code = extractApiErrorCode(err)
    if (code && i18nMap[code]) return i18nMap[code]
  }

  if (isApiErrorObject(err)) {
    if (isNetworkError(err)) return err.message || fallback
    return translateReasonGlobally(err) ?? fallback
  }

  // 前端自己抛的错误（文件读不出、图片太大……），文案是本地化过的
  if (err instanceof Error) return err.message || fallback

  return fallback
}
