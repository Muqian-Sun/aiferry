/**
 * Admin Settings API endpoints
 * Handles system settings management for administrators
 */

import { apiClient } from "../client";

export type SchedulingThresholdPlatformType =
  | "openai"
  | "anthropic"
  | "grok"
  | "kimi"
  | "zhipu"
  | "minimax"
  | "opencode_go"

export type AccountSchedulingThresholdsMap = Record<SchedulingThresholdPlatformType, number>

// 与后端 AllowedSchedulingThresholdPlatforms 保持一致（deepseek 为余额型，
// 走余额检测而非用量阈值；minimax Coding/Token Plan 与 OpenCode GO 有滚动窗口）。
export const SCHEDULING_THRESHOLD_PLATFORMS: SchedulingThresholdPlatformType[] = [
  "openai",
  "anthropic",
  "grok",
  "kimi",
  "zhipu",
  "minimax",
  "opencode_go",
]

export function normalizeAccountSchedulingThresholdsMap(
  input?: Partial<Record<SchedulingThresholdPlatformType, number>> | null,
): AccountSchedulingThresholdsMap {
  const result = {} as AccountSchedulingThresholdsMap
  for (const platform of SCHEDULING_THRESHOLD_PLATFORMS) {
    const value = input?.[platform]
    result[platform] = typeof value === "number" && Number.isFinite(value)
      ? Math.min(100, Math.max(1, Math.trunc(value)))
      : 100
  }
  return result
}

export function sanitizeAccountSchedulingThresholdsMap(
  input?: Partial<Record<SchedulingThresholdPlatformType, number>> | null,
): AccountSchedulingThresholdsMap {
  return normalizeAccountSchedulingThresholdsMap(input)
}

/**
 * System settings interface
 */
export interface SystemSettings {
  grok_default_text_model: string;
  grok_cross_client_model_map_enabled: boolean;
  grok_default_base_url_mode: string;

  // Per-platform account auto-pause thresholds (100 = disabled)
  account_scheduling_thresholds: AccountSchedulingThresholdsMap;

  // Identity patch configuration (Claude -> Gemini)
  enable_identity_patch: boolean;
  identity_patch_prompt: string;

  // Ops Monitoring (vNext)
  ops_monitoring_enabled: boolean; // 只读：取部署配置 OPS_ENABLED，PUT 不接受
  ops_realtime_monitoring_enabled: boolean;
  ops_query_mode_default: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds: number;

  // Claude Code version check
  min_claude_code_version: string;
  max_claude_code_version: string;

  // 分组隔离

  // Gateway forwarding behavior
  openai_ttft_mode: string;
  enable_fingerprint_unification: boolean;
  enable_metadata_passthrough: boolean;
  enable_cch_signing: boolean;
  enable_claude_oauth_system_prompt_injection: boolean;
  claude_oauth_system_prompt: string;
  claude_oauth_system_prompt_blocks: string;
  enable_anthropic_cache_ttl_1h_injection: boolean;
  rewrite_message_cache_control: boolean;
  enable_client_dateline_normalization: boolean;
  antigravity_user_agent_version: string;
  openai_codex_user_agent: string;
  openai_codex_client_version: string;
  openai_codex_client_version_synced: string;
  openai_codex_version_auto_sync_enabled: boolean;
  // codex_cli_only 加固
  min_codex_version: string;
  max_codex_version: string;
  codex_cli_only_blacklist: string;
  codex_cli_only_whitelist: string;
  codex_cli_only_allow_app_server_clients: boolean;
  codex_cli_only_engine_fingerprint_signals: string;
  web_search_emulation_enabled?: boolean;

  // 风控中心功能开关
  risk_control_enabled: boolean;

  // Cyber session block
  cyber_session_block_enabled: boolean;
  cyber_session_block_ttl_seconds: number;

  // Channel Monitor feature switch
  channel_monitor_hide_throughput?: boolean;
  channel_monitor_hide_user_ranking?: boolean;

  // Available Channels feature switch

  // Affiliate (邀请返利) feature switch

  // OpenAI fast/flex policy
  openai_fast_policy_settings?: OpenAIFastPolicySettings;

  // 利润门（全站一档）：账号倍率 > 用户倍率 × (1 − min_margin − safety_buffer) 的资源不派
  profit_control_enabled: boolean;
  profit_min_margin: number;
  profit_safety_buffer: number;
}

