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
import pricingAPI from './pricing'
import adminPaymentAPI from './payment'
import riskControlAPI from './riskControl'

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
  pricing: pricingAPI,
  payment: adminPaymentAPI,
  riskControl: riskControlAPI
}

export {
  dashboardAPI,
  usersAPI,
  accountsAPI,
  proxiesAPI,
  redeemAPI,
  announcementsAPI,
  settingsAPI,
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
  adminPaymentAPI,
  riskControlAPI
}

export default adminAPI

// Re-export types used by components
export type { BalanceHistoryItem } from './users'
export type { ErrorPassthroughRule, CreateRuleRequest, UpdateRuleRequest } from './errorPassthrough'
export type { TLSFingerprintProfile, CreateProfileRequest, UpdateProfileRequest } from './tlsFingerprintProfile'
export type { ContentModerationConfig, ContentModerationLog, ModerationMode } from './riskControl'
