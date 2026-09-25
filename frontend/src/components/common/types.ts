/**
 * Common component types
 */
import type Icon from '@/components/icons/Icon.vue'

export interface Column {
  key: string
  label: string
  sortable?: boolean
  class?: string
  formatter?: (value: any, row: any) => string
}

export type IconName = InstanceType<typeof Icon>['$props']['name']

/** 筛选标签的一个选项。value 为空串表示「全部」，不作为选项出现。 */
export interface FilterOption {
  value: string | number
  label: string
}
