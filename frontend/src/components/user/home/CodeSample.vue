<template>
  <!--
    首屏代码块：四个页签（Python / curl / Node / Claude Code），每段都只说一件事——换 base_url 和 key。
    纯展示、不发请求；高亮用手写的分段而不是正则，只分字符串 / 键名 / 注释三类。
  -->
  <div class="overflow-hidden rounded-lg bg-af-code-sheet text-af-code-ink" data-testid="code-sample">
    <div class="flex items-center border-b border-af-code-hairline pl-1 pr-2">
      <div role="tablist" :aria-label="t('userUi.home.codeSample.label')" class="flex min-w-0 flex-1 overflow-x-auto">
        <button
          v-for="tab in TABS"
          :key="tab"
          type="button"
          role="tab"
          :aria-selected="active === tab"
          :class="[
            'shrink-0 px-3 py-2.5 text-xs font-medium transition-colors',
            active === tab ? 'text-af-code-ink shadow-[inset_0_-2px_0_rgb(var(--af-code-accent))]' : 'text-af-code-ink-3 hover:text-af-code-ink'
          ]"
          @click="active = tab"
        >
          {{ t(`userUi.home.codeSample.tabs.${tab}`) }}
        </button>
      </div>
      <button
        type="button"
        class="shrink-0 rounded-md px-2 py-1 text-xs text-af-code-ink-3 transition-colors hover:text-af-code-ink"
        :aria-label="t('userUi.home.codeSample.copy')"
        @click="copy"
      >
        {{ copied ? t('userUi.home.codeSample.copied') : t('userUi.home.codeSample.copy') }}
      </button>
    </div>
    <pre class="overflow-x-auto px-5 py-4 font-mono text-13 leading-6" tabindex="0"><code><template v-for="(line, i) in lines" :key="i"><span
      v-for="(seg, j) in line"
      :key="j"
      :class="SEGMENT_CLASS[seg.kind]"
    >{{ seg.text }}</span>{{ i < lines.length - 1 ? '\n' : '' }}</template></code></pre>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'

type Kind = 'plain' | 'string' | 'key' | 'comment'
interface Segment { text: string; kind: Kind }
type Line = Segment[]

const TABS = ['python', 'curl', 'node', 'claudeCode'] as const
type Tab = (typeof TABS)[number]

const SEGMENT_CLASS: Record<Kind, string> = {
  plain: '',
  string: 'text-af-code-string',
  key: 'text-af-code-key',
  comment: 'text-af-code-ink-3'
}

const props = defineProps<{
  /** 站点 API 根地址（不带 /v1）；空则用当前 origin */
  baseUrl: string
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const active = ref<Tab>('python')
const copied = ref(false)

const origin = computed(() => props.baseUrl.trim().replace(/\/+$/, '') || (typeof window !== 'undefined' ? window.location.origin : ''))

const p = (text: string): Segment => ({ text, kind: 'plain' })
const s = (text: string): Segment => ({ text, kind: 'string' })
const k = (text: string): Segment => ({ text, kind: 'key' })
const c = (text: string): Segment => ({ text, kind: 'comment' })

const snippets = computed<Record<Tab, Line[]>>(() => {
  const o = origin.value
  const comment = t('userUi.home.codeSample.comment')
  return {
    python: [
      [c(`# ${comment}`)],
      [p('client = anthropic.Anthropic(')],
      [p('    base_url='), s(`"${o}"`), p(',')],
      [p('    api_key='), s('"sk-…"'), p(',')],
      [p(')')],
      [p('resp = client.messages.create(')],
      [p('    model='), s('"claude-opus-5"'), p(',')],
      [p('    max_tokens=1024,')],
      [p('    messages=[{'), k('"role"'), p(': '), s('"user"'), p(', '), k('"content"'), p(': '), s('"你好"'), p('}],')],
      [p(')')]
    ],
    curl: [
      [c(`# ${comment}`)],
      [p(`curl ${o}/v1/chat/completions \\`)],
      [p('  -H '), s('"Authorization: Bearer sk-…"'), p(' \\')],
      [p('  -H '), s('"Content-Type: application/json"'), p(' \\')],
      [p("  -d '{"), k('"model"'), p(': '), s('"gpt-5.5"'), p(', '), k('"messages"'), p(': [{'), k('"role"'), p(': '), s('"user"'), p(', '), k('"content"'), p(': '), s('"你好"'), p("}]}'")]
    ],
    node: [
      [c(`// ${comment}`)],
      [p('const client = new OpenAI({')],
      [p('  baseURL: '), s(`"${o}/v1"`), p(',')],
      [p('  apiKey: '), s('"sk-…"'), p(',')],
      [p('})')],
      [p('const r = await client.responses.create({')],
      [p('  model: '), s('"gpt-5.5"'), p(',')],
      [p('  input: '), s('"你好"'), p(',')],
      [p('})')]
    ],
    claudeCode: [
      [c(`# ${comment}`)],
      [p('export ANTHROPIC_BASE_URL='), s(o)],
      [p('export ANTHROPIC_AUTH_TOKEN='), s('sk-…')],
      [p('claude')]
    ]
  }
})

const lines = computed(() => snippets.value[active.value])
const plainText = computed(() => lines.value.map((line) => line.map((seg) => seg.text).join('')).join('\n'))

let timer: ReturnType<typeof setTimeout> | null = null
async function copy() {
  const ok = await copyToClipboard(plainText.value)
  if (!ok) return
  copied.value = true
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => (copied.value = false), 1500)
}
</script>
