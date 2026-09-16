import { describe, expect, it } from 'vitest'
import { DEFAULT_ADMIN_PORT, adminConsoleLoginUrl } from '../adminConsoleUrl'

describe('adminConsoleLoginUrl', () => {
  it('points at the admin port on the same host instead of the current port', () => {
    expect(adminConsoleLoginUrl({ protocol: 'http:', hostname: '192.168.1.10' }, 9001)).toBe('http://192.168.1.10:9001/login')
    expect(adminConsoleLoginUrl({ protocol: 'https:', hostname: 'gateway.example.com' }, DEFAULT_ADMIN_PORT)).toBe('https://gateway.example.com:8081/login')
  })
})
