<template>
  <!--
    登录 / 注册按钮下方一行「我已阅读并同意 …」，前面一个圆圈让用户勾选（muqian 2026-09-29：条款不弹窗）。
    没勾就提交由页面在表单里报错；勾上记住这一版条款，下次进来默认已勾。
  -->
  <div v-if="documents.length > 0" class="flex items-start gap-2 px-0.5" data-testid="login-agreement">
    <span class="relative mt-[2px] inline-flex h-4 w-4 flex-shrink-0">
      <input
        :id="inputId"
        type="checkbox"
        :checked="accepted"
        class="agreement-check"
        data-testid="login-agreement-checkbox"
        @change="handleChange"
      />
      <svg
        class="agreement-tick pointer-events-none absolute inset-0 m-auto h-2.5 w-2.5"
        viewBox="0 0 12 12"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M2.5 6.2 5 8.5 9.5 3.5" />
      </svg>
    </span>
    <p class="min-w-0 flex-1 text-[13px] leading-5 text-af-ink-2">
      <label :for="inputId" class="cursor-pointer">
        {{ t('legal.loginAgreementPrompt.checkboxPrefix') }}
      </label>
      <template v-for="(doc, index) in documents" :key="doc.id || doc.title">
        <RouterLink
          :to="documentRoute(doc)"
          target="_blank"
          rel="noopener noreferrer"
          class="font-medium text-af-ink underline-offset-4 hover:underline"
        >
          {{ t('legal.loginAgreementPrompt.documentTitle', { title: doc.title }) }}
        </RouterLink>
        <span v-if="index < documents.length - 1">{{ t('legal.loginAgreementPrompt.documentSeparator') }}</span>
      </template>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LoginAgreementDocument } from '@/types'

const { t } = useI18n()

const props = defineProps<{
  accepted: boolean
  documents: LoginAgreementDocument[]
}>()

const emit = defineEmits<{
  accept: []
  reject: []
}>()

const inputId = 'login-agreement-consent'
const documents = computed(() => props.documents.filter((doc) => doc.title.trim()))

function documentRoute(doc: LoginAgreementDocument) {
  return {
    name: 'LegalDocument',
    params: {
      documentId: doc.id || doc.title,
    },
  }
}

function handleChange(event: Event): void {
  if ((event.target as HTMLInputElement).checked) {
    emit('accept')
  } else {
    emit('reject')
  }
}
</script>

<style scoped>
/* 圆形勾选框：原生 checkbox 去掉系统外观，未勾是空心圈，勾上填主色、叠一个 on-brand 色对勾（明暗主题都跟 token） */
.agreement-check {
  appearance: none;
  margin: 0;
  width: 1rem;
  height: 1rem;
  border-radius: 9999px;
  border: 1.5px solid rgb(var(--af-hairline-strong));
  background-color: rgb(var(--af-sheet));
  cursor: pointer;
  transition: background-color 0.12s ease, border-color 0.12s ease;
}

.agreement-check:hover {
  border-color: rgb(var(--af-ink-3));
}

.agreement-check:checked {
  border-color: rgb(var(--af-brand));
  background-color: rgb(var(--af-brand));
}

.agreement-tick {
  color: rgb(var(--af-on-brand));
  opacity: 0;
}

.agreement-check:checked + .agreement-tick {
  opacity: 1;
}

.agreement-check:focus-visible {
  outline: 2px solid rgb(var(--af-brand) / 0.4);
  outline-offset: 2px;
}
</style>
