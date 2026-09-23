<template>
  <!--
    设置类页面的一行（muqian 2026-09-23 定「两栏设置式」）：左边标题 + 一句说明（约 1/3），右边表单或内容。
    行与行之间的细线由父容器 divide-y 画；上下留白固定 py-8，页面容器用 -mt-8 抵掉第一行的顶部留白。
    窄屏上下叠放。右栏限宽，输入框不会拉满整行。
  -->
  <section class="grid gap-x-12 gap-y-4 py-8 lg:grid-cols-[minmax(0,17rem)_minmax(0,1fr)]" :aria-labelledby="headingId">
    <div class="min-w-0">
      <h2 :id="headingId" class="text-sm font-semibold text-af-ink">{{ title }}</h2>
      <p v-if="description" class="mt-1 text-13 leading-5 text-af-ink-3">{{ description }}</p>
      <slot name="aside" />
    </div>
    <div class="min-w-0 max-w-2xl">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
let counter = 0
defineProps<{ title: string; description?: string }>()
const headingId = `settings-row-${++counter}`
</script>
