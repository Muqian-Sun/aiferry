/** 指标行的一格。 */
export interface StatItem {
  key?: string
  label: string
  value: string
  /** 数值旁的小字说明，如「标准价 $12.00」 */
  hint?: string
  /** 数值旁的动作链接，如「前往充值」 */
  link?: { to: string; label: string }
  /** 数值旁的页内动作（不跳路由），如列表页摘要「上架但无渠道 3 · 筛选」 */
  action?: { label: string; onClick: () => void }
  /** 数值的文字色，不传是正文色；只用于状态（如利润为负时 text-af-danger） */
  valueClass?: string
  /**
   * 整数从 0 跳到位（style.css 的 .count-up，与模型页页首数字同一套）：要祖先挂 v-reveal 触发；
   * value 仍要给，作读屏与测试用的真实值
   */
  countTo?: number
}

/** 页内页签（账务四页签、记录 / 错误页签）。 */
export interface SectionTab {
  key: string
  label: string
  /** 绑定路由时给出 path；不给则由 modelValue 控制。 */
  to?: string
  count?: number
}
