<template>
  <!--
    五条特色各一幅「会动」的示意图（muqian 2026-09-22 定五条，9-23 定去底纹、加颜色、动画自己动）：
    - passthrough：四条官方协议各自的品牌色汇到一个 base_url，请求点沿线流下去
    - failover：渠道 A 超时转红 → 同模型换渠道 → 渠道 B 转绿，循环演示
    - cache：同一会话固定上游，三次请求缓存命中率逐次涨上去
    - privacy：请求内容被抹掉不保存，只留一行账
    - noSale：数据往外流被拦在墨色横条上，下面三个去向全是 ✕
    全是 DOM + CSS 关键帧，不发请求、不放真实数字（示意值），减少动态效果偏好下静止在完成态。
  -->
  <div class="flex h-56 w-full items-center justify-center" :data-testid="`home-figure-${kind}`" aria-hidden="true">
    <!-- 1. 尽量透传 -->
    <div v-if="kind === 'passthrough'" class="w-full max-w-[19rem]">
      <ul class="grid grid-cols-2 gap-2">
        <li
          v-for="route in PROTOCOL_ROUTES"
          :key="route.key"
          class="fig-proto rounded-lg px-3 py-1.5 text-center font-mono text-[11px]"
          :style="{ '--proto': PROTOCOL_COLORS[route.key] }"
        >
          {{ t(`userUi.home.protocols.${route.key}`) }}
        </li>
      </ul>
      <div class="relative mx-auto h-9 w-px bg-af-hairline-strong">
        <span class="fig-flow-dot absolute left-1/2 top-0 h-2.5 w-2.5 -translate-x-1/2 rounded-full bg-af-brand" />
      </div>
      <div class="flex items-center gap-3 rounded-xl border border-af-brand/35 bg-af-brand-tint px-4 py-3">
        <span class="flex h-9 w-9 items-center justify-center rounded-full bg-af-brand text-af-sheet"><Icon name="server" size="sm" /></span>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-af-ink">{{ t('userUi.home.features.figure.passthrough.endpoint') }}</p>
          <p class="truncate font-mono text-[11px] text-af-ink-3">{{ t('userUi.home.features.figure.passthrough.note') }}</p>
        </div>
        <span class="fig-pulse h-2 w-2 shrink-0 rounded-full bg-af-success" />
      </div>
    </div>

    <!-- 2. 不做不同模型兜底 -->
    <div v-else-if="kind === 'failover'" class="fig-failover w-full max-w-[19rem] space-y-2">
      <p class="mx-auto flex w-fit items-center gap-1.5 rounded-full border border-af-hairline px-3 py-1 font-mono text-[11px] text-af-ink-2">
        <Icon name="bolt" size="xs" class="text-af-brand" />
        claude-opus-5
      </p>
      <div class="fig-fo-a flex items-center gap-3 rounded-xl border border-af-hairline px-4 py-2.5">
        <span class="fig-fo-a-mark flex h-6 w-6 items-center justify-center rounded-full bg-af-sunken text-af-ink-3"><Icon name="x" size="xs" /></span>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-af-ink">{{ t('userUi.home.features.figure.failover.channelA') }}</p>
          <p class="fig-fo-a-text font-mono text-[11px] text-af-ink-3">{{ t('userUi.home.features.figure.failover.timeout') }}</p>
        </div>
      </div>
      <p class="fig-fo-arrow flex items-center justify-center gap-1.5 font-mono text-[11px] text-af-ink-3">
        <Icon name="arrowDown" size="xs" />
        {{ t('userUi.home.features.figure.failover.switched') }}
      </p>
      <div class="fig-fo-b flex items-center gap-3 rounded-xl border border-af-hairline px-4 py-2.5">
        <span class="fig-fo-b-mark flex h-6 w-6 items-center justify-center rounded-full bg-af-sunken text-af-ink-3"><Icon name="check" size="xs" /></span>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-af-ink">{{ t('userUi.home.features.figure.failover.channelB') }}</p>
          <p class="fig-fo-b-text font-mono text-[11px] text-af-ink-3">200 OK · 186ms</p>
        </div>
      </div>
    </div>

    <!-- 3. 最大程度缓存 -->
    <div v-else-if="kind === 'cache'" class="w-full max-w-[19rem]">
      <div class="flex items-center justify-between">
        <p class="text-sm font-semibold text-af-ink">{{ t('userUi.home.features.figure.cache.session') }} · a1f3</p>
        <p class="flex items-center gap-1.5 rounded-full bg-af-brand-tint px-3 py-1 font-mono text-[11px] text-af-brand">
          <Icon name="link" size="xs" />{{ t('userUi.home.features.figure.cache.pinned') }}
        </p>
      </div>
      <ul class="mt-4 space-y-3">
        <li
          v-for="(row, index) in CACHE_ROWS"
          :key="row.hit"
          class="fig-cache-row"
          :style="{ '--i': index, '--w': `${row.hit}%` }"
        >
          <div class="flex items-center justify-between font-mono text-[11px]">
            <span class="text-af-ink-3">{{ t('userUi.home.features.figure.cache.request') }} {{ index + 1 }}</span>
            <span :class="row.hit ? 'text-af-brand' : 'text-af-ink-3'">{{ t('userUi.home.features.figure.cache.hit') }} {{ row.hit }}%</span>
          </div>
          <div class="mt-1.5 h-2 overflow-hidden rounded-full bg-af-sunken">
            <div class="fig-cache-bar h-full rounded-full bg-af-brand" />
          </div>
        </li>
      </ul>
    </div>

    <!-- 4. 不记录用户数据 -->
    <div v-else-if="kind === 'privacy'" class="fig-privacy w-full max-w-[19rem]">
      <div class="flex items-center justify-between">
        <p class="text-sm font-semibold text-af-ink">{{ t('userUi.home.features.figure.privacy.request') }}</p>
        <p class="fig-privacy-chip flex items-center gap-1.5 rounded-full border border-af-hairline px-3 py-1 font-mono text-[11px] text-af-ink-3">
          <Icon name="eyeOff" size="xs" />{{ t('userUi.home.features.figure.privacy.notStored') }}
        </p>
      </div>
      <div class="fig-privacy-body mt-4 space-y-2">
        <div class="h-2.5 w-11/12 rounded bg-af-ink-4" />
        <div class="h-2.5 w-full rounded bg-af-ink-4" />
        <div class="h-2.5 w-2/3 rounded bg-af-ink-4" />
      </div>
      <div class="mt-5 flex items-center justify-between rounded-xl border border-af-success/35 bg-af-success-tint px-3 py-2 font-mono text-[11px]">
        <span class="flex items-center gap-1.5 text-af-success"><Icon name="check" size="xs" />{{ t('userUi.home.features.figure.privacy.kept') }}</span>
        <span class="text-af-ink-2">8 → 21 tokens · $0.0004</span>
      </div>
    </div>

    <!-- 5. 不出售用户数据 -->
    <div v-else class="w-full max-w-[19rem]">
      <div class="mx-auto flex w-fit items-center gap-2 rounded-full border border-af-hairline px-4 py-2">
        <Icon name="lock" size="sm" class="text-af-brand" />
        <span class="text-sm font-semibold text-af-ink">{{ t('userUi.home.features.figure.noSale.yourData') }}</span>
      </div>
      <div class="relative mx-auto h-9 w-px bg-af-hairline-strong">
        <span class="fig-block-dot absolute left-1/2 top-0 h-2.5 w-2.5 -translate-x-1/2 rounded-full bg-af-ink-3" />
      </div>
      <p class="mx-auto flex w-fit items-center gap-1.5 rounded-full bg-af-ink px-4 py-1.5 font-mono text-[11px] text-af-sheet">
        <Icon name="shield" size="xs" />
        {{ t('userUi.home.features.figure.noSale.barrier') }}
      </p>
      <ul class="mt-5 grid grid-cols-3 gap-2">
        <li
          v-for="target in NO_SALE_TARGETS"
          :key="target"
          class="flex items-center justify-center gap-1 rounded-lg border border-dashed border-af-danger/40 bg-af-danger-tint px-2 py-1.5 font-mono text-[11px] text-af-danger"
        >
          <Icon name="x" size="xs" />
          {{ t(`userUi.home.features.figure.noSale.${target}`) }}
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { PROTOCOL_ROUTES, type ProtocolRoute } from './protocols'

