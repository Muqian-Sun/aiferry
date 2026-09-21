/** 指标行的一格。 */
export interface StatItem {
  key?: string
  label: string
  value: string
  /** 数值旁的小字说明，如「标准价 $12.00」 */
  hint?: string
}

/** 页内页签（账务四页签、记录 / 错误页签）。 */
export interface SectionTab {
  key: string
  label: string
  /** 绑定路由时给出 path；不给则由 modelValue 控制。 */
  to?: string
  count?: number
}
