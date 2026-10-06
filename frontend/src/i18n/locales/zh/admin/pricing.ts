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
    basis: '$ / 百万 Token · 毛利按售价算（没单独定售价的项按官方价 × {rate}），低于 {margin} 的承接利润门会跳过',
    basisGateOff: '$ / 百万 Token · 毛利按售价算（没单独定售价的项按官方价 × {rate}）· 利润门已关闭',
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
    webSearchDelegate: '联网搜索计费项',
    webSearchDelegateHint: 'Claude Code 配第三方模型时，那次单独的搜索请求交给这个模型执行：它的官方价就是「联网搜索」计费项（Token × 用户倍率 + 每次搜索按原价），承接它的渠道就是执行渠道；不对用户上架',
    search: {
      toggle: '联网搜索',
      title: '联网搜索',
      officialNote: '按官方原价收，不乘用户倍率；空着按厂商公开价收',
      upstreamNote: '官方价设了的项必须填；没填的按官方价记成本',
      units: {
        search_price_per_call: '$ / 千次',
        x_post_price: '$ / 千条',
        x_user_price: '$ / 千个'
      },
      defaultPlaceholder: '默认 {price}',
      officialRef: '官方 {price}',
      officialDefaultRef: '官方 {price}（厂商公开价）',
      optional: '选填'
    },
    catalogName: '目录标识',
    sameName: '同名',
    upstreamModelHint: '这个渠道给这个模型用的模型名；留空 = 与目录模型标识同名。用户只能请求目录模型标识，转发时只转换这一次。',
    officialRef: '官方 {price}',
    officialUnset: '官方未设',
    officialReadOnly: '这个视图里官方价只作参考，要改官方价请切到「按模型」。',
    required: '必填',
    newRow: '新加',
    noChannels: '还没有渠道承接这个模型。点「加渠道」选一个渠道并填上游价。',
    noModels: '这个渠道还没有承接任何模型。点「加模型」选模型并填上游价。',
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
    segmentHint: '按单次请求的输入 Token 数（输入 + 缓存写 + 缓存读）落在哪一段，整条请求按那一段的价计费。上面一行是第 1 段；各段没填的价按第 1 段算。',
    remove: '移除',
    addChannel: '加渠道',
    addModel: '加模型',
    searchModel: '搜索模型',
    noMatch: '没有可加的',
    copyFromSibling: '从同上游的渠道复制价格',
    fillFromPriceFile: '按价格文件带官方价',
    sale: {
      label: '售价',
      hint: '没填 = 官方价 × {ratio}',
      scope: '实付 = 售价 × 用户折扣',
      cell: '售价 · {item}',
      clear: '全部清空',
      segmentAbove: '超过 {tokens} Token',
      invalid: '售价：有格式不对的价'
    },
    saleFill: {
      trigger: '按比例填售价',
      hint: '空着的售价按「官方价 × 比例」填上（分段一起），已填的不动；填完记得保存这一块。',
      ratio: '比例'
    },
    discountFill: {
      trigger: '按折扣填上游价',
      hint: '空着的上游价按「官方价 × 折扣」填上（分段一起折），已填的不动；填完记得保存这一块。',
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
      title: '上游忙闲时',
      note: '上游在这些时段整单按倍数收；只影响渠道成本与利润门，不影响向用户收的钱。',
      noneHint: '上游全天一个价',
      useDeepSeek: '按 DeepSeek 官方',
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
      endHint: '结束填 00:00 表示到当天结束',
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
      peak: '忙闲时第 {index} 段：{error}'
    },
    discardTitle: '放弃未保存的修改？',
    discardMessage: '有 {count} 块改了还没保存，继续会丢掉这些修改。',
    discard: '放弃修改'
  }
}
