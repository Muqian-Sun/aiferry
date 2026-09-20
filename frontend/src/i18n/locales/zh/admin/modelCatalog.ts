export default {
  modelCatalog: {
    title: '模型目录',
    description: '平台模型的基准价与别名，计价从这里取。',
    search: '搜索模型标识、展示名或厂商',
    create: '新建模型',
    edit: '编辑模型',
    empty: '目录还是空的，先播种或手动新建。',
    seed: '从价格文件播种',
    seeding: '播种中…',
    seedDone: '播种完成：新增 {inserted}，刷新 {refreshed}，跳过管理员改过的 {skipped}',
    seedPartial: '{summary}；另有 {failed} 条写入失败：{errors}',
    deleteTitle: '删除目录条目',
    deleteConfirm: '删除后别名、分档和分时定价会一起删掉。确定删除？',
    fullReplaceHint: '保存是整条覆盖。分档和分时定价按原值写回，本页暂不编辑。',
    fields: {
      modelId: '模型标识',
      displayName: '展示名',
      vendor: '厂商',
      billingMode: '计费模式',
      status: '上架状态',
      managedBy: '维护方',
      inputPrice: '输入价（$/token）',
      outputPrice: '输出价（$/token）'
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
