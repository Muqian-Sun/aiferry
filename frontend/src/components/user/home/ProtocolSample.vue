<template>
  <!--
    「浏览器窗口」样式的接入示例：四条官方协议各一个页签，REQUEST（curl）+ RESPONSE 上下两栏。
    纯展示、不发请求；高亮用手写分段而不是正则，只分字符串 / 键名 / 注释 / 标记四类。
  -->
  <div class="sheet-card overflow-hidden" data-testid="protocol-sample">
    <div class="flex items-center gap-1.5 border-b border-af-hairline px-4 py-3">
      <span class="h-2 w-2 rounded-full bg-af-hairline-strong" aria-hidden="true" />
      <span class="h-2 w-2 rounded-full bg-af-hairline-strong" aria-hidden="true" />
      <span class="h-2 w-2 rounded-full bg-af-hairline-strong" aria-hidden="true" />
    </div>
    <div class="px-5 pt-4">
      <div class="flex items-center justify-between gap-3">
        <div role="tablist" :aria-label="t('userUi.home.quickstart.title')" class="flex gap-1 overflow-x-auto scrollbar-hide">
          <button
            v-for="route in PROTOCOL_ROUTES"
            :key="route.key"
            type="button"
            role="tab"
            :aria-selected="active === route.key"
            :data-testid="`protocol-tab-${route.key}`"
            :class="[
              'shrink-0 rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors',
              active === route.key ? 'bg-af-ink text-af-sheet' : 'text-af-ink-2 hover:bg-af-sunken hover:text-af-ink'
            ]"
            @click="active = route.key"
          >
            {{ t(`userUi.home.quickstart.sample.tabs.${route.key}`) }}
          </button>
        </div>
        <button
          type="button"
          class="btn btn-ghost btn-sm shrink-0"
          :aria-label="t('userUi.home.quickstart.sample.copy')"
          @click="copy"
        >
          <Icon :name="copied ? 'check' : 'copy'" size="sm" />
          {{ copied ? t('userUi.home.quickstart.sample.copied') : t('userUi.home.quickstart.sample.copy') }}
        </button>
      </div>
      <p class="mt-3 font-mono text-xs text-af-ink-4">
        <span class="mr-2 rounded bg-af-sunken px-1.5 py-0.5 font-medium text-af-ink-2">POST</span>{{ activeRoute.path }}
      </p>
    </div>
    <div class="px-5 pb-5 pt-3">
      <p class="text-[11px] font-medium tracking-wide text-af-ink-4">{{ t('userUi.home.quickstart.sample.request') }}</p>
      <pre class="mt-2 overflow-x-auto rounded-lg bg-af-sunken px-4 py-3 font-mono text-13 leading-6 text-af-ink" tabindex="0"><code><template v-for="(line, i) in request" :key="i"><span
        v-for="(seg, j) in line"
        :key="j"
        :class="SEGMENT_CLASS[seg.kind]"
      >{{ seg.text }}</span>{{ i < request.length - 1 ? '\n' : '' }}</template></code></pre>
      <p class="mt-4 text-[11px] font-medium tracking-wide text-af-ink-4">{{ t('userUi.home.quickstart.sample.response') }}</p>
      <pre class="mt-2 overflow-x-auto rounded-lg bg-af-sunken px-4 py-3 font-mono text-13 leading-6 text-af-ink" tabindex="0"><code><template v-for="(line, i) in response" :key="i"><span
        v-for="(seg, j) in line"
        :key="j"
        :class="SEGMENT_CLASS[seg.kind]"
      >{{ seg.text }}</span>{{ i < response.length - 1 ? '\n' : '' }}</template></code></pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { PROTOCOL_ROUTES, type ProtocolRoute } from './protocols'

type Kind = 'plain' | 'string' | 'key' | 'comment' | 'flag'
interface Segment { text: string; kind: Kind }
type Line = Segment[]

const SEGMENT_CLASS: Record<Kind, string> = {
  plain: '',
  string: 'text-af-brand',
  key: 'text-af-ink-2',
  comment: 'text-af-ink-4',
  flag: 'text-af-ink-3'
}

