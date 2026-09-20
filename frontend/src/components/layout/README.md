# Layout Components

两套壳，各自服务一个站点入口（`src/apps/user` / `src/apps/admin`），互不引用：

| 站点 | 壳 | 位置 |
|---|---|---|
| 用户站 | `SiteShell`（顶部导航，无侧栏） | `src/components/user/shell/` |
| 管理后台 | `AppLayout`（侧栏 + 顶栏） | 本目录 |
| 两站共用 | `AuthLayout`（登录 / 注册 / 找回密码） | 本目录 |

`scripts/check-site-split.mjs` 与 `src/app/__tests__/siteSplit.spec.ts` 保证用户站产物里不含管理端代码——
用户站视图不要 import 本目录的 `AppLayout` / `AppHeader` / `TablePageLayout` / `sidebar/*`。

## 用户站：`components/user/shell/`

页面就是一张面（sheet），没有第二层卡片，区块之间只用 hairline；样式只用 `*-af-*` token 工具类与 `style.css` 的一层原语
（`.btn-*` / `.input*` / `.badge*` / `.modal-*` …）。`src/components/user/shell/__tests__/userTemplateTokens.spec.ts`
扫描用户站模板，出现旧 gray 色阶、`dark:` 变体、`card`、渐变、玻璃等即红。

| 文件 | 职责 |
|---|---|
| `SiteShell.vue` | 唯一的壳。`variant: 'public' \| 'console'`；`title / description` 覆盖路由 meta；`hideHeader` 隐藏页头；`flush` 满宽出血并纵向填满（自定义页 iframe）；slot `actions`（页头右侧，至多一个 primary 按钮）、`tabs`（页内页签）。console 变体接管新手引导 tour |
| `SiteNav.vue` | 56px 顶栏：品牌、页签、余额、公告、文档、语言、主题、用户菜单；`<lg` 页签下沉第二行 |
| `NavTabs.vue` / `navItems.ts` | 页签渲染与纯函数构建（按功能开关 / simple mode / 自定义菜单） |
| `PageHeader.vue` | 20px 标题 + 说明 + 右侧操作 + 底 hairline，标题来自 `usePageTitle` |
| `SheetSection.vue` | 页内区块：可选标题行 + `#actions`；兄弟区块之间一条 hairline |
| `StatRow.vue` | 指标行：竖 hairline 分隔的行内数字，不做瓷砖 |
| `SectionTabs.vue` | 页内下划线页签（可绑路由或本地状态） |
| `StatusState.vue` | 加载 / 空 / 错误三态，`data-testid="status-{kind}"` |

用法：

```vue
<template>
  <SiteShell>
    <template #actions>
      <button class="btn btn-primary btn-md">创建密钥</button>
    </template>
    <StatRow :items="stats" />
    <SheetSection title="请求记录">…</SheetSection>
  </SiteShell>
</template>
```

页面标题与说明来自路由 meta 的 `titleKey` / `descriptionKey`（见 `src/apps/user/routes.ts`）。

## 管理后台：`AppLayout.vue`

侧栏 + 顶栏的经典后台版式，内容区放在默认 slot。`AppHeader.vue` / `sidebar/*` 由 `AppLayout` 自动挂载，不需要单独引用；
`TablePageLayout.vue` 是管理端表格页的页头 + 表格容器。管理端沿用 `style.css` 的二层类（`.card*`、`.stat-*`、`.page-*` 等）。

```vue
<template>
  <AppLayout>
    <div class="card p-6">…</div>
  </AppLayout>
</template>
```

## 共用：`AuthLayout.vue`

认证页：sunken 底、400px 的 sheet 表单面、品牌在上。默认 slot 放表单，`#footer` 放链接行。站名 / logo 来自管理员配置
（`useAppStore().siteName / siteLogo`），未配置时用 `utils/branding.ts` 的默认值。

```vue
<template>
  <AuthLayout>
    <form @submit.prevent="handleLogin">…</form>
    <template #footer>
      <router-link to="/register">注册</router-link>
    </template>
  </AuthLayout>
</template>
```

## Store 依赖

- `useAuthStore`：登录态、角色、退出
- `useAppStore`：站名 / logo / 公开设置、提示消息、（管理端）侧栏状态
