/** 渠道健康（V2 被动监控）：管理站汇总配置面板的文案与错误分类名。用户站服务状态页的文案在 userUi.serviceStatus */
export default {
  channelMonitorV2: {
    errorCategories: {
      content_policy: '内容策略', authentication: '认证失败', context_limit: '上下文超限', invalid_request: '请求格式', model_unsupported: '模型不支持', quota_or_balance: '额度或余额', account_pool_unavailable: '账号池不可用', rate_or_capacity: '限流或容量', timeout: '超时', transport_or_stream: '传输或流', upstream_forbidden: '上游拒绝', not_found: '资源不存在', client_cancelled: '客户端取消', upstream_5xx: '上游 5xx', internal: '内部错误', other: '其他'
    },
    settings: {
      visibility: {
        title: '用户端展示',
        description: '用户在「渠道状态」页能看到哪些数据。',
        hideThroughput: '对用户隐藏吞吐速率（RPM / TPM）',
        hideThroughputHint:
          '开启后，用户端渠道状态页与用户 API 不返回 RPM/TPM，避免用「速率 × 时间窗」反推集群规模。管理员仍可见完整指标；错误率、延迟、缓存率照常展示。',
        hideUserRanking: '对用户隐藏用户排行',
        hideUserRankingHint:
          '开启后，用户端渠道状态页不再显示「用户排行」，用户 API 也不返回排行数据。管理员仍可查看。',
      },
      save: '保存',
      loading: '加载中...',
      loadFailed: '配置加载失败',
      saveSuccess: '配置已保存',
      saveFailed: '配置保存失败',
      enableTitle: '启用汇总',
      enableHint: '关闭后只停止这里的汇总；整个渠道健康功能的开关在「设置 › 开关」',
      refreshTitle: '汇总频率',
      refreshHint: '影响矩阵时间粒度与刷新节奏',
      refreshAria: '汇总频率',
      platformsTitle: '平台与模型',
      platformsHint: '留空 = 展示全部真实模型名；填写后仅名单内单独成行，其余归入「其他」',
      modelsPlaceholder: '留空=全部真实模型；或填写主流模型名单（其余归其他）',
      badgeAllModels: '全部模型',
      badgeOther: '+ 其他',
      errorsTitle: '错误分类与忽略',
      errorsHint:
        '勾选「忽略」的类别不计入错误率与健康分，仍在错误原因列表中以灰色显示并标记忽略。未匹配的错误归入「其他」。',
      ignoredSummary: '已忽略 {ignored} 类 · 计入错误率 {counted} 类',
      healthTitle: '健康阈值',
      healthHint: '控制用户端色块和整体评分。默认阈值较宽松，避免少量错误或低缓存率立即显示异常。',
      fields: {
        minimumSample: '最小样本数',
        warningError: '错误率关注 %',
        criticalError: '错误率异常 %',
        targetTtft: 'TTFT 目标 ms',
        warningTtft: 'TTFT 关注 ms',
        criticalTtft: 'TTFT 异常 ms',
        warningCache: '缓存率关注 %',
        criticalCache: '缓存率异常 %',
      },
      namedModelsEmpty: '各平台模型列表为空：将展示全部真实模型名（不归入「其他」）。',
      namedModelsCount: '将展示 {count} 个命名模型维度；名单外模型归入各平台「其他」。',
      userContractTitle: '用户端展示约定',
      userContract: {
        health: '健康色三指标：错误率 60% + 首 Token P50 20% + 缓存率 20%（阈值可在上方配置）',
        trend: '趋势可切换色块矩阵 / 折线图（错误率 · 缓存率 · 首 Token）',
        latency: '延迟展示 AVG · P50 · P90；不展示绝对请求数 / 错误数',
        models: '模型列表留空时展示真实模型名，不会全部归入「其他」',
      },
    },
  },
}
