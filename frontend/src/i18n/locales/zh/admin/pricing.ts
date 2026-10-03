export default {
  pricing: {
    description: '官方价与每个渠道的上游价。给模型加一个渠道，就是让这个渠道承接这个模型。',
    views: {
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
    basis: '单价按 $ / 百万 Token。毛利按默认售价（官方价 × {rate}）算；最低毛利率 {margin}，低于它的承接，利润门会跳过。',
    basisGateOff: '单价按 $ / 百万 Token。毛利按默认售价（官方价 × {rate}）算；利润门已关闭。',
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
    officialHint: '售价 = 官方价 × 用户倍率（默认 {rate}）',
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
    segmentsCount: '{count} 段',
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
    channelState: {
      ok: '可调度',
      paused: '暂不可调度',
      disabled: '已停用',
      missing: '渠道不存在'
    },
    changes: '改了 {count} 处',
    undo: '撤销',
    saving: '保存中…',
    listSeparator: '、',
    issueSeparator: '；',
    issues: {
      missing: '{fields}还没填',
      invalid: '{fields}格式不对（只能填非负数）',
      segment: '第 {index} 段：{error}',
      upstreamModel: '上游模型名只能是一个具体的名字，不能带 * 或空格'
    },
    discardTitle: '放弃未保存的修改？',
    discardMessage: '有 {count} 块改了还没保存，继续会丢掉这些修改。',
    discard: '放弃修改'
  }
}
