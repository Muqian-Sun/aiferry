/**
 * Layout Components
 * 只导出两站共用的布局。AppLayout / AppHeader 是管理站的壳，按路径直接引入，
 * 不从这里导出——用户站的登录页等经由本入口引用 AuthLayout，不能顺带把管理站代码拉进来。
 */

export { default as AuthLayout } from './AuthLayout.vue'
