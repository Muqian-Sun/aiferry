import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserEditModal from '../UserEditModal.vue'

const { update, updateUserAttributeValues } = vi.hoisted(() => ({
  update: vi.fn(),
  updateUserAttributeValues: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { update },
    userAttributes: { updateUserAttributeValues }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() })
}))

// useStepUp pulls in the API client, which needs the real i18n instance.
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key
  })
}))

const mountModal = (concurrency: number) => mount(UserEditModal, {
  props: {
    show: true,
    user: { id: 7, email: 'user@example.test', username: 'user', notes: '', role: 'user', concurrency, rpm_limit: 0 } as never
  },
  global: {
    stubs: {
      BaseDialog: {
        props: ['show', 'title'],
        template: '<div v-if="show"><slot /><slot name="footer" /></div>'
      },
      Select: true,
      Icon: true,
      UserAttributeForm: true,
      TotpStepUpDialog: true
    }
  }
})

describe('UserEditModal concurrency', () => {
  beforeEach(() => {
    update.mockReset()
    updateUserAttributeValues.mockReset()
    update.mockResolvedValue({})
  })

  // Regression coverage for issue #5977: the gateway treats concurrency <= 0 as
  // unlimited (AcquireUserSlot) and both the batch limits endpoint and the bulk
  // edit modal accept 0, so this dialog must not be the only place that rejects
  // it — doing so blocked every other edit on such a user.
  it('saves an unlimited (0) concurrency instead of blocking the whole form', async () => {
    const wrapper = mountModal(0)

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ concurrency: 0 }))
    expect(wrapper.emitted('success')).toBeTruthy()
  })

  it('still rejects a negative concurrency', async () => {
    const wrapper = mountModal(3)

    await wrapper.get('[data-test="concurrency-input"]').setValue('-1')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(update).not.toHaveBeenCalled()
  })
})

describe('UserEditModal rate multiplier', () => {
  beforeEach(() => {
    update.mockReset().mockResolvedValue({})
    updateUserAttributeValues.mockReset()
  })

  // 用户价 = 目录价 × rate_multiplier：编辑框预填用户当前值，保存时随表单一起提交。
  it('prefills the user multiplier and submits it with the form', async () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: { id: 7, email: 'user@example.test', username: 'user', notes: '', role: 'user', concurrency: 1, rpm_limit: 0, rate_multiplier: 0.5, custom_rate_multiplier: 0.5 } as never
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
          Select: true,
          Icon: true,
          UserAttributeForm: true,
          TotpStepUpDialog: true
        }
      }
    })

    const input = wrapper.get('[data-testid="user-rate-multiplier"]')
    expect((input.element as HTMLInputElement).value).toBe('0.5')
    await input.setValue('2')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ rate_multiplier: 2 }))
  })
})
