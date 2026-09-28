/**
 * 系统设置页的全部状态与逻辑。
 *
 * A6（2026-09-25）拆小节时把原 SettingsView.vue 的 <script setup> 整体搬到这里，逻辑未改（只删了页签切换）；
 * SettingsView 调用一次后 provide，各小节组件（settings/sections/*Section.vue）用 useSettingsPageContext() 取用。
 * 返回值里只放模板实际用到的绑定（由 Vue 编译器分析各小节模板得出）。
 */
import { ref, reactive, computed, onMounted, watch, inject, type InjectionKey, nextTick, type Ref } from "vue";
import type { SettingsSectionKey } from "./sections";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import {
  normalizeAccountSchedulingThresholdsMap,
  sanitizeAccountSchedulingThresholdsMap,
  SCHEDULING_THRESHOLD_PLATFORMS,
} from "@/api/admin/settings";
import type {
  SystemSettings,
  UpdateSettingsRequest,
  OpenAIFastPolicyRule,
  WebSearchEmulationConfig,
  WebSearchProviderConfig,
  WebSearchTestResult,
} from "@/api/admin/settings";
import type { Proxy } from "@/types";
import { extractApiErrorMessage } from "@/utils/apiError";
import { useAppStore } from "@/stores";
import {
  parseFingerprintSignalsToRows,
  serializeFingerprintRowsToJSON,
  defaultFingerprintSignalRows,
  type FingerprintSignalRow,
} from "../codexFingerprintSignals";

