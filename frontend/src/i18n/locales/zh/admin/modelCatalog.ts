export default {
  modelCatalog: {
    title: '模型目录',
    description: '平台模型的基准价与别名，计价从这里取。',
    search: '搜索模型标识、展示名或厂商',
    create: '新建模型',
    edit: '编辑模型',
    empty: '目录还是空的，先播种或手动新建。',
    diagnose: '诊断',
    diagnosis: {
      title: '资源诊断 · {model}',
      empty: '该模型没有绑定资源。',
      followAccount: '跟随账号',
      columns: {
        account: '资源',
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
    fullReplaceHint: '保存是整条覆盖。按 Token 的区间分档和分时定价按原值写回，本页暂不编辑；图片 / 视频分档在上方编辑。',
    listedRequiresPrice: '上架的模型必须配好价格，用户才能看到并调用。',
    noResources: '无资源',
    fields: {
      modelId: '模型标识',
      displayName: '展示名',
      vendor: '厂商',
      billingMode: '计费模式',
      status: '上架状态',
      managedBy: '维护方',
      resources: '资源',
      inputPrice: '输入价（$/token）',
      outputPrice: '输出价（$/token）',
      perRequestPrice: '每次默认价（$）',
      perImagePrice: '每张默认价（$，分档未命中时用）',
      perSecondPrice: '每秒默认价（$，分档未命中时用）',
      searchPricePerCall: '内置搜索每次调用价（$，留空用内置单价 0.01）'
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
      title: '绑定资源',
      hint: '上架后由这些资源承接请求；优先级留空则跟随资源自身的优先级。',
      search: '搜索资源名称',
      noResults: '没有匹配的资源',
      add: '添加',
      priority: '优先级',
      remove: '移除',
      empty: '尚未绑定资源'
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
