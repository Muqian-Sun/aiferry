<template>
  <!--
    表单分区标题（A5-c）：同时是 FormPageShell 左侧导航的锚点（data-form-section / data-form-section-label）。
    弹窗里也照样渲染。分区之间一条 hairline；页面上第一个分区不画线。
    紧跟标题的那一块自带的上边线 / 上内边距去掉，免得标题下面再叠一条线。
    scroll-mt 把管理站顶栏（sticky，--af-topbar-h）的高度让出来，点导航跳过来时标题不会压在顶栏下面；
    窄屏下分节导航也吸在顶栏下（h-10），再多让 2.5rem。
  -->
  <h2
    :data-form-section="section"
    :data-form-section-label="title"
    class="scroll-mt-[calc(var(--af-topbar-h)+3.5rem)] lg:scroll-mt-[calc(var(--af-topbar-h)+1rem)] border-t border-af-hairline pt-6 text-[15px] font-semibold leading-6 text-af-ink first:border-t-0 first:pt-0 [&+*]:border-t-0 [&+*]:pt-0 [&:not(:first-child)]:!mt-8"
  >
    {{ title }}
  </h2>
</template>

<script setup lang="ts">
defineProps<{
  /** 分区 id，导航用它定位 */
  section: string
  /** 分区名，同时是导航文案 */
  title: string
}>()
</script>
