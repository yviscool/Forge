import { useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { Dialog } from '../components/ui/dialog'
import { Button } from '../components/ui/button'

function DialogHarness() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button onClick={() => setOpen(true)}>打开对话框</Button>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title="确认操作"
        footer={
          <>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button onClick={() => setOpen(false)}>确认</Button>
          </>
        }
      >
        <p>这里是风险说明。</p>
      </Dialog>
    </>
  )
}

describe('Dialog accessibility', () => {
  it('labels the dialog, traps focus, closes on Escape, and restores focus', async () => {
    document.body.innerHTML = '<div id="root"></div>'
    const user = userEvent.setup()
    render(<DialogHarness />, { container: document.getElementById('root')! })

    const trigger = screen.getByRole('button', { name: '打开对话框' })
    await user.click(trigger)

    const dialog = screen.getByRole('dialog', { name: '确认操作' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(document.getElementById('root')).toHaveAttribute('inert')
    await waitFor(() => expect(screen.getByRole('button', { name: '关闭' })).toHaveFocus())

    await user.tab({ shift: true })
    expect(screen.getByRole('button', { name: '确认' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '关闭' })).toHaveFocus()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(document.getElementById('root')).not.toHaveAttribute('inert')
    expect(trigger).toHaveFocus()
  })
})
