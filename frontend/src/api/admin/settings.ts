/**
 * Admin Settings API endpoints
 * Handles system settings management for administrators
 */

import { apiClient } from "../client";

/**
 * System settings interface
 */
export interface SystemSettings {
  // Ops Monitoring (vNext)
  ops_monitoring_enabled: boolean; // 只读：取部署配置 OPS_ENABLED，PUT 不接受
  ops_realtime_monitoring_enabled: boolean;
  ops_query_mode_default: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds: number;

  // 只读：Web Search 模拟是否生效（有配了 Key 的服务商）
  web_search_emulation_enabled?: boolean;

  // 风控中心功能开关
  risk_control_enabled: boolean;

  // Cyber session block
  cyber_session_block_enabled: boolean;
  cyber_session_block_ttl_seconds: number;

  // Channel Monitor feature switch
  channel_monitor_hide_throughput?: boolean;
  channel_monitor_hide_user_ranking?: boolean;

  // 利润门（全站一档）：账号倍率 > 用户倍率 × (1 − profit_min_margin) 的资源不派；0 = 关
  profit_min_margin: number;
}

export interface UpdateSettingsRequest {
  ops_realtime_monitoring_enabled?: boolean;
  ops_query_mode_default?: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds?: number;
  // 风控中心功能开关
  risk_control_enabled?: boolean;

  // Cyber session block
  cyber_session_block_enabled?: boolean;
  cyber_session_block_ttl_seconds?: number;

  // Channel Monitor feature switch
  channel_monitor_hide_throughput?: boolean;
  channel_monitor_hide_user_ranking?: boolean;

  profit_min_margin?: number;
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

// --- Web Search Emulation Config ---

// 后台只配服务商与 Key：配了带 Key 的服务商就生效，不限次数、不走服务商级代理
export interface WebSearchProviderConfig {
  type: "brave" | "tavily";
  api_key: string;
  api_key_configured: boolean;
  expires_at: number | null;
}

export interface WebSearchEmulationConfig {
  // 只读：有配了 Key 的服务商（渠道表单据它决定显不显示渠道级开关）
  enabled: boolean;
  providers: WebSearchProviderConfig[];
}

export interface UpdateWebSearchEmulationRequest {
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
  config: UpdateWebSearchEmulationRequest,
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

export const settingsAPI = {
  getSettings,
  updateSettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  testWebSearchEmulation,
};

export default settingsAPI;
