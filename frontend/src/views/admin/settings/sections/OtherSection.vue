<template>
  <div class="space-y-6">
    <!-- 最低毛利率（利润门，全站一档；填 0 = 关） -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.profitControl.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.profitControl.description") }}
        </p>
      </div>
      <div class="p-6">
        <div class="sm:max-w-xs">
          <label class="input-label">{{ t("admin.settings.profitControl.minMargin") }}</label>
          <input
            v-model.number="form.profit_min_margin"
            type="number"
            min="0"
            max="0.99"
            step="0.01"
            class="input"
            data-testid="profit-control-min-margin"
          />
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t("admin.settings.profitControl.hint") }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › 其它（上线收口 P4，2026-09-27）：网关的冷却、转发、客户端、上游探测都写进后端代码，
// 这里只剩最低毛利率（利润门，填 0 = 关），随总表单保存；联网搜索模拟 2026-10-04 删了（方案页第三版）。
import { useSettingsPageContext } from '../useSettingsPage'

const { form, t } = useSettingsPageContext()
</script>
