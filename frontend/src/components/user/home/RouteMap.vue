<template>
  <!--
    首页航线图：你的应用（一把 Key）→ AiFerry → 四条官方协议航线，末端标出能抵达的厂商。
    纯展示、静态、不发请求；连线用 CSS 边线画，端点路径用 mono；辅助技术按列表逐条读。
  -->
  <figure class="route-map" :aria-label="t('userUi.home.routeMap.label')">
    <div class="grid min-w-0 gap-y-6 lg:grid-cols-[auto_minmax(0,1fr)] lg:items-center lg:gap-x-0">
      <!-- 源节点 → 码头 -->
      <div class="flex items-center">
        <div class="rounded-md border border-af-hairline-strong px-3.5 py-2.5">
          <div class="text-13 font-medium text-af-ink">{{ t('userUi.home.routeMap.source') }}</div>
          <div class="mt-0.5 font-mono text-xs text-af-ink-3">{{ t('userUi.home.routeMap.sourceHint') }}</div>
        </div>
        <span class="h-px w-8 bg-af-brand/60" aria-hidden="true"></span>
        <div class="flex items-center gap-2 rounded-md bg-af-brand px-3.5 py-2.5 text-af-on-brand">
          <img :src="logo" alt="" class="h-5 w-5 object-contain brightness-0 invert" aria-hidden="true" />
          <span class="text-13 font-semibold">{{ siteName }}</span>
        </div>
        <span class="hidden h-px w-8 bg-af-brand/60 lg:block" aria-hidden="true"></span>
      </div>

      <!-- 四条航线：一条竖 rail，每条一个横 stub -->
      <ol class="relative ml-4 min-w-0 border-l border-af-brand/60 lg:ml-0">
        <li
          v-for="route in routes"
          :key="route.key"
          class="relative flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 py-2.5 pl-8 before:absolute before:left-0 before:top-1/2 before:h-px before:w-6 before:bg-af-brand/60"
        >
          <span class="text-sm font-medium text-af-ink">{{ t(`userUi.home.routeMap.routes.${route.key}`) }}</span>
          <code class="break-all font-mono text-xs text-af-ink-3">{{ route.path }}</code>
          <span class="ml-auto text-xs text-af-ink-3">{{ t(`userUi.home.routeMap.vendors.${route.key}`) }}</span>
        </li>
      </ol>
    </div>
  </figure>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ siteName: string; logo: string }>()
const { t } = useI18n()

/** 四类官方协议与代表厂商；与后端目录的 protocols 枚举（anthropic / responses / chat_completions / gemini）一一对应 */
const routes = [
  { key: 'messages', path: '/v1/messages' },
  { key: 'responses', path: '/v1/responses' },
  { key: 'chat', path: '/v1/chat/completions' },
  { key: 'gemini', path: '/v1beta/models/{model}:generateContent' }
] as const
</script>
