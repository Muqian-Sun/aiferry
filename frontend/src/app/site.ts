/**
 * 当前构建的站点。由 vite 配置按 VITE_APP 在编译期写成字面量，
 * 另一个站点的分支在构建时被整体剪掉。
 */
export type AppSite = 'user' | 'admin'

export const APP_SITE: AppSite = import.meta.env.VITE_APP

export const IS_ADMIN_SITE = APP_SITE === 'admin'
