/**
 * 上架提示（2026-10-04 D6，muqian 定：不拦上架，但上架时提示、模型列表标「没有能调度的渠道」）。
 * 能不能派到请求由后端按调度器同一口径算好（列表接口的 schedulable_channels / unschedulable_reason）。
 */
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

type Translate = (key: string, values?: Record<string, unknown>) => string

/** 没有一个渠道能派到请求（列表接口没带这个字段时按「能派」处理，不误报） */
export function hasNoSchedulableChannel(entry: Pick<ModelCatalogEntry, 'schedulable_channels'>): boolean {
  return entry.schedulable_channels === 0
}

/** 派不到的原因（一句话）；原因不认识时给通用说法 */
export function unschedulableReasonText(entry: Pick<ModelCatalogEntry, 'unschedulable_reason'>, t: Translate): string {
  const reason = entry.unschedulable_reason
  return reason && ['no_bindings', 'channels_disabled', 'profit_gate'].includes(reason)
    ? t(`admin.modelCatalog.unschedulable.reasons.${reason}`)
    : t('admin.modelCatalog.unschedulable.reasons.unknown')
}

/** 上架确认里的模型清单：「gpt-5.5（承接的渠道毛利都低于最低毛利率…）、…」 */
export function unschedulableListText(entries: Array<Pick<ModelCatalogEntry, 'model_id' | 'unschedulable_reason'>>, t: Translate): string {
  return entries
    .map((entry) => t('admin.modelCatalog.unschedulable.item', { model: entry.model_id, reason: unschedulableReasonText(entry, t) }))
    .join(t('admin.modelCatalog.unschedulable.separator'))
}
