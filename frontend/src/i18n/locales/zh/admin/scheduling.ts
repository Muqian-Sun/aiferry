export default {
  scheduling: {
    loadFailed: '调度没有加载出来',
    refreshFailed: '最近一次刷新失败，显示的是上一次的数据',
    window: { label: '时间范围', '15m': '15 分钟', '1h': '1 小时', '24h': '24 小时' },
    refresh: {
      auto: '每 {seconds} 秒刷新 · {time}',
      paused: '页面不在前台，已暂停刷新'
    },
    warnings: {
      load_unavailable: '并发数据没取到：顺序按优先级与最近使用排，进行中与排队显示为「—」。',
      sticky_unavailable: '粘性会话数没取到，以下不显示粘性会话。'
    },
    stats: {
      models: '模型',
      channels: '可调度渠道',
      inUse: '进行中',
      waiting: '排队',
      requests: '请求数',
      dataThrough: '统计到 {time}',
      unrouted: '没选到渠道'
    },
    models: {
      title: '各模型的调度顺序',
      hint: '#1 是下一个新请求会落的渠道；已有粘性会话的请求先回到原渠道。排队 = 能接但并发已满；虚线框的此刻不会被选。',
      search: '搜索模型',
      empty: '还没有上架并绑定渠道的模型',
      noMatch: '没有匹配的模型',
      noLane: '没有能承接这个模型的渠道'
    },
    protocols: {
      label: '入站协议',
      only: '仅 {protocol}',
      anthropic: 'Messages',
      chat_completions: 'Chat',
      responses: 'Responses',
      gemini: 'Gemini'
    },
    traffic: {
      requests: '{count} 次请求',
      failures: '{count} 次失败',
      unrouted: '{count} 次没选到渠道'
    },
    item: {
      direct: '直连',
      queue: '排队',
      blocked: '不会被选',
      load: '并发 {inUse}/{max}',
      waiting: '排队 {count}',
      loadUnknown: '并发 —',
      requests: '{count} 次请求',
      share: '{count} 次请求（{percent}%）',
      sticky: '粘性会话 {count}',
      recoversIn: '{reason}，{duration}后恢复'
    },
    reasons: {
      missing: '渠道不存在',
      disabled: '已停用',
      unschedulable: '已停调',
      expired: '已过期',
      overloaded: '过载冷却中',
      rate_limited: '限流中',
      temp_unschedulable: '临时停调',
      model_rate_limited: '该模型限流中',
      model_transient_blocked: '该模型短时熔断',
      grok_model_blocked: '该模型冷却中',
      proxy_quarantined: '代理隔离中',
      profit_threshold: '低于最低毛利率',
      model_not_supported: '不支持该模型',
      shadow_parent_unhealthy: '母账号不可用',
      compact_unsupported: '不支持 compact',
      protocol_unsupported: '承接不了该协议',
      rpm_limited: 'RPM 已满',
      other: '其他原因（{reason}）'
    }
  }
}
