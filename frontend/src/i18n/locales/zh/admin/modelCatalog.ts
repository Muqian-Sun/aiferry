export default {
  modelCatalog: {
    description: '平台模型的官方价与别名。',
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
      segments: '分 {count} 段',
      unpriced: '未配价'
    },
    bulk: {
      list: '上架',
      unlist: '下架',
      selectEntry: '选择 {model}',
      nothingToDo: '选中的条目已经是目标状态',
      listedDone: '已上架 {count} 个模型',
      unlistedDone: '已下架 {count} 个模型',
      partial: '成功 {done} 个，失败 {failed} 个（失败的仍留在选中集里）：{errors}'
    },
    editor: {
      vendorHint: '用小写厂商标识（anthropic / openai / gemini / xai…），用户站的厂商页签与图标按它匹配。',
      vendorNone: '（不设厂商）',
      vendorCustom: '其他（手填）…',
      vendorCustomPlaceholder: '厂商标识，如 anthropic',
      modelIdPlaceholder: '如 claude-sonnet-4-5',
      units: {
        perMillion: '$ / 百万 Token'
      },
      lookup: {
        loading: '正在查价格文件…'
      }
    },
    // 新建 / 编辑模型弹窗（2026-10-03）：新建两步（模型 → 定价与渠道），编辑一步
    dialog: {
      steps: {
        model: '模型',
        pricing: '定价与渠道'
      },
      next: '下一步：定价与渠道',
      done: '完成',
      lookup: {
        idle: '输入模型标识后，按价格文件带出官方价。',
        found: '价格文件里有：{prices}（每百万 Token），建好后就是官方价，下一步可以改。',
        missing: '价格文件里没有这个模型的按 Token 价，下一步手填官方价。',
        error: '查价失败，下一步手填官方价。'
      },
      createFailed: '模型创建失败',
      exists: '目录里已有这个模型：在列表里编辑或上架它。',
      // 编辑时把标识改成了目录里另一条已有的标识（后端 409 MODEL_CATALOG_ENTRY_EXISTS）
      idTaken: '目录里已有这个模型标识，换一个标识，或去列表里编辑已有的那条。',
      saveFailed: '模型保存失败',
      pricingHint: '这就是价格页里这个模型的那一块：可以改官方价；加一个渠道就是让它承接这个模型。改完点这一块的「保存」。',
      pricingLoading: '正在加载价格…',
      pricingMissing: '价格页里找不到这个模型（只列按 Token 计费的模型）。',
      listing: '上架',
      listingReady: '打开后用户就能看到并调用这个模型。',
      listedHint: '已上架：用户能看到并调用这个模型。',
      listingBlocked: {
        unsaved: '先保存上面这一块再上架。',
        price: '官方价输入、输出还没填齐，不能上架。',
        channel: '还没有渠道承接这个模型，不能上架。'
      },
      listingFailed: '上架状态修改失败',
      listingRule: '上架要求官方价填齐，并且至少有一个渠道承接。',
      unsavedPricing: '定价这一块还有没保存的改动：先保存，或点那一块的「撤销」。',
      pricingElsewhere: '官方价、分段和承接的渠道在价格页改。',
      openPricing: '去价格页 →'
    },
    // 模型详情抽屉（A5）
    drawer: {
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
        perRequest: '按次',
        per: {
          per_request: '{price} / 次',
          image: '{price} / 张',
          video: '{price} / 秒'
        },
        searchPerCall: '内置搜索',
        listPrice: '标价',
        maxReasoning: '最高推理倍率'
      },
      segments: '按 Token 分段',
      segmentsHint: '按单次请求的输入 Token 数（输入 + 缓存写 + 缓存读）落在哪一段，整条请求按那一段的价计费；第一段就是上面的标价。缓存价留空时按本段输入价同比例折算。单位：美元 / 百万 Token',
      segmentColumns: {
        range: '输入 Token',
        input: '输入',
        output: '输出',
        cacheWrite: '缓存写 5 分钟',
        cacheWrite1h: '缓存写 1 小时',
        cacheRead: '缓存读'
      },
      tiers: '分档',
      mediaTiersHint: '命中档位按档位价计，没命中按上面的标价。',
      timePricing: '分时定价',
      timezone: '时区 {timezone}',
      weekdaysOnly: '仅工作日',
      notSchedulable: '不可调度',
      channelsHint: '这些渠道此刻能不能调度；各入口协议能不能承接，点「诊断」看。',
      channelsFallback: '渠道状态没取到，切换一下页签可重试。'
    },
    diagnose: '诊断',
    diagnosis: {
      title: '渠道诊断 · {model}',
      empty: '该模型没有绑定渠道。',
      columns: {
        account: '渠道',
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
    noResources: '无渠道',
    fields: {
      modelId: '模型标识',
      displayName: '展示名',
      vendor: '厂商',
      billingMode: '计费模式',
      status: '上架状态',
      resources: '承接渠道'
    },
    segments: {
      abovePlaceholder: '如 272000',
      errors: {
        required: '填这一段从超过多少 Token 开始。',
        integer: 'Token 数要填正整数。',
        notAscending: 'Token 数要比上一段大。',
        invalidPrice: '这一段有价格格式不对（只能填非负数）。',
        noPrice: '这一段至少填一个价。'
      }
    },
    bindings: {
      title: '承接这个模型的渠道'
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
