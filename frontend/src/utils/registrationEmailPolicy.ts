const EMAIL_SUFFIX_TOKEN_SPLIT_RE = /[\s,，]+/
const EMAIL_SUFFIX_INVALID_CHAR_RE = /[^a-z0-9.-]/g
const EMAIL_SUFFIX_INVALID_CHAR_CHECK_RE = /[^a-z0-9.-]/
const EMAIL_SUFFIX_PREFIX_RE = /^@+/
const EMAIL_SUFFIX_WILDCARD_PREFIX = '*.'
const EMAIL_SUFFIX_MESSAGE_VISIBLE_LIMIT = 5
const EMAIL_SUFFIX_DOMAIN_PATTERN =
  /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/

// normalizeRegistrationEmailSuffixDomain converts raw input into a canonical domain token.
// Exact domains are returned without "@"; wildcard domains keep the "*." prefix.
export function normalizeRegistrationEmailSuffixDomain(raw: string): string {
  let value = String(raw || '').trim().toLowerCase()
  if (!value) {
    return ''
  }

  value = value.replace(EMAIL_SUFFIX_PREFIX_RE, '')
  return normalizeRegistrationEmailSuffixToken(value, false)
}

export function normalizeRegistrationEmailSuffixDomains(
  items: string[] | null | undefined
): string[] {
  if (!items || items.length === 0) {
    return []
  }

  const seen = new Set<string>()
  const normalized: string[] = []
  for (const item of items) {
    const domain = normalizeRegistrationEmailSuffixDomain(item)
    if (!isRegistrationEmailSuffixDomainValid(domain) || seen.has(domain)) {
      continue
    }
    seen.add(domain)
    normalized.push(domain)
  }
  return normalized
}

export function parseRegistrationEmailSuffixWhitelistInput(input: string): string[] {
  if (!input || !input.trim()) {
    return []
  }

  const seen = new Set<string>()
  const normalized: string[] = []

  for (const token of input.split(EMAIL_SUFFIX_TOKEN_SPLIT_RE)) {
    const domain = normalizeRegistrationEmailSuffixDomainStrict(token)
    if (!isRegistrationEmailSuffixDomainValid(domain) || seen.has(domain)) {
      continue
    }
    seen.add(domain)
    normalized.push(domain)
  }

  return normalized
}

export function normalizeRegistrationEmailSuffixWhitelist(
  items: string[] | null | undefined
): string[] {
  return normalizeRegistrationEmailSuffixDomains(items).map(toCanonicalRegistrationEmailSuffix)
}

function extractRegistrationEmailDomain(email: string): string {
  const raw = String(email || '').trim().toLowerCase()
  if (!raw) {
    return ''
  }
  const atIndex = raw.indexOf('@')
  if (atIndex <= 0 || atIndex >= raw.length - 1) {
    return ''
  }
  if (raw.indexOf('@', atIndex + 1) !== -1) {
    return ''
  }
  return raw.slice(atIndex + 1)
}

export function isRegistrationEmailSuffixAllowed(
  email: string,
  whitelist: string[] | null | undefined
): boolean {
  const normalizedWhitelist = normalizeRegistrationEmailSuffixWhitelist(whitelist)
  if (normalizedWhitelist.length === 0) {
    return true
  }
  const emailDomain = extractRegistrationEmailDomain(email)
  if (!emailDomain) {
    return false
  }
  const emailSuffix = `@${emailDomain}`
  return normalizedWhitelist.some((allowed) => {
    if (allowed.startsWith('@')) {
      return allowed === emailSuffix
    }
    if (allowed.startsWith(EMAIL_SUFFIX_WILDCARD_PREFIX)) {
      const base = allowed.slice(EMAIL_SUFFIX_WILDCARD_PREFIX.length)
      return emailDomain === base || emailDomain.endsWith(`.${base}`)
    }
    return false
  })
}

// 列出前几项，其余写成「等 N 个」：这一截跟在列表后面、用空格隔开，不当成列表的一项（否则中文会成「a、等 7 项」）
export function formatRegistrationEmailSuffixWhitelistForMessage(
  whitelist: string[] | null | undefined,
  options: {
    separator: string
    /** hidden = 没列出来的个数，total = 总数 */
    more: (hidden: number, total: number) => string
  }
): string {
  const normalizedWhitelist = normalizeRegistrationEmailSuffixWhitelist(whitelist)
  const visible = normalizedWhitelist.slice(0, EMAIL_SUFFIX_MESSAGE_VISIBLE_LIMIT)
  const hiddenCount = normalizedWhitelist.length - visible.length
  const listed = visible.join(options.separator)
  return hiddenCount > 0 ? `${listed} ${options.more(hiddenCount, normalizedWhitelist.length)}` : listed
}

// Pasted domains should be strict: any invalid character drops the whole token.
function normalizeRegistrationEmailSuffixDomainStrict(raw: string): string {
  let value = String(raw || '').trim().toLowerCase()
  if (!value) {
    return ''
  }
  value = value.replace(EMAIL_SUFFIX_PREFIX_RE, '')
  return normalizeRegistrationEmailSuffixToken(value, true)
}

export function isRegistrationEmailSuffixDomainValid(domain: string): boolean {
  if (!domain) {
    return false
  }
  if (domain.startsWith(EMAIL_SUFFIX_WILDCARD_PREFIX)) {
    return EMAIL_SUFFIX_DOMAIN_PATTERN.test(domain.slice(EMAIL_SUFFIX_WILDCARD_PREFIX.length))
  }
  return !domain.includes('*') && EMAIL_SUFFIX_DOMAIN_PATTERN.test(domain)
}

function normalizeRegistrationEmailSuffixToken(value: string, strict: boolean): string {
  if (value.startsWith(EMAIL_SUFFIX_WILDCARD_PREFIX)) {
    const domain = value.slice(EMAIL_SUFFIX_WILDCARD_PREFIX.length)
    if (strict && (!domain || EMAIL_SUFFIX_INVALID_CHAR_CHECK_RE.test(domain))) {
      return ''
    }
    return `${EMAIL_SUFFIX_WILDCARD_PREFIX}${domain.replace(EMAIL_SUFFIX_INVALID_CHAR_RE, '')}`
  }

  if (value === '*') {
    return strict ? '' : value
  }

  if (strict && EMAIL_SUFFIX_INVALID_CHAR_CHECK_RE.test(value)) {
    return ''
  }
  return value.replace(/[*]/g, '').replace(EMAIL_SUFFIX_INVALID_CHAR_RE, '')
}

function toCanonicalRegistrationEmailSuffix(domain: string): string {
  return domain.startsWith(EMAIL_SUFFIX_WILDCARD_PREFIX) ? domain : `@${domain}`
}
