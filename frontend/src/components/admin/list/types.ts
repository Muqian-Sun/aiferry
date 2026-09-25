import type { IconName } from '@/components/common/types'

export type { FilterOption, IconName } from '@/components/common/types'

/**
 * 表格一行的操作（A4）。primary 的以图标按钮直接显示在行尾（最多两个），其余进「⋯」菜单。
 * 危险操作（删除、撤销…）一律放菜单里、红字，调用方负责二次确认。
 */
export interface RowAction {
  key: string
  label: string
  icon?: IconName
  primary?: boolean
  danger?: boolean
  disabled?: boolean
  /** 在它前面画一条分隔线（只对菜单项生效） */
  dividerBefore?: boolean
  onSelect: () => void
}
