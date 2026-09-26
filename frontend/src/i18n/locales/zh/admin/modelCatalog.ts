export default {
  modelCatalog: {
    description: '平台模型的标价与别名；收入、成本都按标价乘倍率算。',
    search: '搜索模型标识、展示名、厂商或别名',
    create: '新建模型',
    edit: '编辑模型',
    empty: '目录还是空的，先从价格文件导入或手动新建。',
    noMatch: '没有匹配的条目',
    noneListed: '还没有上架的模型',
    showAll: '看全部模型',
    summaryStats: {
      total: '模型',
      listed: '已上架',
      listedWithoutResources: '上架但无渠道',
      showThem: '筛选'
    },
    filtered: '筛选后 {count} 个',
    aliasCount: '{count} 个别名',
    filters: {
      noVendor: '（无厂商）',
      withResources: '有渠道',
      withoutResources: '无渠道'
    },
    columns: {
      model: '模型',
      price: '标价',
      channels: '承接渠道数',
      status: '状态',
      perMillion: '输入 / 输出 · 每百万 Token',
      perUnit: {
        per_request: '每次',
        image: '每张',
        video: '每秒'
      },
      tiers: '{count} 档',
      unpriced: '未配价'
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
      vendorNone: '（不设厂商）',
      vendorCustom: '其他（手填）…',
      vendorCustomPlaceholder: '厂商标识，如 anthropic',
      modelIdPlaceholder: '如 claude-sonnet-4-5',
      channels: '承接的渠道',
      morePrices: '更多价格',
      morePricesFilled: '已填 {count} 项',
      morePricesHint: '图片、音频（以及按 Token 以外计费时的缓存）的单价；留空即未配置。',
      units: {
        perMillion: '$ / 百万 Token',
        perCall: '$ / 次',
        perImage: '$ / 张',
        perSecond: '$ / 秒'
      },
      lookup: {
        idle: '输入模型 ID 后会从价格文件自动带出厂商、计费方式和价格。',
        editIdle: '可以按价格文件重新带出厂商、计费方式和价格。',
        loading: '正在查价格文件…',
        applied: '已从价格文件带出厂商、计费方式和价格，可以再改。',
        found: '价格文件里有这个模型。',
        apply: '用价格文件的价格',
        missing: '价格文件里没有这个模型，厂商和价格要手填。',
        error: '查价失败，厂商和价格要手填。',
        refill: '按价格文件带价'
      }
    },
    // 新建 / 编辑模型独立页
    formPage: {
      backToList: '模型',
      backToListAction: '返回模型列表',
      loading: '正在加载模型…',
      notFound: '找不到模型 #{id}，可能已被删除。',
      loadFailed: '模型加载失败：{message}',
      retry: '重试',
      saved: '模型已保存'
    },
    // 模型详情抽屉（A5）
    drawer: {
      eyebrow: '模型 #{id}',
      tabs: {
        overview: '概况',
        channels: '渠道'
      },
      unpricedBanner: '已上架但没有配价：用户看不到这个模型。',
      unboundBanner: '已上架但没有绑定渠道：用户调用这个模型会失败。',
      listedDone: '已上架 {model}',
      unlistedDone: '已下架 {model}',
      aliases: '别名',
      notes: '备注',
      updatedAt: '更新时间',
      prices: '标价',
      perMillionHint: '按 Token 计的标价，单位：美元 / 百万 Token',
      price: {
        input: '输入',
        output: '输出',
        cacheWrite: '缓存写入 · 5 分钟',
        cacheWrite1h: '缓存写入 · 1 小时',
        cacheRead: '缓存读取',
        imageInput: '图片输入',
        imageOutput: '图片输出',
        imageCacheRead: '图片缓存读取',
        audioInput: '音频输入',
        audioOutput: '音频输出',
        // 服务档位：priority 档与 Fast 同一档（与用户站「服务档位」同名），别写成「优先级」——渠道页签的「优先级」是调度优先级
        inputPriority: 'Fast 档 · 输入',
        outputPriority: 'Fast 档 · 输出',
        cacheWritePriority: 'Fast 档 · 缓存写入',
        cacheReadPriority: 'Fast 档 · 缓存读取',
        perRequest: '按次',
        per: {
          per_request: '{price} / 次',
          image: '{price} / 张',
          video: '{price} / 秒'
        },
        searchPerCall: '内置搜索',
        longContext: '长上下文 {op} {threshold}',
        listPrice: '标价',
        fast: 'Fast 档倍率',
        flex: 'Flex 档倍率',
        maxReasoning: '最高推理倍率'
      },
      tiers: '分档',
      mediaTiersHint: '命中档位按档位价计，没命中按上面的标价。',
      tokenTier: '{min} – {max} Token',
      tokenTierOpen: '{min} Token 以上',
      timePricing: '分时定价',
      timezone: '时区 {timezone}',
      weekdaysOnly: '仅工作日',
      bind: '去绑定',
      priority: '优先级 {value}',
      notSchedulable: '不可调度',
      channelsHint: '这些渠道此刻能不能调度；各入口协议能不能承接，点「诊断」看。',
      channelsFallback: '渠道状态没取到，下面只列出绑定的渠道。'
    },
    diagnose: '诊断',
    diagnosis: {
      title: '渠道诊断 · {model}',
      empty: '该模型没有绑定渠道。',
      followAccount: '跟随渠道',
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
    seed: '从价格文件导入',
    seeding: '导入中…',
    seedDone: '导入完成：新增 {inserted}，更新 {refreshed}，跳过手动改过的 {skipped}',
    seedPartial: '{summary}；另有 {failed} 条写入失败：{errors}',
    deleteTitle: '删除目录条目',
    deleteConfirm: '删除后别名、分档和分时定价会一起删掉。确定删除？',
    fullReplaceHint: '保存是整条覆盖。本页没列出的项（按 Token 的区间分档、分时定价、Fast 档价、长上下文与倍率）按原值写回；图片 / 视频分档在上方编辑。',
    listedRequiresPrice: '上架的模型必须配好价格，用户才能看到并调用。',
    noResources: '无渠道',
    fields: {
      modelId: '模型标识',
      displayName: '展示名',
      vendor: '厂商',
      billingMode: '计费模式',
      status: '上架状态',
      resources: '承接渠道',
      inputPrice: '输入价',
      outputPrice: '输出价',
      perRequestPrice: '每次标价',
      perImagePrice: '每张标价（分档未命中时用）',
      perSecondPrice: '每秒标价（分档未命中时用）',
      searchPricePerCall: '内置搜索每次调用价（留空用内置单价 0.01）',
      cacheWritePrice: '缓存写入 · 5 分钟',
      cacheWrite1hPrice: '缓存写入 · 1 小时',
      cacheReadPrice: '缓存读取',
      imageInputPrice: '图片输入价',
      imageOutputPrice: '图片输出价',
      imageCacheReadPrice: '图片缓存读取价',
      audioInputPrice: '音频输入价',
      audioOutputPrice: '音频输出价'
    },
    tiers: {
      title: '分档单价',
      hint: {
        image: '按输出尺寸分档，每张价；上架必须填每张标价。',
        video: '按分辨率分档，每秒价；上架必须填每秒标价。'
      },
      tier: '档位',
      price: '单价（$）',
      add: '加一档',
      remove: '移除',
      empty: '未配分档，按标价计。'
    },
    bindings: {
      title: '承接这个模型的渠道',
      hint: '勾上的渠道承接这个模型的请求；优先级留空则跟随渠道自身的优先级。',
      selected: '已选 {count} 个',
      search: '搜索渠道名称或 ID',
      boundOnly: '只看已选',
      loading: '正在加载渠道…',
      loadFailed: '渠道列表加载失败',
      retry: '重试',
      noResults: '没有匹配的渠道',
      noChannels: '还没有渠道，先到「渠道」页添加。',
      inactive: '已停用',
      priority: '优先级',
      priorityFollow: '跟随渠道'
    },
    status: {
      listed: '已上架',
      unlisted: '未上架'
    },
    billingModes: {
      token: '按 Token',
      per_request: '按次',
      image: '按图片',
      video: '按视频'
    }
  }
}
