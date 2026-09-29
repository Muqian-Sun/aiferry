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
| A6 | 设置拆成 13 个小节页（二级导航 + 路由）、每节保存 + 离开提醒、两栏布局、功能子设置回功能页、删邀请返利卡片 | 已合 | #68 |
| F1 | 渠道 / 模型添加表单重做：先选「第三方 key / 成品号」、key 不选平台、渠道表单里直接勾选模型、去掉模型白名单（映射只改名）、模型新建 / 编辑改独立页（按模型 ID 带价、每百万 Token 填价、渠道可勾选） | 已合 | #69 |
| A7 | 数据页：概览 / 收款概览一张面、用量页摘要 + 四页签（分析里放单色图）、审查页签统一、渠道健康去重复说明、运维图表换 token 色、渠道抽屉用量单色无卡片 | 已合 | #70 |
| A8 | 收尾：删 25 个无人引用的文件与渠道表单弹窗形态、187 个死词条；「账号」→「渠道」217 条；换皮检查覆盖整个管理站打包；表单 / 保存栏 / 筛选零碎样式 | 已开 PR | #71 |

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
- **新建渠道**（`CreateAccountModal`）：第一块是 `AccessSourcePicker`（`components/account/accessSources.ts`）：先选「第三方 key / 成品号」。
  - **第三方 key 不选平台、也不选来源**（muqian 追问「渠道里面怎么还有来源」后改掉：第一版给 key 做了一排厂商来源，等于换名的平台选择）。
    只填协议 + 地址 + Key；常用官方地址做一个下拉菜单快捷填入（`KeyAddressPresetMenu` / `keyAddress.ts`：后端官方地址表 +
    国产国际站）。厂商按地址识别：官方域名表由后端随 `protocol-defaults` 下发（`vendor_hosts`，与
    `OfficialVendorOfURL` 同一张表），认不出就提示「按中转处理」。识别出的厂商才有它的专属选项，都在地址下面：
    Kimi / 智谱 / MiniMax 的「按量 / Coding 套餐」（地址能分出来就跟地址走，MiniMax 两种套餐同地址要管理员选）、
    智谱团队版；OpenCode 的 Zen / Go 由地址定。**提交时不带平台**，后端 `resolveCreateAccountPlatform` 按地址推导。
    **2026-09-29 更新（muqian 定）**：删掉海外四家（Anthropic、OpenAI、Gemini、Grok）的官方 Key。官方地址表与域名表只剩国产厂商
    与 OpenCode，快捷填入里没有这四家与 Grok 区域；管理员手填这四家的官方域名也不拦，一律按中转处理（`Vendor()` 为空），
    厂商特化只对成品号。原来 key 专属的「Gemini 档位」、Grok 地址预设、按厂商给的 API Key 提示都删了。
    表单里的 `form.platform` 对 key 只是占位。原「Antigravity 第三方 key」就是普通中转 key。
  - 成品号要选是哪家的账号（授权流程各家不同）：Claude / ChatGPT / Gemini / Antigravity / Grok / AWS Bedrock / Vertex·Claude / Vertex·Gemini。
  - **默认只露必填项**（muqian「还是太繁琐，要填的东西太多了」，12:5x 定：只留必填其余收起、默认勾模型、名称仍必填）：
    接入方式 → 名称 → 地址（key）/ 哪家的账号与授权方式（成品号）→ API Key → 承接的模型（一行摘要，点「修改」展开）→ 创建；
    备注、到期、并发 / 优先级 / 倍率、配额、代理、池模式、错误码、请求头覆写、协议开关、模型改名、智谱团队版、
    倍率探测等全部在默认收起的「更多设置」里，不点开就按原来的默认值建。套餐只在地址分不出来时问（MiniMax、智谱 Anthropic 地址），
    分得出就写在识别提示里（「按地址识别为 Kimi · Coding 套餐」）。
  - **承接的模型默认勾选**：识别出的厂商（成品号即它的平台）在目录里已上架的对话模型；生图 / 视频 / 向量走扩展端点、另有承接条件，
    默认不勾（目录列表多返回 `extension_endpoints`，口径同绑定校验）；中转不勾；管理员动过勾选就不再自动改。
  - 所有建号路径（含 OAuth 批量、Grok SSO）走 `createAccountRecord`：映射打 `model_mapping_rename_only`，建好后
    `PUT /admin/accounts/:id/catalog-entries` 写入勾选的模型；绑定失败不回滚建号，提示去编辑页再勾。
