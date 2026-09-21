<template>
  <!-- 法律文档：公开壳 + 单列正文（管理站也会渲染这一页） -->
  <SiteShell variant="public">
    <div class="mx-auto max-w-3xl py-10">
      <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />

      <StatusState v-else-if="loadError" kind="error" :title="t('legal.loadFailed')" :description="t('legal.retryLater')" />

      <StatusState v-else-if="!currentDocument" kind="empty" :title="t('legal.notFound')" :description="t('legal.notFoundDescription')" />

      <article v-else>
        <header class="mb-8 border-b border-af-hairline pb-6">
          <p class="text-13 font-medium text-af-ink-3">{{ documentTypeLabel }}</p>
          <h1 class="mt-1 break-words text-28 font-semibold text-af-ink">{{ currentDocument.title }}</h1>
          <p v-if="updatedAt" class="mt-2 text-13 text-af-ink-3">{{ t('legal.updatedAt', { date: updatedAt }) }}</p>
        </header>

        <div v-if="hasContent" class="legal-document-content" v-html="renderedHtml"></div>
        <p v-else class="text-sm text-af-ink-3">{{ t('legal.empty') }}</p>
      </article>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import { getLocale } from '@/i18n'
import { useAppStore } from '@/stores/app'
import type { LoginAgreementDocument } from '@/types'
import zhAdminCompliance from '../../../../docs/legal/admin-compliance.zh.md?raw'
import enAdminCompliance from '../../../../docs/legal/admin-compliance.en.md?raw'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const settings = computed(() => appStore.cachedPublicSettings)
const loading = ref(!settings.value)
const loadError = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const documentId = computed(() => String(route.params.documentId || ''))
const isAdminComplianceDocument = computed(() => documentId.value === 'admin-compliance')
const documents = computed(() => settings.value?.login_agreement_documents ?? [])
const updatedAt = computed(() =>
  isAdminComplianceDocument.value ? '' : settings.value?.login_agreement_updated_at || ''
)
const documentTypeLabel = computed(() =>
  isAdminComplianceDocument.value ? t('legal.adminCompliance') : t('legal.loginAgreement')
)

const currentDocument = computed<LoginAgreementDocument | null>(() => {
  if (isAdminComplianceDocument.value) {
    return {
      id: 'admin-compliance',
      title: t('adminCompliance.title'),
      content_md: getLocale() === 'zh' ? zhAdminCompliance : enAdminCompliance
    }
  }
  const id = documentId.value
  if (!id) {
    return null
  }
  return documents.value.find((doc) => doc.id === id) ?? null
})

const hasContent = computed(() => Boolean(currentDocument.value?.content_md?.trim()))

const renderedHtml = computed(() => {
  const content = currentDocument.value?.content_md?.trim() || ''
  if (!content) {
    return ''
  }
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

onMounted(async () => {
  loadError.value = false
  const loadedSettings = await appStore.fetchPublicSettings()
  if (!loadedSettings) {
    loadError.value = true
  }
  loading.value = false
})
</script>

<style scoped>
.legal-document-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
  color: inherit;
}

.legal-document-content :deep(h1) {
  @apply mb-4 mt-8 border-b border-af-hairline pb-3 text-2xl font-semibold;
}

.legal-document-content :deep(h2) {
  @apply mb-3 mt-7 text-2xl font-bold;
}

.legal-document-content :deep(h3) {
  @apply mb-2 mt-6 text-xl font-semibold;
}

.legal-document-content :deep(h4) {
  @apply mb-2 mt-5 text-lg font-semibold;
}

.legal-document-content :deep(p) {
  @apply mb-4 text-af-ink-2;
}

.legal-document-content :deep(a) {
  @apply text-af-brand underline underline-offset-4 hover:text-af-brand-hover;
}

.legal-document-content :deep(ul) {
  @apply mb-4 list-disc pl-6;
}

.legal-document-content :deep(ol) {
  @apply mb-4 list-decimal pl-6;
}

.legal-document-content :deep(li) {
  @apply mb-1 text-af-ink-2;
}

.legal-document-content :deep(blockquote) {
  @apply my-5 border-l-2 border-af-hairline-strong pl-4 text-af-ink-2;
}

.legal-document-content :deep(code) {
  @apply rounded bg-af-sunken px-1.5 py-0.5 font-mono text-sm;
}

.legal-document-content :deep(pre) {
  @apply my-5 overflow-x-auto rounded-md border border-af-hairline bg-af-sunken p-4 text-af-ink;
}

.legal-document-content :deep(pre code) {
  @apply bg-transparent p-0 text-inherit;
}

.legal-document-content :deep(table) {
  @apply my-5 block w-full overflow-x-auto border-collapse;
}

.legal-document-content :deep(th) {
  @apply border border-af-hairline bg-af-sunken px-3 py-2 text-left font-medium;
}

.legal-document-content :deep(td) {
  @apply border border-af-hairline px-3 py-2;
}

.legal-document-content :deep(img) {
  @apply my-5 h-auto max-w-full rounded-lg;
}

.legal-document-content :deep(hr) {
  @apply my-7 border-af-hairline;
}
</style>