export type FigureKind = 'passthrough' | 'failover' | 'cache' | 'privacy' | 'noSale'

defineProps<{ kind: FigureKind }>()

const { t } = useI18n()

/** 四条协议各用所属厂商的品牌色（与图标云同一套来源：lobehub 品牌色） */
const PROTOCOL_COLORS: Record<ProtocolRoute['key'], string> = {
  messages: '#D97757',
  responses: '#10A37F',
  chat: '#10A37F',
  gemini: '#1C69FF'
}

/** 示意值：同一会话三次请求，缓存命中率逐次上升 */
const CACHE_ROWS = [{ hit: 0 }, { hit: 62 }, { hit: 91 }] as const
const NO_SALE_TARGETS = ['thirdParty', 'ads', 'brokers'] as const
</script>

<style scoped>
/* 协议小块：用厂商品牌色描边与文字，底色同色极淡 */
.fig-proto {
  border: 1px solid var(--proto);
  color: var(--proto);
  background-color: color-mix(in srgb, var(--proto) 8%, transparent);
}

/* 1. 请求点沿线往下流，2.4s 一趟 */
.fig-flow-dot {
  animation: fig-flow 2.4s ease-in-out infinite;
}

@keyframes fig-flow {
  0% {
    transform: translate(-50%, -4px);
    opacity: 0;
  }
  15%,
  85% {
    opacity: 1;
  }
  100% {
    transform: translate(-50%, 32px);
    opacity: 0;
  }
}

