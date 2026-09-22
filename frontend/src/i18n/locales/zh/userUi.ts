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
      heroTitle: '一把 API Key，\n接入所有主流模型。',
      heroDescription: '按 Anthropic、OpenAI、Gemini 的官方协议原样转发，SDK 不用换，只改一行 base_url。每次请求逐条记账，用量与费用同一本账。',
      getStarted: '开始使用',
      goToConsole: '进入控制台',
      viewPricing: '查看模型与价格',
      codeSample: {
        label: '接入示例',
        comment: '只改这两行',
        copy: '复制',
        copied: '已复制',
        tabs: { python: 'Python', curl: 'curl', node: 'Node', claudeCode: 'Claude Code' }
      },
      stats: {
        models: '模型',
        vendors: '厂商',
        protocols: '官方协议，原样透传',
        ledgerLabel: '请求记账，可导出',
        ledgerValue: '逐条'
      },
      catalog: {
        title: '模型与标价',
        description: '标价，USD / 百万 Token，与模型页同源；实付 = 标价 × 账户倍率。',
        viewAll: '查看全部 {count} 个模型'
      },
      protocols: {
        title: '四条官方协议，原样透传',
        description: '不做私有格式转换：请求体与响应按各家官方协议原样经过，SDK、流式、工具调用都和直连一样。'
      },
      routeMap: {
        routes: {
          messages: 'Anthropic Messages',
          responses: 'OpenAI Responses',
          chat: 'Chat Completions',
          gemini: 'Gemini Generate'
        },
        vendors: {
          messages: 'Claude',
          responses: 'GPT',
          chat: 'GPT、DeepSeek、Qwen、Grok',
          gemini: 'Gemini'
        }
      },
      steps: {
        title: '接入只需三步',
        create: { title: '创建密钥', body: '在控制台生成一把 API Key，可按分组与额度限制用途。' },
        baseUrl: { title: '换一行 base_url', body: '保留原有 SDK 与请求格式，只把上游地址改成本站。' },
        watch: { title: '发起请求，回来看用量', body: '每次请求的 Token、耗时与费用逐条记录，随时导出。' }
      },
      facts: {
        title: '你能核对的事',
        protocol: { title: '官方协议直连', body: '不做私有格式转换，请求体与响应按各家官方协议原样透传。' },
        pricing: { title: '价格公开', body: '模型页列出每个模型的官方参考价，按 Token 计费，先看价再用。' },
        ledger: { title: '逐条可查', body: '用量页按请求列出模型、Token、耗时与费用，支持筛选与 CSV 导出。' },
        balance: { title: '余额透明', body: '可用余额与冻结金额分开显示，每一笔扣费都能追溯到具体请求。' }
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
        avgLatency: '平均耗时'
      },
      sections: {
        trend: '用量趋势',
        models: '模型用量',
        records: '请求明细'
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
      allVendors: '全部厂商',
      count: '{count} 个模型',
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
