/**
 * 新建 / 编辑用户共用的表单状态与校验（muqian 2026-09-30 方案第四节）。
 *
 * 并发、RPM：清空按 0（= 不限）处理，负数、小数不能保存；倍率：留空 = 跟全站默认，填了必须是非负数；
 * 初始余额（只在新建）：留空按 0，负数不能保存。数字框的 v-model 会自动转成数字，所以这里收 string | number。
 */

export type UserRole = 'user' | 'admin'

export interface UserFormState {
  email: string
  password: string
  username: string
  role: UserRole
  rate_multiplier: string | number
  balance: string | number
  concurrency: string | number
  rpm_limit: string | number
  notes: string
}

export type UserFormField = 'email' | 'password' | 'rate_multiplier' | 'balance' | 'concurrency' | 'rpm_limit'
export type UserFormErrors = Partial<Record<UserFormField, string>>

export function emptyUserForm(): UserFormState {
  return {
    email: '',
    password: '',
    username: '',
    role: 'user',
    rate_multiplier: '',
    balance: '',
    concurrency: '',
    rpm_limit: '',
    notes: ''
  }
}

const text = (raw: string | number): string => String(raw ?? '').trim()

/** 并发 / RPM：空 = 0；非负整数才行，否则 null */
export function parseLimit(raw: string | number): number | null {
  const value = text(raw)
  if (value === '') return 0
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : null
}

/** 倍率：空 = undefined（跟全站默认）；非负数才行，否则 null */
export function parseRate(raw: string | number): number | undefined | null {
  const value = text(raw)
  if (value === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

/** 金额：空 = 0；非负数才行，否则 null */
export function parseAmount(raw: string | number): number | null {
  const value = text(raw)
  if (value === '') return 0
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

export const MIN_PASSWORD_LENGTH = 6

export interface ValidatedUserForm {
  concurrency: number
  rpm_limit: number
  /** undefined = 跟全站默认 */
  rate_multiplier: number | undefined
  balance: number
}

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 逐个字段校验，有问题的字段记进 errors；errors 为空时 values 可直接用来拼请求 */
export function validateUserForm(
  form: UserFormState,
  mode: 'create' | 'edit',
  t: Translate
): { errors: UserFormErrors; values: ValidatedUserForm } {
  const errors: UserFormErrors = {}
  if (!form.email.trim()) errors.email = t('admin.users.emailRequired')
  const password = form.password.trim()
  if (mode === 'create' && !password) errors.password = t('admin.users.form.passwordRequired')
  else if (password && password.length < MIN_PASSWORD_LENGTH) {
    errors.password = t('admin.users.form.passwordTooShort', { min: MIN_PASSWORD_LENGTH })
  }

  const concurrency = parseLimit(form.concurrency)
  if (concurrency === null) errors.concurrency = t('admin.users.form.nonNegativeInteger')
  const rpmLimit = parseLimit(form.rpm_limit)
  if (rpmLimit === null) errors.rpm_limit = t('admin.users.form.nonNegativeInteger')
  const rate = parseRate(form.rate_multiplier)
  if (rate === null) errors.rate_multiplier = t('admin.users.form.nonNegativeNumber')
  const balance = mode === 'create' ? parseAmount(form.balance) : 0
  if (balance === null) errors.balance = t('admin.users.form.nonNegativeNumber')

  return {
    errors,
    values: {
      concurrency: concurrency ?? 0,
      rpm_limit: rpmLimit ?? 0,
      rate_multiplier: rate ?? undefined,
      balance: balance ?? 0
    }
  }
}

/** 16 位随机密码（去掉易混字符）；用 crypto.getRandomValues，不用 Math.random */
export function randomPassword(): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  const bytes = new Uint32Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (value) => chars.charAt(value % chars.length)).join('')
}
