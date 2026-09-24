<template>
  <div class="flex flex-wrap gap-2">
    <button
      v-for="preset in presets"
      :key="preset.mode + ':' + preset.protocol + ':' + preset.url"
      type="button"
      data-testid="cn-base-url-preset"
      :class="[
        'rounded-lg px-3 py-1 text-xs transition-colors',
        isActive(preset)
          ? 'bg-af-brand-tint text-af-brand'
          : 'bg-af-sunken text-af-ink-2 hover:bg-af-brand-tint hover:text-af-brand-hover'
      ]"
      @click="emit('select', preset)"
    >
      {{ preset.label }} ({{ displayUrl(preset.url) }})
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CN_BASE_URL_PRESETS, type CnBaseUrlPreset } from './credentialsBuilder'

// 国产供应商快捷端点：点击把预设地址（及对应账号类型/协议）回填到调用方。
// 与 Grok 预设一致，仅作快速填充，输入框仍接受任意第三方转发地址。
const props = defineProps<{
  platform: 'kimi' | 'zhipu' | 'deepseek' | 'minimax'
  /** 当前已选账号类型，用于高亮匹配的预设 */
  mode?: 'payg' | 'coding'
}>()

const emit = defineEmits<{
  (e: 'select', preset: CnBaseUrlPreset): void
}>()

// 展示该平台全部协议的预设：payg/coding 两档都展示，点击即同时切换账号类型。
const presets = computed(() => CN_BASE_URL_PRESETS[props.platform] ?? [])

const isActive = (preset: CnBaseUrlPreset) => props.mode != null && preset.mode === props.mode

const displayUrl = (url: string) => url.replace(/^https?:\/\//i, '')
</script>
