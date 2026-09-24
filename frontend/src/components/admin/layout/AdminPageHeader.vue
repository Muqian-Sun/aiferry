<template>
  <!--
    管理站页头（A3）：标题从顶栏挪进内容区，和用户站控制台同一个 PageHeader（28px 标题 + 一行说明）。
    同组页面（订阅 · 套餐、订单 · 收款概览、内容审核 · 提示词）共用组标题，下方页签切换，侧栏只留一个入口。
  -->
  <PageHeader v-if="title" :title="title" :description="description">
    <!-- 页面级操作（A4）：主按钮、刷新、工具菜单放在标题右侧，和用户站控制台一致 -->
    <template v-if="$slots.actions" #actions>
      <slot name="actions" />
    </template>
    <template v-if="group" #tabs>
      <SectionTabs :tabs="groupTabs" :label="title" />
    </template>
  </PageHeader>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { resolveRouteMetaKeys } from '@/router/title'
import PageHeader from '@/components/user/shell/PageHeader.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import type { SectionTab } from '@/components/user/shell/types'
import { ADMIN_PAGE_GROUPS, type AdminPageGroupKey } from './adminPageGroups'

const route = useRoute()
const { t } = useI18n()

const group = computed(() => {
  const key = route.meta.pageGroup as AdminPageGroupKey | undefined
  return key ? ADMIN_PAGE_GROUPS[key] : null
})

// 标题 / 描述与 document.title 共用同一解析；页标题与侧栏文案用同一组 nav.* 键
const metaKeys = computed(() => resolveRouteMetaKeys(route))

const title = computed(() => {
  if (group.value) return t(group.value.titleKey)
  const key = metaKeys.value.titleKey
  return key ? t(key) : ((route.meta.title as string) || '')
})

const description = computed(() => {
  if (group.value) return t(group.value.descriptionKey)
  const key = metaKeys.value.descriptionKey
  return key ? t(key) : ((route.meta.description as string) || '')
})

const groupTabs = computed<SectionTab[]>(() =>
  (group.value?.tabs ?? []).map((tab) => ({ key: tab.path, label: t(tab.labelKey), to: tab.path }))
)
</script>