/* 端点状态灯：呼吸 */
.fig-pulse {
  animation: fig-pulse 2s ease-in-out infinite;
}

@keyframes fig-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgb(var(--af-success) / 0.45);
  }
  50% {
    box-shadow: 0 0 0 5px rgb(var(--af-success) / 0);
  }
}

/* 2. 换渠道演示：6s 一轮——0-28% 两条待定；32% A 转红；52% 出现「换渠道」；64% B 转绿；96% 复位 */
.fig-failover .fig-fo-a,
.fig-failover .fig-fo-a-mark,
.fig-failover .fig-fo-a-text,
.fig-failover .fig-fo-arrow,
.fig-failover .fig-fo-b,
.fig-failover .fig-fo-b-mark,
.fig-failover .fig-fo-b-text {
  animation-duration: 6s;
  animation-timing-function: ease-out;
  animation-iteration-count: infinite;
}

.fig-failover .fig-fo-a {
  animation-name: fig-fo-a;
}

.fig-failover .fig-fo-a-mark {
  animation-name: fig-fo-a-mark;
}

.fig-failover .fig-fo-a-text {
  animation-name: fig-fo-a-text;
}

.fig-failover .fig-fo-arrow {
  animation-name: fig-fo-arrow;
}

.fig-failover .fig-fo-b {
  animation-name: fig-fo-b;
}

.fig-failover .fig-fo-b-mark {
  animation-name: fig-fo-b-mark;
}

.fig-failover .fig-fo-b-text {
  animation-name: fig-fo-b-text;
}

@keyframes fig-fo-a {
  0%,
  28%,
  96%,
  100% {
    border-color: rgb(var(--af-hairline));
    background-color: transparent;
  }
  32%,
  92% {
    border-color: rgb(var(--af-danger) / 0.45);
    background-color: rgb(var(--af-danger-tint));
  }
}

@keyframes fig-fo-a-mark {
  0%,
  28%,
  96%,
  100% {
    background-color: rgb(var(--af-sunken));
    color: rgb(var(--af-ink-3));
  }
  32%,
  92% {
    background-color: rgb(var(--af-danger));
    color: rgb(var(--af-sheet));
  }
}

@keyframes fig-fo-a-text {
  0%,
  28%,
  96%,
  100% {
    color: rgb(var(--af-ink-3));
  }
  32%,
  92% {
    color: rgb(var(--af-danger));
  }
}

