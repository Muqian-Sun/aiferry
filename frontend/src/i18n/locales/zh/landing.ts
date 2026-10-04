export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    login: '登录',
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API 密钥用量查询',
    subtitle: '输入你的 API 密钥，查看实时消费金额与使用状态',
    placeholder: 'sk-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中…',
    // 查询要把密钥发到本站服务器（GET /v1/usage 带 Authorization），只是页面不保存它
    privacyNote: '密钥只用来向本站服务器查询用量，页面不会保存它',
    dateRange: '统计范围：',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: '密钥限额模式',
    walletBalance: '余额',
    keyStatus: {
      active: '可用',
      disabled: '已停用',
      quota_exhausted: '额度已用完',
      expired: '已过期'
    },
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuotaIn: '已用额度（{window}）',
    windows: {
      fiveHours: '5 小时',
      day: '每日',
      sevenDays: '7 天',
      week: '每周',
      month: '每月'
    },
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    billingType: '计费方式',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API 密钥',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    // 网关鉴权失败的 code → 文案
    errors: {
      INVALID_API_KEY: '密钥无效或已停用',
      API_KEY_DISABLED: '密钥无效或已停用',
      USER_NOT_FOUND: '密钥无效或已停用',
      USER_INACTIVE: '密钥所属的账户已被停用',
      ACCESS_DENIED: '当前 IP 不在这把密钥允许的范围内',
      INVALID_AUTH_RATE_LIMITED: '无效查询太多，请稍后再试'
    },
    noDailyUsage: '这段时间没有用量',
  },

  // Setup Wizard
  setup: {
    title: 'AiFerry 安装向导',
    description: '配置 AiFerry 实例',
    errors: {
      database: '连不上数据库，请检查主机、端口、用户名、密码和数据库名称',
      redis: '连不上 Redis，请检查主机、端口和密码',
      install: '安装失败，请检查上面的配置后重试'
    },
    database: {
      title: '数据库配置',
      description: '连接 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 6 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配',
      adminPort: '管理后台端口',
      adminPortHint: '管理页面与管理 API 只在该端口提供，建议只对内网开放',
      adminPortConflict: '管理后台端口不能与当前服务端口相同'
    },
    ready: {
      adminConsole: '管理后台地址',
      title: '准备安装',
      description: '检查配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中…',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中…',
      completeInstallation: '完成安装',
      completed: '安装完成',
      redirecting: '正在跳转到登录页面…',
      restarting: '服务正在重启，请稍候…',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
