# 管理站改版（A 线）：计划与进度

> 给接手的会话和协作者：**改管理站（`VITE_APP=admin`）前先读这份。** 每做完一步，更新下面的「进度」表和「下一步」。入口：`DEV_GUIDE.md` 顶部指向这里。
> 带示意图的完整方案：<https://claude.ai/artifact/9fVCqXYRfsJGUdwkfUK76Z>（拿到链接即可查看）。
>
> 最后更新：2026-09-24，A5 合入之后。

## 目标

管理站的观感和用法向用户站看齐：

- 一张纸，不做卡片框；兄弟区块之间只用 hairline 分隔；装饰色收成墨色。
- 状态色只给真正的异常。正常状态是「灰点 + 文字」；异常用红色，并直接写出原因。
- 列表页结构统一：页头（标题在内容区，主操作在右上）→ 数字摘要 → 工具行 → 表格 → 选中行才出现的批量条 → 分页。
- 看详情用右侧抽屉，不离开列表；改配置进独立页面或对话框。

## 已定决策（muqian，2026-09-24）

| # | 问题 | 决定 |
|---|---|---|
| 1 | 导航里模型和渠道谁在前 | 渠道在前 |
| 2 | 新建 / 编辑渠道用什么形式 | 独立页面 |
| 3 | 版本徽章（原来对上游检查更新、可升级回滚） | 只显示版本号 |
| 4 | 设置页的保存方式 | 每节自己保存 |
| 5 | 内容审核与提示词审查 | 合并成一个「审查」入口、两个页签 |
| 6 | 渠道健康 V1（旧版主动探测） | 删掉（后端代码和表保留，运行时不走） |
| 7 | 运维页 | 这次只换外壳和图表颜色，不改结构 |

## 进度

| 阶段 | 内容 | 状态 | PR |
|---|---|---|---|
| A1 | 全站换皮：af-* token、换皮脚本、build 里的换皮检查 | 已合 | #62 |
| A2 | 仪表盘重做（今日 / 累计数字带、需要处理、单色图）；渠道 / 用户页默认列精简 | 已合 | #63 |
| A3 | 外壳与导航：页头进内容区、导航分区重排、未开启功能灰显、去上游痕迹、渠道健康 V1 下线、补运维监控开关 | 已合 | #64 |
| A4 | 列表页模板：一套共用构件，套到用户、订阅、套餐、订单、兑换码、公告、代理、操作日志、模型 | 已合 | #65 |
| A5 | 渠道列表套模板；渠道 / 用户 / 模型详情抽屉；新建 / 编辑渠道改成独立页面 | 已合 | #67 |
| A6 | 设置拆页 | **下一步** | |
| A7 | 数据页（概览、用量、渠道健康、审查、收款概览） | 未开始 | |
| A8 | 收尾 | 未开始 | |

同期合入、与改版无关的后端工作：大语言模型多模态上线前修复（#66）。内容包括：跨协议图片 / PDF / 音频转换，转不了的返回 400；`image_generation` 工具开关，默认关；音频 token 单价（迁移 254）。

## 构件：写一个管理站页面用什么

**参考实现**
- 标准列表页：`frontend/src/views/admin/UsersView.vue`
- 复杂列表 + 抽屉：`frontend/src/views/admin/AccountsView.vue`
- 分节表单页：`frontend/src/views/admin/AccountCreateView.vue`、`AccountEditView.vue`

**页面骨架**
- `components/layout/AppLayout.vue`：页头由路由 meta 生成。页级动作放 `#header-actions` 插槽，最多一个「⋯」工具菜单加一个 `btn btn-primary btn-md`。
- `components/layout/TablePageLayout.vue`：插槽依次是 `#summary`、`#filters`、`#table`、`#bulk`、`#pagination`。单元格内边距 12px。
- 数字摘要：`components/user/shell/StatRow.vue`（`StatItem` 可带 `action`，例如「异常 1 · 筛选」）。只放能便宜拿到的真数；接口失败就不渲染，不要摆一排 0。

**列表构件**（`components/admin/list/`）

| 构件 | 用途 |
|---|---|
| `ListToolbar` | 工具行：默认插槽放搜索和筛选，`#end` 放刷新 / 列设置等小图标 |
| `FilterChip` | 筛选标签：没选时是虚线，选了变实心并带 ✕ |
| `ColumnSettingsMenu` + `composables/useColumnSettings` | 列设置。`storageKey: 'admin-<page>-columns'`，改默认隐藏列时 version + 1 |
| `PopoverMenu` / `MenuItem` | 挂到 body 的弹出菜单；`MenuItem divider` 是分隔线 |
| `RowActions` | 行尾：`primary` 的一两个动作显示为图标，其余进「⋯」；`danger` 的只进菜单 |
| `BulkBar` | 批量条，只在选中时出现。按钮用 `.bulk-btn` / `.bulk-btn-danger`；`#meta` 插槽放计数旁的 `.bulk-link` |
| `DetailDrawer` / `DetailField` | 右侧详情抽屉。插槽：`#subtitle`、`#actions`、`#banner`；页签复用 `SectionTabs`。层级在对话框之下；Esc 挂在捕获阶段，上面叠着对话框或菜单时只关它们 |
| `MiniSwitch` | 表格行里的小开关，点击不触发整行 |
| `InlineShell` | 与 `BaseDialog` 同接口，只原地渲染正文。用来把旧对话框组件放进抽屉页签（`layout="inline"`） |

