<script setup lang="ts">
/**
 * 运维页最上面的系统资源（muqian 2026-10-04「把系统资源放在最前面」）：一行六格，不画框、不用卡片。
 * 每格「标签 + 数字 + 一根细用量条」（2026-10-04 muqian：单纯的文字数字不直观）：
 * 有容量的（CPU / 内存 / 数据库连接 / Redis 连接）按占用比例画，条上一根细刻度是提醒线；
 * 协程没有容量，按异常线折算；后台任务每个任务一个圆点。
 * 正常时墨色，到提醒线变黄、到异常线变红。阈值沿用原页头的那一套（CPU / 内存与智能诊断共用）。
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { OpsJobHeartbeat, OpsSystemMetricsSnapshot } from '@/api/admin/ops'

const props = defineProps<{
  metrics: OpsSystemMetricsSnapshot | null
  jobs: OpsJobHeartbeat[]
}>()

const { t, te } = useI18n()

type Level = 'normal' | 'warning' | 'critical'

const CPU_WARNING_PERCENT = 80
const CPU_CRITICAL_PERCENT = 95
const MEMORY_WARNING_PERCENT = 85
const MEMORY_CRITICAL_PERCENT = 95
const POOL_WARNING_PERCENT = 70
const POOL_CRITICAL_PERCENT = 90
const GOROUTINE_WARNING = 8_000
const GOROUTINE_CRITICAL = 15_000

function num(v: number | null | undefined): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

function levelOf(value: number | null, warning: number, critical: number): Level {
  if (value == null) return 'normal'
  if (value >= critical) return 'critical'
  if (value >= warning) return 'warning'
  return 'normal'
}

function levelClass(level: Level): string {
  if (level === 'critical') return 'text-af-danger'
  if (level === 'warning') return 'text-af-warning'
  return 'text-af-ink'
}

function barClass(level: Level): string {
  if (level === 'critical') return 'bg-af-danger'
  if (level === 'warning') return 'bg-af-warning'
  return 'bg-af-ink-2'
}

function clampPercent(value: number | null): number | null {
  return value == null ? null : Math.min(100, Math.max(0, value))
}

function jobFailed(hb: OpsJobHeartbeat): boolean {
  return !!hb.last_error_at && (!hb.last_success_at || hb.last_error_at > hb.last_success_at)
}

const failedJobs = computed(() => props.jobs.filter((hb) => hb && jobFailed(hb)).length)

interface Stat {
  key: string
  label: string
  value: string
  detail?: string
  level: Level
  /** 用量条：0–100；没有就不画条 */
  percent?: number | null
  /** 提醒线在条上的位置（0–100） */
  warnAt?: number
  /** 后台任务：每个任务一个圆点 */
  dots?: Array<{ key: string; label: string; failed: boolean }>
  onClick?: () => void
}

const showJobs = ref(false)

