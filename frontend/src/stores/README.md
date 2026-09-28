# Pinia stores

| Store | 谁用 | 放什么 |
| --- | --- | --- |
| `auth.ts` | 两站 | 登录态（`user` / `token` / `refreshToken`，localStorage 键 `auth_token` `auth_user` `refresh_token` `token_expires_at`）、`isAdmin` / `isSimpleMode`（`user.run_mode === 'simple'`）、`checkAuth` / `refreshUser` / `login` / `logout`、待完成的 OAuth 注册会话 |
| `app.ts` | 两站 | 公开设置（`fetchPublicSettings` / `initFromInjectedConfig` 读 `window.__APP_CONFIG__`，`cachedPublicSettings` / `publicSettingsLoaded`）、站点品牌（`siteName` `siteLogo` `apiBaseUrl` `docUrl` `contactInfo`）、`backendModeEnabled`、Toast（`showSuccess` / `showError` / …）、全局 loading；管理站侧栏的折叠 / 移动端开合状态也在这里（用户站没有侧栏，不读） |
| `announcements.ts` | 用户站 | 公告列表、`unreadCount`、登录公告弹窗（`requestNotice` 由 App 在登录 / 进站时登记，`noticeOpen` 由 `components/user/AnnouncementNotice` 消费，「今日不再弹出」记在 localStorage）、`markAsRead` / `markAllAsRead` |
| `subscriptions.ts` | 用户站 | 当前用户的订阅与轮询 |
| `payment.ts` | 用户站 | 支付流程状态 |
| `onboarding.ts` | 用户站 | 新手引导重放回调 |
| `adminSettings.ts` | 管理站 | 管理端设置缓存 |

用户站入口不得 import `admin*` store（`app/__tests__/siteSplit.spec.ts` 守着 import 图）。

## 约定

- 读公开设置一律走 `appStore.cachedPublicSettings`，不要各自再拉 `/settings/public`；功能开关的语义（opt-in / opt-out、未加载时视为什么）在 `utils/featureFlags.ts`。
- 组件测试里 mock 具体的 store 模块（`vi.mock('@/stores/auth', …)`），不要装真 pinia 再 patch。
