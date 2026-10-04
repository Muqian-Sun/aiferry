<template>
  <div
    class="flex items-center justify-between border-t border-af-hairline bg-af-sheet px-4 py-3 sm:px-6"
  >
    <div class="flex flex-1 items-center justify-between sm:hidden">
      <!-- Mobile pagination -->
      <button
        @click="goToPage(page - 1)"
        :disabled="page === 1"
        :class="PLAIN_STEP"
      >
        {{ t('pagination.previous') }}
      </button>
      <span class="text-sm text-af-ink-2">
        {{ t('pagination.pageOf', { page, total: totalPages }) }}
      </span>
      <button
        @click="goToPage(page + 1)"
        :disabled="page === totalPages"
        :class="PLAIN_STEP"
      >
        {{ t('pagination.next') }}
      </button>
    </div>

    <div class="hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
      <!-- Desktop pagination info -->
      <div class="flex items-center space-x-4">
        <!-- 只有一页时只写总数；多页写「第 1–20 条，共 N 条」 -->
        <p class="text-sm tabular-nums text-af-ink-3">
          {{ totalPages <= 1 ? t('pagination.totalOnly', { total }) : t('pagination.range', { from: fromItem, to: toItem, total }) }}
        </p>

        <!-- 每页条数：小号文字按钮 + 菜单，不放大框下拉（2026-10-05 走查） -->
        <PopoverMenu v-if="showPageSizeSelector" align="start" width-class="w-32">
          <template #trigger="{ open }">
            <button
              type="button"
              class="inline-flex h-8 items-center gap-1 rounded-md px-2 text-sm tabular-nums text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
              :class="open ? 'bg-af-sunken text-af-ink' : ''"
              data-testid="pagination-page-size"
            >
              {{ t('pagination.perPageCount', { size: pageSize }) }}
              <Icon name="chevronDown" size="xs" />
            </button>
          </template>
          <MenuItem
            v-for="option in pageSizeSelectOptions"
            :key="option.value"
            :checked="option.value === pageSize"
            @click="handlePageSizeChange(option.value)"
          >
            {{ t('pagination.perPageCount', { size: option.value }) }}
          </MenuItem>
        </PopoverMenu>

        <div v-if="showJump" class="flex items-center space-x-2">
          <span class="text-sm text-af-ink-2">{{ t('pagination.jumpTo') }}</span>
          <input
            v-model="jumpPage"
            type="number"
            min="1"
            :max="totalPages"
            class="input w-20 text-sm"
            :placeholder="t('pagination.jumpPlaceholder')"
            @keyup.enter="submitJump"
          />
          <button type="button" class="btn btn-ghost btn-sm" @click="submitJump">
            {{ t('pagination.jumpAction') }}
          </button>
        </div>
      </div>

      <!-- Desktop pagination buttons -->
      <!-- 只有一页时不画「‹ 1 ›」 -->
      <nav
        v-if="totalPages > 1"
        class="inline-flex items-center gap-0.5"
        :aria-label="t('common.pagination')"
      >
        <!-- Previous button -->
        <button
          @click="goToPage(page - 1)"
          :disabled="page === 1"
          :class="PLAIN_ARROW"
          :aria-label="t('pagination.previous')"
        >
          <Icon name="chevronLeft" size="md" />
        </button>

        <!-- Page numbers -->
        <button
          v-for="(pageNum, index) in visiblePages"
          :key="`${pageNum}-${index}`"
          @click="typeof pageNum === 'number' && goToPage(pageNum)"
          :disabled="typeof pageNum !== 'number'"
          :class="[
            'inline-flex h-8 min-w-8 items-center justify-center px-2 text-sm tabular-nums transition-colors',
            pageNum === page
              ? 'font-semibold text-af-ink underline decoration-2 underline-offset-[6px]'
              : 'text-af-ink-3 hover:text-af-ink',
            typeof pageNum !== 'number' && 'cursor-default'
          ]"
          :aria-label="
            typeof pageNum === 'number' ? t('pagination.goToPage', { page: pageNum }) : undefined
          "
          :aria-current="pageNum === page ? 'page' : undefined"
        >
          {{ pageNum }}
        </button>

        <!-- Next button -->
        <button
          @click="goToPage(page + 1)"
          :disabled="page === totalPages"
          :class="PLAIN_ARROW"
          :aria-label="t('pagination.next')"
        >
          <Icon name="chevronRight" size="md" />
        </button>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PopoverMenu from './PopoverMenu.vue'
