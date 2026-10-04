<template>
  <div class="empty-state">
    <!-- Icon -->
    <div class="mb-5 flex h-20 w-20 items-center justify-center rounded-lg bg-af-sunken">
      <slot name="icon">
        <component v-if="icon" :is="icon" class="empty-state-icon h-10 w-10" aria-hidden="true" />
        <svg
          v-else
          class="empty-state-icon h-10 w-10"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          stroke-width="1.5"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
          />
        </svg>
      </slot>
    </div>

    <!-- Title -->
    <h3 class="empty-state-title">
      {{ displayTitle }}
    </h3>

    <!-- Description -->
    <p class="empty-state-description">
      {{ displayDescription }}
    </p>

    <!-- Action：有筛选时没结果，不给「创建第一个」这类引导 -->
    <div v-if="!filtered && (actionText || $slots.action)" class="mt-6">
      <slot name="action">
        <component
          :is="actionTo ? 'RouterLink' : 'button'"
          v-if="actionText"
          :to="actionTo"
          @click="!actionTo && $emit('action')"
          class="btn btn-primary"
        >
          <Icon v-if="actionIcon" name="plus" size="md" class="mr-2" />
          {{ actionText }}
        </component>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

interface Props {
  icon?: Component | string
  title?: string
  description?: string
  actionText?: string
  actionTo?: string | object
  actionIcon?: boolean
  message?: string
  /**
   * 列表带着搜索 / 筛选条件却没有结果：换成「没有符合条件的结果」，忽略 title / description / 操作按钮。
   * 调用方按自己的筛选状态传；不传时按「列表本来就是空的」显示。
   */
  filtered?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  description: '',
  actionIcon: true,
  filtered: false
})

const displayTitle = computed(() => (props.filtered ? t('common.noMatch') : props.title || t('common.noData')))
const displayDescription = computed(() => (props.filtered ? t('common.noMatchHint') : props.description))

defineEmits(['action'])
</script>
