<template>
  <!-- 第三方 key 的常用官方地址：选一条把协议和地址一起填进去（只是快捷填入，地址仍可随便改）。 -->
  <select
    :value="''"
    class="input"
    data-testid="key-address-preset"
    :aria-label="t('admin.accounts.keyAddress.presetPlaceholder')"
    @change="onChange"
  >
    <option value="" disabled>{{ t('admin.accounts.keyAddress.presetPlaceholder') }}</option>
    <optgroup v-for="group in groups" :key="group.vendor" :label="vendorLabel(group.vendor)">
      <option
        v-for="item in group.items"
        :key="item.index"
        :value="String(item.index)"
        :data-protocol="item.preset.protocol"
        :data-url="item.preset.url"
        :data-mode="item.preset.mode ?? ''"
      >
        {{ optionLabel(item.preset) }}
      </option>
    </optgroup>
  </select>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { vendorLabel } from '@/components/modelPlaza/catalog'
import type { KeyAddressPreset } from './keyAddress'

const props = defineProps<{
  presets: KeyAddressPreset[]
}>()

const emit = defineEmits<{
  select: [preset: KeyAddressPreset]
}>()

const { t } = useI18n()

// 厂商分组按常用程度排：国产、OpenCode；表里没有的排最后。
// 海外四家（Anthropic / OpenAI / Gemini / Grok）没有 key 的官方地址，指向它们的 key 按中转处理
const VENDOR_ORDER = ['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go']

const groups = computed(() => {
  const byVendor = new Map<string, { index: number; preset: KeyAddressPreset }[]>()
  props.presets.forEach((preset, index) => {
    const list = byVendor.get(preset.vendor)
    if (list) list.push({ index, preset })
    else byVendor.set(preset.vendor, [{ index, preset }])
  })
  const rank = (vendor: string) => {
    const i = VENDOR_ORDER.indexOf(vendor)
    return i >= 0 ? i : VENDOR_ORDER.length
  }
  return [...byVendor.entries()]
    .sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
    .map(([vendor, items]) => ({ vendor, items }))
})

function modeLabel(mode: KeyAddressPreset['mode']): string {
  if (mode === 'payg' || mode === 'coding') return t(`admin.accounts.cnProviders.accountMode.${mode}`)
  if (mode === 'zen' || mode === 'go') return t(`admin.accounts.opencodeGo.accountMode.${mode}`)
  return ''
}

function optionLabel(preset: KeyAddressPreset): string {
  const name = preset.label ?? [modeLabel(preset.mode), t(`admin.accounts.protocolEndpoints.protocols.${preset.protocol}`)].filter(Boolean).join(' · ')
  return `${name} — ${preset.url.replace(/^https?:\/\//i, '')}`
}

function onChange(event: Event) {
  const select = event.target as HTMLSelectElement
  const preset = props.presets[Number(select.value)]
  // 选完回到占位项：它是动作菜单，不是一个要保持的值
  select.value = ''
  if (preset) emit('select', preset)
}
</script>