import MenuItem from './MenuItem.vue'
import { getConfiguredTablePageSizeOptions, normalizeTablePageSize } from '@/utils/tablePreferences'
import { setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()

/**
 * 分页不画边框（控制台单色为主、不要方块，muqian 2026-09-23；管理站 A4 起同一套）：页码是纯数字，
 * 当前页墨色加粗 + 下划线，箭头无框。
 */
const PLAIN_ARROW = 'inline-flex h-8 w-8 items-center justify-center text-af-ink-3 transition-colors hover:text-af-ink disabled:cursor-not-allowed disabled:opacity-40'
const PLAIN_STEP = 'inline-flex items-center text-sm font-medium text-af-ink-2 transition-colors hover:text-af-ink disabled:cursor-not-allowed disabled:opacity-40'

interface Props {
  total: number
  page: number
  pageSize: number
  pageSizeOptions?: number[]
  showPageSizeSelector?: boolean
  showJump?: boolean
}

interface Emits {
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', pageSize: number): void
}

const props = withDefaults(defineProps<Props>(), {
  pageSizeOptions: () => getConfiguredTablePageSizeOptions(),
  showPageSizeSelector: true,
  showJump: false
})

const emit = defineEmits<Emits>()

const totalPages = computed(() => Math.ceil(props.total / props.pageSize))

const fromItem = computed(() => {
  if (props.total === 0) return 0
  return (props.page - 1) * props.pageSize + 1
})

const toItem = computed(() => {
  const to = props.page * props.pageSize
  return to > props.total ? props.total : to
})

const pageSizeSelectOptions = computed(() => {
  const options = Array.from(
    new Set([
      ...getConfiguredTablePageSizeOptions(),
      normalizeTablePageSize(props.pageSize)
    ])
  ).sort((a, b) => a - b)

  return options.map((size) => ({
    value: size,
    label: String(size)
  }))
})

const jumpPage = ref('')

const visiblePages = computed(() => {
  const pages: (number | string)[] = []
  const maxVisible = 7
  const total = totalPages.value

  if (total <= maxVisible) {
    // Show all pages if total is small
    for (let i = 1; i <= total; i++) {
      pages.push(i)
    }
  } else {
    // Always show first page
    pages.push(1)

    const start = Math.max(2, props.page - 2)
    const end = Math.min(total - 1, props.page + 2)

    // Add ellipsis before if needed
    if (start > 2) {
      pages.push('...')
    }

    // Add middle pages
    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    // Add ellipsis after if needed
    if (end < total - 1) {
      pages.push('...')
    }

    // Always show last page
    pages.push(total)
  }

  return pages
})

const goToPage = (newPage: number) => {
  if (newPage >= 1 && newPage <= totalPages.value && newPage !== props.page) {
    emit('update:page', newPage)
  }
}

const handlePageSizeChange = (value: string | number | boolean | null) => {
  if (value === null || typeof value === 'boolean') return
  const newPageSize = normalizeTablePageSize(typeof value === 'string' ? parseInt(value, 10) : value)
  setPersistedPageSize(newPageSize)
  emit('update:pageSize', newPageSize)
}

const submitJump = () => {
  const value = jumpPage.value.trim()
  if (!value) return
  const pageNum = Number.parseInt(value, 10)
  if (Number.isNaN(pageNum)) return
  const nextPage = Math.min(Math.max(pageNum, 1), totalPages.value)
  jumpPage.value = ''
  goToPage(nextPage)
}
</script>