export function useSettingsPage(currentSection: Ref<SettingsSectionKey>) {
  const { t } = useI18n();
  const appStore = useAppStore();

  const loading = ref(true);
  const loadFailed = ref(false);
  const saving = ref(false);

  // Upstream billing probe state
  const upstreamBillingProbeLoading = ref(true);
  const upstreamBillingProbeSaving = ref(false);
  const upstreamBillingProbeForm = reactive({
    enabled: true,
    interval_minutes: 30,
  });

  const ollamaCloudUsageLoading = ref(true);
  const ollamaCloudUsageSaving = ref(false);
  const ollamaCloudUsageForm = reactive({
    enabled: false,
    interval_minutes: 60,
    debounce_minutes: 1,
  });

  // Overload Cooldown (529) 状态
  const overloadCooldownLoading = ref(true);
  const overloadCooldownSaving = ref(false);
  const overloadCooldownForm = reactive({
    enabled: true,
    cooldown_minutes: 10,
  });

  // Rate Limit Cooldown (429) 状态
  const rateLimit429CooldownLoading = ref(true);
  const rateLimit429CooldownSaving = ref(false);
  const rateLimit429CooldownForm = reactive({
    enabled: true,
    cooldown_seconds: 5,
  });

  // Stream Timeout 状态
  const streamTimeoutLoading = ref(true);
  const streamTimeoutSaving = ref(false);
  const streamTimeoutForm = reactive({
    enabled: true,
    action: "temp_unsched" as "temp_unsched" | "error" | "none",
    temp_unsched_minutes: 5,
    threshold_count: 3,
    threshold_window_minutes: 10,
  });

  // Rectifier 状态
  const rectifierLoading = ref(true);
  const rectifierSaving = ref(false);
  const rectifierForm = reactive({
    enabled: true,
    thinking_signature_enabled: true,
    thinking_budget_enabled: true,
    apikey_signature_enabled: false,
    apikey_signature_patterns: [] as string[],
  });

  // Beta Policy 状态
  const betaPolicyLoading = ref(true);
  const betaPolicySaving = ref(false);
  const betaPolicyForm = reactive({
    rules: [] as Array<{
      beta_token: string;
      action: "pass" | "filter" | "block";
      scope: "all" | "oauth" | "apikey" | "bedrock";
      error_message?: string;
      model_whitelist?: string[];
      fallback_action?: "pass" | "filter" | "block";
      fallback_error_message?: string;
    }>,
  });

  // OpenAI Fast/Flex Policy 状态
  const openaiFastPolicyForm = reactive({
    rules: [] as OpenAIFastPolicyRule[],
  });
  // 标记 openai_fast_policy_settings 是否已成功从后端加载，
  // 避免后端 GET 出错或字段缺失时，保存把默认规则覆盖成空数组。
  const openaiFastPolicyLoaded = ref(false);

  type ClaudeOAuthSystemPromptPreset =
    | "billing"
    | "system"
    | "expansion"
    | "custom";

  interface ClaudeOAuthSystemPromptBlock {
    id: string;
    enabled: boolean;
    expanded: boolean;
    type: "text";
    preset: ClaudeOAuthSystemPromptPreset;
    text: string;
    cacheControlEnabled: boolean;
    cacheControlTTL: string;
  }

  interface ClaudeOAuthSystemPromptRawBlock {
    enabled?: boolean;
    type?: string;
    text?: string;
    cache_control?: unknown;
  }

  const defaultClaudeCodeSystemPrompt =
    "You are Claude Code, Anthropic's official CLI for Claude.";

  const defaultClaudeCodeExpansionPrompt = `You are an interactive agent that helps users with software engineering tasks. Use the instructions below and the tools available to you to assist the user.

IMPORTANT: Assist with authorized security testing, defensive security, CTF challenges, and educational contexts. Refuse requests for destructive techniques, DoS attacks, mass targeting, supply chain compromise, or detection evasion for malicious purposes. Dual-use security tools (C2 frameworks, credential testing, exploit development) require clear authorization context: pentesting engagements, CTF competitions, security research, or defensive use cases.
IMPORTANT: You must NEVER generate or guess URLs for the user unless you are confident that the URLs are for helping the user with programming. You may use URLs provided by the user in their messages or local files.

# Tone and style
 - Only use emojis if the user explicitly requests it. Avoid using emojis in all communication unless asked.
 - Your responses should be short and concise.
 - When referencing specific functions or pieces of code include the pattern file_path:line_number to allow the user to easily navigate to the source code location.
 - When referencing GitHub issues or pull requests, use the owner/repo#123 format (e.g. anthropics/claude-code#100) so they render as clickable links.
 - Do not use a colon before tool calls. Your tool calls may not be shown directly in the output, so text like "Let me read the file:" followed by a read tool call should just be "Let me read the file." with a period.`;

  let claudeOAuthSystemPromptBlockID = 0;

  function nextClaudeOAuthSystemPromptBlockID(): string {
    claudeOAuthSystemPromptBlockID += 1;
    return `claude-oauth-system-prompt-block-${claudeOAuthSystemPromptBlockID}`;
  }

  function normalizeClaudeOAuthSystemPromptCacheTTL(value: unknown): string {
    return typeof value === "string" && value.trim() ? value.trim() : "5m";
  }

  function detectClaudeOAuthSystemPromptPreset(
    text: string,
  ): ClaudeOAuthSystemPromptPreset {
    const trimmed = text.trim();
    if (trimmed === "{billing_header}") {
      return "billing";
    }
    if (
      trimmed === "{claude_code_system_prompt}" ||
      trimmed === defaultClaudeCodeSystemPrompt
    ) {
      return "system";
    }
    if (
      trimmed === "{claude_code_expansion_prompt}" ||
      trimmed === defaultClaudeCodeExpansionPrompt
    ) {
      return "expansion";
    }
    return "custom";
  }

  function normalizeClaudeOAuthSystemPromptBlockText(
    text: string,
    expansionPrompt = "",
  ): string {
    const trimmed = text.trim();
    if (trimmed === "{claude_code_system_prompt}") {
      return defaultClaudeCodeSystemPrompt;
    }
    if (trimmed === "{claude_code_expansion_prompt}") {
      return expansionPrompt.trim() || defaultClaudeCodeExpansionPrompt;
    }
    return text;
  }

  function createClaudeOAuthSystemPromptBlock(
    overrides: Partial<ClaudeOAuthSystemPromptBlock> = {},
  ): ClaudeOAuthSystemPromptBlock {
    const text = overrides.text ?? "";
    return {
      id: nextClaudeOAuthSystemPromptBlockID(),
      enabled: overrides.enabled ?? true,
      expanded: overrides.expanded ?? true,
      type: "text",
      preset: overrides.preset ?? detectClaudeOAuthSystemPromptPreset(text),
      text,
      cacheControlEnabled: overrides.cacheControlEnabled ?? false,
      cacheControlTTL: overrides.cacheControlTTL ?? "5m",
    };
  }

  function createDefaultClaudeOAuthSystemPromptBlocks(
    expansionPrompt = "",
  ): ClaudeOAuthSystemPromptBlock[] {
    const normalizedExpansionPrompt = expansionPrompt.trim();
    const expansionText =
      normalizedExpansionPrompt || defaultClaudeCodeExpansionPrompt;

    return [
      createClaudeOAuthSystemPromptBlock({
        preset: "billing",
        text: "{billing_header}",
      }),
      createClaudeOAuthSystemPromptBlock({
        preset: "system",
        text: defaultClaudeCodeSystemPrompt,
      }),
      createClaudeOAuthSystemPromptBlock({
        preset:
          expansionText === defaultClaudeCodeExpansionPrompt
            ? "expansion"
            : "custom",
        text: expansionText,
        cacheControlEnabled: true,
        cacheControlTTL: "5m",
      }),
    ];
  }

  function parseClaudeOAuthSystemPromptCacheControl(cacheControl: unknown): {
    enabled: boolean;
    ttl: string;
  } {
    if (cacheControl === true) {
      return { enabled: true, ttl: "5m" };
    }
    if (
      cacheControl &&
      typeof cacheControl === "object" &&
      !Array.isArray(cacheControl)
    ) {
      return {
        enabled: true,
        ttl: normalizeClaudeOAuthSystemPromptCacheTTL(
          (cacheControl as Record<string, unknown>).ttl,
        ),
      };
    }
    return { enabled: false, ttl: "5m" };
  }

  function parseClaudeOAuthSystemPromptBlocks(
    raw: string,
    expansionPrompt = "",
  ): ClaudeOAuthSystemPromptBlock[] {
    const trimmed = raw.trim();
    if (!trimmed) {
      return createDefaultClaudeOAuthSystemPromptBlocks(expansionPrompt);
    }

    try {
      const parsed = JSON.parse(trimmed) as
        | ClaudeOAuthSystemPromptRawBlock[]
        | { blocks?: ClaudeOAuthSystemPromptRawBlock[] };
      const rawBlocks = Array.isArray(parsed)
        ? parsed
        : Array.isArray(parsed.blocks)
          ? parsed.blocks
          : [];

      if (rawBlocks.length === 0) {
        return createDefaultClaudeOAuthSystemPromptBlocks(expansionPrompt);
      }

      return rawBlocks.map((block) => {
        const cacheControl = parseClaudeOAuthSystemPromptCacheControl(
          block.cache_control,
        );
        const text = normalizeClaudeOAuthSystemPromptBlockText(
          typeof block.text === "string" ? block.text : "",
          expansionPrompt,
        );
        return createClaudeOAuthSystemPromptBlock({
          enabled: block.enabled !== false,
          type: "text",
          text,
          preset: detectClaudeOAuthSystemPromptPreset(text),
          cacheControlEnabled: cacheControl.enabled,
          cacheControlTTL: cacheControl.ttl,
        });
      });
    } catch (_error) {
      return createDefaultClaudeOAuthSystemPromptBlocks(expansionPrompt);
    }
  }

  function serializeClaudeOAuthSystemPromptBlocksToJSON(
    blocks: ClaudeOAuthSystemPromptBlock[],
  ): string {
    const source =
      blocks.length > 0
        ? blocks
        : [
            createClaudeOAuthSystemPromptBlock({
              enabled: false,
              preset: "custom",
              text: "",
            }),
          ];

    const rawBlocks = source.map((block) => {
      const raw: ClaudeOAuthSystemPromptRawBlock = {
        enabled: block.enabled,
        type: block.type || "text",
        text: block.text,
      };
      if (block.cacheControlEnabled) {
        raw.cache_control = {
          type: "ephemeral",
          ttl: normalizeClaudeOAuthSystemPromptCacheTTL(block.cacheControlTTL),
        };
      }
      return raw;
    });

    return JSON.stringify(rawBlocks, null, 2);
  }

  const defaultClaudeOAuthSystemPromptBlocks =
    serializeClaudeOAuthSystemPromptBlocksToJSON(
      createDefaultClaudeOAuthSystemPromptBlocks(),
    );

  const claudeOAuthSystemPromptBlocks = ref<ClaudeOAuthSystemPromptBlock[]>(
    createDefaultClaudeOAuthSystemPromptBlocks(),
  );

  const claudeOAuthSystemPromptPresetOptions = computed(() => [
    {
      value: "billing",
      label: t("admin.settings.gatewayForwarding.systemBlockPresetBilling"),
    },
    {
      value: "system",
      label: t("admin.settings.gatewayForwarding.systemBlockPresetIdentity"),
    },
    {
      value: "expansion",
      label: t("admin.settings.gatewayForwarding.systemBlockPresetExpansion"),
    },
    {
      value: "custom",
      label: t("admin.settings.gatewayForwarding.systemBlockPresetCustom"),
    },
  ]);

  const claudeOAuthSystemPromptBlockTypeOptions = computed(() => [
    {
      value: "text",
      label: t("admin.settings.gatewayForwarding.systemBlockTypeText"),
    },
  ]);

  const claudeOAuthSystemPromptCacheTTLOptions = computed(() => [
    { value: "5m", label: t("admin.settings.gatewayForwarding.cacheTTL5m") },
    { value: "1h", label: t("admin.settings.gatewayForwarding.cacheTTL1h") },
  ]);

  function getClaudeOAuthPresetLabel(
    preset: ClaudeOAuthSystemPromptPreset,
  ): string {
    return (
      claudeOAuthSystemPromptPresetOptions.value.find(
        (option) => option.value === preset,
      )?.label || t("admin.settings.gatewayForwarding.systemBlockPresetCustom")
    );
  }

  function syncClaudeOAuthSystemPromptBlocksFormField(): void {
    form.claude_oauth_system_prompt_blocks =
      serializeClaudeOAuthSystemPromptBlocksToJSON(
        claudeOAuthSystemPromptBlocks.value,
      );
  }

  function addClaudeOAuthSystemPromptBlock(): void {
    claudeOAuthSystemPromptBlocks.value.push(
      createClaudeOAuthSystemPromptBlock({
        expanded: true,
        preset: "custom",
        text: "",
      }),
    );
    syncClaudeOAuthSystemPromptBlocksFormField();
  }

  function toggleClaudeOAuthSystemPromptBlock(index: number): void {
    const block = claudeOAuthSystemPromptBlocks.value[index];
    if (!block) {
      return;
    }
    block.expanded = !block.expanded;
  }

  function removeClaudeOAuthSystemPromptBlock(index: number): void {
    claudeOAuthSystemPromptBlocks.value.splice(index, 1);
    syncClaudeOAuthSystemPromptBlocksFormField();
  }

  function moveClaudeOAuthSystemPromptBlock(
    index: number,
    direction: -1 | 1,
  ): void {
    const targetIndex = index + direction;
    if (
      targetIndex < 0 ||
      targetIndex >= claudeOAuthSystemPromptBlocks.value.length
    ) {
      return;
    }
    const blocks = claudeOAuthSystemPromptBlocks.value;
    const current = blocks[index];
    blocks[index] = blocks[targetIndex];
    blocks[targetIndex] = current;
    syncClaudeOAuthSystemPromptBlocksFormField();
  }

  function applyClaudeOAuthSystemPromptPreset(
    index: number,
    value: string | number | boolean | null,
  ): void {
    const block = claudeOAuthSystemPromptBlocks.value[index];
    if (!block) {
      return;
    }
    const preset = String(value || "custom") as ClaudeOAuthSystemPromptPreset;
    block.preset = preset;
    block.type = "text";
    if (preset === "billing") {
      block.text = "{billing_header}";
      block.cacheControlEnabled = false;
      block.cacheControlTTL = "5m";
    } else if (preset === "system") {
      block.text = defaultClaudeCodeSystemPrompt;
      block.cacheControlEnabled = false;
      block.cacheControlTTL = "5m";
    } else if (preset === "expansion") {
      block.text =
        form.claude_oauth_system_prompt.trim() ||
        defaultClaudeCodeExpansionPrompt;
      block.cacheControlEnabled = true;
      block.cacheControlTTL = "5m";
    }
    syncClaudeOAuthSystemPromptBlocksFormField();
  }

  function markClaudeOAuthSystemPromptBlockCustom(
    block: ClaudeOAuthSystemPromptBlock,
  ): void {
    block.preset = detectClaudeOAuthSystemPromptPreset(block.text);
    syncClaudeOAuthSystemPromptBlocksFormField();
  }

  function resetClaudeOAuthSystemPromptBlocks(): void {
    claudeOAuthSystemPromptBlocks.value = createDefaultClaudeOAuthSystemPromptBlocks(
      form.claude_oauth_system_prompt,
    );
    syncClaudeOAuthSystemPromptBlocksFormField();
  }

  type SettingsForm = Omit<
    SystemSettings,
    // A6-4：这几项挪到了功能页（渠道健康 / 审查），设置页不再读写
    | "channel_monitor_hide_throughput"
    | "channel_monitor_hide_user_ranking"
    | "cyber_session_block_enabled"
    | "cyber_session_block_ttl_seconds"
    // 只读：取部署配置 OPS_ENABLED，侧栏据它显示运维入口，设置页不读写
    | "ops_monitoring_enabled"
  > & {
    account_scheduling_thresholds: ReturnType<typeof normalizeAccountSchedulingThresholdsMap>;
  };

  const schedulingThresholdPlatforms = SCHEDULING_THRESHOLD_PLATFORMS;

  const form = reactive<SettingsForm>({
    account_scheduling_thresholds: normalizeAccountSchedulingThresholdsMap(),
    risk_control_enabled: false,
    grok_default_text_model: "grok-4.5",
    grok_cross_client_model_map_enabled: false,
    grok_default_base_url_mode: "cli",
    // Identity patch (Claude -> Gemini)
    enable_identity_patch: true,
    identity_patch_prompt: "",
    // Ops monitoring (vNext)
    ops_realtime_monitoring_enabled: true,
    ops_query_mode_default: "auto",
    ops_metrics_interval_seconds: 60,
    // Claude Code version check
    min_claude_code_version: "",
    max_claude_code_version: "",
    // Gateway forwarding behavior
    openai_ttft_mode: "semantic",
    enable_fingerprint_unification: true,
    enable_metadata_passthrough: false,
    enable_cch_signing: false,
    enable_claude_oauth_system_prompt_injection: true,
    claude_oauth_system_prompt: "",
    claude_oauth_system_prompt_blocks: defaultClaudeOAuthSystemPromptBlocks,
    enable_anthropic_cache_ttl_1h_injection: false,
    rewrite_message_cache_control: false,
    enable_client_dateline_normalization: true,
    antigravity_user_agent_version: "",
    openai_codex_user_agent: "",
    openai_codex_client_version: "",
    // 只读展示：自动同步任务写入的官方最新稳定版，不参与提交（提交载荷按字段显式构造）
    openai_codex_client_version_synced: "",
    openai_codex_version_auto_sync_enabled: true,
    // codex_cli_only 加固
    min_codex_version: "",
    max_codex_version: "",
    codex_cli_only_blacklist: "",
    codex_cli_only_whitelist: "",
    codex_cli_only_allow_app_server_clients: false,
    codex_cli_only_engine_fingerprint_signals: "",
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

  // ── codex_cli_only 黑/白名单结构化编辑（行 ↔ JSON）──
  interface CodexClientRow {
    originator: string;
    uaContains: string; // 逗号分隔，序列化时拆成 ua_contains 数组
    skipEngineFingerprint?: boolean; // 仅白名单：命中即跳过引擎指纹门
  }
  const codexBlacklistRows = ref<CodexClientRow[]>([]);
  const codexWhitelistRows = ref<CodexClientRow[]>([]);
  const codexFingerprintRows = ref<FingerprintSignalRow[]>([]);
  const codexFingerprintNoRequired = computed(
    () => !codexFingerprintRows.value.some((r) => r.required),
  );
  function addCodexFingerprintRow(): void {
    codexFingerprintRows.value.push({ type: "header_exact", match: "", required: false });
  }
  function removeCodexFingerprintRow(i: number): void {
    codexFingerprintRows.value.splice(i, 1);
  }

  function parseCodexEntriesToRows(raw: string): CodexClientRow[] {
    if (!raw || !raw.trim()) return [];
    try {
      const arr = JSON.parse(raw);
      if (!Array.isArray(arr)) return [];
      return arr.map((e) => ({
        originator: typeof e?.originator === "string" ? e.originator : "",
        uaContains: Array.isArray(e?.ua_contains)
          ? e.ua_contains
              .filter((x: unknown) => typeof x === "string")
              .join(", ")
          : "",
        skipEngineFingerprint: e?.skip_engine_fingerprint === true,
      }));
    } catch {
      return [];
    }
  }

  function serializeCodexRowsToJSON(rows: CodexClientRow[]): string {
    const entries = rows
      .map((r) => {
        const entry: {
          originator: string;
          ua_contains: string[];
          skip_engine_fingerprint?: boolean;
        } = {
          originator: r.originator.trim(),
          ua_contains: r.uaContains
            .split(",")
            .map((s) => s.trim())
            .filter((s) => s.length > 0),
        };
        if (r.skipEngineFingerprint) entry.skip_engine_fingerprint = true;
        return entry;
      })
      .filter((e) => e.originator !== "" || e.ua_contains.length > 0);
    return entries.length > 0 ? JSON.stringify(entries) : "";
  }

  function addCodexBlacklistRow(): void {
    codexBlacklistRows.value.push({ originator: "", uaContains: "" });
  }
  function removeCodexBlacklistRow(i: number): void {
    codexBlacklistRows.value.splice(i, 1);
  }
  function addCodexWhitelistRow(): void {
    codexWhitelistRows.value.push({
      originator: "",
      uaContains: "",
      skipEngineFingerprint: false,
    });
  }
  function removeCodexWhitelistRow(i: number): void {
    codexWhitelistRows.value.splice(i, 1);
  }

  const codexSyncedVersionLabel = computed(() => {
    const synced = form.openai_codex_client_version_synced?.trim();
    if (!synced) return "";
    return t("admin.settings.gatewayForwarding.openaiCodexVersionSyncedValue", {
      version: synced,
    });
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
      if (!form.claude_oauth_system_prompt_blocks?.trim()) {
        form.claude_oauth_system_prompt_blocks =
          defaultClaudeOAuthSystemPromptBlocks;
      }
      claudeOAuthSystemPromptBlocks.value = parseClaudeOAuthSystemPromptBlocks(
        form.claude_oauth_system_prompt_blocks,
        form.claude_oauth_system_prompt,
      );
      syncClaudeOAuthSystemPromptBlocksFormField();
      codexBlacklistRows.value = parseCodexEntriesToRows(
        form.codex_cli_only_blacklist,
      );
      codexWhitelistRows.value = parseCodexEntriesToRows(
        form.codex_cli_only_whitelist,
      );
      codexFingerprintRows.value = form.codex_cli_only_engine_fingerprint_signals
        ? parseFingerprintSignalsToRows(form.codex_cli_only_engine_fingerprint_signals)
        : defaultFingerprintSignalRows();
      form.account_scheduling_thresholds = normalizeAccountSchedulingThresholdsMap(
        settings.account_scheduling_thresholds,
      );

      // Load OpenAI fast/flex policy rules from bulk settings.
      // 仅当 payload 真的包含该字段时填充并标记为已加载；否则保持表单空值，
      // 让 saveSettings 在未加载时跳过该字段，防止覆盖后端默认规则。
      if (
        settings.openai_fast_policy_settings &&
        Array.isArray(settings.openai_fast_policy_settings.rules)
      ) {
        openaiFastPolicyForm.rules =
          settings.openai_fast_policy_settings.rules.map((rule) => ({
            ...rule,
            user_ids: rule.user_ids ? [...rule.user_ids] : [],
            model_whitelist: rule.model_whitelist
              ? [...rule.model_whitelist]
              : [],
          }));
        openaiFastPolicyLoaded.value = true;
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
      const claudeOAuthSystemPromptBlocksJSON =
        serializeClaudeOAuthSystemPromptBlocksToJSON(
          claudeOAuthSystemPromptBlocks.value,
        );
      form.claude_oauth_system_prompt_blocks =
        claudeOAuthSystemPromptBlocksJSON;

      const payload: UpdateSettingsRequest = {
        grok_default_text_model:
          form.grok_default_text_model.trim() || "grok-4.5",
        grok_cross_client_model_map_enabled:
          form.grok_cross_client_model_map_enabled,
        grok_default_base_url_mode: form.grok_default_base_url_mode,
        enable_identity_patch: form.enable_identity_patch,
        identity_patch_prompt: form.identity_patch_prompt,
        min_claude_code_version: form.min_claude_code_version,
        max_claude_code_version: form.max_claude_code_version,
        openai_ttft_mode:
          form.openai_ttft_mode === "visible" ? "visible" : "semantic",
        enable_fingerprint_unification: form.enable_fingerprint_unification,
        enable_metadata_passthrough: form.enable_metadata_passthrough,
        enable_cch_signing: form.enable_cch_signing,
        enable_claude_oauth_system_prompt_injection:
          form.enable_claude_oauth_system_prompt_injection,
        claude_oauth_system_prompt: form.claude_oauth_system_prompt?.trim()
          ? form.claude_oauth_system_prompt
          : "",
        claude_oauth_system_prompt_blocks: claudeOAuthSystemPromptBlocksJSON,
        enable_anthropic_cache_ttl_1h_injection:
          form.enable_anthropic_cache_ttl_1h_injection,
        rewrite_message_cache_control: form.rewrite_message_cache_control,
        enable_client_dateline_normalization:
          form.enable_client_dateline_normalization,
        antigravity_user_agent_version:
          form.antigravity_user_agent_version?.trim() || "",
        openai_codex_user_agent:
          form.openai_codex_user_agent?.trim() || "",
        openai_codex_client_version:
          form.openai_codex_client_version?.trim() || "",
        openai_codex_version_auto_sync_enabled:
          form.openai_codex_version_auto_sync_enabled,
        min_codex_version: form.min_codex_version?.trim() || "",
        max_codex_version: form.max_codex_version?.trim() || "",
        codex_cli_only_allow_app_server_clients:
          form.codex_cli_only_allow_app_server_clients,
        codex_cli_only_engine_fingerprint_signals: serializeFingerprintRowsToJSON(
          codexFingerprintRows.value,
        ),
        codex_cli_only_blacklist: serializeCodexRowsToJSON(
          codexBlacklistRows.value,
        ),
        codex_cli_only_whitelist: serializeCodexRowsToJSON(
          codexWhitelistRows.value,
        ),
        risk_control_enabled: form.risk_control_enabled,
        profit_control_enabled: form.profit_control_enabled,
        profit_min_margin: form.profit_min_margin,
        profit_safety_buffer: form.profit_safety_buffer,
      };

      // 仅当 openai_fast_policy_settings 已成功从后端加载时才回写，
      // 否则省略整个字段，让后端保留既有规则（含默认值）。
      if (openaiFastPolicyLoaded.value) {
        payload.openai_fast_policy_settings = {
          rules: openaiFastPolicyForm.rules.map((rule) => {
            const whitelist = (rule.model_whitelist || [])
              .map((p) => p.trim())
              .filter((p) => p !== "");
            const hasWhitelist = whitelist.length > 0;
            return {
              service_tier: rule.service_tier,
              action: rule.action,
              scope: rule.scope,
              user_ids:
                rule.user_ids && rule.user_ids.length > 0
                  ? [...rule.user_ids]
                  : undefined,
              error_message:
                rule.action === "block" ? rule.error_message : undefined,
              model_whitelist: hasWhitelist ? whitelist : undefined,
              fallback_action: hasWhitelist
                ? rule.fallback_action || "pass"
                : undefined,
              fallback_error_message:
                hasWhitelist && rule.fallback_action === "block"
                  ? rule.fallback_error_message
                  : undefined,
            };
          }),
        };
      }

      payload.account_scheduling_thresholds = sanitizeAccountSchedulingThresholdsMap(
        form.account_scheduling_thresholds,
      );

      const updated = await adminAPI.settings.updateSettings(payload);
      for (const [key, value] of Object.entries(updated)) {
        if (key === "openai_fast_policy_settings") continue;
        if (value !== null && value !== undefined) {
          (form as Record<string, unknown>)[key] = value;
        }
      }
      form.account_scheduling_thresholds = normalizeAccountSchedulingThresholdsMap(
        updated.account_scheduling_thresholds,
      );
      // Refresh OpenAI fast/flex policy from server response
      if (
        updated.openai_fast_policy_settings &&
        Array.isArray(updated.openai_fast_policy_settings.rules)
      ) {
        openaiFastPolicyForm.rules =
          updated.openai_fast_policy_settings.rules.map((rule) => ({
            ...rule,
            user_ids: rule.user_ids ? [...rule.user_ids] : [],
            model_whitelist: rule.model_whitelist
              ? [...rule.model_whitelist]
              : [],
          }));
        openaiFastPolicyLoaded.value = true;
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

  async function loadUpstreamBillingProbeSettings() {
    upstreamBillingProbeLoading.value = true;
    try {
      Object.assign(
        upstreamBillingProbeForm,
        await adminAPI.accounts.getUpstreamBillingProbeSettings(),
      );
    } catch (_error: unknown) {
      // Keep defaults when this optional setting cannot be loaded.
    } finally {
      upstreamBillingProbeLoading.value = false;
    }
  }

  async function saveUpstreamBillingProbeSettings(): Promise<boolean> {
    upstreamBillingProbeSaving.value = true;
    try {
      const updated = await adminAPI.accounts.updateUpstreamBillingProbeSettings({
        ...upstreamBillingProbeForm,
      });
      Object.assign(upstreamBillingProbeForm, updated);
      appStore.showSuccess(t("admin.settings.upstreamBillingProbe.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(
          error,
          t("admin.settings.upstreamBillingProbe.saveFailed"),
        ),
      );
      return false;
    } finally {
      upstreamBillingProbeSaving.value = false;
    }
  }

  async function loadOllamaCloudUsageSettings() {
    ollamaCloudUsageLoading.value = true;
    try {
      Object.assign(
        ollamaCloudUsageForm,
        await adminAPI.accounts.getOllamaCloudUsageSettings(),
      );
    } catch (_error: unknown) {
      // Keep the fail-safe disabled defaults when this optional setting cannot be loaded.
    } finally {
      ollamaCloudUsageLoading.value = false;
    }
  }

  async function saveOllamaCloudUsageSettings(): Promise<boolean> {
    ollamaCloudUsageSaving.value = true;
    try {
      const updated = await adminAPI.accounts.updateOllamaCloudUsageSettings({
        ...ollamaCloudUsageForm,
      });
      Object.assign(ollamaCloudUsageForm, updated);
      appStore.showSuccess(t("admin.settings.ollamaCloudUsage.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(error, t("admin.settings.ollamaCloudUsage.saveFailed")),
      );
      return false;
    } finally {
      ollamaCloudUsageSaving.value = false;
    }
  }

  // Overload Cooldown 方法
  async function loadOverloadCooldownSettings() {
    overloadCooldownLoading.value = true;
    try {
      const settings = await adminAPI.settings.getOverloadCooldownSettings();
      Object.assign(overloadCooldownForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      overloadCooldownLoading.value = false;
    }
  }

  async function saveOverloadCooldownSettings(): Promise<boolean> {
    overloadCooldownSaving.value = true;
    try {
      const updated = await adminAPI.settings.updateOverloadCooldownSettings({
        enabled: overloadCooldownForm.enabled,
        cooldown_minutes: overloadCooldownForm.cooldown_minutes,
      });
      Object.assign(overloadCooldownForm, updated);
      appStore.showSuccess(t("admin.settings.overloadCooldown.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(
          error,
          t("admin.settings.overloadCooldown.saveFailed"),
        ),
      );
      return false;
    } finally {
      overloadCooldownSaving.value = false;
    }
  }

  // Rate Limit Cooldown (429) 方法
  async function loadRateLimit429CooldownSettings() {
    rateLimit429CooldownLoading.value = true;
    try {
      const settings = await adminAPI.settings.getRateLimit429CooldownSettings();
      Object.assign(rateLimit429CooldownForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      rateLimit429CooldownLoading.value = false;
    }
  }

  async function saveRateLimit429CooldownSettings(): Promise<boolean> {
    rateLimit429CooldownSaving.value = true;
    try {
      const updated = await adminAPI.settings.updateRateLimit429CooldownSettings({
        enabled: rateLimit429CooldownForm.enabled,
        cooldown_seconds: rateLimit429CooldownForm.cooldown_seconds,
      });
      Object.assign(rateLimit429CooldownForm, updated);
      appStore.showSuccess(t("admin.settings.rateLimit429Cooldown.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(
          error,
          t("admin.settings.rateLimit429Cooldown.saveFailed"),
        ),
      );
      return false;
    } finally {
      rateLimit429CooldownSaving.value = false;
    }
  }

  // Stream Timeout 方法
  async function loadStreamTimeoutSettings() {
    streamTimeoutLoading.value = true;
    try {
      const settings = await adminAPI.settings.getStreamTimeoutSettings();
      Object.assign(streamTimeoutForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      streamTimeoutLoading.value = false;
    }
  }

  async function saveStreamTimeoutSettings(): Promise<boolean> {
    streamTimeoutSaving.value = true;
    try {
      const updated = await adminAPI.settings.updateStreamTimeoutSettings({
        enabled: streamTimeoutForm.enabled,
        action: streamTimeoutForm.action,
        temp_unsched_minutes: streamTimeoutForm.temp_unsched_minutes,
        threshold_count: streamTimeoutForm.threshold_count,
        threshold_window_minutes: streamTimeoutForm.threshold_window_minutes,
      });
      Object.assign(streamTimeoutForm, updated);
      appStore.showSuccess(t("admin.settings.streamTimeout.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(
          error,
          t("admin.settings.streamTimeout.saveFailed"),
        ),
      );
      return false;
    } finally {
      streamTimeoutSaving.value = false;
    }
  }

  // Rectifier 方法
  async function loadRectifierSettings() {
    rectifierLoading.value = true;
    try {
      const settings = await adminAPI.settings.getRectifierSettings();
      Object.assign(rectifierForm, settings);
      // 确保 patterns 是数组（旧数据可能为 null）
      if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
        rectifierForm.apikey_signature_patterns = [];
      }
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      rectifierLoading.value = false;
    }
  }

  async function saveRectifierSettings(): Promise<boolean> {
    rectifierSaving.value = true;
    try {
      const updated = await adminAPI.settings.updateRectifierSettings({
        enabled: rectifierForm.enabled,
        thinking_signature_enabled: rectifierForm.thinking_signature_enabled,
        thinking_budget_enabled: rectifierForm.thinking_budget_enabled,
        apikey_signature_enabled: rectifierForm.apikey_signature_enabled,
        apikey_signature_patterns: rectifierForm.apikey_signature_patterns.filter(
          (p) => p.trim() !== "",
        ),
      });
      Object.assign(rectifierForm, updated);
      if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
        rectifierForm.apikey_signature_patterns = [];
      }
      appStore.showSuccess(t("admin.settings.rectifier.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(error, t("admin.settings.rectifier.saveFailed")),
      );
      return false;
    } finally {
      rectifierSaving.value = false;
    }
  }

  const betaPolicyActionOptions = computed(() => [
    { value: "pass", label: t("admin.settings.betaPolicy.actionPass") },
    { value: "filter", label: t("admin.settings.betaPolicy.actionFilter") },
    { value: "block", label: t("admin.settings.betaPolicy.actionBlock") },
  ]);

  const betaPolicyScopeOptions = computed(() => [
    { value: "all", label: t("admin.settings.betaPolicy.scopeAll") },
    { value: "oauth", label: t("admin.settings.betaPolicy.scopeOAuth") },
    { value: "apikey", label: t("admin.settings.betaPolicy.scopeAPIKey") },
    { value: "bedrock", label: t("admin.settings.betaPolicy.scopeBedrock") },
  ]);

  // Beta Policy 方法
  const betaDisplayNames: Record<string, string> = {
    "fast-mode-2026-02-01": "Fast Mode",
    "context-1m-2025-08-07": "Context 1M",
  };

  // 快捷预设：按 beta_token 定义预设方案
  const betaPresets: Record<
    string,
    Array<{
      label: string;
      description: string;
      action: "pass" | "filter" | "block";
      model_whitelist: string[];
      fallback_action: "pass" | "filter" | "block";
    }>
  > = {
    "context-1m-2025-08-07": [
      {
        label: t("admin.settings.betaPolicy.presetOpusOnly"),
        description: t("admin.settings.betaPolicy.presetOpusOnlyDesc"),
        action: "pass",
        model_whitelist: ["claude-opus-4-6"],
        fallback_action: "filter",
      },
    ],
  };

  // 常用模型模式（具体 ID + 通配符示例）
  const commonModelPatterns = [
    "claude-opus-4-6",
    "claude-sonnet-4-6",
    "claude-opus-*",
    "claude-sonnet-*",
  ];

  function getBetaDisplayName(token: string): string {
    return betaDisplayNames[token] || token;
  }

  function applyBetaPreset(
    rule: (typeof betaPolicyForm.rules)[number],
    preset: {
      action: "pass" | "filter" | "block";
      model_whitelist: string[];
      fallback_action: "pass" | "filter" | "block";
    },
  ) {
    rule.action = preset.action;
    rule.model_whitelist = [...preset.model_whitelist];
    rule.fallback_action = preset.fallback_action;
  }

  function addQuickPattern(
    rule: (typeof betaPolicyForm.rules)[number],
    pattern: string,
  ) {
    if (!rule.model_whitelist) rule.model_whitelist = [];
    if (!rule.model_whitelist.includes(pattern)) {
      rule.model_whitelist.push(pattern);
    }
  }

  async function loadBetaPolicySettings() {
    betaPolicyLoading.value = true;
    try {
      const settings = await adminAPI.settings.getBetaPolicySettings();
      betaPolicyForm.rules = settings.rules;
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      betaPolicyLoading.value = false;
    }
  }

  // ==================== OpenAI Fast/Flex Policy ====================

  const openaiFastPolicyTierOptions = computed(() => [
    { value: "all", label: t("admin.settings.openaiFastPolicy.tierAll") },
    {
      value: "priority",
      label: t("admin.settings.openaiFastPolicy.tierPriority"),
    },
    {
      value: "ultrafast",
      label: t("admin.settings.openaiFastPolicy.tierUltrafast"),
    },
    { value: "flex", label: t("admin.settings.openaiFastPolicy.tierFlex") },
    { value: "missing", label: t("admin.settings.openaiFastPolicy.tierMissing") },
  ]);

  const openaiFastPolicyActionOptions = computed(() => [
    { value: "pass", label: t("admin.settings.openaiFastPolicy.actionPass") },
    { value: "filter", label: t("admin.settings.openaiFastPolicy.actionFilter") },
    {
      value: "force_priority",
      label: t("admin.settings.openaiFastPolicy.actionForcePriority"),
    },
    { value: "block", label: t("admin.settings.openaiFastPolicy.actionBlock") },
  ]);

  function openaiFastPolicyActionSummary(
    action: OpenAIFastPolicyRule["action"],
  ) {
    return t(`admin.settings.openaiFastPolicy.summaryAction.${action}`);
  }

  function hasOpenAIFastPolicyTargetModels(rule: OpenAIFastPolicyRule) {
    return Boolean(rule.model_whitelist?.some((pattern) => pattern.trim() !== ""));
  }

  const openaiFastPolicyScopeOptions = computed(() => [
    { value: "all", label: t("admin.settings.openaiFastPolicy.scopeAll") },
    { value: "oauth", label: t("admin.settings.openaiFastPolicy.scopeOAuth") },
    { value: "apikey", label: t("admin.settings.openaiFastPolicy.scopeAPIKey") },
    {
      value: "bedrock",
      label: t("admin.settings.openaiFastPolicy.scopeBedrock"),
    },
  ]);

  function addOpenAIFastPolicyRule() {
    openaiFastPolicyForm.rules.push({
      service_tier: "priority",
      action: "filter",
      scope: "all",
      user_ids: [],
      error_message: "",
      model_whitelist: [],
      fallback_action: "pass",
      fallback_error_message: "",
    });
  }

  function removeOpenAIFastPolicyRule(index: number) {
    openaiFastPolicyForm.rules.splice(index, 1);
  }

  function addOpenAIFastPolicyModelPattern(rule: OpenAIFastPolicyRule) {
    if (!rule.model_whitelist) rule.model_whitelist = [];
    rule.model_whitelist.push("");
  }

  function removeOpenAIFastPolicyModelPattern(
    rule: OpenAIFastPolicyRule,
    idx: number,
  ) {
    rule.model_whitelist?.splice(idx, 1);
  }

  async function saveBetaPolicySettings(): Promise<boolean> {
    betaPolicySaving.value = true;
    try {
      // Clean up empty patterns before saving
      const cleanedRules = betaPolicyForm.rules.map((rule) => {
        const whitelist = rule.model_whitelist?.filter((p) => p.trim() !== "");
        const hasWhitelist = whitelist && whitelist.length > 0;
        return {
          beta_token: rule.beta_token,
          action: rule.action,
          scope: rule.scope,
          error_message: rule.error_message,
          model_whitelist: hasWhitelist ? whitelist : undefined,
          fallback_action: hasWhitelist
            ? rule.fallback_action || "pass"
            : undefined,
          fallback_error_message:
            hasWhitelist && rule.fallback_action === "block"
              ? rule.fallback_error_message
              : undefined,
        };
      });
      const updated = await adminAPI.settings.updateBetaPolicySettings({
        rules: cleanedRules,
      });
      betaPolicyForm.rules = updated.rules;
      appStore.showSuccess(t("admin.settings.betaPolicy.saved"));
      return true;
    } catch (error: unknown) {
      appStore.showError(
        extractApiErrorMessage(error, t("admin.settings.betaPolicy.saveFailed")),
      );
      return false;
    } finally {
      betaPolicySaving.value = false;
    }
  }

  onMounted(async () => {
    await Promise.allSettled([
      loadSettings(),
      loadUpstreamBillingProbeSettings(),
      loadOllamaCloudUsageSettings(),
      loadOverloadCooldownSettings(),
      loadRateLimit429CooldownSettings(),
      loadStreamTimeoutSettings(),
      loadRectifierSettings(),
      loadBetaPolicySettings(),
    ]);
    // 加载后的规整（watch / 子组件回写）都跑完再取基线，页面一打开不应显示「有未保存的修改」
    await nextTick();
    await nextTick();
    markAllClean();
  });

  // =========================
  // A6-2 每节保存
  // =========================
  // 同一时间只有当前小节可能有改动：切走时有改动会先问「放弃 / 留下」，放弃就恢复成已保存的值。
  // 所以保存某一节时照旧整份提交总表单（其它节都等于已保存值，结果等于只存这一节）。
  // （后端本身支持只发部分字段：没发送的值类型字段不写库，见 setting_handler_update.go omittedSettingKeys。）
  // 改动判断：每块状态与「上次加载 / 保存后的基线」比较。

  /** 走独立接口保存的卡片：各自一块状态、一个保存函数，固定属于某一节 */
  const SECTION_SUB_PARTS: Record<
    string,
    { section: SettingsSectionKey; state: object; save: () => Promise<boolean> }
  > = {
    overloadCooldown: { section: "cooldown", state: overloadCooldownForm, save: saveOverloadCooldownSettings },
    rateLimit429Cooldown: { section: "cooldown", state: rateLimit429CooldownForm, save: saveRateLimit429CooldownSettings },
    streamTimeout: { section: "cooldown", state: streamTimeoutForm, save: saveStreamTimeoutSettings },
    rectifier: { section: "forwarding", state: rectifierForm, save: saveRectifierSettings },
    betaPolicy: { section: "forwarding", state: betaPolicyForm, save: saveBetaPolicySettings },
    upstreamBillingProbe: { section: "upstream", state: upstreamBillingProbeForm, save: saveUpstreamBillingProbeSettings },
    ollamaCloudUsage: { section: "upstream", state: ollamaCloudUsageForm, save: saveOllamaCloudUsageSettings },
  };
  /** 没有总表单字段的小节：保存时不必整份提交 */
  const SECTIONS_WITHOUT_MAIN_FIELDS: SettingsSectionKey[] = ["upstream"];

  /** 总表单保存时会读到的全部状态（form 之外的是保存前才同步进 form 的编辑态） */
  function mainState() {
    return {
      form,
      claudeOAuthSystemPromptBlocks: claudeOAuthSystemPromptBlocks.value,
      codexBlacklistRows: codexBlacklistRows.value,
      codexWhitelistRows: codexWhitelistRows.value,
      codexFingerprintRows: codexFingerprintRows.value,
      openaiFastPolicyForm,
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
    claudeOAuthSystemPromptBlocks.value = copy.claudeOAuthSystemPromptBlocks;
    codexBlacklistRows.value = copy.codexBlacklistRows;
    codexWhitelistRows.value = copy.codexWhitelistRows;
    codexFingerprintRows.value = copy.codexFingerprintRows;
    Object.assign(openaiFastPolicyForm, copy.openaiFastPolicyForm);
    Object.assign(webSearchConfig, copy.webSearchConfig);
  }

  const baselines = reactive<Record<string, string>>({});
  const restorePoints = new Map<string, unknown>();
  function markClean(part: string) {
    if (part === "main") {
      baselines.main = serializeMain();
      restorePoints.set("main", JSON.parse(JSON.stringify(mainState())));
      return;
    }
    const snapshot = JSON.stringify(SECTION_SUB_PARTS[part].state);
    baselines[part] = snapshot;
    restorePoints.set(part, JSON.parse(snapshot));
  }
  function markAllClean() {
    markClean("main");
    for (const part of Object.keys(SECTION_SUB_PARTS)) markClean(part);
  }

  const dirtyParts = computed(() => {
    const dirty = new Set<string>();
    if (baselines.main !== undefined && serializeMain() !== baselines.main) dirty.add("main");
    for (const [part, { state }] of Object.entries(SECTION_SUB_PARTS)) {
      if (baselines[part] !== undefined && JSON.stringify(state) !== baselines[part]) dirty.add(part);
    }
    return dirty;
  });
  // 总表单的改动记在「改的时候所在的小节」上（只有当前小节能改）
  const mainDirtyOwner = ref<SettingsSectionKey | null>(null);
  watch(
    () => dirtyParts.value.has("main"),
    (dirty) => {
      mainDirtyOwner.value = dirty ? currentSection.value : null;
    },
  );

  function isSectionDirty(key: SettingsSectionKey): boolean {
    const dirty = dirtyParts.value;
    if (dirty.has("main") && mainDirtyOwner.value === key) return true;
    return Object.entries(SECTION_SUB_PARTS).some(([part, sub]) => sub.section === key && dirty.has(part));
  }

  const sectionSaving = ref(false);
  async function saveSection(key: SettingsSectionKey): Promise<void> {
    if (sectionSaving.value) return;
    sectionSaving.value = true;
    try {
      const dirty = new Set(dirtyParts.value);
      const subDirty = Object.entries(SECTION_SUB_PARTS).some(([part, sub]) => sub.section === key && dirty.has(part));
      // 总表单：有改动才提交；本节什么都没改时（输入框里回车）照旧整份提交，和拆页前一致
      const saveMain = !SECTIONS_WITHOUT_MAIN_FIELDS.includes(key) && (dirty.has("main") || !subDirty);
      if (saveMain && (await saveSettings())) {
        await nextTick();
        markClean("main");
      }
      for (const [part, sub] of Object.entries(SECTION_SUB_PARTS)) {
        if (sub.section === key && dirty.has(part) && (await sub.save())) markClean(part);
      }
    } finally {
      sectionSaving.value = false;
    }
  }

  /** 放弃这一节的改动：恢复成上次加载 / 保存后的值（不重新请求） */
  function discardSection(key: SettingsSectionKey) {
    const dirty = dirtyParts.value;
    if (dirty.has("main") && mainDirtyOwner.value === key) {
      restoreMain(restorePoints.get("main") as ReturnType<typeof mainState>);
    }
    for (const [part, sub] of Object.entries(SECTION_SUB_PARTS)) {
      if (sub.section === key && dirty.has(part)) {
        Object.assign(sub.state, JSON.parse(JSON.stringify(restorePoints.get(part))));
      }
    }
  }

return {
    addClaudeOAuthSystemPromptBlock,
    addCodexBlacklistRow,
    addCodexFingerprintRow,
    addCodexWhitelistRow,
    addOpenAIFastPolicyModelPattern,
    addOpenAIFastPolicyRule,
    addQuickPattern,
    addWebSearchProvider,
    apiKeyVisible,
    applyBetaPreset,
    applyClaudeOAuthSystemPromptPreset,
    betaPolicyActionOptions,
    betaPolicyForm,
    betaPolicyLoading,
    betaPolicyScopeOptions,
    betaPresets,
    claudeOAuthSystemPromptBlockTypeOptions,
    claudeOAuthSystemPromptBlocks,
    claudeOAuthSystemPromptCacheTTLOptions,
    claudeOAuthSystemPromptPresetOptions,
    codexBlacklistRows,
    codexFingerprintNoRequired,
    codexFingerprintRows,
    codexSyncedVersionLabel,
    codexWhitelistRows,
    commonModelPatterns,
    copyApiKey,
    discardSection,
    expandedProviders,
    form,
    formatSubscribedAt,
    getBetaDisplayName,
    getClaudeOAuthPresetLabel,
    hasOpenAIFastPolicyTargetModels,
    isSectionDirty,
    loadFailed,
    loading,
    markClaudeOAuthSystemPromptBlockCustom,
    moveClaudeOAuthSystemPromptBlock,
    ollamaCloudUsageForm,
    ollamaCloudUsageLoading,
    openTestDialog,
    openaiFastPolicyActionOptions,
    openaiFastPolicyActionSummary,
    openaiFastPolicyForm,
    openaiFastPolicyScopeOptions,
    openaiFastPolicyTierOptions,
    overloadCooldownForm,
    overloadCooldownLoading,
    parseSubscribedAt,
    quotaPercentage,
    rateLimit429CooldownForm,
    rateLimit429CooldownLoading,
    rectifierForm,
    rectifierLoading,
    removeClaudeOAuthSystemPromptBlock,
    removeCodexBlacklistRow,
    removeCodexFingerprintRow,
    removeCodexWhitelistRow,
    removeOpenAIFastPolicyModelPattern,
    removeOpenAIFastPolicyRule,
    removeWebSearchProvider,
    resetClaudeOAuthSystemPromptBlocks,
    resetWebSearchUsage,
    saveOllamaCloudUsageSettings,
    saveSection,
    saveUpstreamBillingProbeSettings,
    schedulingThresholdPlatforms,
    sectionSaving,
    streamTimeoutForm,
    streamTimeoutLoading,
    t,
    testWebSearchProvider,
    toggleClaudeOAuthSystemPromptBlock,
    toggleProviderExpand,
    upstreamBillingProbeForm,
    upstreamBillingProbeLoading,
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
