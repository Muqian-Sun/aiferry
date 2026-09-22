/**
 * 用户站新壳与新页面的文案。旧 nav.* 键仍被管理端和过渡期页面使用，这里不复用其措辞。
 */
export default {
  userUi: {
    nav: {
      usage: '用量',
      keys: '密钥',
      models: '模型',
      billing: '账务',
      account: '账户',
      batchImage: '批量生图',
      more: '更多',
      product: '产品',
      pricing: '模型与价格',
      docs: '文档',
      console: '控制台',
      login: '登录',
      openMenu: '打开导航',
      primaryNav: '主导航'
    },
    footer: {
      home: '首页',
      product: '产品',
      help: '文档与帮助',
      legal: '协议',
      register: '注册'
    },
    topbar: {
      balance: '余额',
      available: '可用',
      frozen: '冻结',
      total: '总额',
      language: '语言',
      theme: '主题',
      switchToLight: '切换到浅色',
      switchToDark: '切换到深色',
      accountMenu: '账户菜单'
    },
    home: {
      hero: {
        eyebrow: '{models} 个模型 · 4 条官方协议 · 逐条记账',
        title: '一把 API Key，\n接入所有主流模型。',
        description: '按 Anthropic、OpenAI、Gemini 的官方协议原样转发，SDK 不用换，只改一行 base_url；每次请求逐条记账，用量与费用同一本账。',
        getStarted: '开始使用',
        goToConsole: '进入控制台',
        viewPricing: '查看模型与价格',
        vendorsLabel: '已上架的厂商'
      },
      features: {
        eyebrow: '为什么用这里',
        title: '把复杂留给我们，把接口留给你',
        description: '一条接口后面是统一的调度、计价与记账；你只管换 base_url。',
        items: {
          protocol: { title: '官方协议直连', body: '同协议的资源优先，不做私有格式转换；SDK、流式、工具调用都和直连一样。' },
          noFallback: { title: '绝不偷换模型', body: '你请求哪个模型就是哪个模型；唯一的兜底是同模型换渠道，不会静默降级。' },
          pricing: { title: '上架即标价', body: '模型页列出每个上架模型的标价；实付 = 标价 × 账户倍率，先看价再用。' },
          sticky: { title: '同一对话固定上游', body: '一段会话固定落在同一上游，缓存命中不被打断，长对话更便宜也更稳。' },
          ledger: { title: '逐条可查', body: '用量页按请求列出模型、Token、耗时与费用，支持筛选与 CSV 导出。' },
          balance: { title: '余额透明', body: '可用余额与冻结金额分开显示，每一笔扣费都能追溯到具体请求。' }
        }
      },
      quickstart: {
        eyebrow: '快速接入',
        title: '几行代码，接入全部模型',
        description: '四条官方协议原样支持，换 API Key 与 base_url 即可调用。',
        steps: {
          create: { title: '创建密钥', body: '在控制台「密钥」里生成一把 API Key，可设额度与限速。' },
          baseUrl: { title: '替换 base_url', body: '保留原有 SDK 与请求格式，只把上游地址改成本站。' },
          call: { title: '发起请求，回来看账', body: '每次请求的 Token、耗时与费用逐条记录，随时导出。' }
        },
        links: {
          docs: { title: '接入文档', body: '在 IDE / Agent 中调用' },
          clients: { title: '客户端配置', body: '在各类客户端使用' }
        },
        sample: {
          request: 'REQUEST',
          response: 'RESPONSE',
          copy: '复制请求',
          copied: '已复制',
          tabs: { messages: 'Messages', responses: 'Responses', chat: 'Chat', gemini: 'Gemini' }
        }
      },
      catalog: {
        eyebrow: '价格公开',
        title: '模型与标价',
        description: '标价为本站目录价（USD / 百万 Token），与模型页同源；实付 = 标价 × 账户倍率。',
        viewAll: '查看全部 {count} 个模型'
      },
      clients: {
        eyebrow: '可用集成',
        title: '在你熟悉的客户端里用',
        description: '这些工具只要改 base_url 和 key 就能接上；每一个在「密钥 → 使用密钥」里都有可复制的配置片段。',
        cta: '去密钥页取配置',
        items: {
          claude: '环境变量指到本站，Anthropic Messages 协议原样走。',
          codex: 'OpenAI Responses 协议；HTTP 与 WebSocket 两种传输都有配置。',
          gemini: 'Gemini generateContent 协议，配 API key 即可。',
          grok: 'OpenAI Chat 兼容协议，指向本站的 /v1。',
          opencode: '写一份 provider 配置，模型名从模型页里挑。'
        },
        sdk: { title: '任何 OpenAI / Anthropic SDK', body: '官方 SDK 与所有兼容库都能直接用：改 base_url，其余不动。' }
      },
      stats: {
        models: '上架模型',
        vendors: '厂商',
        protocols: '官方协议，原样透传',
        ledgerLabel: '请求记账，可导出',
        ledgerValue: '逐条'
      },
      cta: {
        eyebrow: '官方协议 · 上架即标价 · 逐条记账',
        title: '准备好开始了吗？',
        getStarted: '免费开始',
        goToConsole: '进入控制台'
      }
    },
    billing: {
      title: '账务',
      description: '充值、订阅、订单与邀请返利',
      tabs: {
        recharge: '充值',
        subscriptions: '订阅',
        orders: '订单',
        affiliate: '邀请'
      }
    },
    usage: {
      title: '用量',
      description: '请求、Token 与费用，逐条可查',
      stats: {
        requests: '请求',
        tokens: 'Token',
        cost: '费用',
        standardCost: '标准价',
        balance: '可用余额',
        recharge: '前往充值',
        totalCost: '累计消耗',
        totalRequests: '累计请求',
        rate: '当前速率（近 5 分钟）',
        todayCost: '今日费用',
        todayRequests: '今日请求',
        todayTokens: '今日 Token',
        avgLatency: '平均耗时'
      },
      sections: {
        announcements: '公告',
        trend: '用量趋势',
        models: '模型用量',
        records: '请求明细'
      },
      announcements: {
        unread: '{count} 条未读'
      },
      trend: {
        tokens: 'Token',
        requests: '请求',
        cost: '费用',
        rangeSummary: '区间内 {requests} 次请求 · {tokens} Token · 费用 {cost}',
        empty: '这段时间没有数据'
      },
      share: '占比',
      retry: '重试',
      loadFailed: '这一块没有加载出来',
      loadFailedHint: '接口暂时不可用，其他区块不受影响。',
      empty: '这段时间没有请求',
      emptyHint: '换个时间范围，或先创建一把密钥发起第一次调用。'
    },
    account: {
      title: '账户',
      description: '资料、安全与通知',
      sections: {
        profile: '基本信息',
        security: '安全',
        notifications: '通知'
      }
    },
    models: {
      title: '模型',
      description: '已上架的模型与标价，按 Token 或按次计费',
      allVendors: '全部',
      allBilling: '全部计费',
      searchHint: '搜索模型、别名（按 / 聚焦）',
      count: '{count} 个模型',
      view: { label: '视图', table: '表格', grid: '网格' },
      yourPrice: '你的价格',
      yourPriceHint: '= 标价 × {multiplier}',
      multiplierNote: '你的账户倍率是 {multiplier}。',
      perMillionShort: '$ / 百万 Token',
      timePricing: '分时',
      weekdaysOnly: '仅工作日',
      columns: {
        model: '模型',
        vendor: '厂商',
        billing: '计费',
        input: '输入',
        output: '输出',
        cacheRead: '缓存读取'
      },
      perMillion: '美元 / 百万 Token',
      listPrice: '标价',
      priceNote: '标价为本站目录价（美元 / 每百万 Token），按次计费的模型不列 Token 单价。你的实付 = 标价 × 账户倍率，逐条记录在用量页。',
      copyId: '复制模型 ID',
      copied: '已复制',
      empty: '暂无可用模型',
      noSearchResult: '没有匹配的模型',
      loadFailed: '模型目录没有加载出来'
    },
    notFound: {
      title: '页面不存在',
      description: '你要找的页面不存在，或者已经被移动。',
      back: '返回上一页',
      home: '回到首页'
    },
    status: {
      loading: '加载中'
    }
  }
}
