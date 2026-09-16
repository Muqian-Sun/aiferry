/**
 * API Client for Sub2API Backend
 * Central export point for all API modules
 */

// Re-export the HTTP client
export { apiClient } from './client'

// Auth API
export { authAPI, isTotp2FARequired, type LoginResponse } from './auth'

// User APIs
export { keysAPI } from './keys'
export { usageAPI } from './usage'
export { userAPI } from './user'
export { redeemAPI, type RedeemHistoryItem } from './redeem'
export { paymentAPI } from './payment'
export { userGroupsAPI } from './groups'
export { userChannelsAPI } from './channels'
export * as batchImageAPI from './batchImage'
export { totpAPI } from './totp'
export { passkeyAPI, type PasskeyCredentialSummary } from './passkey'
export { default as announcementsAPI } from './announcements'
export { channelMonitorUserAPI } from './channelMonitor'

// 管理 API 不从这里导出：管理端代码直接从 '@/api/admin' 引入，用户站经由本入口不会带上管理端代码

// Default export
export { default } from './client'
