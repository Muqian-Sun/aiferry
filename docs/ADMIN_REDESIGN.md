# 管理站改版（A 线）：计划与进度

> 给接手的会话和协作者：**改管理站（`VITE_APP=admin`）前先读这份。** 每做完一步，更新下面的「进度」表和「下一步」。入口：`DEV_GUIDE.md` 顶部指向这里。
> 带示意图的完整方案：<https://claude.ai/artifact/9fVCqXYRfsJGUdwkfUK76Z>（拿到链接即可查看）。
>
> 最后更新：2026-09-25，A6 开 PR 之后。

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
| 8 | 列表页数字摘要（A4 / A5 已放在用户、兑换码、模型、渠道页） | 保留（2026-09-25） |
| 9 | 设置里「管理 API Key」「客户端 IP」「面板限流」放哪 | 加一节「安全 › 访问与限流」，共 13 节（2026-09-25） |
| 10 | 功能自己的子设置（渠道健康的「对用户隐藏」、审查的会话封禁） | 挪回各自功能页，设置 › 开关只留总开关（2026-09-25） |
| 11 | 添加渠道的开头 | 先选「第三方 key / 成品号」：key 不选平台（厂商从地址识别），成品号只选哪家的账号再授权（2026-09-25） |
| 12 | 模型页的添加模型 | 填模型 ID 自动从价格文件带出厂商 / 计费 / 价格，价格按每百万 token 填，独立分节页（2026-09-25） |

## 进度

| 阶段 | 内容 | 状态 | PR |
|---|---|---|---|
| A1 | 全站换皮：af-* token、换皮脚本、build 里的换皮检查 | 已合 | #62 |
| A2 | 仪表盘重做（今日 / 累计数字带、需要处理、单色图）；渠道 / 用户页默认列精简 | 已合 | #63 |
| A3 | 外壳与导航：页头进内容区、导航分区重排、未开启功能灰显、去上游痕迹、渠道健康 V1 下线、补运维监控开关 | 已合 | #64 |
| A4 | 列表页模板：一套共用构件，套到用户、订阅、套餐、订单、兑换码、公告、代理、操作日志、模型 | 已合 | #65 |
| A5 | 渠道列表套模板；渠道 / 用户 / 模型详情抽屉；新建 / 编辑渠道改成独立页面 | 已合 | #67 |
| A6 | 设置拆成 13 个小节页（二级导航 + 路由）、每节保存 + 离开提醒、两栏布局、功能子设置回功能页、删邀请返利卡片 | 已开 PR | #68 |
| F1 | 渠道 / 模型添加表单重做：先选「第三方 key / 成品号」、key 不选平台、渠道表单里直接勾选模型、去掉模型白名单（映射只改名）、模型新建 / 编辑改独立页（按模型 ID 带价、每百万 Token 填价、渠道可勾选） | 已开 PR | #69 |
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

### A6 实施结果（给后来者）

- **结构**：`views/admin/SettingsView.vue` 只剩二级导航 + 小节外壳；状态与逻辑在 `views/admin/settings/useSettingsPage.ts`
  （原 `<script setup>` 整体搬来，逻辑未改）；13 个小节模板在 `views/admin/settings/sections/*Section.vue`，
  注册表与分组在 `views/admin/settings/sections.ts`。路由 `/settings/:section?`，缺省落到第一节。
- **每节保存**：所有小节都渲染、`v-show` 切换。改动判断 = 每块状态与「上次加载 / 保存后的基线」比较
  （`useSettingsPage` 末尾的 A6-2 段）。切走时有改动先问「放弃 / 留下」，所以同一时间只有当前小节可能有改动，
  保存某一节时照旧整份提交总表单（沿用 `saveSettings` 里跨小节的校验与规整）；走独立接口的 8 张卡片
  （冷却、流超时、整流、Beta 策略、面板限流、上游余额探测、Ollama）归所在小节的保存栏保存。
- **两栏布局**：小节里的卡片结构统一（标题头 + 正文），在外壳上一处样式实现（`SettingsView.vue` 的 `.settings-blocks`）。
  新加卡片照 `<div class="card"><div>标题头</div><div>正文</div></div>` 写即可。
