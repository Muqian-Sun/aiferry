export default {
  pricing: {
    views: {
      label: '查看方式',
      model: '按模型',
      channel: '按渠道'
    },
    searchModels: '搜索模型或渠道',
    searchChannels: '搜索渠道或模型',
    filters: {
      vendor: '厂商',
      status: '上架',
      focus: '只看',
      problems: '有问题的',
      unsaved: '未保存的'
    },
    basis: '$ / 百万 Token · 最低毛利 {margin}',
    basisGateOff: '$ / 百万 Token · 利润门已关',
    unsavedBlocks: '{count} 块未保存',
    empty: '没有符合条件的模型或渠道',
    loadFailed: '价格加载失败',
    reload: '重新加载',
    columns: {
      channel: '渠道',
      model: '模型',
      upstreamModel: '上游模型名',
      input_price: '输入',
      output_price: '输出',
      cache_read_price: '缓存读',
      cache_write_price: '缓存写 5 分钟',
      cache_write_1h_price: '缓存写 1 小时',
      search_price_per_call: '每次 web 搜索',
      x_post_price: 'X 搜索 · 帖子',
      x_user_price: 'X 搜索 · 主页',
      segments: '分段',
      margin: '毛利',
      status: '状态',
      actions: '操作'
    },
    official: '官方价',
    costGroup: '成本价',
    webSearchDelegate: '联网搜索计费项',
    webSearchDelegateHint: 'Claude Code 第三方模型的联网搜索由它执行；不对用户上架',
    search: {
      toggle: '联网搜索',
      title: '联网搜索',
      units: {
        search_price_per_call: '$ / 千次',
        x_post_price: '$ / 千条',
        x_user_price: '$ / 千个'
      },
      defaultPlaceholder: '公开价 {price}',
      officialRef: '官方 {price}',
      officialNone: '官方不收',
      free: '免费'
    },
    sameName: '同名',
    upstreamModelHint: '留空 = 与目录标识同名',
    officialRef: '官方 {price}',
    officialUnset: '官方未设',
    required: '必填',
    newRow: '新加',
    noChannels: '还没有渠道',
    noModels: '还没有模型',
    noVendor: '未设厂商',
    status: {
      listed: '已上架',
      unlisted: '未上架'
    },
    channelCount: '{count} 个渠道',
    modelCount: '{count} 个模型',
    priority: '优先级 {priority}',
    segmentsNone: '不分段',
    segmentsCount: '分 {count} 段',
    segmentAbove: '超过',
    segmentInherit: '同第 1 段',
    segmentAdd: '加一段',
    remove: '移除',
    addChannel: '加渠道',
    addModel: '加模型',
    searchModel: '搜索模型',
    noMatch: '没有可加的',
    copyFromSibling: '从同上游的渠道复制价格',
    fillFromPriceFile: '按价格文件带官方价',
    sale: {
      label: '售价',
      cell: '售价 · {item}',
      clear: '全部清空',
      segmentAbove: '超过 {tokens} Token',
      invalid: '售价：有格式不对的价',
      peakInvalid: '售价忙闲时：有没填对的时段或日期'
    },
    saleFill: {
      trigger: '按比例填售价',
      ratio: '比例'
    },
    discountFill: {
      trigger: '按折扣填成本价',
      prefix: '官方价 ×',
      ratio: '折扣',
      apply: '填入'
    },
    priceFileMissing: '价格文件里没有这个模型的按 Token 价',
    marginAfterSave: '保存后计算',
    gateSkips: '利润门会跳过',
    peak: {
      toggle: '忙闲时',
      none: '不分忙闲时',
      summary: '忙时 ×{multiplier}',
      official: {
        title: '官方忙闲时',
        noneHint: '全天一个价'
      },
      sale: {
        title: '售价忙闲时',
        noneHint: '全天一个价',
        follow: '跟官方忙闲时（{summary}）',
        followShort: '跟官方忙闲时',
        custom: '单独设',
        noneShort: '全天一个价'
      },
      upstream: {
        title: '上游忙闲时',
        noneHint: '全天一个价'
      },
      excludeDates: '节假日（全天按平时）',
      excludeDatesPlaceholder: '2026-10-01 2026-10-02 …',
      excludeDatesInvalid: '日期格式应为 YYYY-MM-DD',
      excludeDatesCount: '{count} 天',
      useOfficial: '按官方忙闲时',
      clear: '改成不分忙闲时',
      timezone: '时区',
      zones: {
        beijing: '北京时间',
        utc: 'UTC',
        pacific: '美西时间'
      },
      weekdaysOnly: '只在工作日（周末全天按平时）',
      start: '开始',
      end: '结束',
      multiplier: '倍数',
      add: '加一个时段',
      margin: '忙时 {margin}',
      gateSkips: '忙时利润门会跳过',
      errors: {
        time: '时间没填或格式不对',
        order: '开始要早于结束',
        multiplier: '倍数要大于 0、最多两位小数',
        overlap: '和别的时段重叠'
      }
    },
    channelState: {
      ok: '可调度',
      paused: '暂不可调度',
      error: '异常',
      disabled: '已停用',
      missing: '渠道不存在'
    },
    changes: '改了 {count} 处',
    undo: '撤销',
    saving: '保存中…',
    listSeparator: '、',
    issueSeparator: '；',
    issues: {
      missing: '{fields} 还没填',
      invalid: '{fields} 格式不对（只能填非负数）',
      segment: '第 {index} 段：{error}',
      upstreamModel: '上游模型名只能是一个具体的名字，不能带 * 或空格',
      peak: '忙闲时第 {index} 段：{error}',
      peakDates: '忙闲时的节假日有写错的日期'
    },
    discardTitle: '放弃未保存的修改？',
    discardMessage: '有 {count} 块改了还没保存，继续会丢掉这些修改。',
    discard: '放弃修改'
  }
}
