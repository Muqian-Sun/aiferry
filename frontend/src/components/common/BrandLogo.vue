<template>
  <!--
    站点 logo：后台配置了自定义 logo 就用图片；否则画内置的 AiFerry 标——无框的摆渡船（驾驶舱 / 三扇舷窗 / 船身 / 浪线 + 右上角星芒），
    用 currentColor 画，颜色由使用处的文字色决定，深浅主题自动跟随（muqian 2026-09-23：logo 不要框、不要彩色）。
    与 public/logo.svg（favicon）是同一套路径，改一处要同步另一处。
  -->
  <img v-if="src" :src="src" :alt="alt" class="object-contain" />
  <svg
    v-else
    viewBox="7 3 34 38"
    fill="currentColor"
    fill-rule="evenodd"
    xmlns="http://www.w3.org/2000/svg"
    :role="alt ? 'img' : undefined"
    :aria-label="alt || undefined"
    :aria-hidden="alt ? undefined : 'true'"
    data-testid="brand-logo-mark"
  >
    <path d="M35 5l1.05 2.95L39 9l-2.95 1.05L35 13l-1.05-2.95L31 9l2.95-1.05L35 5Z" />
    <path
      d="M20 11H26.5A1.5 1.5 0 0 1 28 12.5V14.5A1.5 1.5 0 0 1 26.5 16H20A1.5 1.5 0 0 1 18.5 14.5V12.5A1.5 1.5 0 0 1 20 11ZM21.6 12.5H24.9A.6 .6 0 0 1 25.5 13.1V13.9A.6 .6 0 0 1 24.9 14.5H21.6A.6 .6 0 0 1 21 13.9V13.1A.6 .6 0 0 1 21.6 12.5Z"
    />
    <path
      d="M14.8 17H33.2A1.8 1.8 0 0 1 35 18.8V22.2A1.8 1.8 0 0 1 33.2 24H14.8A1.8 1.8 0 0 1 13 22.2V18.8A1.8 1.8 0 0 1 14.8 17ZM16.7 19.25H18.7A.7 .7 0 0 1 19.4 19.95V21.05A.7 .7 0 0 1 18.7 21.75H16.7A.7 .7 0 0 1 16 21.05V19.95A.7 .7 0 0 1 16.7 19.25ZM23 19.25H25A.7 .7 0 0 1 25.7 19.95V21.05A.7 .7 0 0 1 25 21.75H23A.7 .7 0 0 1 22.3 21.05V19.95A.7 .7 0 0 1 23 19.25ZM29.3 19.25H31.3A.7 .7 0 0 1 32 19.95V21.05A.7 .7 0 0 1 31.3 21.75H29.3A.7 .7 0 0 1 28.6 21.05V19.95A.7 .7 0 0 1 29.3 19.25Z"
    />
    <path d="M8.9 25.5h30.2c.8 0 1.3.85.95 1.55l-2.5 4.9A2 2 0 0 1 35.8 33H12.2a2 2 0 0 1-1.75-1.05l-2.5-4.9A1.05 1.05 0 0 1 8.9 25.5Z" />
    <path
      d="M8.5 38c2.6-1.9 5.2-1.9 7.8 0s5.2 1.9 7.8 0 5.2-1.9 7.8 0 5.2 1.9 7.8 0"
      fill="none"
      stroke="currentColor"
      stroke-width="2.4"
      stroke-linecap="round"
      stroke-opacity=".5"
    />
  </svg>
</template>

<script setup lang="ts">
defineProps<{
  /** 后台配置的自定义 logo 地址（已清洗）；空则画内置标 */
  src?: string
  /** 替代文字；空表示纯装饰 */
  alt?: string
}>()
</script>
