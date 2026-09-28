import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";

import SettingsView from "../SettingsView.vue";

const {
  getSettings,
  updateSettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  getOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  updateRateLimit429CooldownSettings,
  getStreamTimeoutSettings,
  getRectifierSettings,
  getBetaPolicySettings,
  getUpstreamBillingProbeSettings,
  updateUpstreamBillingProbeSettings,
  getOllamaCloudUsageSettings,
  updateOllamaCloudUsageSettings,
  getGroups,
  listProxies,
  fetchPublicSettings,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  getWebSearchEmulationConfig: vi.fn(),
  updateWebSearchEmulationConfig: vi.fn(),
  getOverloadCooldownSettings: vi.fn(),
  getRateLimit429CooldownSettings: vi.fn(),
  updateRateLimit429CooldownSettings: vi.fn(),
  getStreamTimeoutSettings: vi.fn(),
  getRectifierSettings: vi.fn(),
  getBetaPolicySettings: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({
    enabled: true,
    interval_minutes: 30,
  }),
  updateUpstreamBillingProbeSettings: vi.fn().mockImplementation(async (payload) => payload),
  getOllamaCloudUsageSettings: vi.fn().mockResolvedValue({
    enabled: false,
    interval_minutes: 60,
    debounce_minutes: 1,
  }),
  updateOllamaCloudUsageSettings: vi.fn().mockImplementation(async (payload) => payload),
  getGroups: vi.fn(),
  listProxies: vi.fn(),
  fetchPublicSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

const localeRef = vi.hoisted(() => ({ value: "zh-CN" }));

vi.mock("@/api/admin", () => ({
  adminAPI: {
    settings: {
      getSettings,
      updateSettings,
      getWebSearchEmulationConfig,
      updateWebSearchEmulationConfig,
      getOverloadCooldownSettings,
      getRateLimit429CooldownSettings,
      updateRateLimit429CooldownSettings,
      getStreamTimeoutSettings,
      getRectifierSettings,
      getBetaPolicySettings,
    },
    accounts: {
      getUpstreamBillingProbeSettings,
      updateUpstreamBillingProbeSettings,
      getOllamaCloudUsageSettings,
      updateOllamaCloudUsageSettings,
    },
    groups: {
      getAll: getGroups,
    },
    proxies: {
      list: listProxies,
    },
  },
}));

vi.mock("@/stores", () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning: vi.fn(),
    showInfo: vi.fn(),
    fetchPublicSettings,
  }),
}));

vi.mock("@/composables/useClipboard", () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn(),
  }),
}));

vi.mock("@/utils/apiError", () => ({
  extractApiErrorMessage: () => "error",
}));

// A6：当前小节来自路由 /settings/:section，导航调用 router.push 切换
vi.mock("vue-router", async () => {
  const { reactive } = await import("vue");
  const route = reactive({ params: {} as Record<string, string> });
  return {
    onBeforeRouteLeave: () => {},
    onBeforeRouteUpdate: () => {},
    useRoute: () => route,
    useRouter: () => ({
      push: async (to: { params?: Record<string, string> }) => {
        route.params = { ...(to.params ?? {}) };
      },
    }),
  };
});

vi.mock("vue-i18n", async () => {
  const actual = await vi.importActual<typeof import("vue-i18n")>("vue-i18n");
  const translations: Record<string, string> = {
    "admin.settings.upstreamBillingProbe.title": "上游倍率自动探测",
    "admin.settings.upstreamBillingProbe.description": "定期获取 OpenAI API Key 所连接上游 Sub2API 站点声明的计费倍率。",
    "admin.settings.upstreamBillingProbe.enabled": "启用全局自动探测",
    "admin.settings.upstreamBillingProbe.enabledHint": "开启后，仅对账号自身已启用自动检测的账号执行定时探测。",
    "admin.settings.upstreamBillingProbe.intervalMinutes": "探测周期（分钟）",
    "admin.settings.upstreamBillingProbe.intervalHint": "范围 5–1440 分钟。",
    "admin.settings.upstreamBillingProbe.saved": "上游倍率自动探测设置已保存",
    "admin.settings.upstreamBillingProbe.saveFailed": "保存上游倍率自动探测设置失败",
    "admin.settings.openaiFastPolicy.summaryTargetModels": "目标模型",
    "admin.settings.openaiFastPolicy.summaryAllModels": "全部模型",
    "admin.settings.openaiFastPolicy.summaryOtherModels": "其他模型",
    "admin.settings.openaiFastPolicy.summaryAction.filter": "过滤",
    "admin.settings.openaiFastPolicy.summaryAction.pass": "透传",
  };
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) =>
        (translations[key] ?? key).replace(/\{(\w+)\}/g, (_, token) => params?.[token] ?? `{${token}}`),
      locale: localeRef,
    }),
  };
});