- **一个 key 只承接一个协议**：地址编辑器改成「协议下拉 + 地址」一行（`ProtocolEndpointsEditor`）；官方地址表多协议时只取
  一个（换套餐保留当前协议；原来按平台挑默认协议的 `preferredProtocolFor` 在 2026-09-29 表里只剩国产厂商与 OpenCode 后删了，
  编辑页还没配协议时取 chat_completions）。
  换套餐时地址还是上一个套餐的官方地址就换成新套餐同协议的官方地址。修掉了「用默认预填新建 OpenAI / 国产 key 被后端拒」。
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

### A7 实施结果（给后来者）

- **共同做法**：数据页不再套卡片，区块之间只用 hairline 分隔（`SheetSection`）；页级控件（时间范围、刷新、工具菜单）放页头
  `#header-actions`；页内页签用 `SectionTabs`，图表里的小切换（指标、来源）用 `bg-af-sunken` 的分段按钮。
  图表单色：单指标趋势一律 `UsageMetricTrend`（一条墨线 + 淡面积）；分布一律「表格 + 墨色占比条」（`components/charts/ShareBar.vue`）。
  趋势按连续时间桶补零在 `utils/trendBuckets.ts`（概览、用量「分析」共用）。
- **概览**（`DashboardView`）：数字、需要处理、趋势、模型分布 / 用户消费榜、Top 12 用户都是一张面；时间范围与粒度挪到页头（只作用于趋势以下）。
- **用量**（`views/admin/UsageView.vue`）：区间摘要（请求 / Token / 缓存命中率 / 实付 / 成本 / 平均响应，口径同概览，接口失败不出现）+
  页签 明细（默认）· 错误 · 排行 · 分析。分析 = 用量趋势（粒度 + Token / 请求 / 费用）+ 模型分布 + 端点分布。
  刷新在页头、导出 Excel 与清理进页头「⋯」；筛选组件（`UsageFilters`）只剩筛选、重置和列设置插槽，加了 `analysis` 模式（不显示计费模式）。
  删掉 `UsageStatsCards`、`TokenUsageTrend`（五色双轴折线）及其用例。
- **端点分布**（`EndpointDistributionChart`）：多色环形图换成与模型分布同样的占比列。两张分布图都去掉了自带的卡片外框。
- **收款概览**（`AdminPaymentDashboardView`）：天数切换与刷新进页头；每日收款（`DailyRevenueChart`）从彩色双轴改成单色一条线，
  各币种收入与订单数做成页签；支付方式按笔数画占比条、充值排行用 `#1` 文字，不再用颜色区分。删了无人引用的
  `PaymentMethodChart`、`TopUsersLeaderboard`。
- **审查**：内容审核（`RiskControlView`）四张彩色图标瓷砖换成 `StatRow`，「刷新状态」「设置」进页头；状态格的值写实际生效状态
  （总开关关着写「风险总开关关闭」），原来那个同义徽章去掉。提示词（`PromptAuditView`）的「事件 · 配置」换成 `SectionTabs`，
  内容去掉卡片外框，配置版本挪到页头右侧。
- **渠道健康**：面板开头的说明与页头说明重复，删掉面板里那段；页头说明原来写「监测可用性、延迟和状态」不准确（这页只有汇总配置），
  换成面板那段话。
- **运维**：按拍板第 7 条不动结构。6 个图表的写死颜色换成 token：单系列（切换率、延迟分布）墨色，吞吐两系列用分类色第 1、2 槽，
  错误趋势 / 分布用危险 / 警示色 + 两档墨色；健康分圆环跟分数同一个状态色类。原 `isDarkMode` 是一次性读 `document.classList`、
  不随主题切换更新，改用 `useChartTheme` 后会跟着重算。
- **渠道抽屉「用量」**：`AccountStatsModal` 换成 `AccountUsagePanel`（只有抽屉内联一种形态）：摘要一行 → 趋势（单色，费用 / 请求 / Token）
  → 今日、30 天合计、峰值日的明细（`DetailField`）→ 模型 / 入站 / 上游端点分布。原来的三色双轴图和 9 张小卡片去掉。
- **顺手修的**：`DateRangePicker` 面板原来固定左对齐，放在页头右侧时溢出视口、撑出横向滚动条；现在打开时量一下，放不下就右对齐
  （用户站页头同样受益）。渠道列表「compact」状态点的 emerald / rose 写死色换成状态色 token。
