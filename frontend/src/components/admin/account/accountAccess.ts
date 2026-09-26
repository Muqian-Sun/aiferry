// 接入方式的叫法（纯函数、只依赖类型）：渠道列表名称行、渠道抽屉副标题（共用 PlatformTypeBadge）都用它。
import type { Account } from '@/types'

/** 接入方式的 i18n key：第三方 key / OAuth 授权 / Setup Token / AWS Bedrock / Vertex 服务账号…… */
export function accountAccessKey(row: Pick<Account, 'platform' | 'type' | 'credentials'>): string {
  const mode = String(row.credentials?.auth_mode || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
  if (row.platform === 'openai' && row.type === 'oauth') {
    if (mode === 'agentidentity') return 'admin.accounts.access.agentIdentity'
    if (mode === 'personalaccesstoken') return 'admin.accounts.access.personalAccessToken'
  }
  switch (row.type) {
    case 'apikey':
      return 'admin.accounts.access.apikey'
    case 'setup-token':
      return 'admin.accounts.access.setupToken'
    case 'bedrock':
      return 'admin.accounts.access.bedrock'
    case 'service_account':
      return 'admin.accounts.access.serviceAccount'
    default:
      return 'admin.accounts.access.oauth'
  }
}
