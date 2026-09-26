<template>
  <!--
    新建 / 编辑模型（muqian 2026-09-25：原来列表页里的弹窗改成整页 /model-catalog/new、/model-catalog/:id/edit）。
    编辑时先取条目；厂商下拉的选项来自目录里已有的厂商。保存成功回列表，取消也回列表。
  -->
  <AppLayout>
    <div class="mb-6 flex min-w-0 items-center gap-2 text-sm">
      <RouterLink
        to="/model-catalog"
        class="inline-flex shrink-0 items-center gap-1 text-af-ink-3 transition-colors hover:text-af-ink"
        data-testid="model-catalog-form-back"
      >
        <Icon name="arrowLeft" size="sm" />
        {{ t('admin.modelCatalog.formPage.backToList') }}
      </RouterLink>
      <template v-if="entry">
        <span class="text-af-ink-4" aria-hidden="true">/</span>
        <span class="min-w-0 truncate font-mono font-medium text-af-ink">{{ entry.model_id }}</span>
      </template>
    </div>

    <div v-if="loading" class="flex items-center gap-2 py-16 text-sm text-af-ink-3" data-testid="model-catalog-form-loading">
      <Icon name="refresh" size="sm" class="animate-spin" />
      {{ t('admin.modelCatalog.formPage.loading') }}
    </div>

    <div v-else-if="loadError" class="max-w-xl py-8" data-testid="model-catalog-form-error">
      <p class="text-base text-af-ink">
        {{
          loadError === 'not-found'
            ? t('admin.modelCatalog.formPage.notFound')
            : t('admin.modelCatalog.formPage.loadFailed', { message: loadError })
        }}
      </p>
      <div class="mt-5 flex items-center gap-3">
        <RouterLink to="/model-catalog" class="btn btn-primary">{{ t('admin.modelCatalog.formPage.backToListAction') }}</RouterLink>
        <button v-if="loadError !== 'not-found'" type="button" class="btn btn-secondary" @click="load">
          {{ t('admin.modelCatalog.formPage.retry') }}
        </button>
      </div>
    </div>

    <CatalogEntryEditor v-else :entry="entry" :vendor-options="vendorOptions" @close="back" @saved="onSaved" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import CatalogEntryEditor from '@/components/admin/catalog/CatalogEntryEditor.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

/** 编辑页的条目 ID；新建页没有 */
const routeId = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))

const entry = ref<ModelCatalogEntry | null>(null)
const vendorOptions = ref<string[]>([])
const loading = ref(true)
/** 'not-found' 或错误信息；空串表示没出错 */
const loadError = ref('')

let loadSeq = 0

async function load() {
  const seq = ++loadSeq
  entry.value = null
  loadError.value = ''
  const isEdit = routeId.value !== ''
  const id = Number(routeId.value)
  if (isEdit && (!Number.isInteger(id) || id <= 0)) {
    loading.value = false
    loadError.value = 'not-found'
    return
  }
  loading.value = true
  const [entryResult, listResult] = await Promise.allSettled([
    isEdit ? adminAPI.modelCatalog.getEntry(id) : Promise.resolve(null),
    adminAPI.modelCatalog.listEntries()
  ])
  if (seq !== loadSeq) return
  loading.value = false
  if (entryResult.status === 'rejected') {
    const status = (entryResult.reason as { status?: number } | null)?.status
    loadError.value = status === 404 ? 'not-found' : extractApiErrorMessage(entryResult.reason, t('common.error'))
    return
  }
  // 目录列表拿不到不挡填写：厂商下拉只剩「自定义」
  if (listResult.status === 'fulfilled') {
    vendorOptions.value = [...new Set(listResult.value.map((item) => item.vendor).filter(Boolean))].sort()
  } else {
    console.error('Failed to load model catalog:', listResult.reason)
  }
  entry.value = entryResult.value
}

function back() {
  void router.push('/model-catalog')
}

function onSaved() {
  appStore.showSuccess(t('admin.modelCatalog.formPage.saved'))
  back()
}

watch(routeId, () => void load(), { immediate: true })
</script>