- **浏览器走查**（dev 8081，浅色）：概览、用量（明细 / 分析）、收款概览、渠道健康、运维已看过；日期面板修复后 `scrollWidth == clientWidth`。
  审查两页在共享 dev 库里功能没开，直达地址被守卫跳到设置（A3 遗留），没看到实页，靠用例。深色、窄屏没过。

### A8 实施结果（给后来者）

- **删无人引用的文件**：从两站入口（`src/apps/{admin,user}/main.ts`）按 AST 收集 import / 动态 import，找出 25 个到不了的文件
  （旧定价卡片一族、`components/common/index.ts` 等桶文件、运维旧邮件 / 运行时设置卡、订单详情 / 订单表、订阅进度小卡、
  二维码支付弹窗……），连同只测它们的 9 份用例删掉；删完再扫到不动点（0 个）。扫描脚本用探针文件做过负向验证。
- **弹窗形态删干净**：新建 / 编辑渠道只剩整页壳 `FormPageShell`（`layout` 属性去掉）；定时测试只剩抽屉内联；`InlineShell` 删除。
- **死词条**：删了 187 个（整棵 `admin.channels` 旧定价表单、运维旧卡片、订阅进度小卡等）。全量扫描（字面量 + 真前缀 + 命名空间字面量）
  下管理站命名空间已没有死词条；用户站 / 公共命名空间还剩 131 个（`modelPlaza.table.*`、旧 `dashboard.*` 等），不在 A8 范围，没动。
- **「账号」→「渠道」**（只改 zh）：渠道的增删改、调度、批量、列表，以及代理 / 运维 / 概览 / 设置里的调度类说明，共 217 条。
  指上游身份的保留（上游账号、账号池、Free / Google / 服务 / 团队版 / 订阅账号、影子 / 母账号、OAuth 授权步骤里的「您的 Claude 账号」），
  指用户账号的（注册、面板限流、验证码子账号等）不在范围。顺带去掉已下线的「分组」说法；国产厂商套餐选择的标签改叫「计费方式」。
  英文词条没跟着改。
- **换皮检查**：`check:admin-tokens` 加 `--admin-bundle`，从 `src/apps/admin/main.ts` 沿 import 图收集管理站打包会用到的全部文件
  （410 个，与独立扫描一致）一起检查，原目录清单保留、叠加；以后新加的共用文件自动纳入。负向验证：在原目录清单外的
  `utils/proxyExpiry.ts` 塞一个旧类名，检查退出码 1。顺带把登录 / 两步验证 / 设置向导 / 错误徽章 / 模型改名预设等 12 个文件换成 token
  （只属于用户站的文件没动）。
- **零碎样式**：编辑页「同步上游倍率」开关挪出四列网格独占一行；渠道表单里「左说明右开关」的行统一加间距；OAuth 授权第二步
  去掉灰底大卡片和装饰图标；窄屏下分节导航吸在顶栏下（分节标题的滚动留白跟着加大）；渠道健康的保存改成与设置页同一种贴底保存栏；
  提示词审查底部保存栏的左边界跟随侧栏宽度；用量页筛选去掉每个控件上方的标签（文字进 `title`，没选时写「全部 xx」），压成一行；
  收款概览的标签页标题与页签同名。
- **走查**（dev 8081）：深色下概览、用量、渠道列表、渠道编辑、渠道健康、运维、收款概览看过，没发现问题；切回了浅色。
  **窄屏没看成**：浏览器窗口处于全屏，调尺寸不生效；站点禁止被 iframe 嵌入（这是对的），也没法用 iframe 模拟。窄屏需要人工过一遍。
- **仍没做**：用量页筛选没换成 A4 的筛选小标签（用户 / 密钥 / 渠道是带下拉的搜索框，不适合小标签，只做了压缩）；
  运维页自带的看板头没换统一页头（拍板第 7 条）；渠道表单组件仍叫 `CreateAccountModal` / `EditAccountModal`（已无弹窗，改名留给以后）。

## 已知问题（后端 / 接口，未修）

- 兑换码统计接口是 stub，恒返回 0。兑换码页摘要改用列表接口数出来。
- `/admin/users/:id/usage` 是 stub，恒返回 0。用户抽屉的用量改用 `/admin/usage/stats` 按用户筛选。
- 前端 `subscriptions.listByUser` 的类型写成分页，后端实际返回数组（目前没人用）。
- `/subscriptions?user=`、`/accounts?search=` 不读 query，所以抽屉里没做跳转链接。
- DataTable：点行复选框周围的留白也会打开抽屉（点复选框本身不会）。
