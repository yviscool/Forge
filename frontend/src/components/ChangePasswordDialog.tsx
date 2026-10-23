import React, { useState } from 'react'
import { KeyRound } from 'lucide-react'
import { api } from '../api/client'
import { Button } from './ui/button'
import { Dialog } from './ui/dialog'
import { Input } from './ui/input'
import { useToast } from './ui/toast'

interface ChangePasswordDialogProps {
  open: boolean
  onClose: () => void
}

export function ChangePasswordDialog({ open, onClose }: ChangePasswordDialogProps) {
  const toast = useToast()
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const resetFields = () => {
    setOldPassword('')
    setNewPassword('')
    setConfirmPassword('')
    setError('')
  }

  const close = () => {
    if (busy) return
    resetFields()
    onClose()
  }

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    setError('')
    if (newPassword.length < 10) {
      setError('新密码至少需要 10 个字符。')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('两次输入的新密码不一致。')
      return
    }
    if (newPassword === oldPassword) {
      setError('新密码不能与旧密码相同。')
      return
    }
    setBusy(true)
    try {
      await api.changePassword(oldPassword, newPassword)
      toast.success('密码已更新', '其他设备上的旧会话已被注销。')
      resetFields()
      onClose()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '密码更新失败，请重试。')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={close}
      title="修改登录密码"
      eyebrow="Account security"
      width="sm"
      dismissible={!busy}
      footer={
        <>
          <Button variant="ghost" disabled={busy} onClick={close}>
            取消
          </Button>
          <Button type="submit" form="change-password-form" loading={busy}>
            更新密码
          </Button>
        </>
      }
    >
      <form id="change-password-form" onSubmit={submit} className="space-y-4">
        <div className="flex items-start gap-3 rounded-xl border border-sky-200 bg-sky-50 px-3.5 py-3 text-xs leading-5 text-sky-900">
          <KeyRound className="mt-0.5 h-4 w-4 shrink-0" />
          <p>密码至少 10 位。修改成功后，除当前会话外该账号的其他登录设备会下线。</p>
        </div>
        <label className="block space-y-1.5">
          <span className="text-xs font-bold text-slate-700">当前密码</span>
          <Input
            type="password"
            autoComplete="current-password"
            value={oldPassword}
            onChange={(event) => setOldPassword(event.target.value)}
          />
        </label>
        <label className="block space-y-1.5">
          <span className="text-xs font-bold text-slate-700">新密码</span>
          <Input
            type="password"
            autoComplete="new-password"
            value={newPassword}
            onChange={(event) => setNewPassword(event.target.value)}
          />
        </label>
        <label className="block space-y-1.5">
          <span className="text-xs font-bold text-slate-700">确认新密码</span>
          <Input
            type="password"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
          />
        </label>
        {error && (
          <p role="alert" className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-700">
            {error}
          </p>
        )}
      </form>
    </Dialog>
  )
}
