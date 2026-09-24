<template>
  <!--
    登录公告弹窗（muqian 2026-09-24「登录之后要弹出公告页」「每次登录都默认弹，用户可以选择今日不弹」）：
    列出全部公告（新的在前，正文直接展开），底部「今日不再弹出」+「知道了」。关闭即全部标为已读。
    何时弹由 stores/announcements 的 requestNotice 决定（App.vue 在登录 / 进站时登记）。
  -->
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="store.noticeOpen"
        class="modal-overlay"
        style="z-index: 70"
        role="dialog"
        aria-modal="true"
        :aria-label="t('announcements.title')"
        data-testid="announcement-notice"
        @click.self="close"
      >
        <div class="modal-content max-w-2xl" @click.stop>
          <div class="modal-header">
            <h2 class="modal-title">{{ t('announcements.title') }}</h2>
            <button type="button" class="btn btn-ghost btn-icon btn-sm" :aria-label="t('common.close')" @click="close">
              <Icon name="x" size="sm" />
            </button>
          </div>

          <div class="max-h-[60vh] overflow-y-auto">
            <article v-for="item in items" :key="item.id" class="border-t border-af-hairline px-6 py-5 first:border-t-0">
              <div class="flex items-baseline gap-3">
                <span
                  class="h-1.5 w-1.5 shrink-0 self-center rounded-full"
                  :class="item.read_at ? 'bg-transparent' : 'bg-af-ink'"
                  aria-hidden="true"
                />
                <h3 class="min-w-0 flex-1 text-base font-semibold text-af-ink">{{ item.title }}</h3>
                <time class="shrink-0 text-xs tabular-nums text-af-ink-4" :datetime="item.created_at">{{ formatDateOnly(item.created_at) }}</time>
              </div>
              <div class="markdown-body mt-3 pl-[18px]" v-html="render(item.content)"></div>
            </article>
          </div>

          <div class="modal-footer justify-between">
            <label class="flex cursor-pointer items-center gap-2 text-13 text-af-ink-2">
              <input v-model="snoozeToday" type="checkbox" class="h-4 w-4 rounded border-af-hairline" data-testid="announcement-notice-snooze" />
              {{ t('announcements.snoozeToday') }}
            </label>
            <button type="button" class="btn btn-primary btn-sm" data-testid="announcement-notice-close" @click="close">
              {{ t('announcements.gotIt') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatDateOnly } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const store = useAnnouncementStore()
const snoozeToday = ref(false)

marked.setOptions({ breaks: true, gfm: true })

const items = computed(() =>
  [...store.announcements].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
)

function render(content: string): string {
  if (!content) return ''
  return DOMPurify.sanitize(marked.parse(content) as string)
}

function close() {
  store.closeNotice(snoozeToday.value)
  snoozeToday.value = false
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && store.noticeOpen) close()
}

watch(
  () => store.noticeOpen,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
    if (open) document.addEventListener('keydown', onKeydown)
    else document.removeEventListener('keydown', onKeydown)
  }
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  document.body.style.overflow = ''
})
</script>
