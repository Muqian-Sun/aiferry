<template>
  <!--
    登录方式标识：微信仓库里没有官方标，用线性图标代替（不手画品牌标）。颜色随父元素（绑定的墨色、未绑定淡灰）。
  -->
  <Icon v-if="provider === 'email'" name="mail" size="sm" />
  <Icon v-else-if="provider === 'wechat'" name="chatBubble" size="sm" />
  <Icon v-else name="globe" size="sm" />
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

defineProps<{ provider: string }>()
</script>
