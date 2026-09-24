<template>
  <!--
    列表页骨架（A4 起管理站统一）：数字摘要 → 工具行 → 表格 → 批量条 → 分页。
    表格不再包卡片边框，与用户站控制台一致：表头与页面同色，只靠 hairline 分隔。
  -->
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <!-- 固定区域：数字摘要（StatRow） -->
    <div v-if="$slots.summary" class="layout-section-fixed border-b border-af-hairline pb-5">
      <slot name="summary" />
    </div>

    <!-- 固定区域：操作按钮 -->
    <div v-if="$slots.actions" class="layout-section-fixed">
      <slot name="actions" />
    </div>

    <!-- 固定区域：搜索和过滤器 -->
    <div v-if="$slots.filters" class="layout-section-fixed">
      <slot name="filters" />
    </div>

    <!-- 滚动区域：表格 -->
    <div class="layout-section-scrollable">
      <div class="table-scroll-container">
        <slot name="table" />
      </div>
    </div>

    <!-- 固定区域：批量操作条（选中行时才有内容） -->
    <div v-if="$slots.bulk" class="layout-section-fixed empty:hidden">
      <slot name="bulk" />
    </div>

    <!-- 固定区域：分页器 -->
    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
/* 桌面端：Flexbox 布局，表格区在视口内滚动（表头 sticky、虚拟滚动都依赖这个容器） */
.table-page-layout {
  @apply flex flex-col gap-5;
  /* 减去顶栏 + main 的上下内边距 + 管理站页头（AppLayout 量出来的，没有页头时为 0） */
  height: calc(100vh - var(--af-topbar-h) - 3rem - var(--admin-page-header-h, 0px));
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.layout-section-scrollable {
  @apply flex-1 min-h-0 flex flex-col;
}

/* 表格滚动容器 - 增强版表体滚动方案 */
.table-scroll-container {
  @apply flex flex-col overflow-hidden h-full bg-af-sheet;
}

.table-scroll-container :deep(.table-wrapper) {
  /* 表头与页面同色（DataTable 的 table-head-plain 同一口径），吸顶时仍有不透明底 */
  --table-head-bg: var(--af-sheet);
  @apply flex-1 overflow-x-auto overflow-y-auto;
  /* 确保横向滚动条显示在最底部 */
  scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content; /* 关键：确保表格宽度根据内容撑开，从而触发横向滚动 */
  display: table; /* 使用标准 table 布局以支持 sticky 列 */
}

.table-scroll-container :deep(thead) {
  @apply bg-af-sheet;
}

.table-scroll-container :deep(tbody) {
  /* 保持默认 table-row-group 显示，不使用 block */
}

.table-scroll-container :deep(th) {
  @apply px-4 py-2.5 text-left text-13 font-normal text-af-ink-3 border-b border-af-hairline;
}

.table-scroll-container :deep(td) {
  @apply px-4 py-3 text-sm text-af-ink-2 border-b border-af-hairline;
}

/* 移动端：恢复正常滚动 */
.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible border-none bg-transparent;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply flex-none min-h-fit;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
