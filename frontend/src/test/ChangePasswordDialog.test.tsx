import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { api } from '../api/client'
import { ChangePasswordDialog } from '../components/ChangePasswordDialog'
import { ToastProvider } from '../components/ui/toast'

function fillPasswords(
  oldPassword: string,
  newPassword: string,
  confirmPassword: string
) {
  return {
    oldPassword,
    newPassword,
    confirmPassword,
  }
}

describe('ChangePasswordDialog', () => {
  it('validates password length, confirmation, and reuse before calling the API', async () => {
    const user = userEvent.setup()
    const changePassword = vi
      .spyOn(api, 'changePassword')
      .mockResolvedValue({ ok: true })
    render(
      <ToastProvider>
        <ChangePasswordDialog open onClose={vi.fn()} />
      </ToastProvider>
    )

    const oldInput = screen.getByLabelText('当前密码')
    const newInput = screen.getByLabelText('新密码')
    const confirmInput = screen.getByLabelText('确认新密码')
    const values = fillPasswords('OldPassword1!', 'short', 'short')
    await user.type(oldInput, values.oldPassword)
    await user.type(newInput, values.newPassword)
    await user.type(confirmInput, values.confirmPassword)
    await user.click(screen.getByRole('button', { name: '更新密码' }))
    expect(screen.getByRole('alert')).toHaveTextContent('新密码至少需要 10 个字符')

    await user.clear(newInput)
    await user.clear(confirmInput)
    await user.type(newInput, 'NewPassword1!')
    await user.type(confirmInput, 'NewPassword2!')
    await user.click(screen.getByRole('button', { name: '更新密码' }))
    expect(screen.getByRole('alert')).toHaveTextContent('两次输入的新密码不一致')

    await user.clear(oldInput)
    await user.clear(newInput)
    await user.clear(confirmInput)
    await user.type(oldInput, 'SamePassword1!')
    await user.type(newInput, 'SamePassword1!')
    await user.type(confirmInput, 'SamePassword1!')
    await user.click(screen.getByRole('button', { name: '更新密码' }))
    expect(screen.getByRole('alert')).toHaveTextContent('新密码不能与旧密码相同')
    expect(changePassword).not.toHaveBeenCalled()
  })

  it('submits a valid password and closes after success', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const changePassword = vi
      .spyOn(api, 'changePassword')
      .mockResolvedValue({ ok: true })
    render(
      <ToastProvider>
        <ChangePasswordDialog open onClose={onClose} />
      </ToastProvider>
    )

    await user.type(screen.getByLabelText('当前密码'), 'OldPassword1!')
    await user.type(screen.getByLabelText('新密码'), 'NewPassword1!')
    await user.type(screen.getByLabelText('确认新密码'), 'NewPassword1!')
    await user.click(screen.getByRole('button', { name: '更新密码' }))

    await waitFor(() =>
      expect(changePassword).toHaveBeenCalledWith('OldPassword1!', 'NewPassword1!')
    )
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    expect(screen.getByText('密码已更新')).toBeInTheDocument()
  })

  it('surfaces API failures and can be cancelled without submitting', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.spyOn(api, 'changePassword').mockRejectedValue(new Error('旧密码错误'))
    render(
      <ToastProvider>
        <ChangePasswordDialog open onClose={onClose} />
      </ToastProvider>
    )

    await user.type(screen.getByLabelText('当前密码'), 'WrongPassword1!')
    await user.type(screen.getByLabelText('新密码'), 'NewPassword1!')
    await user.type(screen.getByLabelText('确认新密码'), 'NewPassword1!')
    await user.click(screen.getByRole('button', { name: '更新密码' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('旧密码错误')

    await user.click(screen.getByRole('button', { name: '取消' }))
    expect(onClose).toHaveBeenCalledOnce()
  })
})