const props = defineProps<{
  /** 站点 API 根地址（不带 /v1）；空则用当前 origin */
  baseUrl: string
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const active = ref<ProtocolRoute['key']>('messages')
const copied = ref(false)

const origin = computed(() => props.baseUrl.trim().replace(/\/+$/, '') || (typeof window !== 'undefined' ? window.location.origin : ''))
const activeRoute = computed(() => PROTOCOL_ROUTES.find((route) => route.key === active.value) ?? PROTOCOL_ROUTES[0])

const p = (text: string): Segment => ({ text, kind: 'plain' })
const s = (text: string): Segment => ({ text, kind: 'string' })
const k = (text: string): Segment => ({ text, kind: 'key' })
const f = (text: string): Segment => ({ text, kind: 'flag' })

const requests = computed<Record<ProtocolRoute['key'], Line[]>>(() => {
  const o = origin.value
  return {
    messages: [
      [p('curl '), f('-X POST'), p(` ${o}/v1/messages \\`)],
      [p('  '), f('-H'), p(' '), s('"x-api-key: sk-…"'), p(' \\')],
      [p('  '), f('-H'), p(' '), s('"anthropic-version: 2023-06-01"'), p(' \\')],
      [p('  '), f('-d'), p(" '{"), k('"model"'), p(': '), s('"claude-opus-5"'), p(', '), k('"max_tokens"'), p(': 1024,')],
      [p('       '), k('"messages"'), p(': [{'), k('"role"'), p(': '), s('"user"'), p(', '), k('"content"'), p(': '), s('"你好"'), p("}]}'")]
    ],
    responses: [
      [p('curl '), f('-X POST'), p(` ${o}/v1/responses \\`)],
      [p('  '), f('-H'), p(' '), s('"Authorization: Bearer sk-…"'), p(' \\')],
      [p('  '), f('-d'), p(" '{"), k('"model"'), p(': '), s('"gpt-5.6"'), p(', '), k('"input"'), p(': '), s('"你好"'), p("}'")]
    ],
    chat: [
      [p('curl '), f('-X POST'), p(` ${o}/v1/chat/completions \\`)],
      [p('  '), f('-H'), p(' '), s('"Authorization: Bearer sk-…"'), p(' \\')],
      [p('  '), f('-d'), p(" '{"), k('"model"'), p(': '), s('"deepseek-v4"'), p(',')],
      [p('       '), k('"messages"'), p(': [{'), k('"role"'), p(': '), s('"user"'), p(', '), k('"content"'), p(': '), s('"你好"'), p("}]}'")]
    ],
    gemini: [
      [p('curl '), f('-X POST'), p(` ${o}/v1beta/models/gemini-3-pro:generateContent \\`)],
      [p('  '), f('-H'), p(' '), s('"x-goog-api-key: sk-…"'), p(' \\')],
      [p('  '), f('-d'), p(" '{"), k('"contents"'), p(': [{'), k('"parts"'), p(': [{'), k('"text"'), p(': '), s('"你好"'), p("}]}]}'")]
    ]
  }
})

const responses: Record<ProtocolRoute['key'], Line[]> = {
  messages: [
    [p('{'), k('"content"'), p(': [{'), k('"type"'), p(': '), s('"text"'), p(', '), k('"text"'), p(': '), s('"你好！…"'), p('}],')],
    [p(' '), k('"usage"'), p(': {'), k('"input_tokens"'), p(': 8, '), k('"output_tokens"'), p(': 21}}')]
  ],
  responses: [
    [p('{'), k('"output_text"'), p(': '), s('"你好！…"'), p(',')],
    [p(' '), k('"usage"'), p(': {'), k('"input_tokens"'), p(': 8, '), k('"output_tokens"'), p(': 21}}')]
  ],
  chat: [
    [p('{'), k('"choices"'), p(': [{'), k('"message"'), p(': {'), k('"role"'), p(': '), s('"assistant"'), p(', '), k('"content"'), p(': '), s('"你好！…"'), p('}}],')],
    [p(' '), k('"usage"'), p(': {'), k('"total_tokens"'), p(': 29}}')]
  ],
  gemini: [
    [p('{'), k('"candidates"'), p(': [{'), k('"content"'), p(': {'), k('"parts"'), p(': [{'), k('"text"'), p(': '), s('"你好！…"'), p('}]}}],')],
    [p(' '), k('"usageMetadata"'), p(': {'), k('"totalTokenCount"'), p(': 29}}')]
  ]
}

const request = computed(() => requests.value[active.value])
const response = computed(() => responses[active.value])
const plainText = computed(() => request.value.map((line) => line.map((seg) => seg.text).join('')).join('\n'))

let timer: ReturnType<typeof setTimeout> | null = null
async function copy() {
  const ok = await copyToClipboard(plainText.value)
  if (!ok) return
  copied.value = true
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => (copied.value = false), 1500)
}
</script>