const AppLayoutStub = { template: "<div><slot /></div>" };
const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  inheritAttrs: false,
  setup(props, { attrs, emit }) {
    return () =>
      h("input", {
        ...attrs,
        class: "toggle-stub",
        type: "checkbox",
        checked: props.modelValue,
        onChange: (event: Event) => {
          emit("update:modelValue", (event.target as HTMLInputElement).checked);
        },
      });
  },
});

const SelectStub = defineComponent({
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: "",
    },
    options: {
      type: Array,
      default: () => [],
    },
    placeholder: {
      type: String,
      default: "",
    },
  },
  emits: ["update:modelValue", "change"],
  setup(props, { emit }) {
    const onChange = (event: Event) => {
      const target = event.target as HTMLSelectElement;
      emit("update:modelValue", target.value);
      const option =
        (props.options as Array<Record<string, unknown>>).find(
          (item) => String(item.value ?? "") === target.value,
        ) ?? null;
      emit("change", target.value, option);
    };

    return () =>
      h(
        "select",
        {
          class: "select-stub",
          value: props.modelValue ?? "",
          "data-placeholder": props.placeholder,
          onChange,
        },
        (props.options as Array<Record<string, unknown>>).map((option) =>
          h(
            "option",
            {
              key: `${String(option.value ?? "")}:${String(option.label ?? "")}`,
              value: option.value as string,
            },
            String(option.label ?? ""),
          ),
        ),
      );
  },
});

const baseSettingsResponse = {
  grok_default_text_model: "grok-4.5",
  grok_cross_client_model_map_enabled: false,
  enable_identity_patch: false,
  identity_patch_prompt: "",
  ops_monitoring_enabled: false,
  ops_realtime_monitoring_enabled: false,
  ops_query_mode_default: "auto",
  ops_metrics_interval_seconds: 60,
  min_claude_code_version: "",
  max_claude_code_version: "",
  openai_ttft_mode: "semantic",
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_cch_signing: false,
  enable_claude_oauth_system_prompt_injection: true,
  claude_oauth_system_prompt: "",
  claude_oauth_system_prompt_blocks: "",
  enable_anthropic_cache_ttl_1h_injection: false,
  rewrite_message_cache_control: false,
  enable_client_dateline_normalization: true,
  antigravity_user_agent_version: "",
  openai_codex_user_agent: "",
};

function mountView() {
  return mount(SettingsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Select: SelectStub,
        Toggle: ToggleStub,
        Icon: true,
        ConfirmDialog: true,
        ProxySelector: true,
      },
    },
  });
}

async function openGatewayTab(wrapper: ReturnType<typeof mountView>) {
  const gatewayTabButton = wrapper
    .findAll("button")
    .find((node) => node.text().includes("admin.settings.sections.upstream.title"));

  expect(gatewayTabButton).toBeDefined();
  await gatewayTabButton?.trigger("click");
  await flushPromises();
}