const stats = computed<Stat[]>(() => {
  const m = props.metrics
  // 还没拿到数据（含加载中）一律写「—」，不写「没有数据」
  const noData = '—'
  const cpu = num(m?.cpu_usage_percent)
  const mem = num(m?.memory_usage_percent)
  const memUsed = num(m?.memory_used_mb)
  const memTotal = num(m?.memory_total_mb)

  const dbActive = num(m?.db_conn_active)
  const dbIdle = num(m?.db_conn_idle)
  const dbWaiting = num(m?.db_conn_waiting)
  const dbMax = num(m?.db_max_open_conns)
  const dbOpen = dbActive != null && dbIdle != null ? dbActive + dbIdle : null
  const dbPct = dbOpen != null && dbMax ? (dbOpen / dbMax) * 100 : null

  const redisTotal = num(m?.redis_conn_total)
  const redisPool = num(m?.redis_pool_size)
  const redisPct = redisTotal != null && redisPool ? (redisTotal / redisPool) * 100 : null

  const goroutines = num(m?.goroutine_count)

  return [
    {
      key: 'cpu',
      label: t('admin.ops.page.resources.cpu'),
      value: cpu == null ? noData : `${cpu.toFixed(1)}%`,
      level: levelOf(cpu, CPU_WARNING_PERCENT, CPU_CRITICAL_PERCENT),
      percent: clampPercent(cpu),
      warnAt: CPU_WARNING_PERCENT
    },
    {
      key: 'memory',
      label: t('admin.ops.page.resources.memory'),
      value: mem == null ? noData : `${mem.toFixed(1)}%`,
      detail: memUsed != null && memTotal != null ? `${(memUsed / 1024).toFixed(1)} / ${(memTotal / 1024).toFixed(1)} GB` : undefined,
      level: levelOf(mem, MEMORY_WARNING_PERCENT, MEMORY_CRITICAL_PERCENT),
      percent: clampPercent(mem),
      warnAt: MEMORY_WARNING_PERCENT
    },
    {
      key: 'db',
      label: t('admin.ops.page.resources.db'),
      value: m?.db_ok === false ? t('admin.ops.page.resources.down') : dbOpen == null ? noData : dbMax ? `${dbOpen} / ${dbMax}` : String(dbOpen),
      detail: dbWaiting ? t('admin.ops.page.resources.waiting', { count: dbWaiting }) : undefined,
      level: m?.db_ok === false ? 'critical' : levelOf(dbPct, POOL_WARNING_PERCENT, POOL_CRITICAL_PERCENT),
      percent: m?.db_ok === false ? 100 : clampPercent(dbPct),
      warnAt: POOL_WARNING_PERCENT
    },
    {
      key: 'redis',
      label: t('admin.ops.page.resources.redis'),
      value: m?.redis_ok === false ? t('admin.ops.page.resources.down') : redisTotal == null ? noData : redisPool ? `${redisTotal} / ${redisPool}` : String(redisTotal),
      level: m?.redis_ok === false ? 'critical' : levelOf(redisPct, POOL_WARNING_PERCENT, POOL_CRITICAL_PERCENT),
      percent: m?.redis_ok === false ? 100 : clampPercent(redisPct),
      warnAt: POOL_WARNING_PERCENT
    },
    {
      key: 'goroutines',
      label: t('admin.ops.page.resources.goroutines'),
      value: goroutines == null ? noData : goroutines.toLocaleString(),
      detail: goroutines == null ? undefined : t('admin.ops.page.resources.goroutinesScale', { count: GOROUTINE_CRITICAL.toLocaleString() }),
      level: levelOf(goroutines, GOROUTINE_WARNING, GOROUTINE_CRITICAL),
      // 协程没有容量上限：按异常线折算
      percent: goroutines == null ? null : clampPercent((goroutines / GOROUTINE_CRITICAL) * 100),
      warnAt: (GOROUTINE_WARNING / GOROUTINE_CRITICAL) * 100
    },
    {
      key: 'jobs',
      label: t('admin.ops.page.resources.jobs'),
      value: !props.jobs.length
        ? noData
        : failedJobs.value > 0
          ? t('admin.ops.page.resources.jobsFailed', { count: failedJobs.value })
          : t('admin.ops.page.resources.jobsOk', { count: props.jobs.length }),
      level: failedJobs.value > 0 ? 'warning' : 'normal',
      dots: props.jobs.filter(Boolean).map((hb) => ({ key: hb.job_name, label: jobLabel(hb.job_name), failed: jobFailed(hb) })),
      onClick: props.jobs.length ? () => (showJobs.value = true) : undefined
    }
  ]
})

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString()
}


function jobLabel(name: string): string {
  const key = `admin.ops.page.resources.jobNames.${name}`
  return te(key) ? t(key) : name
}

