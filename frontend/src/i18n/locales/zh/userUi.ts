/**
 * 用户站新壳与新页面的文案。旧 nav.* 键仍被管理端和过渡期页面使用，这里不复用其措辞。
 */
export default {
  userUi: {
    nav: {
      overview: '概览',
      usage: '用量明细',
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
      primaryNav: '主导航',
      collapseSidebar: '收起',
      expandSidebar: '展开侧栏'
    },
    footer: {
      register: '注册',
      rights: '保留所有权利'
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
      tabTitle: '一把 Key，摆渡全球大模型',
      legalTabTitle: '使用政策与隐私',
      hero: {
        title: '一把 Key，',
        titleAccent: '摆渡全球大模型',
        description: 'AiFerry 按官方协议原样转发你的请求：不改写、不偷换模型、不出售数据。只改一行 base_url，现有的 SDK 和代码照常运行。',
        getStarted: '开始使用',
        goToConsole: '进入控制台',
        viewPricing: '查看模型与价格',
        points: {
          protocol: '官方协议原样透传',
          sameModel: '同模型换渠道，绝不换模型',
          ledger: '按量计费，逐条可查'
        },
        vendorsLabel: '已接入的厂商'
      },
      features: {
        eyebrow: '我们的做法',
        title: '只做转发这一件事，做到底',
        description: '中间层能少做一点是一点：不改你的请求、不换你的模型、不卖你的数据。',
        items: {
          passthrough: { title: '尽量透传', body: '四条官方协议原样转发，不做私有格式转换、不改写你的请求。SDK、流式、工具调用、多模态，行为与直连一致。' },
          failover: { title: '不做不同模型兜底', body: '你点名的模型就是实际跑的模型。上游超时或报错，只在同一模型的其他渠道之间切换，绝不悄悄降级成便宜模型。' },
          cache: { title: '最大程度缓存', body: '同一会话固定落在同一上游，提示词缓存持续命中。长对话越聊越快，费用也跟着降下来。' },
          privacy: { title: '不记录对话内容', body: '不保存你发送的请求和模型回复，只留一行账：模型、Token、耗时、费用。请求失败只留错误信息排障，30 天后自动删除；跨协议转换时推理摘要缓存 7 天；开启内容审计时的留存方式见隐私政策。' },
          noSale: { title: '不出售用户数据', body: '不卖、不出租、不拿去训练。除完成请求必需的上游模型（开启内容审计时还有审核服务）外，你的请求与账单不会交给任何第三方。' }
        },
        figure: {
          passthrough: { endpoint: '一个 base_url', note: '同协议原样透传' },
          failover: { channelA: '渠道 A', channelB: '渠道 B', timeout: '超时', switched: '同模型换渠道' },
          cache: { session: '会话', pinned: '固定上游', request: '请求', hit: '缓存命中' },
          privacy: { request: '请求正文', notStored: '不保存', kept: '只留这一行' },
          noSale: { yourData: '你的数据', barrier: '不卖 · 不共享 · 不训练', thirdParty: '第三方', ads: '广告', brokers: '数据商' }
        }
      },
      stats: {
        models: '已接入模型',
        vendors: '厂商',
        protocols: '官方协议',
        clients: '客户端配置'
      },
      protocols: { messages: 'Messages', responses: 'Responses', chat: 'Chat', gemini: 'Gemini' }
    },
    overview: {
      title: '概览',
      description: '余额、今日用量与接入信息',
      greeting: {
        lateNight: '夜深了，{name}',
        morning: '早上好，{name}',
        noon: '中午好，{name}',
        afternoon: '下午好，{name}',
        evening: '晚上好，{name}'
      },
      totals: '累计消耗 {cost} · 累计请求 {requests} · 当前 {rpm} RPM',
      quickStart: {
        title: '快速开始',
        description: '把接口地址和密钥填进 SDK 或客户端，就能直接调用。',
        baseUrl: '接口地址',
        key: '我的密钥',
        noKey: '还没有可用的密钥',
        allKeys: '全部密钥',
        createKey: '创建密钥',
        copy: '复制',
        copied: '已复制',
        example: '调用示例',
        exampleHint: '把 $API_KEY 换成上面复制的密钥，或先 export API_KEY=你的密钥',
        exampleMessage: '你好'
      },
      trend: {
        title: '用量趋势',
        range: '近 {days} 天',
        summary: '近 {days} 天 · {requests} 次请求 · {tokens} Token · 费用 {cost}'
      }
    },
    billing: {
      title: '账务',
      description: '充值、订阅、订单与邀请返利',
      tabs: {
        recharge: '充值',
        subscriptions: '订阅'
      }
    },
    usage: {
      title: '用量明细',
      description: '按时间范围查看模型用量与每一次请求',
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
      sections: {
        profile: '基本信息',
        security: '安全',
        notifications: '通知'
      },
      descriptions: {
        profile: '头像、用户名与账户信息',
        security: '第三方登录绑定、密码、双因素认证与通行密钥',
        notifications: '余额不足时发邮件提醒'
      },
      notificationsOff: '管理员没有开启余额提醒',
      rows: {
        overview: '账户概况',
        overviewDesc: '登录身份、余额与计价倍率',
        usernameDesc: '未设置时显示邮箱',
        passwordDesc: '用于邮箱登录，至少 8 个字符'
      }
    },
    summary: {
      keys: '密钥',
      activeKeys: '活跃',
      redeemHistoryDesc: '兑换与管理员调整都记在这里'
    },
    models: {
      title: '模型',
      description: '已上架的模型与标价，按 Token 或按次计费',
      hero: {
        title: '全部模型，',
        titleAccent: '明码标价',
        description: '上架的每个模型与标价都列在这里。搜索或按厂商筛选，复制模型 ID 就能直接调用。'
      },
      allVendors: '全部',
      allBilling: '全部计费',
      searchHint: '搜索模型、别名（按 / 聚焦）',
      vendorTabsLabel: '厂商',
      priceUnit: '价格单位：美元 / 百万 Token',
      yourPriceApplied: '已按你的账户倍率 ×{multiplier} 折算',
      multiplierNote: '你的账户倍率是 {multiplier}。',
      timePricing: '分时',
      weekdaysOnly: '仅工作日',
      prices: {
        input: '输入',
        output: '输出',
        cacheWrite: '缓存写入',
        cacheWrite1h: '缓存写入（1 小时）',
        cacheRead: '缓存读取',
        imageInput: '图片输入',
        imageOutput: '图片输出',
        perRequest: '每次',
        perImage: '每张',
        perSecond: '每秒'
      },
      priceNote: '价格为本站目录标价。你的实付 = 标价 × 账户倍率，逐条记录在用量页。',
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
