<template>
  <div class="space-y-6">
    <!-- Default Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.defaults.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.defaults.description") }}
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label
              class="mb-2 block text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.defaults.defaultBalance") }}
            </label>
            <input
              v-model.number="form.default_balance"
              type="number"
              step="0.01"
              min="0"
              class="input"
              placeholder="0.00"
            />
            <p class="mt-1.5 text-xs text-af-ink-3">
              {{ t("admin.settings.defaults.defaultBalanceHint") }}
            </p>
          </div>
          <div>
            <label
              class="mb-2 block text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.defaults.defaultConcurrency") }}
            </label>
            <input
              v-model.number="form.default_concurrency"
              type="number"
              min="1"
              class="input"
              placeholder="1"
            />
            <p class="mt-1.5 text-xs text-af-ink-3">
              {{ t("admin.settings.defaults.defaultConcurrencyHint") }}
            </p>
          </div>
          <div>
            <label
              class="mb-2 block text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.defaults.defaultUserRpmLimit") }}
            </label>
            <input
              v-model.number="form.default_user_rpm_limit"
              type="number"
              min="0"
              step="1"
              class="input"
              placeholder="0"
            />
            <p class="mt-1.5 text-xs text-af-ink-3">
              {{ t("admin.settings.defaults.defaultUserRpmLimitHint") }}
            </p>
          </div>
        </div>

        <div class="border-t border-af-hairline pt-4">
          <div class="mb-3 flex items-center justify-between">
            <div>
              <label class="font-medium text-af-ink">
                {{ t("admin.settings.defaults.defaultSubscriptions") }}
              </label>
              <p class="text-sm text-af-ink-3">
                {{
                  t("admin.settings.defaults.defaultSubscriptionsHint")
                }}
              </p>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="addDefaultSubscription"
              :disabled="subscriptionPlans.length === 0 || form.default_subscriptions.length >= 1"
            >
              {{ t("admin.settings.defaults.addDefaultSubscription") }}
            </button>
          </div>

          <div
            v-if="form.default_subscriptions.length === 0"
            class="rounded border border-dashed border-af-hairline-strong px-4 py-3 text-sm text-af-ink-3"
          >
            {{ t("admin.settings.defaults.defaultSubscriptionsEmpty") }}
          </div>

          <div v-else class="space-y-3">
            <div
              v-for="(item, index) in form.default_subscriptions"
              :key="`default-sub-${index}`"
              class="grid grid-cols-1 gap-3 rounded border border-af-hairline p-3 md:grid-cols-[1fr_160px_auto]"
            >
              <div>
                <label
                  class="mb-1 block text-xs font-medium text-af-ink-2"
                >
                  {{ t("admin.settings.defaults.subscriptionPlan") }}
                </label>
                <Select
                  v-model="item.plan_id"
                  class="default-sub-plan-select"
                  :options="defaultSubscriptionPlanOptions"
                  :placeholder="t('admin.settings.defaults.subscriptionPlan')"
                />
              </div>
              <div>
                <label
                  class="mb-1 block text-xs font-medium text-af-ink-2"
                >
                  {{
                    t("admin.settings.defaults.subscriptionValidityDays")
                  }}
                </label>
                <input
                  v-model.number="item.validity_days"
                  type="number"
                  min="1"
                  max="36500"
                  class="input h-[42px]"
                />
              </div>
              <div class="flex items-end">
                <button
                  type="button"
                  class="btn btn-secondary default-sub-delete-btn w-full text-af-danger hover:text-af-danger"
                  @click="removeDefaultSubscription(index)"
                >
                  {{ t("common.delete") }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.authSourceDefaults.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.authSourceDefaults.description") }}
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div
          class="flex items-center justify-between rounded border border-af-hairline px-4 py-3"
        >
          <div>
            <label class="font-medium text-af-ink">
              {{ t("admin.settings.authSourceDefaults.requireEmailLabel") }}
            </label>
            <p class="text-sm text-af-ink-3">
              {{ t("admin.settings.authSourceDefaults.requireEmailHint") }}
            </p>
          </div>
          <Toggle v-model="form.force_email_on_third_party_signup" />
        </div>

        <div class="space-y-4">
          <div
            v-for="authSource in authSourceDefaultsMeta"
            :key="authSource.source"
            class="rounded-xl border border-af-hairline p-4"
          >
            <div class="flex items-center justify-between gap-4">
              <div>
                <div class="font-medium text-af-ink">
                  {{ authSource.title }}
                </div>
                <p class="mt-1 text-sm text-af-ink-3">
                  {{ authSource.description }}
                </p>
              </div>
              <Toggle
                v-model="
                  authSourceDefaults[authSource.source].grant_on_signup
                "
                :data-testid="`auth-source-${authSource.source}-enabled`"
              />
            </div>

            <div
              v-if="authSourceDefaults[authSource.source].grant_on_signup"
              :data-testid="`auth-source-${authSource.source}-panel`"
              class="mt-4 space-y-4 border-t border-af-hairline pt-4"
            >
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.authSourceDefaults.enabledHint") }}
              </p>

              <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div>
                  <label
                    class="mb-2 block text-sm font-medium text-af-ink-2"
                  >
                    {{ t("admin.settings.defaults.defaultBalance") }}
                  </label>
                  <input
                    v-model.number="
                      authSourceDefaults[authSource.source].balance
                    "
                    type="number"
                    step="0.01"
                    min="0"
                    class="input"
                    placeholder="0.00"
                  />
                </div>
                <div>
                  <label
                    class="mb-2 block text-sm font-medium text-af-ink-2"
                  >
                    {{ t("admin.settings.defaults.defaultConcurrency") }}
                  </label>
                  <input
                    v-model.number="
                      authSourceDefaults[authSource.source].concurrency
                    "
                    type="number"
                    min="1"
                    class="input"
                    placeholder="5"
                  />
                </div>
              </div>

              <div
                class="flex items-center justify-between rounded border border-af-hairline px-4 py-3"
              >
                <div>
                  <label
                    class="font-medium text-af-ink"
                  >
                    {{ t("admin.settings.authSourceDefaults.grantOnFirstBindLabel") }}
                  </label>
                  <p
                    class="mt-0.5 text-xs text-af-ink-3"
                  >
                    {{ t("admin.settings.authSourceDefaults.grantOnFirstBindHint") }}
                  </p>
                </div>
                <Toggle
                  v-model="
                    authSourceDefaults[authSource.source]
                      .grant_on_first_bind
                  "
                />
              </div>

              <div class="mb-3 flex items-center justify-between">
                <div>
                  <label
                    class="font-medium text-af-ink"
                  >
                    {{ t("admin.settings.authSourceDefaults.defaultSubscriptionsLabel") }}
                  </label>
                  <p class="text-sm text-af-ink-3">
                    {{ t("admin.settings.authSourceDefaults.defaultSubscriptionsHint") }}
                  </p>
                </div>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="
                    addAuthSourceDefaultSubscription(authSource.source)
                  "
                  :disabled="subscriptionPlans.length === 0 || authSourceDefaults[authSource.source].subscriptions.length >= 1"
                >
                  {{
                    t("admin.settings.defaults.addDefaultSubscription")
                  }}
                </button>
              </div>

              <div
                v-if="
                  authSourceDefaults[authSource.source].subscriptions
                    .length === 0
                "
                class="rounded border border-dashed border-af-hairline-strong px-4 py-3 text-sm text-af-ink-3"
              >
                {{ t("admin.settings.authSourceDefaults.noSourceSubscriptions") }}
              </div>

              <div v-else class="space-y-3">
                <div
                  v-for="(item, index) in authSourceDefaults[
                    authSource.source
                  ].subscriptions"
                  :key="`${authSource.source}-sub-${index}`"
                  class="grid grid-cols-1 gap-3 rounded border border-af-hairline p-3 md:grid-cols-[1fr_160px_auto]"
                >
                  <div>
                    <label
                      class="mb-1 block text-xs font-medium text-af-ink-2"
                    >
                      {{ t("admin.settings.defaults.subscriptionPlan") }}
                    </label>
                    <Select
                      v-model="item.plan_id"
                      class="default-sub-plan-select"
                      :options="defaultSubscriptionPlanOptions"
                      :placeholder="t('admin.settings.defaults.subscriptionPlan')"
                    />
                  </div>
                  <div>
                    <label
                      class="mb-1 block text-xs font-medium text-af-ink-2"
                    >
                      {{
                        t(
                          "admin.settings.defaults.subscriptionValidityDays",
                        )
                      }}
                    </label>
                    <input
                      v-model.number="item.validity_days"
                      type="number"
                      min="1"
                      max="36500"
                      class="input h-[42px]"
                    />
                  </div>
                  <div class="flex items-end">
                    <button
                      type="button"
                      class="btn btn-secondary w-full text-af-danger hover:text-af-danger"
                      @click="
                        removeAuthSourceDefaultSubscription(
                          authSource.source,
                          index,
                        )
                      "
                    >
                      {{ t("common.delete") }}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › defaults（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  addAuthSourceDefaultSubscription,
  addDefaultSubscription,
  authSourceDefaults,
  authSourceDefaultsMeta,
  defaultSubscriptionPlanOptions,
  form,
  removeAuthSourceDefaultSubscription,
  removeDefaultSubscription,
  subscriptionPlans,
  t
} = useSettingsPageContext()
</script>

<style scoped>
.default-sub-group-select :deep(.select-trigger) {
  @apply h-[42px];
}

.default-sub-delete-btn {
  @apply h-[42px];
}
</style>