**DataTable**
- `clickable-rows` + `@row-click`：点行打开抽屉。行里的交互元素要 `@click.stop`。
- `row-class`：自带选择列的页面用它给选中行加 `row-selected bg-af-sunken`。

**分节表单页**（`components/admin/form/`）
- `FormPageShell`：左侧分节导航，从正文里的 `data-form-section` 标记自动生成；底部是固定保存栏。
- `FormSectionHeading`：分节标题标记。
- `Create/EditAccountModal` 通过 `layout="page"` 切到整页形态。

## 约定（每一步都要守）

**代码**
- 不新增前端 spec 文件，只改已有的。
- zh / en 词条对称（`pnpm check:i18n`）。新词条放页面自己的命名空间；`common.ts` 只放真正通用的。
- 只用 `af-*` token：`node scripts/codemod-af-tokens.mjs --check src/views/admin src/components/admin src/components/account src/features src/components/common src/components/charts src/components/layout`。build 里还有 `check:admin-tokens` / `check:site-split`。
- 保留现有 spec 用到的每个 `data-test` / `data-testid`。

**验证**
- 每步都要在 `frontend/` 里跑：`pnpm typecheck`、`pnpm lint:check`、`pnpm check:i18n`、换皮检查、全量 `pnpm vitest run`、`pnpm build`。
- 仓库 CI 目前不跑（PR 上没有 check runs），以上检查都在本地做完再推。
- 本地起后端 + 演示数据，逐页截前后对比图：浅色、深色、390px 窄屏；1440 宽下默认列不应横向滚动。

**分支与 PR**
- 分支叫 `feat/admin-<stage>`，一步一个 PR，按顺序合入 main。

## 下一步

### A6 设置拆页

`frontend/src/views/admin/SettingsView.vue` 现在有 12,278 行，页签全靠 `v-show`：security（两段）、gateway（两段）、users、general、agreement、features、payment、email。

1. **拆分**：拆成小节组件，每节一个路由 `/settings/<section>`，用二级导航代替页签。分组如下：
   - 站点：品牌与首页、条款
   - 用户：注册与登录、第三方登录、新用户默认值
   - 网关：重试与冷却、转发行为、Claude Code · Codex、上游余额探测
   - 收款：支付方式
   - 通知：邮件
   - 功能：开关
2. **布局与保存**：两栏布局，与用户站「基本信息」一致（左边写这项是干什么的，右边是控件；参考 `components/user/shell/SettingsRow.vue`）。每节自己保存，改了没存就离开时提醒。
3. **功能设置回到功能页**：审查的 7 个页签、渠道健康、运维的阈值与告警，各自页面上放一个「设置」入口。
4. **删除**：「邀请返利」卡片（SettingsView 里搜 `features.affiliate`），以及数据备份的残留词条。

### A7 数据页

- 概览、用量、渠道健康、审查（A3 已合成一个入口）、收款概览：页签统一、图表单色、去掉各自的重复标题。
- 用量页：数字摘要 + 页签（明细 · 错误 · 排行 · 分析），图表挪到「分析」并改单色。
- 运维页只换外壳和图表颜色。
- 渠道抽屉「用量」页签里的统计卡片（沿用自原 `AccountStatsModal`）也在这一步换成单色、无卡片。

### A8 收尾

- 删无人引用的旧文件和只测它们的用例：`Create/EditAccountModal` 的对话框形态、`AccountStatsModal` / `ScheduledTestsPanel` 的对话框外壳（先确认没人用）、死词条。
- 措辞统一「账号」→「渠道」：表单里仍是「账号名称」等。
- 深色模式、窄屏逐页过一遍；换皮检查扩到所有管理站目录。
- 渠道表单页里几块偏挤的地方：编辑页的「同步上游倍率」开关、flatten namespaces 的说明、OAuth 第二步的卡片样式；窄屏的分节导航不吸顶。

## 已知问题（后端 / 接口，未修）

- 兑换码统计接口是 stub，恒返回 0。兑换码页摘要改用列表接口数出来。
- `/admin/users/:id/usage` 是 stub，恒返回 0。用户抽屉的用量改用 `/admin/usage/stats` 按用户筛选。
- 前端 `subscriptions.listByUser` 的类型写成分页，后端实际返回数组（目前没人用）。
- `/subscriptions?user=`、`/accounts?search=` 不读 query，所以抽屉里没做跳转链接。
- DataTable：点行复选框周围的留白也会打开抽屉（点复选框本身不会）。
