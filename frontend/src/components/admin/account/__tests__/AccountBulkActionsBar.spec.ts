import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('AccountBulkActionsBar', () => {
  it('stays hidden until a row is selected', () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: { selectedIds: [], totalResults: 45, selectingAll: false, allResultsSelected: false }
    })
    expect(wrapper.find('[data-testid="bulk-bar"]').exists()).toBe(false)
  })

  it('offers selecting all filtered results once rows are selected', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: { selectedIds: [1, 2], totalResults: 45, selectingAll: false, allResultsSelected: false }
    })

    const button = wrapper.get('[data-testid="bulk-select-all-results"]')
    expect(button.text()).toContain('admin.accounts.bulkActions.selectAllResults')
    await button.trigger('click')
    expect(wrapper.emitted('select-all-results')).toHaveLength(1)
  })

  it('preserves the upstream billing probe action from v0.1.166', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: { selectedIds: [1], totalResults: 45, selectingAll: false, allResultsSelected: false },
      attachTo: document.body
    })

    // 探测倍率收在「更多」菜单里（菜单挂到 body 上）
    await wrapper.get('[data-testid="bulk-more"]').trigger('click')
    await flushPromises()
    const item = Array.from(document.body.querySelectorAll('button')).find((node) =>
      node.textContent?.includes('admin.accounts.bulkActions.probeUpstreamBilling')
    )
    expect(item).toBeDefined()
    item!.click()
    expect(wrapper.emitted('probe-upstream-billing')).toHaveLength(1)
    wrapper.unmount()
  })
})
