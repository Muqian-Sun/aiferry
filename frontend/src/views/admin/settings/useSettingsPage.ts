/**
 * 系统设置页的全部状态与逻辑。
 *
 * A6（2026-09-25）拆小节时把原 SettingsView.vue 的 <script setup> 整体搬到这里，逻辑未改（只删了页签切换）；
 * SettingsView 调用一次后 provide，各小节组件（settings/sections/*Section.vue）用 useSettingsPageContext() 取用。
 * 返回值里只放模板实际用到的绑定（由 Vue 编译器分析各小节模板得出）。
 *
 * 上线收口 P4（2026-09-27）：冷却 / 流超时 / 整流 / Beta 与 Fast 策略 / 转发行为 / Claude Code 与 Codex /
 * Grok / 调度阈值 / identity patch / 上游余额探测 / Ollama Cloud 用量全部写进后端代码，这里的状态、加载、保存一并删掉。
 * 剩下的都在总表单里（利润门、风控开关），不再有走独立接口单独保存的卡片。
 *
 * 2026-09-28：利润门只剩「最低毛利率」一个数（填 0 = 关）。
 * 2026-10-04：联网搜索模拟（Brave / Tavily）删了（方案页第三版），这里的服务商配置一并删掉。
 */
import { ref, reactive, computed, onMounted, watch, inject, type InjectionKey, nextTick, type Ref } from "vue";
import type { SettingsSectionKey } from "./sections";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import type { SystemSettings, UpdateSettingsRequest } from "@/api/admin/settings";
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
    // A6-4：这两项挪到了审查页，设置页不再读写
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
    // 利润门（全站一档）：最低毛利率，0 = 关
    profit_min_margin: 0,
  });

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
    } catch (error: unknown) {
      loadFailed.value = true;
      console.error(
        extractApiErrorMessage(error, t("admin.settings.failedToLoad")), error,
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
        profit_min_margin: form.profit_min_margin,
      };

      const updated = await adminAPI.settings.updateSettings(payload);
      for (const [key, value] of Object.entries(updated)) {
        if (value !== null && value !== undefined) {
          (form as Record<string, unknown>)[key] = value;
        }
      }
      // Refresh cached settings so sidebar/header update immediately
      await appStore.fetchPublicSettings(true);
      return true;
    } catch (error: unknown) {
      console.error(
        extractApiErrorMessage(error, t("admin.settings.failedToSave")), error,
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
  // （后端本身支持只发部分字段：没发送的字段沿用库里的旧值，见 setting_handler_update.go。）
  // 改动判断：总表单状态与「上次加载 / 保存后的基线」比较。

  /** 总表单保存时会读到的全部状态 */
  function mainState() {
    return { form };
  }
  function serializeMain(): string {
    return JSON.stringify(mainState());
  }
  function restoreMain(saved: ReturnType<typeof mainState>) {
    const copy = JSON.parse(JSON.stringify(saved)) as ReturnType<typeof mainState>;
    Object.assign(form, copy.form);
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
    discardSection,
    form,
    isSectionDirty,
    loadFailed,
    loading,
    saveSection,
    sectionSaving,
    t,
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
