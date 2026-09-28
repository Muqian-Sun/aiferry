/**
 * Structure contracts: channel-monitor-v2 + studio shells must use project
 * design-system utility classes rather than isolated flat RGB skins.
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../..')

function read(rel: string) {
  return readFileSync(resolve(root, rel), 'utf8')
}

describe('channel-monitor-v2 design system structure', () => {
  it('MonitorSettingsPanel uses card, btn-primary, tabs', () => {
    const src = read('features/channel-monitor-v2/MonitorSettingsPanel.vue')
    expect(src).toContain('btn btn-primary')
    expect(src).toContain('class="card')
    expect(src).toContain('tab-active')
    expect(src).toMatch(/max-h-\[min\(40vh/)
  })

  // V1 下线后只剩配置面板；标题由管理站页头给出，页面里不再自带一个
  it('admin ChannelMonitorView renders only the settings panel under the shared page header', () => {
    const src = read('views/admin/ChannelMonitorView.vue')
    expect(src).toContain('MonitorSettingsPanel')
    expect(src).not.toContain('page-title')
  })
})
