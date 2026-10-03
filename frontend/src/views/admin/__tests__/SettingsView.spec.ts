import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";

import SettingsView from "../SettingsView.vue";

const {
  getSettings,
  updateSettings,
  getGroups,
  listProxies,
  fetchPublicSettings,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  getGroups: vi.fn(),
  listProxies: vi.fn(),
  fetchPublicSettings: vi.fn(),
}));

const localeRef = vi.hoisted(() => ({ value: "zh-CN" }));

vi.mock("@/api/admin", () => ({
  adminAPI: {
    settings: {
      getSettings,
      updateSettings,
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
  const translations: Record<string, string> = {};
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
  ops_monitoring_enabled: false,
  ops_realtime_monitoring_enabled: false,
  ops_query_mode_default: "auto",
  ops_metrics_interval_seconds: 60,
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

describe("admin SettingsView", () => {
  beforeEach(() => {
    getSettings.mockReset();
    updateSettings.mockReset();
    getGroups.mockReset();
    listProxies.mockReset();
    fetchPublicSettings.mockReset();
    localeRef.value = "zh-CN";

    getSettings.mockResolvedValue({ ...baseSettingsResponse });
    updateSettings.mockImplementation(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
    }));
    getGroups.mockResolvedValue([]);
    listProxies.mockResolvedValue({
      items: [],
    });
    fetchPublicSettings.mockResolvedValue(undefined);
  });

  it("loads and submits the site-wide minimum margin", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      profit_min_margin: 0.3,
    });

    const wrapper = mountView();

    await flushPromises();
    expect((wrapper.get('[data-testid="profit-control-min-margin"]').element as HTMLInputElement).value).toBe("0.3");
    await wrapper.get('[data-testid="profit-control-min-margin"]').setValue("0.25");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith({
      risk_control_enabled: false,
      profit_min_margin: 0.25,
    });
  });
});
