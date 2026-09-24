import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { announcementsAPI } from '@/api'
import type { UserAnnouncement } from '@/types'

const THROTTLE_MS = 20 * 60 * 1000 // 20 minutes

// 登录公告弹窗（muqian 2026-09-24「每次登录都默认弹，用户可以选择今日不弹」）：
// 「今日不再弹出」按用户记在本机（localStorage，值为本地日期）；本标签页弹过一次就不再弹（sessionStorage），刷新页面不重复弹。
// 两者都只是本机便利：存储不可用时照常弹，不报错。
const snoozeKey = (userId: number) => `aiferry:announcement-notice:snoozed-on:${userId}`
const shownKey = (userId: number) => `aiferry:announcement-notice:shown:${userId}`

function readStorage(storage: () => Storage, key: string): string | null {
  try {
    return storage().getItem(key)
  } catch {
    return null
  }
}

function writeStorage(storage: () => Storage, key: string, value: string) {
  try {
    storage().setItem(key, value)
  } catch {
    // 私密窗口等存储不可用时忽略：下次照常弹
  }
}

function localDate(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}

export const useAnnouncementStore = defineStore('announcements', () => {
  // State
  const announcements = ref<UserAnnouncement[]>([])
  const loading = ref(false)
  const lastFetchTime = ref(0)
  const loaded = ref(false)
  const noticeOpen = ref(false)

  // 等公告取回后要弹窗的用户（登录 / 进站时登记）
  let noticeRequestedFor: number | null = null
  let noticeUserId: number | null = null

  // Getters
  const unreadCount = computed(() =>
    announcements.value.filter((a) => !a.read_at).length
  )

  // Actions
  async function fetchAnnouncements(force = false) {
    const now = Date.now()
    if (!force && lastFetchTime.value > 0 && now - lastFetchTime.value < THROTTLE_MS) {
      return
    }

    // Set immediately to prevent concurrent duplicate requests
    lastFetchTime.value = now

    try {
      loading.value = true
      const all = await announcementsAPI.list(false)
      announcements.value = all.slice(0, 20)
      loaded.value = true
      openRequestedNotice()
    } catch (err: any) {
      // Revert throttle timestamp on failure so retry is allowed
      lastFetchTime.value = 0
      console.error('Failed to fetch announcements:', err)
    } finally {
      loading.value = false
    }
  }

  /**
   * 登录或进站时登记一次弹窗。今天选过「今日不再弹出」就不登记；
   * freshLogin=false（刷新页面恢复登录态）时，本标签页已经弹过也不登记。
   */
  function requestNotice(userId: number, freshLogin: boolean) {
    if (readStorage(() => localStorage, snoozeKey(userId)) === localDate()) return
    if (!freshLogin && readStorage(() => sessionStorage, shownKey(userId))) return
    noticeRequestedFor = userId
    openRequestedNotice()
  }

  function openRequestedNotice() {
    if (noticeRequestedFor === null || !loaded.value) return
    const userId = noticeRequestedFor
    noticeRequestedFor = null
    if (announcements.value.length === 0) return
    noticeUserId = userId
    noticeOpen.value = true
    writeStorage(() => sessionStorage, shownKey(userId), '1')
  }

  /** 关弹窗：全部标为已读；勾了「今日不再弹出」就记下今天的日期 */
  function closeNotice(snoozeToday: boolean) {
    noticeOpen.value = false
    if (snoozeToday && noticeUserId !== null) {
      writeStorage(() => localStorage, snoozeKey(noticeUserId), localDate())
    }
    void markAllAsRead().catch(() => undefined)
  }

  async function markAsRead(id: number) {
    try {
      await announcementsAPI.markRead(id)
      const ann = announcements.value.find((a) => a.id === id)
      if (ann) {
        ann.read_at = new Date().toISOString()
      }
    } catch (err: any) {
      console.error('Failed to mark announcement as read:', err)
    }
  }

  async function markAllAsRead() {
    const unread = announcements.value.filter((a) => !a.read_at)
    if (unread.length === 0) return

    try {
      loading.value = true
      await Promise.all(unread.map((a) => announcementsAPI.markRead(a.id)))
      announcements.value.forEach((a) => {
        if (!a.read_at) {
          a.read_at = new Date().toISOString()
        }
      })
    } catch (err: any) {
      console.error('Failed to mark all as read:', err)
      throw err
    } finally {
      loading.value = false
    }
  }

  function reset() {
    announcements.value = []
    lastFetchTime.value = 0
    loaded.value = false
    noticeOpen.value = false
    noticeRequestedFor = null
    noticeUserId = null
    loading.value = false
  }

  return {
    // State
    announcements,
    loading,
    noticeOpen,
    // Getters
    unreadCount,
    // Actions
    fetchAnnouncements,
    requestNotice,
    closeNotice,
    markAsRead,
    markAllAsRead,
    reset,
  }
})
