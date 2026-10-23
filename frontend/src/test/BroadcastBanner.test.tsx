import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import type { BroadcastMessage } from '../api/client'
import {
  BroadcastBanner,
  DismissBroadcastButton,
} from '../components/BroadcastBanner'
import { useStore } from '../store/useStore'

const message: BroadcastMessage = {
  id: 'message-1',
  contestId: 'contest-1',
  senderName: '监考教师',
  title: '数据更正',
  content: '第二组样例已经更新。',
  level: 'warning',
  createdAt: '2026-10-21T02:00:00.000Z',
}

describe('BroadcastBanner', () => {
  beforeEach(() => {
    useStore.setState({
      activeAlert: null,
      unreadBroadcasts: 0,
    })
  })

  it('shows pinned notices and opens the complete announcement history', async () => {
    const user = userEvent.setup()
    render(<BroadcastBanner notice="请保存代码后离场。" messages={[message]} />)

    expect(screen.getByRole('status')).toHaveTextContent('请保存代码后离场。')
    await user.click(screen.getByRole('button', { name: '查看通知（1）' }))

    const dialog = screen.getByRole('dialog', { name: '考场通知' })
    expect(dialog).toHaveTextContent('请保存代码后离场。')
    expect(dialog).toHaveTextContent(message.content)
    const closeButtons = within(dialog).getAllByRole('button', { name: '关闭' })
    await user.click(closeButtons[closeButtons.length - 1])
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('renders urgent live alerts and clears the acknowledged alert', async () => {
    const user = userEvent.setup()
    useStore.getState().setActiveAlert({
      id: 'alert-1',
      title: '紧急更正',
      content: '请立即重新打开题目描述。',
      level: 'urgent',
      senderName: '主监考',
      createdAt: '2026-10-21T02:10:00.000Z',
    })
    render(<BroadcastBanner />)

    expect(screen.getByRole('dialog', { name: '紧急更正' })).toHaveTextContent(
      '请立即重新打开题目描述。'
    )
    await user.click(screen.getByRole('button', { name: '我已知晓' }))
    await waitFor(() => expect(useStore.getState().activeAlert).toBeNull())
  })

  it('exposes a standalone dismiss action', async () => {
    const user = userEvent.setup()
    useStore.getState().setActiveAlert({
      id: 'alert-2',
      title: '提示',
      content: '测试',
      level: 'info',
      senderName: '系统',
      createdAt: '2026-10-21T02:10:00.000Z',
    })
    render(<DismissBroadcastButton />)

    await user.click(screen.getByRole('button', { name: '关闭广播' }))
    expect(useStore.getState().activeAlert).toBeNull()
  })
})
