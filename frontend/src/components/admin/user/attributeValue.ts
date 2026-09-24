import type { UserAttributeDefinition } from '@/types'

/**
 * 自定义属性的展示值：单选 / 多选把选项值换成显示文本，其余原样。
 * 用户列表的属性列与用户详情抽屉共用。
 */
export function formatAttributeValue(def: UserAttributeDefinition | undefined, value: string): string {
  if (!value || !def) return value

  if (def.type === 'multi_select') {
    try {
      const arr = JSON.parse(value)
      if (Array.isArray(arr)) {
        return arr.map((v) => def.options?.find((o) => o.value === v)?.label || v).join(', ')
      }
    } catch {
      return value
    }
  }

  if (def.type === 'select' && def.options) {
    return def.options.find((o) => o.value === value)?.label || value
  }

  return value
}
