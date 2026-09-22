/**
 * Admin API barrel export
 * Centralized exports for all admin API modules
 */

import dashboardAPI from './dashboard'
import usersAPI from './users'
import accountsAPI from './accounts'
import proxiesAPI from './proxies'
import redeemAPI from './redeem'
import announcementsAPI from './announcements'
import settingsAPI from './settings'
import systemAPI from './system'
import subscriptionsAPI from './subscriptions'
import usageAPI from './usage'
import geminiAPI from './gemini'
import antigravityAPI from './antigravity'
import grokAPI from './grok'
import cnProvidersAPI from './cnProviders'
import userAttributesAPI from './userAttributes'
import opsAPI from './ops'
import errorPassthroughAPI from './errorPassthrough'
import scheduledTestsAPI from './scheduledTests'
import tlsFingerprintProfileAPI from './tlsFingerprintProfile'
import modelCatalogAPI from './modelCatalog'
import channelMonitorAPI from './channelMonitor'
import channelMonitorTemplateAPI from './channelMonitorTemplate'
import adminPaymentAPI from './payment'
import riskControlAPI from './riskControl'
import adminComplianceAPI from './compliance'
import auditAPI from './audit'
import affiliatesAPI from './affiliates'

/**
 * Unified admin API object for convenient access
 */
export const adminAPI = {
  dashboard: dashboardAPI,
  users: usersAPI,
  accounts: accountsAPI,
  proxies: proxiesAPI,
  redeem: redeemAPI,
  announcements: announcementsAPI,
  settings: settingsAPI,
  system: systemAPI,
  subscriptions: subscriptionsAPI,
  usage: usageAPI,
  gemini: geminiAPI,
  antigravity: antigravityAPI,
  grok: grokAPI,
  cnProviders: cnProvidersAPI,
  userAttributes: userAttributesAPI,
  ops: opsAPI,
  errorPassthrough: errorPassthroughAPI,
  scheduledTests: scheduledTestsAPI,
  tlsFingerprintProfiles: tlsFingerprintProfileAPI,
  modelCatalog: modelCatalogAPI,
  channelMonitor: channelMonitorAPI,
  channelMonitorTemplate: channelMonitorTemplateAPI,
  payment: adminPaymentAPI,
  riskControl: riskControlAPI,
  compliance: adminComplianceAPI,
  affiliates: affiliatesAPI,
  audit: auditAPI
}

export {
  dashboardAPI,
  usersAPI,
  accountsAPI,
  proxiesAPI,
  redeemAPI,
  announcementsAPI,
  settingsAPI,
  systemAPI,
  subscriptionsAPI,
  usageAPI,
  geminiAPI,
  antigravityAPI,
  grokAPI,
  cnProvidersAPI,
  userAttributesAPI,
  opsAPI,
  errorPassthroughAPI,
  scheduledTestsAPI,
  tlsFingerprintProfileAPI,
  channelMonitorAPI,
  channelMonitorTemplateAPI,
  adminPaymentAPI,
  riskControlAPI,
  adminComplianceAPI,
  affiliatesAPI,
  auditAPI
}

export default adminAPI

// Re-export types used by components
export type { AuditLog, AuditLogQuery, AuditLogListResponse } from './audit'
export type { BalanceHistoryItem } from './users'
export type { ErrorPassthroughRule, CreateRuleRequest, UpdateRuleRequest } from './errorPassthrough'
export type { TLSFingerprintProfile, CreateProfileRequest, UpdateProfileRequest } from './tlsFingerprintProfile'
export type { ContentModerationConfig, ContentModerationLog, ModerationMode } from './riskControl'
