/**
 * 用户站新壳与新页面的文案。旧 nav.* 键仍被管理端和过渡期页面使用，这里不复用其措辞。
 */
export default {
  userUi: {
    nav: {
      overview: '概览',
      usage: '用量明细',
      keys: '密钥',
      billing: '账务',
      account: '账户',
      batchImage: '批量生图',
      status: '服务状态',
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
      debt: '欠费',
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
      greeting: {
        lateNight: '夜深了，{name}',
        morning: '早上好，{name}',
        noon: '中午好，{name}',
        afternoon: '下午好，{name}',
        evening: '晚上好，{name}'
      },
      range: {
        label: '时间范围',
        days: '近 {days} 天'
      },
      numbers: {
        today: '今日 Token',
        range: '近 {days} 天 Token',
        total: '累计 Token'
      },
      models: {
        other: '其他',
        empty: '这段时间没有用量'
      },
      trend: {
        title: '用量趋势'
      },
      attention: {
        title: '需要处理',
        view: '查看',
        recharge: '充值',
        renew: '续费',
        keyExpiredOne: '密钥「{name}」已过期',
        keyExpired: '{count} 把密钥已过期',
        keyQuotaOne: '密钥「{name}」额度已用尽',
        keyQuota: '{count} 把密钥额度已用尽',
        keyQuotaNearOne: '密钥「{name}」额度已用 {percent}%',
        keyNearLimitOne: '密钥「{name}」{limit}限额已用 {percent}%',
        keyNearLimit: '{count} 把密钥限额将满',
        keyExpiringOne: '密钥「{name}」{days} 天后到期',
        keyExpiring: '{count} 把密钥 7 天内到期',
        balanceEmpty: '余额已用完',
        balanceZero: '余额为 0，调用会被拒绝',
        contactAdmin: '本站没开在线充值，请联系管理员充值',
        contactAdminWith: '本站没开在线充值，请联系管理员充值：{contact}',
        balanceRunway: '余额 {balance}，按近 7 天的用量约够 {days} 天',
        balanceBelowThreshold: '余额 {balance}，低于你设的提醒线 {threshold}',
        subscriptionQuota: '订阅「{name}」{window}额度已用 {percent}%',
        subscriptionExpiring: '订阅「{name}」{days} 天后到期',
        failuresToday: '今天有 {count} 次失败请求'
      },
      subscriptions: {
        title: '订阅额度',
        manage: '管理订阅',
        unlimited: '不限额',
        daily: '今日',
        weekly: '本周',
        monthly: '本月',
        fallbackName: '订阅',
        noExpiry: '长期有效',
        expiresIn: '{days} 天后到期'
      },
      gettingStarted: {
        title: '开始使用',
        description: '把接口地址和密钥填进 SDK 或客户端，就能直接调用。发出第一次请求后，这里会换成用量概览。',
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
      }
    },
    billing: {
      title: '账务',
      description: '充值、订阅与兑换码',
      tabs: {
        recharge: '充值',
        subscriptions: '订阅'
      }
    },
    usage: {
      title: '用量明细',
      description: '钱花在哪了、某一次请求怎么了',
      moreActions: '更多操作',
      moreFilters: '更多筛选',
      clearFilters: '清除筛选',
      costShare: '费用占比',
      stats: {
        requests: '请求',
        tokens: 'Token',
        actualCost: '实付',
        cacheHitRate: '缓存命中',
        avgLatency: '平均耗时',
        failures: '失败请求',
        viewFailures: '查看'
      },
      sections: {
        spend: '费用分布',
        records: '请求明细'
      },
      detail: {
        eyebrow: '请求详情',
        request: '请求信息',
        requestId: '请求 ID',
        requestIdCopied: '已复制请求 ID',
        copy: '复制'
      },
      trend: {
        tokens: 'Token',
        requests: '请求',
        cost: '费用',
        empty: '这段时间没有数据'
      },
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
        overviewDesc: '登录身份、余额与并发',
        usernameDesc: '未设置时显示邮箱',
        passwordDesc: '用于邮箱登录，至少 6 个字符'
      }
    },
    summary: {
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
      weekdaysOnly: '仅工作日',
      openDetail: '查看全部计费项',
      prices: {
        input: '输入',
        output: '输出',
        cacheWrite: '缓存写',
        cacheRead: '缓存读',
        imageInput: '图片输入',
        imageOutput: '图片输出',
        imageCacheRead: '图片缓存读',
        audioInput: '音频输入',
        audioOutput: '音频输出',
        perRequest: '每次',
        perImage: '每张',
        perSecond: '每秒'
      },
      tags: {
        segments: '分 {count} 段',
        tiers: '分 {count} 档',
        cache1h: '1 小时缓存',
        image: '图片',
        audio: '音频',
        search: '联网搜索',
        maxReasoning: '最高推理加价',
        timePricing: '分时'
      },
      detail: {
        standard: '标准价',
        media: '图片与音频',
        unitPrice: '单价',
        tier: '档位',
        defaultTier: '其他档位',
        tools: '工具',
        search: '联网搜索',
        searchOfficialPrice: '每次搜索按次计费',
        xPosts: 'X 搜索 · 帖子',
        xUsers: 'X 搜索 · 主页',
        perThousandCalls: '{price} / 千次',
        perThousandPosts: '{price} / 千条',
        perThousandUsers: '{price} / 千个',
        claudeCodeSearch: 'Claude Code 联网搜索',
        claudeCodeSearchPrice: '输入 {input} · 输出 {output} / 百万 Token，另加 {search} / 千次搜索',
        claudeCodeSearchNote: '用 Claude Code 时，搜索请求由平台代为执行、单独计费',
        other: '其他',
        maxReasoning: '最高推理档',
        maxReasoningRule: 'reasoning_effort=max 时整单 × {multiplier}',
        timePricing: '分时倍率',
        aliases: '别名',
        ttl5m: '5 分钟',
        ttl1h: '1 小时',
        unitPerMillion: '美元 / 百万 Token',
        unitPerRequest: '美元 / 次',
        unitPerImage: '美元 / 张',
        unitPerSecond: '美元 / 秒',
        noPricing: '目录里还没有这个模型的价格。'
      },
      priceNote: '每次请求的实际扣费逐条记录在用量页。',
      segmentRange: '输入 Token',
      segmentNote: '分段计价的模型按单次请求的输入 Token 数（输入 + 缓存写 + 缓存读）落在哪一段，整条请求都按那一段的价格计费。',
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
    },
    serviceStatus: {
      title: '服务状态',
      description: '各模型最近的可用率、首字延迟和缓存命中率，按本站的真实请求统计',
      range: { label: '时间范围', '90m': '90 分钟', '24h': '24 小时', '7d': '7 天', '30d': '30 天' },
      stats: { availability: '可用率', ttft: '首字延迟（中位数）', ttftP90: '九成请求在 {value} 内', cache: '缓存命中率' },
      models: {
        title: '各模型',
        description: '有问题的排在前面；色条从左到右是时间，一格是一段，悬停看那一段的数字',
        search: '搜索模型',
        filter: { label: '按状态筛选', all: '全部', issues: '有问题 {count}', healthy: '正常 {count}' },
        empty: '这段时间还没有模型的数据',
        noMatch: '没有匹配的模型',
        idle: '另有 {count} 个模型这段时间没有请求',
        showIdle: '展开',
        hideIdle: '收起',
        stripLabel: '{model} 各时段状态'
      },
      columns: { availability: '可用率', ttft: '首字延迟', cache: '缓存命中率' },
      legend: { healthy: '正常', warning: '不稳定', critical: '异常', unknown: '请求太少' },
      slot: { detail: '{time}　可用率 {availability} · 首字延迟 {ttft} · 缓存命中率 {cache}', fewRequests: '（请求较少，不评状态）', noRequests: '{time}　没有请求' },
      trend: { title: '整体趋势', empty: '这段时间还没有数据' },
      loadFailed: '服务状态没有加载出来',
      footnote: '数据来自本站的真实请求。首字延迟是从发出请求到收到第一段输出的时间；缓存命中率是输入里命中缓存的 token 占比。'
    }
  }
}
