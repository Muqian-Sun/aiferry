import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: del }
}))

import {
  deleteOllamaCloudUsageSession,
  getOllamaCloudUsage,
  refreshOllamaCloudUsage,
  saveOllamaCloudUsageSession
} from '@/api/admin/accounts'

const state = {
  account_id: 7,
  eligible: true,
  configured: true,
  encryption_key_configured: true
}

describe('admin Ollama Cloud usage API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
  })

  it('keeps session configuration write-only and separate from account updates', async () => {
    get.mockResolvedValueOnce({ data: state })
    put.mockResolvedValueOnce({ data: state })
    del.mockResolvedValueOnce({ data: { ...state, configured: false } })
    post.mockResolvedValueOnce({ data: state })

    await expect(getOllamaCloudUsage(7)).resolves.toEqual(state)
    await expect(saveOllamaCloudUsageSession(7, 'wos-session=secret')).resolves.toEqual(state)
    await expect(refreshOllamaCloudUsage(7)).resolves.toEqual(state)
    await expect(deleteOllamaCloudUsageSession(7)).resolves.toMatchObject({ configured: false })

    expect(put).toHaveBeenNthCalledWith(1, '/admin/accounts/7/ollama-cloud-usage/session', { session: 'wos-session=secret' })
    expect(post).toHaveBeenCalledWith('/admin/accounts/7/ollama-cloud-usage/refresh')
    expect(del).toHaveBeenCalledWith('/admin/accounts/7/ollama-cloud-usage/session')
  })
})