// 后端把任务结果写成空格分隔的 key=value（如 rules=0 enabled=0、window=开始..结束）：认得的项换成中文叫法，
// 时间段按本地时间显示；不是 key=value 的部分与不认得的项原样留着
function formatJobResult(result?: string | null): string {
  if (!result) return '-'
  return result
    .trim()
    .split(/\s+/)
    .map((part) => {
      const eq = part.indexOf('=')
      if (eq <= 0) return part
      const key = part.slice(0, eq)
      let value = part.slice(eq + 1)
      if (key === 'window' && value.includes('..')) {
        value = value.split('..').map((bound) => formatTime(bound)).join(' – ')
      }
      const labelKey = `admin.ops.page.resources.jobResultKeys.${key}`
      return `${te(labelKey) ? t(labelKey) : key} ${value}`
    })
    .join(' · ')
}
</script>

<template>
  <section class="border-t border-af-hairline py-4" data-testid="ops-system-resources">
    <h2 class="mb-3 text-sm font-semibold text-af-ink">{{ t('admin.ops.page.resources.title') }}</h2>
    <dl class="grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-3 lg:grid-cols-6">
      <div v-for="stat in stats" :key="stat.key" class="min-w-0" :data-testid="`ops-resource-${stat.key}`">
        <dt class="text-xs text-af-ink-3">{{ stat.label }}</dt>
        <dd class="m-0 mt-1">
          <div class="flex min-w-0 items-baseline gap-1.5">
            <button
              v-if="stat.onClick"
              type="button"
              class="truncate text-lg font-semibold tabular-nums underline decoration-af-hairline-strong underline-offset-4 hover:decoration-af-ink-3"
              :class="levelClass(stat.level)"
              @click="stat.onClick"
            >
              {{ stat.value }}
            </button>
            <span v-else class="truncate text-lg font-semibold tabular-nums" :class="levelClass(stat.level)">{{ stat.value }}</span>
            <span v-if="stat.detail" class="truncate text-xs tabular-nums text-af-ink-3">{{ stat.detail }}</span>
          </div>
          <!-- 用量条：底色是浅灰轨道，细刻度是提醒线 -->
          <div
            v-if="stat.percent != null"
            class="relative mt-2 h-1.5 overflow-hidden rounded-full bg-af-sunken"
            role="meter"
            :aria-valuenow="Math.round(stat.percent)"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-label="stat.label"
          >
            <div class="h-full rounded-full transition-[width] duration-500" :class="barClass(stat.level)" :style="{ width: `${stat.percent}%` }" />
            <span v-if="stat.warnAt != null" class="absolute inset-y-0 w-px bg-af-hairline-strong" :style="{ left: `${stat.warnAt}%` }" aria-hidden="true" />
          </div>
          <div v-else-if="stat.dots?.length" class="mt-2.5 flex flex-wrap items-center gap-1.5">
            <span
              v-for="dot in stat.dots"
              :key="dot.key"
              class="h-1.5 w-1.5 rounded-full"
              :class="dot.failed ? 'bg-af-danger' : 'bg-af-success'"
              :title="dot.label"
            />
          </div>
        </dd>
      </div>
    </dl>

    <BaseDialog :show="showJobs" :title="t('admin.ops.jobs')" width="wide" @close="showJobs = false">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.page.resources.jobName') }}</th>
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.lastSuccess') }}</th>
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.lastError') }}</th>
            <th class="py-2 font-medium">{{ t('admin.ops.result') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="hb in props.jobs" :key="hb.job_name" class="border-b border-af-hairline align-top">
            <td class="py-2 pr-4 text-xs text-af-ink" :title="hb.job_name">{{ jobLabel(hb.job_name) }}</td>
            <td class="py-2 pr-4 text-xs tabular-nums text-af-ink-2">{{ formatTime(hb.last_success_at) }}</td>
            <td class="py-2 pr-4 text-xs tabular-nums" :class="jobFailed(hb) ? 'text-af-danger' : 'text-af-ink-2'">
              {{ formatTime(hb.last_error_at) }}
              <div v-if="jobFailed(hb) && hb.last_error" class="mt-1 break-all">{{ hb.last_error }}</div>
            </td>
            <td class="py-2 text-xs text-af-ink-2">{{ formatJobResult(hb.last_result) }}</td>
          </tr>
        </tbody>
      </table>
    </BaseDialog>
  </section>
</template>
