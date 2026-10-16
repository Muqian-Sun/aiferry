import { describe, expect, it } from 'vitest'
import { isConsoleShellPath } from '../consoleShell'

describe('isConsoleShellPath', () => {
  it('treats console pages as the console shell', () => {
    for (const path of ['/dashboard', '/keys', '/usage', '/billing/recharge', '/profile/security', '/custom/3']) {
      expect(isConsoleShellPath(path, { authenticated: true })).toBe(true)
    }
  })

  it('keeps public, auth, and payment return pages out of the console shell', () => {
    for (const path of ['/home', '/login', '/legal/1', '/key-usage', '/payment/result', '/payment/stripe-popup', '/missing']) {
      expect(isConsoleShellPath(path, { authenticated: true })).toBe(false)
    }
  })

  it('uses the console shell for the model and status pages only when opened from the console', () => {
    expect(isConsoleShellPath('/model-plaza', { authenticated: true, fromConsole: false })).toBe(false)
    expect(isConsoleShellPath('/model-plaza', { authenticated: false, fromConsole: true })).toBe(false)
    expect(isConsoleShellPath('/model-plaza', { authenticated: true, fromConsole: true })).toBe(true)
    expect(isConsoleShellPath('/status', { authenticated: true, fromConsole: false })).toBe(false)
    expect(isConsoleShellPath('/status', { authenticated: true, fromConsole: true })).toBe(true)
  })

  it('treats the stripe payment popup as outside the console', () => {
    expect(isConsoleShellPath('/payment/stripe', { authenticated: true, stripePopup: false })).toBe(true)
    expect(isConsoleShellPath('/payment/stripe', { authenticated: true, stripePopup: true })).toBe(false)
  })
})
