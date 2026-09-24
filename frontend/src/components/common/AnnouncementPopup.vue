<template>
  <!-- 管理端公告预览（用户站登录后的公告弹窗是 components/user/AnnouncementNotice） -->
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="displayedAnnouncement" class="modal-overlay" style="z-index: 70" role="dialog" aria-modal="true">
        <div class="modal-content max-w-2xl" @click.stop>
          <div class="modal-header items-start">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="badge badge-gray">{{ t('announcements.title') }}</span>
              </div>
              <h2 class="mt-2 text-xl font-semibold leading-tight text-af-ink">{{ displayedAnnouncement.title }}</h2>
              <p class="mt-1.5 text-13 text-af-ink-3">
                <time>{{ formatRelativeWithDateTime(displayedAnnouncement.created_at) }}</time>
              </p>
            </div>
          </div>

          <div class="modal-body max-h-[50vh]">
            <div class="markdown-body" v-html="renderedContent"></div>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-primary btn-sm" data-testid="announcement-popup-dismiss" @click="handleDismiss">
              {{ t('common.close') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { formatRelativeWithDateTime } from '@/utils/format'
import type { Announcement, UserAnnouncement } from '@/types'
import '@/styles/announcement-markdown.css'

type PreviewAnnouncement = Pick<Announcement | UserAnnouncement, 'title' | 'content' | 'created_at'>

const props = withDefaults(defineProps<{
  announcement?: PreviewAnnouncement | null
}>(), {
  announcement: null,
})

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const displayedAnnouncement = computed(() => props.announcement)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const renderedContent = computed(() => {
  const content = displayedAnnouncement.value?.content
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

function handleDismiss() {
  emit('close')
}

watch(
  displayedAnnouncement,
  (popup) => {
    document.body.style.overflow = popup ? 'hidden' : ''
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  document.body.style.overflow = ''
})
</script>
