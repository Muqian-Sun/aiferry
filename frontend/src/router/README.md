# 路由

两个站点各自一张路由表，共用一个工厂与一套守卫：

| 文件 | 作用 |
| --- | --- |
| `createSiteRouter.ts` | 建 router：history、滚动到顶、导航进度条、chunk 失败重载 |
| `siteGuard.ts` | 全局守卫：登录态与站点角色匹配、功能开关门（`requiresPayment` / `requiresRiskControl` 看公开设置；`siteFeature`（订阅 / 批量生图 / 账户安全页）看代码常量 `utils/siteFeatures.ts`）、simple mode、backend mode、`/setup` |
| `routePreload.ts` | 数据到了再换页：路由 `meta.preload` 在 `beforeResolve` 里先发首屏请求，页面挂载时用 `adoptPreloaded` 接手同名同参的结果；用户站用在控制台各页（概览、密钥、用量明细、模型、充值） |
| `defaultAuthedPath.ts` | 登录后落地页：两站都是 `/dashboard`（用户站 = 概览，管理站 = 仪表盘） |
| `setupRedirect.ts` | 安装向导跳转 |
| `title.ts` | 按路由 meta 的 `titleKey` 设页面标题 |
| `meta.d.ts` | 路由 meta 字段类型 |

路由表在各站入口：`apps/user/routes.ts`、`apps/admin/routes.ts`；站点由编译期 `VITE_APP` 决定，另一个站的分支会被 tree-shake 掉（`scripts/check-site-split.mjs` 守着用户站包里没有 `/admin/*`）。

## 用户站（`apps/user/routes.ts`）

公开：`/home`（首页）、`/model-plaza`（模型与标价，对所有人开放，没有开关）、`/login` `/register` 与各家 OAuth 回调、`/forgot-password` `/reset-password`、`/key-usage`、`/legal/:documentId`、支付结果页。

登录后（顶部五个页签）：

| 页签 | 路径 |
| --- | --- |
| 用量（落地页） | `/usage` |
| 密钥 | `/keys` |
| 模型 | `/model-plaza` |
| 账务 | `/billing` → `/billing/recharge` `/billing/subscriptions`（按开关出现） |
| 账户 | `/profile` |

旧路径 `/purchase` `/subscriptions` 都 redirect 到上面。

## 管理站（`apps/admin/routes.ts`）

`/dashboard` `/ops` `/users` `/accounts`（渠道）`/model-catalog` `/subscriptions` `/orders/plans` `/orders` `/orders/dashboard` `/announcements` `/proxies` `/risk-control` `/prompt-audit` `/usage` `/settings` `/profile`，以及 `/setup` `/login`。全部要求管理员角色；非管理员进管理站、管理员进用户站都会被守卫踢回登录页。

## 守卫要点

- `requiresAuth` 默认 true；公开路由要显式写 `requiresAuth: false`。
- 功能开关门只在公开设置**成功加载且明确为 false** 时拦截，瞬时加载失败视为未知，交给后端兜底。
- 新增路由先决定归属哪个站，再决定挂在哪个入口文件；用户站入口不得触达 `views/admin` `components/admin` `api/admin`（`app/__tests__/siteSplit.spec.ts` 守着 import 图）。
