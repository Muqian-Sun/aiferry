/**
 * 系统设置页的全部状态与逻辑。
 *
 * A6（2026-09-25）拆小节时把原 SettingsView.vue 的 <script setup> 整体搬到这里，逻辑未改（只删了页签切换）；
 * SettingsView 调用一次后 provide，各小节组件（settings/sections/*Section.vue）用 useSettingsPageContext() 取用。
 * 返回值里只放模板实际用到的绑定（由 Vue 编译器分析各小节模板得出）。
 *
 * 上线收口 P4（2026-09-27）：冷却 / 流超时 / 整流 / Beta 与 Fast 策略 / 转发行为 / Claude Code 与 Codex /
 * Grok / 调度阈值 / identity patch / 上游余额探测 / Ollama Cloud 用量全部写进后端代码，这里的状态、加载、保存一并删掉。
 * 剩下的都在总表单里（利润门、风控开关）或随总表单一起保存（联网搜索模拟），不再有走独立接口单独保存的卡片。
 */
import { ref, reactive, computed, onMounted, watch, inject, type InjectionKey, nextTick, type Ref } from "vue";
import type { SettingsSectionKey } from "./sections";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import type {
  SystemSettings,
  UpdateSettingsRequest,
  WebSearchEmulationConfig,
  WebSearchProviderConfig,
  WebSearchTestResult,
} from "@/api/admin/settings";
import type { Proxy } from "@/types";
import { extractApiErrorMessage } from "@/utils/apiError";
import { useAppStore } from "@/stores";

