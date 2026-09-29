import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({}),
}))

describe('TotpLoginModal', () => {
  it('does not render inline red text for verification errors', async () => {
    const wrapper = mount(TotpLoginModal, {
      props: {
        tempToken: 'temp-token',
        userEmailMasked: 'u***@example.com',
      },
    })

    ;(wrapper.vm as unknown as { setError: (message: string) => void }).setError('Invalid code')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).not.toContain('Invalid code')
    expect(wrapper.find('.bg-red-50').exists()).toBe(false)
  })
})
