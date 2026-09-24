export default {
  modelCatalog: {
    description: '平台模型的基准价与别名，计价从这里取。',
    search: '搜索模型标识、展示名、厂商或别名',
    create: '新建模型',
    edit: '编辑模型',
    empty: '目录还是空的，先播种或手动新建。',
    noMatch: '没有匹配的条目',
    summaryStats: {
      total: '模型',
      listed: '已上架',
      listedWithoutResources: '上架但无渠道',
      showThem: '筛选'
    },
    filtered: '筛选后 {count} 个',
    aliasCount: '{count} 别名',
    filters: {
      noVendor: '（无厂商）',
      withResources: '有渠道',
      withoutResources: '无渠道'
    },
    columns: {
      price: '标价',
      perMillion: '$ / 百万 Token',
      perUnit: {
        per_request: '每次',
        image: '每张',
        video: '每秒'
      },
      tiers: '{count} 档'
    },
    bulk: {
      list: '上架',
      unlist: '下架',
      nothingToDo: '选中的条目已经是目标状态',
      listedDone: '已上架 {count} 个模型',
      unlistedDone: '已下架 {count} 个模型',
      partial: '成功 {done} 个，失败 {failed} 个（失败的仍留在选中集里）：{errors}'
    },
    editor: {
      basics: '基本信息',
      pricing: '计费与标价',
      vendorHint: '用小写厂商标识（anthropic / openai / gemini / xai…），用户站的厂商页签与图标按它匹配。',
      perMillion: '= ${price} / 百万 Token',
      morePrices: '更多价格',
      morePricesFilled: '已填 {count} 项',
      morePricesHint: '缓存、图片、音频的 Token 单价；留空即未配置。'
    },
    diagnose: '诊断',
    diagnosis: {
      title: '渠道诊断 · {model}',
      empty: '该模型没有绑定渠道。',
      followAccount: '跟随账号',
      columns: {
        account: '渠道',
        priority: '优先级',
        schedulable: '可调度'
      },
      inbound: {
        anthropic: 'Messages',
        chat_completions: 'Chat',
        responses: 'Responses',
        gemini: 'Gemini'
      },
      reasons: {
        disabled: '已停用',
        unschedulable: '已停调',
        expired: '已过期',
        overloaded: '过载冷却中',
        rate_limited: '限流中',
        temp_unschedulable: '临时停调',
        quota_exceeded: '额度已用尽'
      }
    },
    seed: '从价格文件播种',
    seeding: '播种中…',
    seedDone: '播种完成：新增 {inserted}，刷新 {refreshed}，跳过管理员改过的 {skipped}',
    seedPartial: '{summary}；另有 {failed} 条写入失败：{errors}',
    deleteTitle: '删除目录条目',
    deleteConfirm: '删除后别名、分档和分时定价会一起删掉。确定删除？',
    fullReplaceHint: '保存是整条覆盖。本页没列出的项（按 Token 的区间分档、分时定价、优先级价、长上下文与倍率）按原值写回；图片 / 视频分档在上方编辑。',
    listedRequiresPrice: '上架的模型必须配好价格，用户才能看到并调用。',
    noResources: '无渠道',
    fields: {
      modelId: '模型标识',
      displayName: '展示名',
      vendor: '厂商',
      billingMode: '计费模式',
      status: '上架状态',
      managedBy: '维护方',
      resources: '渠道',
      inputPrice: '输入价（$/token）',
      outputPrice: '输出价（$/token）',
      perRequestPrice: '每次默认价（$）',
      perImagePrice: '每张默认价（$，分档未命中时用）',
      perSecondPrice: '每秒默认价（$，分档未命中时用）',
      searchPricePerCall: '内置搜索每次调用价（$，留空用内置单价 0.01）',
      cacheWritePrice: '缓存写入价 · 5 分钟（$/token）',
      cacheWrite1hPrice: '缓存写入价 · 1 小时（$/token）',
      cacheReadPrice: '缓存读取价（$/token）',
      imageInputPrice: '图片输入价（$/token）',
      imageOutputPrice: '图片输出价（$/token）',
      imageCacheReadPrice: '图片缓存读取价（$/token）',
      audioInputPrice: '音频输入价（$/token）',
      audioOutputPrice: '音频输出价（$/token）'
    },
    tiers: {
      title: '分档单价',
      hint: {
        image: '按输出尺寸分档，每张价；上架必须填默认价。',
        video: '按分辨率分档，每秒价；上架必须填默认价。'
      },
      tier: '档位',
      price: '单价（$）',
      add: '加一档',
      remove: '移除',
      empty: '未配分档，按默认价计。'
    },
    bindings: {
      title: '绑定渠道',
      hint: '上架后由这些渠道承接请求；优先级留空则跟随渠道自身的优先级。',
      search: '搜索渠道名称',
      noResults: '没有匹配的渠道',
      add: '添加',
      priority: '优先级',
      remove: '移除',
      empty: '尚未绑定渠道'
    },
    status: {
      listed: '上架',
      unlisted: '下架'
    },
    billingModes: {
      token: '按 Token',
      per_request: '按次',
      image: '按图片',
      video: '按视频'
    },
    managedBy: {
      seed: '播种',
      admin: '管理员'
    }
  }
}
