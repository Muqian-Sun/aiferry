<template>
  <svg
    v-if="iconInfo"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    class="model-icon"
    fill="currentColor"
    fill-rule="evenodd"
  >
    <path v-for="(p, idx) in iconInfo.paths" :key="idx" :d="p" :fill="iconInfo.color" />
  </svg>
  <span v-else class="model-icon-fallback" :style="{ width: size, height: size, fontSize: `calc(${size} * 0.5)` }">
    {{ fallbackText }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { iconData, modelIconKey } from './modelIconData'

const props = withDefaults(defineProps<{
  model: string
  size?: string
}>(), {
  size: '18px'
})

const fallbackText = computed(() => props.model.charAt(0).toUpperCase())
const iconKey = computed(() => modelIconKey(props.model))
const iconInfo = computed(() => iconKey.value ? iconData[iconKey.value] : null)
</script>

<style scoped>
.model-icon {
  flex-shrink: 0;
}
.model-icon-fallback {
  @apply inline-flex shrink-0 items-center justify-center rounded bg-af-sunken font-semibold text-af-ink-3;
}
</style>
