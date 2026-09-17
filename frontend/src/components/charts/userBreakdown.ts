import type { UserBreakdownParams, UserBreakdownResponse } from '@/api/admin/dashboard'

/**
 * 分布图表的「按用户下钻」加载函数。由管理端页面传入，传入才开启下钻；
 * 图表组件自身不引用管理 API，用户站复用这些图表时不会带上管理端代码。
 */
export type UserBreakdownLoader = (params: UserBreakdownParams) => Promise<UserBreakdownResponse>
