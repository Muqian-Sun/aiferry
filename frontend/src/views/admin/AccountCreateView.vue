<template>
  <!--
    添加渠道（A5-c）：原来列表页里的「添加账号」弹窗改成整页 /accounts/new。
    表单本体仍是 CreateAccountModal（外壳是 FormPageShell：左侧分区导航 + 底部保存条），
    校验、OAuth 两步流程、提交与提示都在组件里，和弹窗完全一样。
    表单每次完整成功都会先 created（成功提示已由表单弹出）再 close；OAuth 批量部分失败时只发 created、
    留在第二步显示错误。所以回列表挂在 close 上（取消也走这里），不监听 created。
  -->
  <AppLayout>
    <div class="mb-6 flex items-center text-sm">
      <RouterLink
        to="/accounts"
        class="inline-flex items-center gap-1 text-af-ink-3 transition-colors hover:text-af-ink"
        data-testid="account-form-back"
      >
        <Icon name="arrowLeft" size="sm" />
        {{ t('admin.accounts.formPage.backToList') }}
      </RouterLink>
    </div>

    <CreateAccountModal
      :show="true"
      :proxies="proxies"
      @close="back"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Proxy } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import CreateAccountModal from '@/components/account/CreateAccountModal.vue'

const { t } = useI18n()
const router = useRouter()

const proxies = ref<Proxy[]>([])

function back() {
  void router.push('/accounts')
}

onMounted(async () => {
  try {
    proxies.value = await adminAPI.proxies.getAll()
  } catch (error) {
    // 和列表页一致：代理列表拿不到不挡建渠道，代理下拉为空
    console.error('Failed to load proxies:', error)
  }
})
</script>
