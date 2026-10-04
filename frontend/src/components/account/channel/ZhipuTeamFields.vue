<template>
  <!-- 智谱团队版 Coding Plan：组织 / 项目 ID（可选，填了额度查询走团队版端点） -->
  <div data-testid="zhipu-team-fields">
    <div class="flex items-center">
      <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.title') }}</label>
      <HelpTooltip trigger="click" width-class="w-80">
        <p class="mb-1 font-medium">{{ t('admin.accounts.cnProviders.zhipuTeam.help.title') }}</p>
        <ol class="list-decimal space-y-1 pl-4">
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step1') }}</li>
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step2') }}</li>
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step3') }}</li>
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step4') }}</li>
        </ol>
        <p class="mt-2 break-all rounded bg-black/20 p-1.5 font-mono text-xs leading-relaxed">
          {{ t('admin.accounts.cnProviders.zhipuTeam.help.example') }}
        </p>
      </HelpTooltip>
    </div>
    <div class="mt-2 grid gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.organization') }}</label>
        <input
          :value="organization"
          type="text"
          class="input"
          :placeholder="t('admin.accounts.cnProviders.zhipuTeam.organizationPlaceholder')"
          @input="emit('update:organization', ($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.project') }}</label>
        <input
          :value="project"
          type="text"
          class="input"
          :placeholder="t('admin.accounts.cnProviders.zhipuTeam.projectPlaceholder')"
          @input="emit('update:project', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>
    <p class="input-hint mt-2">{{ t('admin.accounts.cnProviders.zhipuTeam.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

defineProps<{ organization: string; project: string }>()

const emit = defineEmits<{
  'update:organization': [value: string]
  'update:project': [value: string]
}>()

const { t } = useI18n()
</script>
