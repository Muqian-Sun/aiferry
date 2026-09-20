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
    status: {
      loading: '加载中'
    }
  }
}
