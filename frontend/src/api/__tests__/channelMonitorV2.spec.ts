import { afterEach, describe, expect, it, vi } from 'vitest'

// 管理端接口前缀只在管理后台构建中可选，按用例切换站点
const site = vi.hoisted(() => ({ admin: false }))
vi.mock('@/app/site', () => ({
  get IS_ADMIN_SITE() { return site.admin },
  get APP_SITE() { return site.admin ? 'admin' : 'user' },
}))

import { apiClient } from '../client'
import { getMatrix, repeatedArrayParamsSerializer } from '../channelMonitorV2'

afterEach(() => {
  vi.restoreAllMocks()
  site.admin = false
})

describe('channel monitor V2 query serialization', () => {
  it('uses repeated keys without bracket suffixes for array filters', () => {
    const query = repeatedArrayParamsSerializer({
      range: '90m',
      platform: ['openai', 'grok'],
      group_id: [1, 2],
      model: undefined,
      group_by: 'platform_group_model',
    })

    expect(query).toBe('range=90m&platform=openai&platform=grok&group_id=1&group_id=2&group_by=platform_group_model')
    expect(query).not.toContain('%5B%5D')
  })

  it('never reaches the admin monitor API from the user site', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { coverage: {}, group_by: 'platform_group', items: [] },
    })

    await getMatrix({ range: '24h', platforms: [], groupIds: [], models: [] }, 'platform_group', true)

    expect(get).toHaveBeenCalledWith('/channel-monitor-v2/matrix', expect.anything())
  })

  it('sends the matrix grouping with the shared filters', async () => {
    site.admin = true
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { coverage: {}, group_by: 'platform_group', items: [] },
    })

    await getMatrix({ range: '24h', platforms: ['openai'], groupIds: [7], models: [] }, 'platform_group', true)

    expect(get).toHaveBeenCalledWith('/admin/channel-monitor-v2/matrix', expect.objectContaining({
      params: {
        range: '24h',
        platform: ['openai'],
        group_id: [7],
        model: undefined,
        group_by: 'platform_group',
      },
    }))
  })
})