@keyframes fig-fo-arrow {
  0%,
  46%,
  96%,
  100% {
    opacity: 0.25;
  }
  52%,
  92% {
    opacity: 1;
  }
}

@keyframes fig-fo-b {
  0%,
  58%,
  96%,
  100% {
    border-color: rgb(var(--af-hairline));
    background-color: transparent;
  }
  64%,
  92% {
    border-color: rgb(var(--af-success) / 0.45);
    background-color: rgb(var(--af-success-tint));
  }
}

@keyframes fig-fo-b-mark {
  0%,
  58%,
  96%,
  100% {
    background-color: rgb(var(--af-sunken));
    color: rgb(var(--af-ink-3));
  }
  64%,
  92% {
    background-color: rgb(var(--af-success));
    color: rgb(var(--af-sheet));
  }
}

@keyframes fig-fo-b-text {
  0%,
  58%,
  96%,
  100% {
    color: rgb(var(--af-ink-3));
  }
  64%,
  92% {
    color: rgb(var(--af-success));
  }
}

/* 3. 缓存：三行按 --i 依次出现，条从 0 涨到 --w；7s 一轮 */
.fig-cache-row {
  animation: fig-cache-row 7s ease-out calc(var(--i) * 1.1s) infinite;
}

.fig-cache-bar {
  width: 0;
  animation: fig-cache-bar 7s ease-out calc(var(--i) * 1.1s + 0.25s) infinite;
}

@keyframes fig-cache-row {
  0% {
    opacity: 0;
    transform: translateX(-6px);
  }
  6%,
  88% {
    opacity: 1;
    transform: none;
  }
  100% {
    opacity: 0;
  }
}

@keyframes fig-cache-bar {
  0% {
    width: 0;
  }
  14%,
  88% {
    width: var(--w);
  }
  100% {
    width: 0;
  }
}

/* 4. 不保存：内容条 30% 起抹掉，账那一行始终在；5s 一轮 */
.fig-privacy .fig-privacy-body > div {
  animation: fig-privacy-line 5s ease-out infinite;
}

.fig-privacy .fig-privacy-chip {
  animation: fig-privacy-chip 5s ease-out infinite;
}

@keyframes fig-privacy-line {
  0%,
  28%,
  100% {
    opacity: 1;
    transform: none;
  }
  38%,
  90% {
    opacity: 0.15;
    transform: scaleX(0.9);
  }
}

@keyframes fig-privacy-chip {
  0%,
  28%,
  100% {
    color: rgb(var(--af-ink-3));
    border-color: rgb(var(--af-hairline));
  }
  38%,
  90% {
    color: rgb(var(--af-success));
    border-color: rgb(var(--af-success) / 0.45);
  }
}

/* 5. 往外流的点走到拦截条就停住消失；3s 一趟 */
.fig-block-dot {
  animation: fig-block 3s ease-in infinite;
}

@keyframes fig-block {
  0% {
    transform: translate(-50%, -4px);
    opacity: 0;
  }
  15%,
  70% {
    opacity: 1;
  }
  78% {
    transform: translate(-50%, 28px);
    opacity: 1;
  }
  86%,
  100% {
    transform: translate(-50%, 28px);
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .fig-flow-dot,
  .fig-pulse,
  .fig-failover .fig-fo-a,
  .fig-failover .fig-fo-a-mark,
  .fig-failover .fig-fo-a-text,
  .fig-failover .fig-fo-arrow,
  .fig-failover .fig-fo-b,
  .fig-failover .fig-fo-b-mark,
  .fig-failover .fig-fo-b-text,
  .fig-cache-row,
  .fig-cache-bar,
  .fig-privacy .fig-privacy-body > div,
  .fig-privacy .fig-privacy-chip,
  .fig-block-dot {
    animation: none;
  }

  .fig-flow-dot,
  .fig-block-dot {
    opacity: 0;
  }

  .fig-cache-bar {
    width: var(--w);
  }

  .fig-privacy .fig-privacy-body > div {
    opacity: 0.15;
  }
}
</style>
