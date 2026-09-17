import { describe, expect, it } from 'vitest'

import {
  ADMIN_UI_REQUEST_HEADER,
  USER_UI_REQUEST_HEADER,
  isUserTimingAPIPath,
  shouldMarkAdminUIRequest,
  shouldMarkUserUIRequest,
} from '@/api/adminUIRequest'

describe('Admin UI request marker', () => {
  it('uses the stable request header name', () => {
    expect(ADMIN_UI_REQUEST_HEADER).toBe('X-Admin-UI-Request')
  })

  it.each([
    '/admin',
    '/admin/users',
    '/api/v1/admin',
    '/api/v1/admin/accounts?status=active',
    'https://api.example.test/api/v1/admin/dashboard',
  ])('marks Admin API request %s even from the user site', (requestURL) => {
    expect(shouldMarkAdminUIRequest(requestURL, 'user')).toBe(true)
  })

  it.each(['/keys', '/groups/available', '/auth/me', '/announcements', '/dashboard'])(
    'marks every request %s sent from the admin console',
    (requestURL) => {
      expect(shouldMarkAdminUIRequest(requestURL, 'admin')).toBe(true)
    }
  )

  it.each(['/keys', '/api/v1/administer', '/administrator', ''])(
    'does not mark non-admin request %s from the user site',
    (requestURL) => {
      expect(shouldMarkAdminUIRequest(requestURL, 'user')).toBe(false)
    }
  )
})

describe('User UI request marker', () => {
  it('uses the stable request header name', () => {
    expect(USER_UI_REQUEST_HEADER).toBe('X-User-UI-Request')
  })

  it.each([
    '/auth/me',
    '/auth/revoke-all-sessions',
    '/auth/oauth/bind-token',
    '/user',
    '/user/profile',
    '/user/password',
    '/user/notify-email/send-code',
    '/user/totp/status',
    '/user/aff',
    '/user/platform-quotas',
    '/keys',
    '/keys/12',
    '/groups/available',
    '/groups/rates',
    '/channels/available',
    '/usage',
    '/usage/stats',
    '/usage/dashboard/snapshot-v2',
    '/announcements',
    '/announcements/3/read',
    '/redeem',
    '/redeem/history',
    '/subscriptions',
    '/subscriptions/active',
    '/channel-monitors',
    '/channel-monitors/9/status',
    '/payment/config',
    '/payment/plans',
    '/payment/orders',
    '/payment/orders/my',
    '/api/v1/auth/me',
    '/api/v1/keys?page=1',
    'https://api.example.test/api/v1/payment/orders/1',
  ])('marks user timing API %s', (requestURL) => {
    expect(shouldMarkUserUIRequest(requestURL)).toBe(true)
    expect(isUserTimingAPIPath(requestURL)).toBe(true)
  })

  it.each([
    '/auth/login',
    '/settings/public',
    '/admin/users',
    '/groups',
    '/channels',
    '/payment/public/orders/verify',
    '/payment/webhook/stripe',
    '/api/v1/payment/public/orders/resolve',
    '',
  ])('does not mark non-user timing API %s', (requestURL) => {
    expect(shouldMarkUserUIRequest(requestURL)).toBe(false)
  })
})