export function useSettingsPage(currentSection: Ref<SettingsSectionKey>) {
  const { t } = useI18n();
  const appStore = useAppStore();

  const loading = ref(true);
  const loadFailed = ref(false);
  const saving = ref(false);

  type SettingsForm = Omit<
    SystemSettings,
    // A6-4：这几项挪到了功能页（渠道健康 / 审查），设置页不再读写
    | "channel_monitor_hide_throughput"
    | "channel_monitor_hide_user_ranking"
    | "cyber_session_block_enabled"
    | "cyber_session_block_ttl_seconds"
    // 只读：取部署配置 OPS_ENABLED，侧栏据它显示运维入口，设置页不读写
    | "ops_monitoring_enabled"
  >;

  const form = reactive<SettingsForm>({
    risk_control_enabled: false,
    // Ops monitoring (vNext)
    ops_realtime_monitoring_enabled: true,
    ops_query_mode_default: "auto",
    ops_metrics_interval_seconds: 60,
    // 利润门（全站一档）
    profit_control_enabled: false,
    profit_min_margin: 0,
    profit_safety_buffer: 0,
  });

  // Proxies for web search emulation ProxySelector
  const webSearchProxies = ref<Proxy[]>([]);

  // Web Search Emulation config (loaded/saved separately)
  const DEFAULT_WEB_SEARCH_QUOTA_LIMIT = 1000;

  const webSearchConfig = reactive<WebSearchEmulationConfig>({
    enabled: false,
    providers: [],
  });

  const expandedProviders = reactive<Record<number, boolean>>({});
  const apiKeyVisible = reactive<Record<number, boolean>>({});
  const wsTestQuery = ref("");
  const wsTestLoading = ref(false);
  const wsTestResult = ref<WebSearchTestResult | null>(null);
  const wsTestDialogOpen = ref(false);

  function openTestDialog() {
    wsTestResult.value = null;
    wsTestDialogOpen.value = true;
  }

  function toggleProviderExpand(idx: number) {
    expandedProviders[idx] = !expandedProviders[idx];
  }

  function removeWebSearchProvider(idx: number) {
    webSearchConfig.providers.splice(idx, 1);
    // Re-index expandedProviders and apiKeyVisible after removal
    const newExpanded: Record<number, boolean> = {};
    const newVisible: Record<number, boolean> = {};
    for (let i = 0; i < webSearchConfig.providers.length; i++) {
      const oldIdx = i >= idx ? i + 1 : i;
      newExpanded[i] = expandedProviders[oldIdx] ?? false;
      newVisible[i] = apiKeyVisible[oldIdx] ?? false;
    }
    Object.keys(expandedProviders).forEach(
      (k) => delete expandedProviders[Number(k)],
    );
    Object.keys(apiKeyVisible).forEach((k) => delete apiKeyVisible[Number(k)]);
    Object.assign(expandedProviders, newExpanded);
    Object.assign(apiKeyVisible, newVisible);
  }

  function addWebSearchProvider() {
    const idx = webSearchConfig.providers.length;
    webSearchConfig.providers.push({
      type: "brave",
      api_key: "",
      api_key_configured: false,
      quota_limit: DEFAULT_WEB_SEARCH_QUOTA_LIMIT,
      subscribed_at: null,
      proxy_id: null,
      expires_at: null,
    } as WebSearchProviderConfig);
    expandedProviders[idx] = true;
  }

  function formatSubscribedAt(ts: number | null): string {
    if (!ts) return "";
    // Use UTC to avoid timezone drift on repeated edits
    const d = new Date(ts * 1000);
    const y = d.getUTCFullYear();
    const m = String(d.getUTCMonth() + 1).padStart(2, "0");
    const day = String(d.getUTCDate()).padStart(2, "0");
    return `${y}-${m}-${day}`;
  }

  function parseSubscribedAt(dateStr: string): number | null {
    if (!dateStr) return null;
    // Parse as UTC to match formatSubscribedAt
    return Math.floor(new Date(dateStr + "T00:00:00Z").getTime() / 1000);
  }

  function quotaPercentage(provider: WebSearchProviderConfig): number {
    if (!provider.quota_limit || provider.quota_limit <= 0) return 0;
    return ((provider.quota_used ?? 0) / provider.quota_limit) * 100;
  }

  async function resetWebSearchUsage(idx: number) {
    const provider = webSearchConfig.providers[idx];
    if (!provider) return;
    if (!confirm(t("admin.settings.webSearchEmulation.resetUsageConfirm")))
      return;
    try {
      await adminAPI.settings.resetWebSearchUsage({
        provider_type: provider.type,
      });
      provider.quota_used = 0;
      appStore.showSuccess(
        t("admin.settings.webSearchEmulation.resetUsageSuccess"),
      );
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    }
  }

  async function copyApiKey(idx: number) {
    const key = webSearchConfig.providers[idx]?.api_key;
    if (!key) {
      appStore.showError(
        t("admin.settings.webSearchEmulation.apiKeyPlaceholder"),
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(key);
      appStore.showSuccess(t("admin.settings.webSearchEmulation.copied"));
    } catch {
      appStore.showError(t("common.error"));
    }
  }

  async function testWebSearchProvider() {
    wsTestLoading.value = true;
    wsTestResult.value = null;
    try {
      const query =
        wsTestQuery.value.trim() ||
        t("admin.settings.webSearchEmulation.testDefaultQuery");
      wsTestResult.value = await adminAPI.settings.testWebSearchEmulation(query);
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    } finally {
      wsTestLoading.value = false;
    }
  }

  async function loadWebSearchConfig() {
    try {
      const [resp, proxiesResp] = await Promise.all([
        adminAPI.settings.getWebSearchEmulationConfig(),
        adminAPI.proxies.list().catch(() => ({ items: [] as Proxy[] })),
      ]);
      if (resp) {
        webSearchConfig.enabled = resp.enabled || false;
        webSearchConfig.providers = resp.providers || [];
      }
      webSearchProxies.value = proxiesResp.items || [];
    } catch (err: unknown) {
      // 404 is expected when config hasn't been created yet; show error for other failures
      const status = (err as { status?: number })?.status;
      if (status !== 404 && status !== undefined) {
        appStore.showError(extractApiErrorMessage(err, t("common.error")));
      }
    }
  }

  async function saveWebSearchConfig(): Promise<boolean> {
    try {
      for (const p of webSearchConfig.providers) {
        const raw = p.quota_limit;
        if (raw != null && Number(raw) !== 0 && Number(raw) < 1) {
          appStore.showError(
            t("admin.settings.webSearchEmulation.quotaLimitMustBePositive"),
          );
          return false;
        }
      }
      const providers = webSearchConfig.providers.map(
        (p: WebSearchProviderConfig) => ({
          ...p,
          quota_limit: Number(p.quota_limit) > 0 ? Number(p.quota_limit) : null,
        }),
      );
      await adminAPI.settings.updateWebSearchEmulationConfig({
        enabled: webSearchConfig.enabled,
        providers,
      });
      return true;
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
      return false;
    }
  }

  async function loadSettings() {
    loading.value = true;
    loadFailed.value = false;
    try {
      const settings = await adminAPI.settings.getSettings();
      // Only assign non-null values from backend (null means unconfigured, keep defaults)
      for (const [key, value] of Object.entries(settings)) {
        if (value !== null && value !== undefined) {
          (form as Record<string, unknown>)[key] = value;
        }
      }

      // Load web search emulation config separately
      await loadWebSearchConfig();
    } catch (error: unknown) {
      loadFailed.value = true;
      appStore.showError(
        extractApiErrorMessage(error, t("admin.settings.failedToLoad")),
      );
    } finally {
      loading.value = false;
    }
  }

  async function saveSettings(): Promise<boolean> {
    saving.value = true;
    try {
      const payload: UpdateSettingsRequest = {
        risk_control_enabled: form.risk_control_enabled,
        profit_control_enabled: form.profit_control_enabled,
        profit_min_margin: form.profit_min_margin,
        profit_safety_buffer: form.profit_safety_buffer,
      };

      const updated = await adminAPI.settings.updateSettings(payload);
      for (const [key, value] of Object.entries(updated)) {
        if (value !== null && value !== undefined) {
          (form as Record<string, unknown>)[key] = value;
        }
      }
      // Save web search emulation config separately (errors handled internally)
      const wsOk = await saveWebSearchConfig();
      // Refresh cached settings so sidebar/header update immediately
      await appStore.fetchPublicSettings(true);
      if (wsOk) {
        appStore.showSuccess(t("admin.settings.settingsSaved"));
      }
      return wsOk;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(error, t("admin.settings.failedToSave")),
      );
      return false;
    } finally {
      saving.value = false;
    }
  }

  onMounted(async () => {
    await loadSettings();
    // 加载后的规整（watch / 子组件回写）都跑完再取基线，页面一打开不应显示「有未保存的修改」
    await nextTick();
    await nextTick();
    markClean();
  });

  // =========================
  // A6-2 每节保存
  // =========================
  // 同一时间只有当前小节可能有改动：切走时有改动会先问「放弃 / 留下」，放弃就恢复成已保存的值。
  // 所以保存某一节时照旧整份提交总表单（其它节都等于已保存值，结果等于只存这一节）。
  // （后端本身支持只发部分字段：没发送的值类型字段不写库，见 setting_handler_update.go omittedSettingKeys。）
  // 改动判断：总表单状态与「上次加载 / 保存后的基线」比较。

  /** 总表单保存时会读到的全部状态（联网搜索模拟走自己的接口，但跟总表单一起保存） */
  function mainState() {
    return {
      form,
      webSearchConfig,
    };
  }
  // 联网搜索的「已用额度」是即时操作（重置额度直接调接口），不算未保存的改动
  function serializeMain(): string {
    const state = mainState();
    return JSON.stringify({
      ...state,
      webSearchConfig: {
        enabled: state.webSearchConfig.enabled,
        providers: state.webSearchConfig.providers.map(({ quota_used: _used, ...rest }) => rest),
      },
    });
  }
  function restoreMain(saved: ReturnType<typeof mainState>) {
    const copy = JSON.parse(JSON.stringify(saved)) as ReturnType<typeof mainState>;
    Object.assign(form, copy.form);
    Object.assign(webSearchConfig, copy.webSearchConfig);
  }

  const baseline = ref<string | undefined>(undefined);
  let restorePoint: ReturnType<typeof mainState> | undefined;
  function markClean() {
    baseline.value = serializeMain();
    restorePoint = JSON.parse(JSON.stringify(mainState()));
  }

  const mainDirty = computed(() => baseline.value !== undefined && serializeMain() !== baseline.value);
  // 总表单的改动记在「改的时候所在的小节」上（只有当前小节能改）
  const mainDirtyOwner = ref<SettingsSectionKey | null>(null);
  watch(mainDirty, (dirty) => {
    mainDirtyOwner.value = dirty ? currentSection.value : null;
  });

  function isSectionDirty(key: SettingsSectionKey): boolean {
    return mainDirty.value && mainDirtyOwner.value === key;
  }

  const sectionSaving = ref(false);
  /** 保存当前小节：所有可改的项都在总表单里，整份提交（其它节等于已保存值） */
  async function saveSection(): Promise<void> {
    if (sectionSaving.value) return;
    sectionSaving.value = true;
    try {
      // 本节什么都没改时（输入框里回车）照旧整份提交，和拆页前一致
      if (await saveSettings()) {
        await nextTick();
        markClean();
      }
    } finally {
      sectionSaving.value = false;
    }
  }

  /** 放弃这一节的改动：恢复成上次加载 / 保存后的值（不重新请求） */
  function discardSection(key: SettingsSectionKey) {
    if (isSectionDirty(key) && restorePoint) restoreMain(restorePoint);
  }

return {
    addWebSearchProvider,
    apiKeyVisible,
    copyApiKey,
    discardSection,
    expandedProviders,
    form,
    formatSubscribedAt,
    isSectionDirty,
    loadFailed,
    loading,
    openTestDialog,
    parseSubscribedAt,
    quotaPercentage,
    removeWebSearchProvider,
    resetWebSearchUsage,
    saveSection,
    sectionSaving,
    t,
    testWebSearchProvider,
    toggleProviderExpand,
    webSearchConfig,
    webSearchProxies,
    wsTestDialogOpen,
    wsTestLoading,
    wsTestQuery,
    wsTestResult,
  }
}

export type SettingsPage = ReturnType<typeof useSettingsPage>

export const SETTINGS_PAGE_KEY: InjectionKey<SettingsPage> = Symbol('settings-page')

/** 小节组件取系统设置页的共享状态（只能在 SettingsView 之下使用） */
export function useSettingsPageContext(): SettingsPage {
  const page = inject(SETTINGS_PAGE_KEY)
  if (!page) throw new Error('useSettingsPageContext() must be used under SettingsView')
  return page
}
