/**
 * 系统设置页的状态与逻辑。
 *
 * 后台能改的只剩两项（2026-09-27 上线收口 P4 之后）：最低毛利率（利润门，填 0 = 不能亏本）与审查总开关；
 * 其余网关、站点、注册、支付等都写进后端代码或部署配置。2026-10-05 去掉小节与二级导航，一页两行、一个保存栏。
 *
 * 保存只发改过的字段，后端只写请求里带的键（setting_handler_update.go）：两个管理员同时改不同的项时，
 * 后保存的不会把先保存的改回去（2026-10-04 D7）。
 */
import { ref, reactive, computed, onMounted, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import type { SystemSettings, UpdateSettingsRequest } from "@/api/admin/settings";
import { extractApiErrorMessage, extractI18nErrorMessage } from "@/utils/apiError";
import { useAppStore } from "@/stores";

/** 设置页能改的字段 */
const EDITABLE_FIELDS = ["profit_min_margin", "risk_control_enabled"] as const;
type EditableField = (typeof EDITABLE_FIELDS)[number];
type SettingsForm = Pick<SystemSettings, EditableField>;

export function useSettingsPage() {
  const { t } = useI18n();
  const appStore = useAppStore();

  const loading = ref(true);
  const loadFailed = ref(false);
  const saving = ref(false);
  // 加载 / 保存的结果要在页面上看得见（2026-10-04 体验诊断 H4：原来失败只打控制台）
  const loadError = ref("");
  const saveError = ref("");
  const justSaved = ref(false);
  let justSavedTimer: ReturnType<typeof setTimeout> | undefined;

  const form = reactive<SettingsForm>({
    profit_min_margin: 0,
    risk_control_enabled: false,
  });

  /** 上次加载 / 保存后的值：判断有没有改动、放弃时恢复、保存时只发改过的字段 */
  const saved = ref<SettingsForm | null>(null);

  function assignFrom(settings: Partial<SystemSettings>) {
    for (const field of EDITABLE_FIELDS) {
      const value = settings[field];
      // null 表示后端没配，保留默认值
      if (value !== null && value !== undefined) {
        (form as Record<string, unknown>)[field] = value;
      }
    }
  }

  function markClean() {
    saved.value = { ...form };
  }

  const changedFields = computed<EditableField[]>(() =>
    saved.value ? EDITABLE_FIELDS.filter((field) => form[field] !== saved.value?.[field]) : [],
  );
  const dirty = computed(() => changedFields.value.length > 0);

  async function load() {
    loading.value = true;
    loadFailed.value = false;
    loadError.value = "";
    try {
      assignFrom(await adminAPI.settings.getSettings());
      // 加载后的规整（Toggle / 输入框回写）跑完再取基线，页面一打开不应显示「有未保存的修改」
      await nextTick();
      markClean();
    } catch (error: unknown) {
      loadFailed.value = true;
      loadError.value = extractApiErrorMessage(error, t("admin.settings.failedToLoad"));
      console.error(loadError.value, error);
    } finally {
      loading.value = false;
    }
  }

  async function save(): Promise<void> {
    if (saving.value || !dirty.value) return;
    saving.value = true;
    saveError.value = "";
    justSaved.value = false;
    clearTimeout(justSavedTimer);
    try {
      const payload: UpdateSettingsRequest = {};
      for (const field of changedFields.value) {
        (payload as Record<string, unknown>)[field] = form[field];
      }
      assignFrom(await adminAPI.settings.updateSettings(payload));
      markClean();
      // 侧栏「审查」入口跟着公开设置走，保存后立刻刷新
      await appStore.fetchPublicSettings(true);
      justSaved.value = true;
      justSavedTimer = setTimeout(() => (justSaved.value = false), 2500);
    } catch (error: unknown) {
      // 后端带错误码的按码翻成中文（如最低毛利率越界），没有码的用后端原话
      saveError.value = extractI18nErrorMessage(error, t, "admin.settings.errors", t("admin.settings.failedToSave"));
      console.error(saveError.value, error);
    } finally {
      saving.value = false;
    }
  }

  /** 放弃改动：恢复成上次加载 / 保存后的值（不重新请求） */
  function discard() {
    if (saved.value) Object.assign(form, saved.value);
    saveError.value = "";
  }

  onMounted(load);

  return {
    dirty,
    discard,
    form,
    justSaved,
    load,
    loadError,
    loadFailed,
    loading,
    save,
    saveError,
    saving,
    t,
  };
}
