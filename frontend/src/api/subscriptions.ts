/**
 * User Subscription API
 * API for regular users to view their own subscriptions
 */

import { apiClient } from './client'
import type { UserSubscription } from '@/types'

/**
 * Get list of current user's subscriptions
 */
export async function getMySubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions')
  return response.data
}

/**
 * Get current user's active subscriptions
 */
export async function getActiveSubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions/active')
  return response.data
}

export default {
  getMySubscriptions,
  getActiveSubscriptions
}
