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

  // 风控中心功能开关
  risk_control_enabled: boolean;

  // 利润门（全站一档）：上游成本比（渠道给该模型的上游价 ÷ 官方价，逐项、逐段取最高）> 用户倍率 × (1 − profit_min_margin) 的渠道这次请求不派；0 = 关
  profit_min_margin: number;
}

export interface UpdateSettingsRequest {
  ops_realtime_monitoring_enabled?: boolean;
  ops_query_mode_default?: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds?: number;
  // 风控中心功能开关
  risk_control_enabled?: boolean;

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

export const settingsAPI = {
  getSettings,
  updateSettings,
};

export default settingsAPI;
