<template>
  <div>
    <!-- 铃铛按钮：未读时品牌色 + 小圆点，不做 ping 动画 -->
    <button
      type="button"
      class="relative flex h-9 w-9 items-center justify-center rounded-lg transition-colors hover:bg-af-sunken"
      :class="unreadCount > 0 ? 'text-af-brand' : 'text-af-ink-2 hover:text-af-ink'"
      :aria-label="t('announcements.title')"
      data-testid="announcement-bell"
      @click="openModal"
    >
      <Icon name="bell" size="md" />
      <span v-if="unreadCount > 0" class="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-af-danger" aria-hidden="true"></span>
    </button>

    <!-- 公告列表 -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="isModalOpen" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeModal">
          <div class="modal-content max-w-xl" @click.stop>
            <div class="modal-header">
              <div class="min-w-0">
                <h2 class="modal-title">{{ t('announcements.title') }}</h2>
                <p v-if="unreadCount > 0" class="mt-0.5 text-13 text-af-ink-3">
                  <span class="font-medium text-af-brand">{{ unreadCount }}</span> {{ t('announcements.unread') }}
                </p>
              </div>
              <div class="flex items-center gap-2">
                <button v-if="unreadCount > 0" type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="markAllAsRead">
                  {{ t('announcements.markAllRead') }}
                </button>
                <button type="button" class="btn btn-ghost btn-icon btn-sm" :aria-label="t('common.close')" @click="closeModal">
                  <Icon name="x" size="sm" />
                </button>
              </div>
            </div>

            <div class="max-h-[65vh] flex-1 overflow-y-auto">
              <div v-if="loading" class="flex items-center justify-center py-16">
                <span class="spinner h-6 w-6 text-af-ink-3"></span>
              </div>

              <ul v-else-if="announcements.length > 0" class="divide-y divide-af-hairline">
                <li v-for="item in announcements" :key="item.id">
                  <button
                    type="button"
                    class="flex w-full items-center gap-3 px-6 py-4 text-left transition-colors hover:bg-af-sunken"
                    @click="openDetail(item)"
                  >
                    <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="item.read_at ? 'bg-transparent' : 'bg-af-brand'" aria-hidden="true"></span>
                    <span class="min-w-0 flex-1">
                      <span class="block truncate text-sm font-medium text-af-ink">{{ item.title }}</span>
                      <span class="mt-0.5 flex items-center gap-2 text-xs text-af-ink-3">
                        <time>{{ formatRelativeTime(item.created_at) }}</time>
                        <span v-if="!item.read_at" class="badge badge-primary">{{ t('announcements.unread') }}</span>
                      </span>
                    </span>
                    <Icon name="chevronRight" size="sm" class="shrink-0 text-af-ink-4" />
                  </button>
                </li>
              </ul>

              <div v-else class="flex flex-col items-center justify-center py-16 text-center">
                <Icon name="inbox" size="lg" class="text-af-ink-4" />
                <p class="mt-3 text-sm font-medium text-af-ink">{{ t('announcements.empty') }}</p>
                <p class="mt-1 text-xs text-af-ink-3">{{ t('announcements.emptyDescription') }}</p>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 公告详情 -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="detailModalOpen && selectedAnnouncement"
          class="modal-overlay"
          style="z-index: 60"
          role="dialog"
          aria-modal="true"
          @click.self="closeDetail"
        >
          <div class="modal-content max-w-3xl" @click.stop>
            <div class="modal-header items-start">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="badge badge-gray">{{ t('announcements.title') }}</span>
                  <span v-if="!selectedAnnouncement.read_at" class="badge badge-primary">{{ t('announcements.unread') }}</span>
                </div>
                <h2 class="mt-2 text-xl font-semibold leading-tight text-af-ink">{{ selectedAnnouncement.title }}</h2>
                <p class="mt-1.5 text-13 text-af-ink-3">
                  <time>{{ formatRelativeWithDateTime(selectedAnnouncement.created_at) }}</time>
                  · {{ selectedAnnouncement.read_at ? t('announcements.read') : t('announcements.unread') }}
                </p>
              </div>
              <button type="button" class="btn btn-ghost btn-icon btn-sm shrink-0" :aria-label="t('common.close')" @click="closeDetail">
                <Icon name="x" size="md" />
              </button>
            </div>

            <div class="modal-body max-h-[60vh]">
              <div class="markdown-body" v-html="renderMarkdown(selectedAnnouncement.content)"></div>
            </div>

            <div class="modal-footer justify-between">
              <span class="text-xs text-af-ink-3">
                {{ selectedAnnouncement.read_at ? t('announcements.readStatus') : t('announcements.markReadHint') }}
              </span>
              <div class="flex items-center gap-3">
                <button type="button" class="btn btn-secondary btn-sm" @click="closeDetail">{{ t('common.close') }}</button>
                <button v-if="!selectedAnnouncement.read_at" type="button" class="btn btn-primary btn-sm" @click="markAsReadAndClose(selectedAnnouncement.id)">
                  {{ t('announcements.markRead') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAppStore } from '@/stores/app'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeTime, formatRelativeWithDateTime } from '@/utils/format'
import type { UserAnnouncement } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const appStore = useAppStore()
const announcementStore = useAnnouncementStore()

// Configure marked
marked.setOptions({
  breaks: true,
  gfm: true,
})

// Use store state (storeToRefs for reactivity)
const { announcements, loading } = storeToRefs(announcementStore)
const unreadCount = computed(() => announcementStore.unreadCount)

// Local modal state
const isModalOpen = ref(false)
const detailModalOpen = ref(false)
const selectedAnnouncement = ref<UserAnnouncement | null>(null)

// Methods
function renderMarkdown(content: string): string {
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
}

function openModal() {
  isModalOpen.value = true
}

function closeModal() {
  isModalOpen.value = false
}

function openDetail(announcement: UserAnnouncement) {
  selectedAnnouncement.value = announcement
  detailModalOpen.value = true
  if (!announcement.read_at) {
    markAsRead(announcement.id)
  }
}

function closeDetail() {
  detailModalOpen.value = false
  selectedAnnouncement.value = null
}

async function markAsRead(id: number) {
  try {
    await announcementStore.markAsRead(id)
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

async function markAsReadAndClose(id: number) {
  await markAsRead(id)
  appStore.showSuccess(t('announcements.markedAsRead'))
  closeDetail()
}

async function markAllAsRead() {
  try {
    await announcementStore.markAllAsRead()
    appStore.showSuccess(t('announcements.allMarkedAsRead'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

function handleEscape(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (detailModalOpen.value) {
      closeDetail()
    } else if (isModalOpen.value) {
      closeModal()
    }
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
  document.body.style.overflow = ''
})

watch(
  [isModalOpen, detailModalOpen, () => announcementStore.noticeOpen],
  ([modal, detail, popup]) => {
    document.body.style.overflow = (modal || detail || popup) ? 'hidden' : ''
  }
)
</script>
