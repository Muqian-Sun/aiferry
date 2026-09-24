<template>
  <!--
    编辑渠道（A5-c）：原来列表页里的「编辑账号」弹窗改成整页 /accounts/:id/edit。
    账号详情、代理、模型目录三路并行加载；表单本体仍是 EditAccountModal（layout="page"，外壳换成 FormPageShell）。
    表单保存成功会先 updated 再 close，所以回列表挂在 close 上（取消也走这里）；
    updated 只把最新账号写回来——Ollama Cloud 用量面板也会发 updated（不是保存），那时留在本页。
  -->
  <AppLayout>
    <div class="mb-6 flex min-w-0 items-center gap-2 text-sm">
      <RouterLink
        to="/accounts"
        class="inline-flex shrink-0 items-center gap-1 text-af-ink-3 transition-colors hover:text-af-ink"
        data-testid="account-form-back"
      >
        <Icon name="arrowLeft" size="sm" />
        {{ t('admin.accounts.formPage.backToList') }}
      </RouterLink>
      <template v-if="account">
        <span class="text-af-ink-4" aria-hidden="true">/</span>
        <span class="min-w-0 truncate font-medium text-af-ink" data-testid="account-form-name">{{ account.name }}</span>
        <span class="shrink-0 tabular-nums text-af-ink-4">#{{ account.id }}</span>
      </template>
    </div>

    <div v-if="loading" class="flex items-center gap-2 py-16 text-sm text-af-ink-3" data-testid="account-edit-loading">
      <Icon name="refresh" size="sm" class="animate-spin" />
      {{ t('admin.accounts.formPage.loading') }}
    </div>

    <div v-else-if="loadError" class="max-w-xl py-8" data-testid="account-edit-error">
      <p class="text-base text-af-ink">
        {{
          loadError === 'not-found'
            ? t('admin.accounts.formPage.notFound', { id: routeId })
            : t('admin.accounts.formPage.loadFailed', { message: loadError })
        }}
      </p>
      <div class="mt-5 flex items-center gap-3">
        <RouterLink to="/accounts" class="btn btn-primary">{{ t('admin.accounts.formPage.backToListAction') }}</RouterLink>
        <button v-if="loadError !== 'not-found'" type="button" class="btn btn-secondary" @click="load">
          {{ t('admin.accounts.formPage.retry') }}
        </button>
      </div>
    </div>

    <EditAccountModal
      v-else
      layout="page"
      :show="!!account"
      :account="account"
      :proxies="proxies"
      :catalog-entries="catalogEntries"
      @close="back"
      @updated="onUpdated"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import type { Account, Proxy } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import EditAccountModal from '@/components/account/EditAccountModal.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const routeId = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))

const account = ref<Account | null>(null)
const proxies = ref<Proxy[]>([])
const catalogEntries = ref<ModelCatalogEntry[]>([])
const loading = ref(true)
/** 'not-found' 或错误信息；空串表示没出错 */
const loadError = ref('')

let loadSeq = 0

async function load() {
  const seq = ++loadSeq
  const id = Number(routeId.value)
  account.value = null
  loadError.value = ''
  if (!Number.isInteger(id) || id <= 0) {
    loading.value = false
    loadError.value = 'not-found'
    return
  }
  loading.value = true
  const [accountResult, proxiesResult, catalogResult] = await Promise.allSettled([
    adminAPI.accounts.getById(id),
    adminAPI.proxies.getAll(),
    adminAPI.modelCatalog.listEntries()
  ])
  if (seq !== loadSeq) return
  loading.value = false

  if (accountResult.status === 'rejected') {
    const status = (accountResult.reason as { status?: number } | null)?.status
    loadError.value = status === 404 ? 'not-found' : extractApiErrorMessage(accountResult.reason, t('common.error'))
    return
  }
  // 代理 / 模型目录拿不到不挡编辑（和列表页一致）：代理下拉为空，已上架模型显示为空
  if (proxiesResult.status === 'fulfilled') {
    proxies.value = proxiesResult.value
  } else {
    console.error('Failed to load proxies:', proxiesResult.reason)
  }
  if (catalogResult.status === 'fulfilled') {
    catalogEntries.value = catalogResult.value.filter((entry) =>
      (entry.bindings ?? []).some((binding) => binding.account_id === id)
    )
  } else {
    console.error('Failed to load model catalog:', catalogResult.reason)
  }
  account.value = accountResult.value
}

function back() {
  void router.push('/accounts')
}

// 和列表页的 handleAccountUpdated 一样把最新账号写回；保存后紧跟的 close 负责回列表
function onUpdated(next: Account) {
  account.value = next
}

watch(routeId, () => void load(), { immediate: true })
</script>
