<template>
  <!-- 列表工具行（A4）：左边搜索 + 筛选标签，右边刷新 / 列设置等小图标。整行 32px 高，窄屏自动换行。 -->
  <div class="flex flex-wrap items-center gap-2" data-testid="list-toolbar">
    <slot />
    <div v-if="$slots.end" class="ml-auto flex items-center gap-0.5">
      <slot name="end" />
    </div>
  </div>
</template>
