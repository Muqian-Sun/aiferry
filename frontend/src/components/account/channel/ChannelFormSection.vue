<template>
  <!-- 渠道弹窗的一个分区（上游 / 调度与限额 / 备注）：分区之间一条 hairline，第一个分区不画线 -->
  <section
    class="space-y-4 border-t border-af-hairline pt-5 first:border-t-0 first:pt-0"
    :data-testid="`channel-section-${section}`"
  >
    <h3 class="text-[15px] font-semibold leading-6 text-af-ink">{{ title }}</h3>
    <slot />
  </section>
</template>

<script setup lang="ts">
defineProps<{
  /** 分区 id：upstream / scheduling / notes */
  section: string
  title: string
}>()
</script>