export interface UpdateSettingsRequest {
  grok_default_text_model?: string;
  grok_cross_client_model_map_enabled?: boolean;
  grok_default_base_url_mode?: string;
  account_scheduling_thresholds?: AccountSchedulingThresholdsMap;
  enable_identity_patch?: boolean;
  identity_patch_prompt?: string;
  ops_realtime_monitoring_enabled?: boolean;
  ops_query_mode_default?: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds?: number;
  min_claude_code_version?: string;
  max_claude_code_version?: string;
  openai_ttft_mode?: string;
  enable_fingerprint_unification?: boolean;
  enable_metadata_passthrough?: boolean;
  enable_cch_signing?: boolean;
  enable_claude_oauth_system_prompt_injection?: boolean;
  claude_oauth_system_prompt?: string;
  claude_oauth_system_prompt_blocks?: string;
  enable_anthropic_cache_ttl_1h_injection?: boolean;
  rewrite_message_cache_control?: boolean;
  enable_client_dateline_normalization?: boolean;
  antigravity_user_agent_version?: string;
  openai_codex_user_agent?: string;
  openai_codex_client_version?: string;
  openai_codex_version_auto_sync_enabled?: boolean;
  // codex_cli_only 加固
  min_codex_version?: string;
  max_codex_version?: string;
  codex_cli_only_blacklist?: string;
  codex_cli_only_whitelist?: string;
  codex_cli_only_allow_app_server_clients?: boolean;
  codex_cli_only_engine_fingerprint_signals?: string;
  // 风控中心功能开关
  risk_control_enabled?: boolean;

  // Cyber session block
  cyber_session_block_enabled?: boolean;
  cyber_session_block_ttl_seconds?: number;

  // Channel Monitor feature switch
  channel_monitor_hide_throughput?: boolean;
  channel_monitor_hide_user_ranking?: boolean;

  // Available Channels feature switch

  // Affiliate (邀请返利) feature switch

  // OpenAI fast/flex policy
  openai_fast_policy_settings?: OpenAIFastPolicySettings;

  profit_control_enabled?: boolean;
  profit_min_margin?: number;
  profit_safety_buffer?: number;
}

/**
 * Get all system settings
 * @returns System settings
 */
export async function getSettings(): Promise<SystemSettings> {
  const { data } = await apiClient.get<SystemSettings>("/admin/settings");
  return data;
}

/**
 * Update system settings
 * @param settings - Partial settings to update
 * @returns Updated settings
 */
export async function updateSettings(
  settings: UpdateSettingsRequest,
): Promise<SystemSettings> {
  const { data } = await apiClient.put<SystemSettings>(
    "/admin/settings",
    settings,
  );
  return data;
}

// ==================== Overload Cooldown Settings ====================

/**
 * Overload cooldown settings interface (529 handling)
 */
export interface OverloadCooldownSettings {
  enabled: boolean;
  cooldown_minutes: number;
}

export async function getOverloadCooldownSettings(): Promise<OverloadCooldownSettings> {
  const { data } = await apiClient.get<OverloadCooldownSettings>(
    "/admin/settings/overload-cooldown",
  );
  return data;
}

export async function updateOverloadCooldownSettings(
  settings: OverloadCooldownSettings,
): Promise<OverloadCooldownSettings> {
  const { data } = await apiClient.put<OverloadCooldownSettings>(
    "/admin/settings/overload-cooldown",
    settings,
  );
  return data;
}

// ==================== 429 Rate Limit Cooldown Settings ====================

export interface RateLimit429CooldownSettings {
  enabled: boolean;
  cooldown_seconds: number;
}

export async function getRateLimit429CooldownSettings(): Promise<RateLimit429CooldownSettings> {
  const { data } = await apiClient.get<RateLimit429CooldownSettings>(
    "/admin/settings/rate-limit-429-cooldown",
  );
  return data;
}

export async function updateRateLimit429CooldownSettings(
  settings: RateLimit429CooldownSettings,
): Promise<RateLimit429CooldownSettings> {
  const { data } = await apiClient.put<RateLimit429CooldownSettings>(
    "/admin/settings/rate-limit-429-cooldown",
    settings,
  );
  return data;
}

// ==================== Stream Timeout Settings ====================

/**
 * Stream timeout settings interface
 */
export interface StreamTimeoutSettings {
  enabled: boolean;
  action: "temp_unsched" | "error" | "none";
  temp_unsched_minutes: number;
  threshold_count: number;
  threshold_window_minutes: number;
}

/**
 * Get stream timeout settings
 * @returns Stream timeout settings
 */
export async function getStreamTimeoutSettings(): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.get<StreamTimeoutSettings>(
    "/admin/settings/stream-timeout",
  );
  return data;
}

/**
 * Update stream timeout settings
 * @param settings - Stream timeout settings to update
 * @returns Updated settings
 */
export async function updateStreamTimeoutSettings(
  settings: StreamTimeoutSettings,
): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.put<StreamTimeoutSettings>(
    "/admin/settings/stream-timeout",
    settings,
  );
  return data;
}

// ==================== Rectifier Settings ====================

/**
 * Rectifier settings interface
 */
