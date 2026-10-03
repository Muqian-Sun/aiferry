<template>
  <!-- 按官方价 × 折扣快填：只填这一块里空着的上游价格子（含分段），填过的不动；填完还要点这一块的「保存」 -->
  <PopoverMenu width-class="w-80" @open="onOpen">
    <template #trigger>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="disabled" data-testid="pricing-discount-fill">
        {{ t('admin.pricing.discountFill.trigger') }}<Icon name="chevronDown" size="xs" />
      </button>
    </template>
    <div class="space-y-2 px-3 py-2">
      <p class="text-xs text-af-ink-3">{{ t('admin.pricing.discountFill.hint') }}</p>
      <form class="flex items-center gap-2" @submit.prevent="apply">
        <span class="whitespace-nowrap text-13 text-af-ink-2">{{ t('admin.pricing.discountFill.prefix') }}</span>
        <input
          v-model="ratioText"
          type="text"
          inputmode="decimal"
          autocomplete="off"
          placeholder="0.03"
          :aria-label="t('admin.pricing.discountFill.ratio')"
          class="input h-8 w-20 px-2 py-1 text-right text-13 tabular-nums"
          data-testid="pricing-discount-ratio"
        />
        <button type="submit" class="btn btn-primary btn-sm" :disabled="ratio == null" data-testid="pricing-discount-apply">
          {{ t('admin.pricing.discountFill.apply') }}
        </button>
      </form>
    </div>
  </PopoverMenu>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { PopoverMenu } from '@/components/admin/list'
import { parseDiscount } from './pricingDraft'

defineProps<{ disabled?: boolean }>()

const emit = defineEmits<{ apply: [ratio: number] }>()

const { t } = useI18n()

const ratioText = ref('')
const ratio = computed(() => parseDiscount(ratioText.value))

function onOpen() {
  ratioText.value = ''
}

function apply() {
  if (ratio.value != null) emit('apply', ratio.value)
}
</script>
