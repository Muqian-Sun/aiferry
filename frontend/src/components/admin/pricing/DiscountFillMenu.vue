<template>
  <!--
    按官方价 × 折扣快填：只填这一块里空着的格子（含分段），填过的不动；填完还要点这一块的「保存」。
    kind = upstream 填各渠道的上游价，sale 填售价（muqian 2026-10-06）。
  -->
  <PopoverMenu width-class="w-80" @open="onOpen">
    <template #trigger>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="disabled" :data-testid="kind === 'sale' ? 'pricing-sale-fill' : 'pricing-discount-fill'">
        {{ t(`${textKey}.trigger`) }}<Icon name="chevronDown" size="xs" />
      </button>
    </template>
    <template #default="{ close }">
      <div class="space-y-2 px-3 py-2">
        <p class="text-xs text-af-ink-3">{{ t(`${textKey}.hint`) }}</p>
        <!-- 「填入」用 click 而不是表单提交：点按钮时弹出层会先关掉卸载，表单的提交就发不出去了 -->
        <div class="flex items-center gap-2">
          <span class="whitespace-nowrap text-13 text-af-ink-2">{{ t('admin.pricing.discountFill.prefix') }}</span>
          <input
            v-model="ratioText"
            type="text"
            inputmode="decimal"
            autocomplete="off"
            :placeholder="kind === 'sale' ? '0.0667' : '0.03'"
            :aria-label="t(`${textKey}.ratio`)"
            class="input h-8 w-20 px-2 py-1 text-right text-13 tabular-nums"
            data-testid="pricing-discount-ratio"
            @keydown.enter.prevent="applyAndClose(close)"
          />
          <button type="button" class="btn btn-primary btn-sm" :disabled="ratio == null" data-testid="pricing-discount-apply" @click="apply">
            {{ t('admin.pricing.discountFill.apply') }}
          </button>
        </div>
      </div>
    </template>
  </PopoverMenu>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { PopoverMenu } from '@/components/admin/list'
import { parseDiscount } from './pricingDraft'

const props = withDefaults(defineProps<{ disabled?: boolean; kind?: 'upstream' | 'sale' }>(), { disabled: false, kind: 'upstream' })
const textKey = computed(() => (props.kind === 'sale' ? 'admin.pricing.saleFill' : 'admin.pricing.discountFill'))

const emit = defineEmits<{ apply: [ratio: number] }>()

const { t } = useI18n()

const ratioText = ref('')
// 上游价可以按 0 折填（免费的上游）；售价比例必须大于 0
const ratio = computed(() => parseDiscount(ratioText.value, props.kind === 'upstream'))

function onOpen() {
  ratioText.value = ''
}

function apply() {
  if (ratio.value != null) emit('apply', ratio.value)
}

/** 回车：填完顺手关掉弹出层（点按钮时弹出层自己会关） */
function applyAndClose(close: () => void) {
  if (ratio.value == null) return
  apply()
  close()
}
</script>