describe("admin SettingsView", () => {
  beforeEach(() => {
    getSettings.mockReset();
    updateSettings.mockReset();
    getWebSearchEmulationConfig.mockReset();
    updateWebSearchEmulationConfig.mockReset();
    getOverloadCooldownSettings.mockReset();
    getRateLimit429CooldownSettings.mockReset();
    updateRateLimit429CooldownSettings.mockReset();
    getStreamTimeoutSettings.mockReset();
    getRectifierSettings.mockReset();
    getBetaPolicySettings.mockReset();
    getUpstreamBillingProbeSettings.mockReset();
    updateUpstreamBillingProbeSettings.mockReset();
    getOllamaCloudUsageSettings.mockReset();
    updateOllamaCloudUsageSettings.mockReset();
    getGroups.mockReset();
    listProxies.mockReset();
    fetchPublicSettings.mockReset();
    showError.mockReset();
    showSuccess.mockReset();
    localeRef.value = "zh-CN";

    getSettings.mockResolvedValue({ ...baseSettingsResponse });
    updateSettings.mockImplementation(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
    }));
    getWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    updateWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    getOverloadCooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_minutes: 10,
    });
    getRateLimit429CooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_seconds: 5,
    });
    updateRateLimit429CooldownSettings.mockImplementation(async (payload) => payload);
    getStreamTimeoutSettings.mockResolvedValue({
      enabled: true,
      action: "temp_unsched",
      temp_unsched_minutes: 5,
      threshold_count: 3,
      threshold_window_minutes: 10,
    });
    getRectifierSettings.mockResolvedValue({
      enabled: true,
      thinking_signature_enabled: true,
      thinking_budget_enabled: true,
      apikey_signature_enabled: false,
      apikey_signature_patterns: [],
    });
    getBetaPolicySettings.mockResolvedValue({
      rules: [],
    });
    getUpstreamBillingProbeSettings.mockResolvedValue({
      enabled: true,
      interval_minutes: 30,
    });
    updateUpstreamBillingProbeSettings.mockImplementation(async (payload) => payload);
    getOllamaCloudUsageSettings.mockResolvedValue({
      enabled: false,
      interval_minutes: 60,
      debounce_minutes: 1,
    });
    updateOllamaCloudUsageSettings.mockImplementation(async (payload) => payload);
    getGroups.mockResolvedValue([]);
    listProxies.mockResolvedValue({
      items: [],
    });
    fetchPublicSettings.mockResolvedValue(undefined);
  });

  it("loads and submits the site-wide profit gate settings", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      profit_control_enabled: true,
      profit_min_margin: 0.3,
      profit_safety_buffer: 0.05,
    });

    const wrapper = mountView();

    await flushPromises();
    expect((wrapper.get('[data-testid="profit-control-enabled"]').element as HTMLInputElement).checked).toBe(true);
    expect((wrapper.get('[data-testid="profit-control-min-margin"]').element as HTMLInputElement).value).toBe("0.3");
    await wrapper.get('[data-testid="profit-control-safety-buffer"]').setValue("0.1");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        profit_control_enabled: true,
        profit_min_margin: 0.3,
        profit_safety_buffer: 0.1,
      }),
    );
  });

  it("submits Anthropic cache TTL injection gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      enable_anthropic_cache_ttl_1h_injection: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enable_anthropic_cache_ttl_1h_injection: true,
      }),
    );
  });

  it("submits message cache_control rewrite gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      rewrite_message_cache_control: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        rewrite_message_cache_control: true,
      }),
    );
  });

  it("submits Claude OAuth system prompt injection gateway settings", async () => {
    const blocks = `[{"type":"text","text":"custom block","cache_control":true}]`;
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      enable_claude_oauth_system_prompt_injection: false,
      claude_oauth_system_prompt_blocks: blocks,
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enable_claude_oauth_system_prompt_injection: false,
      }),
    );
    const payload = updateSettings.mock.calls[0][0] as {
      claude_oauth_system_prompt_blocks: string;
    };
    expect(JSON.parse(payload.claude_oauth_system_prompt_blocks)).toEqual([
      {
        enabled: true,
        type: "text",
        text: "custom block",
        cache_control: {
          type: "ephemeral",
          ttl: "5m",
        },
      },
    ]);
  });

  it("submits Antigravity user agent version gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      antigravity_user_agent_version: "1.23.2",
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        antigravity_user_agent_version: "1.23.2",
      }),
    );
  });

  it("summarizes target and other-model actions, then switches to all models", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      openai_fast_policy_settings: {
        rules: [
          {
            service_tier: "all",
            action: "filter",
            scope: "all",
            model_whitelist: ["gpt-5.6-sol"],
            fallback_action: "pass",
          },
        ],
      },
    });
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const summary = wrapper.get('[data-testid="openai-fast-policy-summary-0"]');
    expect(summary.text()).toContain("目标模型");
    expect(summary.text()).toContain("过滤");
    expect(summary.text()).toContain("其他模型");
    expect(summary.text()).toContain("透传");

    await wrapper
      .get(
        '[role="group"][aria-labelledby="openai-fast-policy-models-label-0"] input[type="text"]',
      )
      .setValue("");
    expect(summary.text()).toContain("全部模型");
    expect(summary.text()).toContain("过滤");
    expect(summary.text()).not.toContain("其他模型");
    expect(summary.text()).not.toContain("透传");
  });

  it("loads and saves upstream billing probe settings from the gateway tab", async () => {
    getUpstreamBillingProbeSettings.mockResolvedValueOnce({
      enabled: false,
      interval_minutes: 45,
    });

    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const card = wrapper.get('[data-testid="upstream-billing-probe-settings"]');
    expect(card.isVisible()).toBe(true);
    expect(card.text()).toContain("上游倍率自动探测");
    expect(
      (card.get('[data-testid="upstream-billing-probe-enabled"]').element as HTMLInputElement)
        .checked,
    ).toBe(false);
    expect(card.find('[data-testid="upstream-billing-probe-interval"]').exists()).toBe(false);

    await card.get('[data-testid="upstream-billing-probe-enabled"]').setValue(true);
    await card.get('[data-testid="upstream-billing-probe-interval"]').setValue(60);
    await wrapper.get('[data-testid="settings-section-upstream"]').trigger("submit.prevent");
    await flushPromises();

    expect(updateUpstreamBillingProbeSettings).toHaveBeenCalledWith({
      enabled: true,
      interval_minutes: 60,
    });
    expect(showSuccess).toHaveBeenCalledWith("上游倍率自动探测设置已保存");
  });

  it("loads and saves configurable Grok cross-client model mapping", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      grok_default_text_model: "grok-4.1-fast",
      grok_cross_client_model_map_enabled: true,
    });
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const modelInput = wrapper.get('[data-testid="grok-default-text-model"]');
    const mappingToggle = wrapper.get(
      '[data-testid="grok-cross-client-model-map-toggle"]',
    );
    expect((modelInput.element as HTMLInputElement).value).toBe("grok-4.1-fast");
    expect((mappingToggle.element as HTMLInputElement).checked).toBe(true);

    await modelInput.setValue("grok-custom-text");
    await mappingToggle.setValue(false);
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    const payload = updateSettings.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(payload.grok_default_text_model).toBe("grok-custom-text");
    expect(payload.grok_cross_client_model_map_enabled).toBe(false);
  });

  it("loads and saves the OpenAI Responses first-token metric mode", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      openai_ttft_mode: "visible",
    });
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const modeSelect = wrapper.get('[data-testid="openai-ttft-mode"]');
    expect((modeSelect.element as HTMLSelectElement).value).toBe("visible");

    await modeSelect.setValue("semantic");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    const payload = updateSettings.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(payload.openai_ttft_mode).toBe("semantic");
  });

  it("loads fail-safe-off Ollama Cloud usage refresh settings and saves an explicit opt-in", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const card = wrapper.get('[data-testid="ollama-cloud-usage-global-settings"]');
    expect(card.isVisible()).toBe(true);
    expect(
      (card.get('[data-testid="ollama-cloud-usage-global-enabled"]').element as HTMLInputElement)
        .checked,
    ).toBe(false);
    expect(card.find('[data-testid="ollama-cloud-usage-global-interval"]').exists()).toBe(false);

    await card.get('[data-testid="ollama-cloud-usage-global-enabled"]').setValue(true);
    await card.get('[data-testid="ollama-cloud-usage-global-debounce"]').setValue(3);
    await card.get('[data-testid="ollama-cloud-usage-global-interval"]').setValue(90);
    await wrapper.get('[data-testid="settings-section-upstream"]').trigger("submit.prevent");
    await flushPromises();

    expect(updateOllamaCloudUsageSettings).toHaveBeenCalledWith({
      enabled: true,
      interval_minutes: 90,
      debounce_minutes: 3,
    });
  });
});
