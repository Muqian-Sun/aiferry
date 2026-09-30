import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'

const { changePasswordMock } = vi.hoisted(() => ({
  changePasswordMock: vi.fn(),
}))

vi.mock('@/api', () => ({
  userAPI: {
    changePassword: changePasswordMock
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        const translations: Record<string, string> = {
          'profile.changePassword': 'Change Password',
          'profile.currentPassword': 'Current Password',
          'profile.newPassword': 'New Password',
          'profile.confirmNewPassword': 'Confirm New Password',
          'profile.passwordHint': 'Password must be at least 6 characters long',
          'profile.changingPassword': 'Changing...',
          'profile.changePasswordButton': 'Change Password',
          'profile.passwordsNotMatch': 'New passwords do not match',
          'profile.passwordTooShort': 'Password must be at least 6 characters long',
          'profile.passwordChangeSuccess': 'Password changed successfully',
          'profile.passwordChangeFailed': 'Failed to change password'
        }
        return translations[key] ?? key
      }
    })
  }
})

describe('ProfilePasswordForm', () => {
  it('shows validation failures inline', async () => {
    const wrapper = mount(ProfilePasswordForm)

    await wrapper.get('#old_password').setValue('old-password')
    await wrapper.get('#new_password').setValue('new-password')
    await wrapper.get('#confirm_password').setValue('different-password')
    await wrapper.get('form').trigger('submit.prevent')

    expect(changePasswordMock).not.toHaveBeenCalled()
    expect(wrapper.get('.input-error-text').text()).toContain('New passwords do not match')
  })

  it.each([
    [{ status: 400, code: 'PASSWORD_INCORRECT', message: 'current password is incorrect' }, 'current password is incorrect'],
    [{ response: { data: { detail: 'backend failure' } } }, 'backend failure'],
    [{}, 'Failed to change password'],
  ])('shows API failure %j inline', async (error, expectedMessage) => {
    changePasswordMock.mockRejectedValue(error)

    const wrapper = mount(ProfilePasswordForm)

    await wrapper.get('#old_password').setValue('old-password')
    await wrapper.get('#new_password').setValue('new-password')
    await wrapper.get('#confirm_password').setValue('new-password')
    await wrapper.get('form').trigger('submit.prevent')

    expect(changePasswordMock).toHaveBeenCalledWith('old-password', 'new-password')
    expect(wrapper.get('.input-error-text').text()).toContain(expectedMessage)
  })
})
