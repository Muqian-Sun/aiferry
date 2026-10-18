import { computed, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SchedulingChannel, SchedulingOverview } from '@/api/admin/scheduling'

/** 调度页各处共用的文案与渠道名查找 */
export function useSchedulingLabels(overview: Ref<SchedulingOverview | null>) {
  const { t, te } = useI18n()

  const channelsById = computed(() => new Map<number, SchedulingChannel>((overview.value?.channels ?? []).map((channel) => [channel.id, channel])))

  function channelOf(accountID: number | undefined): SchedulingChannel | undefined {
    return accountID ? channelsById.value.get(accountID) : undefined
  }

  function channelName(accountID: number | undefined): string {
    if (!accountID) return '-'
    return channelsById.value.get(accountID)?.name || `#${accountID}`
  }

  /** 原因码 → 文案；不认识的码原样带出，不吞掉 */
  function reasonLabel(reason: string | undefined): string {
    const key = `admin.scheduling.reasons.${reason}`
    return reason && te(key) ? t(key) : t('admin.scheduling.reasons.other', { reason: reason || '-' })
  }

  function labelOr(prefix: string, code: string | undefined): string {
    const key = `${prefix}.${code}`
    return code && te(key) ? t(key) : code || '-'
  }

  function protocolLabel(protocol: string | undefined): string {
    return labelOr('admin.scheduling.protocols', protocol)
  }

  return { t, channelOf, channelName, reasonLabel, labelOr, protocolLabel }
}
