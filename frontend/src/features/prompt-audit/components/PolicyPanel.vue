<template>
  <!-- 页面上能改的只有风险分类（2026-10-05）；工作线程、队列、节点策略写死在后端 -->
  <section aria-labelledby="prompt-policy-title" class="py-6">
    <div>
      <h2 id="prompt-policy-title" class="text-base font-semibold text-af-ink">{{ t('admin.promptAudit.policy.title') }}</h2>
      <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.promptAudit.policy.description') }}</p>
    </div>

    <fieldset class="mt-4">
      <legend class="sr-only">{{ t('admin.promptAudit.policy.title') }}</legend>
      <div class="grid gap-x-6 gap-y-1 sm:grid-cols-2 lg:grid-cols-3">
        <label v-for="scanner in SCANNER_CATALOG" :key="scanner.id" class="flex cursor-pointer items-center gap-2 py-1.5 text-sm text-af-ink-2">
          <input type="checkbox" :checked="draft.scanners.includes(scanner.id)" :aria-label="scannerLabel(scanner.id)" @change="toggleScanner(scanner.id)" />
          <span>{{ scannerLabel(scanner.id) }}</span>
        </label>
      </div>
    </fieldset>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PromptAuditDraft } from '../types'
import { cloneData, SCANNER_CATALOG } from '../viewModel'

const props = defineProps<{ draft: PromptAuditDraft }>()
const emit = defineEmits<{ (event: 'update:draft', value: PromptAuditDraft): void }>()
const { t } = useI18n()
function toggleScanner(id: string) {
  const selected = new Set(props.draft.scanners)
  if (selected.has(id)) selected.delete(id)
  else selected.add(id)
  emit('update:draft', { ...cloneData(props.draft), scanners: SCANNER_CATALOG.map((item) => item.id).filter((item) => selected.has(item)) })
}
function scannerLabel(id: string): string {
  return t(`admin.promptAudit.scanners.${id}`)
}
</script>
