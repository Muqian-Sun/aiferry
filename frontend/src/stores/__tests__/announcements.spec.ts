import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAnnouncementStore } from '@/stores/announcements'

const list = vi.fn()
const markRead = vi.fn()

vi.mock('@/api', () => ({
  announcementsAPI: {
    list: (...args: unknown[]) => list(...args),
    markRead: (...args: unknown[]) => markRead(...args),
  },
}))

const announcement = {
  id: 1,
  title: '维护',
  content: '今晚升级',
  created_at: '2026-10-16T00:00:00Z',
  updated_at: '2026-10-16T00:00:00Z',
}

describe('useAnnouncementStore notice', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    list.mockReset()
    markRead.mockReset()
    list.mockResolvedValue([announcement])
    markRead.mockResolvedValue({ message: 'ok' })
  })

  it('holds the notice until the user is in the console', async () => {
    const store = useAnnouncementStore()

    store.requestNotice(7, true)
    await store.fetchAnnouncements(true)

    expect(store.noticeOpen).toBe(false)

    store.setInConsole(true)
    expect(store.noticeOpen).toBe(true)
  })

  it('hides the notice outside the console and shows it again on return', async () => {
    const store = useAnnouncementStore()
    store.setInConsole(true)
    store.requestNotice(7, true)
    await store.fetchAnnouncements(true)
    expect(store.noticeOpen).toBe(true)

    store.setInConsole(false)
    expect(store.noticeOpen).toBe(false)

    store.setInConsole(true)
    expect(store.noticeOpen).toBe(true)
  })

  it('does not reopen after the user dismisses it', async () => {
    const store = useAnnouncementStore()
    store.setInConsole(true)
    store.requestNotice(7, true)
    await store.fetchAnnouncements(true)

    store.closeNotice(false)
    store.setInConsole(false)
    store.setInConsole(true)

    expect(store.noticeOpen).toBe(false)
  })
})
