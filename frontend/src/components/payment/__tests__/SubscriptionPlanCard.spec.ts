import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import type { SubscriptionPlan } from "@/types/payment";
import type { UserSubscription } from "@/types";
import SubscriptionPlanCard from "../SubscriptionPlanCard.vue";

const i18n = createI18n({
  legacy: false,
  locale: "en",
  fallbackWarn: false,
  missingWarn: false,
  messages: {
    en: {
      payment: {
        days: "days",
        weeks: "weeks",
        months: "months",
        perMonth: "month",
        models: "Models",
        planCard: {
          quota: "Quota",
          unlimited: "Unlimited",
          models: "Models",
          blockedByActive: "Blocked by active subscription",
        },
        subscribeNow: "Subscribe now",
        renewNow: "Renew now",
      },
    },
  },
});

const basePlan = (): SubscriptionPlan => ({
  id: 1,
  name: "Pro",
  description: "",
  price: 10,
  features: [],
  validity_days: 30,
  validity_unit: "day",
  for_sale: true,
  sort_order: 0,
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  models: [
    { entry_id: 199, model_id: "gpt-5.6", display_name: "GPT 5.6" },
    { entry_id: 27, model_id: "claude-sonnet-4-5", display_name: "" },
  ],
});

const activeSub = (planId: number, status: UserSubscription["status"] = "active"): UserSubscription =>
  ({ id: 1, user_id: 5, plan_id: planId, status, starts_at: "", expires_at: null,
    daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0,
    daily_window_start: null, weekly_window_start: null, monthly_window_start: null,
    created_at: "", updated_at: "" }) as UserSubscription;

const mountPlanCard = (overrides: Partial<SubscriptionPlan> = {}, activeSubscriptions?: UserSubscription[]) =>
  mount(SubscriptionPlanCard, {
    props: { plan: { ...basePlan(), ...overrides }, activeSubscriptions },
    global: { plugins: [i18n, createPinia()] },
  });

describe("SubscriptionPlanCard", () => {
  it("renders the plan model set by display name, falling back to model_id", () => {
    const models = mountPlanCard().get('[data-testid="plan-models"]').text();
    expect(models).toContain("GPT 5.6 / claude-sonnet-4-5");
  });

  it("shows the unlimited quota row when no limit is set, otherwise the limits", () => {
    expect(mountPlanCard().text()).toContain("payment.planCard.unlimited");
    const limited = mountPlanCard({ daily_limit_usd: 1.5, weekly_limit_usd: 7 }).text();
    expect(limited).toContain("$1.5");
    expect(limited).toContain("$7");
    expect(limited).not.toContain("payment.planCard.unlimited");
  });

  it("offers renewal for the plan the user already holds", () => {
    const wrapper = mountPlanCard({}, [activeSub(1)]);
    expect(wrapper.get('[data-testid="plan-select"]').text()).toBe("payment.renewNow");
    expect(wrapper.get('[data-testid="plan-select"]').attributes("disabled")).toBeUndefined();
    expect(wrapper.find('[data-testid="plan-blocked"]').exists()).toBe(false);
  });

  // 同一用户同一时间只能一条有效订阅：别的套餐有效时按钮禁用并提示
  it("disables purchase while another plan is active", () => {
    const wrapper = mountPlanCard({}, [activeSub(2)]);
    expect(wrapper.get('[data-testid="plan-select"]').attributes("disabled")).toBeDefined();
    expect(wrapper.get('[data-testid="plan-blocked"]').text()).toBe("payment.planCard.blockedByActive");
  });

  it("does not block when the other subscription is not active", () => {
    const wrapper = mountPlanCard({}, [activeSub(2, "expired")]);
    expect(wrapper.get('[data-testid="plan-select"]').attributes("disabled")).toBeUndefined();
    expect(wrapper.find('[data-testid="plan-blocked"]').exists()).toBe(false);
  });

  // #4607：管理端保存的单位是复数（months/weeks），此前用户侧只匹配单数
  // 'month'，「1 个月」的套餐卡片被显示成「1天」。测试环境的 vue-i18n 为
  // runtime-only 构建，t() 原样返回 key，故按 key 断言单位分支。
  it("renders plural admin-form validity units instead of mislabeled days (#4607)", () => {
    expect(mountPlanCard({ validity_days: 1, validity_unit: "months" }).text()).toContain("/ payment.perMonth");
    expect(mountPlanCard({ validity_days: 3, validity_unit: "months" }).text()).toContain("/ 3payment.months");
    expect(mountPlanCard({ validity_days: 2, validity_unit: "weeks" }).text()).toContain("/ 2payment.weeks");
    expect(mountPlanCard({ validity_days: 30, validity_unit: "day" }).text()).toContain("/ 30payment.days");
  });

  it("uses the configured currency symbol while preserving USD for legacy plans", () => {
    const cnyPlan = mountPlanCard({ currency: "CNY", original_price: 20 }).text();

    expect(cnyPlan).toContain("¥10CNY");
    expect(cnyPlan).toContain("¥20CNY");
    expect(mountPlanCard({ currency: "USD" }).text()).toContain("$10USD");
    expect(mountPlanCard({ currency: "" }).text()).toContain("$10");
  });

  it.each([
    ["long Chinese", "企业全球加速专业订阅套餐（含高级模型与优先支持）"],
    ["long English", "Enterprise Global Acceleration Subscription with Priority Support"],
    ["unbroken token", "EnterpriseGlobalAccelerationSubscriptionWithPrioritySupport1234567890"],
  ])("keeps the full %s plan title accessible in a bounded two-line area", (_label, name) => {
    const wrapper = mountPlanCard({ name });
    const title = wrapper.get("h3");

    expect(title.text()).toBe(name);
    expect(title.attributes("title")).toBe(name);
    expect(title.classes()).toEqual(expect.arrayContaining([
      "min-w-0",
      "break-words",
      "line-clamp-2",
      "[overflow-wrap:anywhere]",
    ]));
    expect(title.classes()).not.toContain("truncate");
  });

  it("keeps title, price, description, and purchase action in separate bounded regions", () => {
    const wrapper = mountPlanCard({
      name: "Enterprise Global Acceleration Subscription with Priority Support",
      price: 123.45,
      currency: "USD",
      description: "Includes advanced models and priority support.",
    });
    const title = wrapper.get("h3");
    const price = wrapper.findAll("span").find((node) => node.text() === "123.45");

    expect(title.element.parentElement?.classList).toContain("min-w-0");
    expect(title.element.parentElement?.classList).toContain("flex-1");
    expect(price?.element.parentElement?.parentElement?.classList).toContain("shrink-0");
    expect(price?.element.parentElement?.parentElement?.textContent).toContain("/ 30payment.days");
    expect(wrapper.get("p").text()).toBe("Includes advanced models and priority support.");
    expect(wrapper.get("button").text()).toBe("payment.subscribeNow");
  });
});