export interface RectifierSettings {
  enabled: boolean;
  thinking_signature_enabled: boolean;
  thinking_budget_enabled: boolean;
  apikey_signature_enabled: boolean;
  apikey_signature_patterns: string[];
}

/**
 * Get rectifier settings
 * @returns Rectifier settings
 */
export async function getRectifierSettings(): Promise<RectifierSettings> {
  const { data } = await apiClient.get<RectifierSettings>(
    "/admin/settings/rectifier",
  );
  return data;
}

/**
 * Update rectifier settings
 * @param settings - Rectifier settings to update
 * @returns Updated settings
 */
export async function updateRectifierSettings(
  settings: RectifierSettings,
): Promise<RectifierSettings> {
  const { data } = await apiClient.put<RectifierSettings>(
    "/admin/settings/rectifier",
    settings,
  );
  return data;
}

// ==================== OpenAI Fast Policy Settings ====================

/**
 * OpenAI fast/flex policy rule interface.
 * Matches backend dto.OpenAIFastPolicyRule.
 */
export interface OpenAIFastPolicyRule {
  service_tier: "all" | "priority" | "flex" | "ultrafast" | "missing";
  action: "pass" | "filter" | "block" | "force_priority";
  scope: "all" | "oauth" | "apikey" | "bedrock";
  user_ids?: number[];
  error_message?: string;
  model_whitelist?: string[];
  fallback_action?: "pass" | "filter" | "block" | "force_priority";
  fallback_error_message?: string;
}

/**
 * OpenAI fast/flex policy settings interface.
 */
export interface OpenAIFastPolicySettings {
  rules: OpenAIFastPolicyRule[];
}

// ==================== Beta Policy Settings ====================

/**
 * Beta policy rule interface
 */
export interface BetaPolicyRule {
  beta_token: string;
  action: "pass" | "filter" | "block";
  scope: "all" | "oauth" | "apikey" | "bedrock";
  error_message?: string;
  model_whitelist?: string[];
  fallback_action?: "pass" | "filter" | "block";
  fallback_error_message?: string;
}

/**
 * Beta policy settings interface
 */
export interface BetaPolicySettings {
  rules: BetaPolicyRule[];
}

/**
 * Get beta policy settings
 * @returns Beta policy settings
 */
export async function getBetaPolicySettings(): Promise<BetaPolicySettings> {
  const { data } = await apiClient.get<BetaPolicySettings>(
    "/admin/settings/beta-policy",
  );
  return data;
}

/**
 * Update beta policy settings
 * @param settings - Beta policy settings to update
 * @returns Updated settings
 */
export async function updateBetaPolicySettings(
  settings: BetaPolicySettings,
): Promise<BetaPolicySettings> {
  const { data } = await apiClient.put<BetaPolicySettings>(
    "/admin/settings/beta-policy",
    settings,
  );
  return data;
}

// --- Web Search Emulation Config ---

export interface WebSearchProviderConfig {
  type: "brave" | "tavily";
  api_key: string;
  api_key_configured: boolean;
  quota_limit: number | null;
  subscribed_at: number | null;
  quota_used?: number;
  proxy_id: number | null;
  expires_at: number | null;
}

export interface WebSearchEmulationConfig {
  enabled: boolean;
  providers: WebSearchProviderConfig[];
}

export interface WebSearchTestResult {
  provider: string;
  results: { url: string; title: string; snippet: string; page_age?: string }[];
  query: string;
}

export async function getWebSearchEmulationConfig(): Promise<WebSearchEmulationConfig> {
  const { data } = await apiClient.get<WebSearchEmulationConfig>(
    "/admin/settings/web-search-emulation",
  );
  return data;
}

export async function updateWebSearchEmulationConfig(
  config: WebSearchEmulationConfig,
): Promise<WebSearchEmulationConfig> {
  const { data } = await apiClient.put<WebSearchEmulationConfig>(
    "/admin/settings/web-search-emulation",
    config,
  );
  return data;
}

export async function testWebSearchEmulation(
  query: string,
): Promise<WebSearchTestResult> {
  const { data } = await apiClient.post<WebSearchTestResult>(
    "/admin/settings/web-search-emulation/test",
    { query },
  );
  return data;
}

export async function resetWebSearchUsage(payload: {
  provider_type: string;
}): Promise<void> {
  await apiClient.post(
    "/admin/settings/web-search-emulation/reset-usage",
    payload,
  );
}

export const settingsAPI = {
  getSettings,
  updateSettings,
  getOverloadCooldownSettings,
  updateOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  updateRateLimit429CooldownSettings,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  getRectifierSettings,
  updateRectifierSettings,
  getBetaPolicySettings,
  updateBetaPolicySettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  testWebSearchEmulation,
  resetWebSearchUsage,
};

export default settingsAPI;