- **设置接口支持只发部分字段**：请求里没带的字段不写库（`setting_handler_update.go` 的 `omittedSettingKeys`）。
  功能页（渠道健康、审查）就是这样只发自己那几项的。

### F1 实施结果（muqian 2026-09-25 提，插在 A7 前；给后来者）

- **决策**：key 不选平台；渠道表单里直接勾选目录模型；**模型白名单去掉**（muqian：「去掉」），映射保留为可选的「模型改名」。
- **新建渠道**（`CreateAccountModal`）：第一块是 `AccessSourcePicker`，来源表在 `components/account/accessSources.ts`。
  - key 的来源只决定预填哪家的官方地址和厂商专属选项（国产套餐、Gemini 档位、Grok 预设）；**提交时不带平台**，
    后端 `resolveCreateAccountPlatform` 按地址推导（认得出官方厂商即该厂商，中转按协议归族）。「自定义中转」不预填地址。
    原「Antigravity 第三方 key」并进自定义中转（后端早就按普通中转对待标签为 antigravity 的 key）。
  - 成品号：Claude / ChatGPT / Gemini / Antigravity / Grok / AWS Bedrock / Vertex·Claude / Vertex·Gemini。
  - 所有建号路径（含 OAuth 批量、Grok SSO）走 `createAccountRecord`：映射打 `model_mapping_rename_only`，建好后
    `PUT /admin/accounts/:id/catalog-entries` 写入勾选的模型；绑定失败不回滚建号，提示去编辑页再勾。
- **一个 key 只承接一个协议**：地址编辑器改成「协议下拉 + 地址」一行（`ProtocolEndpointsEditor`）；官方地址表多协议时只取
  一个（`preferredProtocolFor`：Anthropic→anthropic、OpenAI / Grok→responses、Gemini→gemini、其余→chat_completions）。
  换来源用新来源的默认协议，同一来源换模式（按量 / 套餐）保留当前协议。修掉了「用默认预填新建 OpenAI / 国产 key 被后端拒」。
- **编辑渠道**（`EditAccountModal`）：「已上架模型」从只读改成可勾选（`CatalogEntryPicker`，按渠道读 / 写绑定，勾选变了才写）；
  五块白名单 / 映射合成一块 `ModelRenameEditor`。旧映射整份按改名行展示，**同名行（旧白名单）保留**：对承接没影响，
  但 Antigravity / xAI 这类自带模型表的上游靠它扩表、批量生图也按映射列模型。spark 影子账号不打标记（后端只放行两个键）。
- **批量编辑**：映射同样只改名、带标记。同名预设只对自带模型表的上游（Antigravity、Grok）保留（`renamePresetsFor`）。
- **模型新建 / 编辑**：独立页 `/model-catalog/new`、`/model-catalog/:id/edit`（`ModelCatalogEntryFormView` + `CatalogEntryEditor`）。
  新建时输入模型 ID 400ms 后查价格文件（`GET /admin/model-catalog/price-lookup`），表单没被改过就自动带出厂商 / 计费 / 价格，
  改过了只提示「用价格文件的价格」；编辑页有「按价格文件带价」按钮。价格按每百万 Token 填（`PriceInput`，存 $/token）。
  厂商改下拉（目录已有厂商 + 自定义）。承接的渠道改成全量可勾选列表（`CatalogChannelPicker`）。
- **后端**（F1-a 提交 + 本 PR）：`model_mapping_rename_only` 语义、按渠道读写绑定、key 建号推导平台、按模型 ID 查价；
  目录列表多返回 `vendor_platform`（厂商族，渠道表单按它分组，对照表只在后端 `CatalogVendorPlatform`）。
- **没做 / 待定**：
  - 浏览器走查截图没做：本次会话里 Chrome 扩展一直连不上。已部署到 dev（8081），`/accounts/new`、`/accounts/:id/edit`、
    `/model-catalog/new` 需要人工过一遍（浅色 / 深色 / 窄屏）。
  - 批量生图的模型列表仍从渠道映射里取（`batchImageModelsFromAccountMapping`），不看目录绑定；要改成看绑定是单独的后端改动。
  - 后端 `GET /admin/accounts/antigravity/default-model-mapping` 前端已不再调用（Antigravity 不再预填默认表，由后端叠加）。

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
