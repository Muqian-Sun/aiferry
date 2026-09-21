<template>
  <!--
    账务：一个页头 + 页签，子页面（充值 / 订阅 / 订单 / 兑换 / 邀请）由 router-view 渲染。
    页签按功能开关出现，与子路由的 requiresPayment / requiresSubscription 守卫一致。
  -->
  <SiteShell :title="t('userUi.billing.title')" :description="description">
    <template #tabs>
      <SectionTabs :tabs="tabs" :label="t('userUi.billing.title')" />
    </template>
    <RouterView />
  </SiteShell>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import { buildBillingTabs } from './billingTabs'
import { useBillingFlags } from './useBillingFlags'

const { t } = useI18n()
const flags = useBillingFlags()
const tabs = computed(() => buildBillingTabs(flags.value, t))
// 说明只列实际出现的页签，支付关闭时不写「充值、订单」
const description = computed(() => tabs.value.map((tab) => tab.label).join('、'))
</script>
