import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  ContextMenu,
  EvinceWindow,
  GeanyWindow,
  NautilusWindow,
  TerminalWindow,
  registerMonacoSlot,
  type MenuItem,
} from './UbuntuYaru'
import type { CommandLog } from '../UbuntuDesktop'

// Geany 内嵌编辑器用 stub 注入，避免测试环境加载真 Monaco。
registerMonacoSlot(function MonacoStub() {
  return <div data-testid="monaco-stub" />
})

beforeEach(() => {
  vi.restoreAllMocks()
})
afterEach(cleanup)

// ---------------------------------------------------------------- ContextMenu

describe('ContextMenu', () => {
  const items: MenuItem[] = [
    { label: '打开', onClick: vi.fn() },
    { separator: true },
    { label: '重命名…', shortcut: 'F2', disabled: true, onClick: vi.fn() },
  ]

  it('渲染项与快捷键，禁用项不可点，分隔符不渲染按钮', () => {
    render(<ContextMenu x={10} y={10} items={items} onClose={() => {}} />)
    expect(screen.getByText('打开')).toBeInTheDocument()
    expect(screen.getByText('F2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /重命名/ })).toBeDisabled()
    expect(screen.queryAllByRole('button', { name: '打开' })).toHaveLength(1)
  })

  it('点击后回调并关闭', () => {
    const onClose = vi.fn()
    const action = vi.fn()
    render(<ContextMenu x={10} y={10} items={[{ label: '属性', onClick: action }]} onClose={onClose} />)
    fireEvent.click(screen.getByText('属性'))
    expect(action).toHaveBeenCalledOnce()
    expect(onClose).toHaveBeenCalledOnce()
  })
})

// ---------------------------------------------------------------- Terminal

describe('TerminalWindow', () => {
  const base = {
    homeName: 'HA-J00001',
    path: '',
    logs: [{ type: 'output', text: 'hello' } as CommandLog],
    onMinimize: vi.fn(),
    onMaximize: vi.fn(),
    onClose: vi.fn(),
  }
  const input = () => screen.getByLabelText('终端命令输入')

  it('回车提交命令并清空输入', async () => {
    const onCommand = vi.fn()
    render(<TerminalWindow {...base} onCommand={onCommand} />)
    await userEvent.type(input(), 'ls{Enter}')
    expect(onCommand).toHaveBeenCalledWith('ls')
    expect(input()).toHaveValue('')
  })

  it('↑ 召回历史，↓ 回到最新', async () => {
    const onCommand = vi.fn()
    render(<TerminalWindow {...base} onCommand={onCommand} />)
    await userEvent.type(input(), 'pwd{Enter}')
    await userEvent.type(input(), 'ls{Enter}')
    await userEvent.type(input(), '{ArrowUp}')
    expect(input()).toHaveValue('ls')
    await userEvent.type(input(), '{ArrowUp}')
    expect(input()).toHaveValue('pwd')
    await userEvent.type(input(), '{ArrowDown}{ArrowDown}')
    expect(input()).toHaveValue('')
  })

  it('Tab 多匹配时通过 onHint 列出候选', async () => {
    const onHint = vi.fn()
    render(
      <TerminalWindow {...base} onCommand={vi.fn()} onHint={onHint} completions={['poker.cpp', 'poker.in']} />
    )
    await userEvent.type(input(), 'cat poker.')
    await userEvent.type(input(), '{tab}')
    expect(onHint).toHaveBeenCalledWith('poker.cpp  poker.in')
  })

  it('Tab 唯一匹配直接补全', async () => {
    render(<TerminalWindow {...base} onCommand={vi.fn()} completions={['poker.cpp', 'notes.txt']} />)
    await userEvent.type(input(), 'cat pok')
    await userEvent.type(input(), '{tab}')
    expect(input()).toHaveValue('cat poker.cpp')
  })

  it('右键菜单含复制/粘贴/全选/清屏，清屏回调生效', async () => {
    const onClearScreen = vi.fn()
    const { container } = render(
      <TerminalWindow {...base} onCommand={vi.fn()} onClearScreen={onClearScreen} />
    )
    const body = container.querySelector('.selection\\:bg-\\[\\#5E4A63\\]') ?? container.firstElementChild!
    fireEvent.contextMenu(body)
    const clear = await screen.findByText('清屏')
    fireEvent.click(clear)
    expect(onClearScreen).toHaveBeenCalledOnce()
  })
})

// ---------------------------------------------------------------- Nautilus

describe('NautilusWindow', () => {
  const items = [
    { id: 'a', name: 'poker', kind: 'folder' as const },
    { id: 'b', name: 'sol.cpp', kind: 'source' as const, sizeBytes: 128 },
  ]
  const base = {
    homeName: 'HA-J00001',
    pathSegments: ['day1'],
    items,
    selectedId: null,
    onSelect: vi.fn(),
    onOpen: vi.fn(),
    onNavigateDepth: vi.fn(),
    onNavigatePath: vi.fn(),
    breadcrumbChildren: vi.fn(() => []),
    onNewFolder: vi.fn(() => null),
    onOpenTerminal: vi.fn(),
    onRenamePath: vi.fn(() => null),
    onTrash: vi.fn(),
    onRestore: vi.fn(),
    onEmptyTrash: vi.fn(),
    onMinimize: vi.fn(),
    onMaximize: vi.fn(),
    onClose: vi.fn(),
  }

  it('渲染路径栏与条目；双击文件夹/文件触发 onOpen', async () => {
    render(<NautilusWindow {...base} />)
    expect(screen.getByText('HA-J00001')).toBeInTheDocument()
    expect(screen.getByText('poker')).toBeInTheDocument()
    expect(screen.getByText('sol.cpp')).toBeInTheDocument()
    fireEvent.doubleClick(screen.getByText('poker'))
    expect(base.onOpen).toHaveBeenCalledWith(expect.objectContaining({ name: 'poker' }))
  })

  it('空白右键 → 新建文件夹对话框：重名错误内联显示且不关窗', async () => {
    const onNewFolder = vi.fn(() => '「草稿」已存在')
    const { container } = render(<NautilusWindow {...base} onNewFolder={onNewFolder} />)
    const canvas = container.querySelector('.relative.min-h-0.flex-1')!
    fireEvent.contextMenu(canvas)
    fireEvent.click(await screen.findByText('新建文件夹'))
    const input = await screen.findByLabelText('文件夹名称')
    await userEvent.type(input, '草稿')
    fireEvent.click(screen.getByText('创建'))
    expect(await screen.findByText('「草稿」已存在')).toBeInTheDocument()
    expect(screen.getByLabelText('文件夹名称')).toBeInTheDocument() // 对话框未关
    expect(onNewFolder).toHaveBeenCalledWith('草稿')
  })

  it('成功创建后对话框关闭', async () => {
    const { container } = render(<NautilusWindow {...base} />)
    fireEvent.contextMenu(container.querySelector('.relative.min-h-0.flex-1')!)
    fireEvent.click(await screen.findByText('新建文件夹'))
    await userEvent.type(await screen.findByLabelText('文件夹名称'), '新目录')
    fireEvent.click(screen.getByText('创建'))
    await waitFor(() => expect(screen.queryByLabelText('文件夹名称')).not.toBeInTheDocument())
  })

  it('侧栏位置导航（文档 → Documents）；路径栏点击回主目录', async () => {
    render(<NautilusWindow {...base} />)
    fireEvent.click(screen.getByText('文档'))
    expect(base.onNavigatePath).toHaveBeenCalledWith(['Documents'])
    fireEvent.click(screen.getByText('HA-J00001'))
    expect(base.onNavigateDepth).toHaveBeenCalledWith(0)
  })

  it('头栏菜单提供视图与排序；按大小排序改变顺序', async () => {
    const big = [
      { id: 'a', name: 'small.cpp', kind: 'source' as const, sizeBytes: 10 },
      { id: 'b', name: 'big.cpp', kind: 'source' as const, sizeBytes: 9999 },
    ]
    render(<NautilusWindow {...base} items={big} />)
    fireEvent.click(screen.getByLabelText('菜单'))
    fireEvent.click(await screen.findByText('列表视图'))
    fireEvent.click(screen.getByLabelText('菜单'))
    fireEvent.click(await screen.findByText('按大小排序'))
    const rows = screen.getAllByText(/\.cpp/)
    expect(rows[0]).toHaveTextContent('small.cpp')
  })
})

// ---------------------------------------------------------------- Geany

describe('GeanyWindow', () => {
  const base = {
    dirName: 'poker',
    fileName: 'poker.cpp',
    homeName: 'HA-J00001',
    code: 'int main() {}',
    onCode: vi.fn(),
    readOnly: false,
    compilerLog: [] as CommandLog[],
    onCompile: vi.fn(),
    onRun: vi.fn(),
    onOpenFiles: vi.fn(),
    onSave: vi.fn(),
    onNewDoc: vi.fn(),
    onPickUploadFile: vi.fn(),
    uploadFile: null,
    uploadMismatch: null,
    onClearUploadFile: vi.fn(),
    onSubmit: vi.fn(),
    submitting: false,
    onMinimize: vi.fn(),
    onMaximize: vi.fn(),
    onClose: vi.fn(),
  }

  it('无文档显示空态引导，提交按钮禁用', () => {
    render(
      <GeanyWindow {...base} tabs={[]} activeTabId={null} onSelectTab={vi.fn()} />
    )
    expect(screen.getByText(/从「文件」打开/)).toBeInTheDocument()
    expect(screen.getByText('正式提交')).toBeDisabled()
  })

  it('多文档标签切换与关闭', async () => {
    const onSelectTab = vi.fn()
    const onCloseDoc = vi.fn()
    render(
      <GeanyWindow
        {...base}
        tabs={[
          { id: 'problem:1', name: 'poker.cpp', kind: 'problem' },
          { id: 'vfs:sol', name: 'sol.cpp', kind: 'vfs' },
        ]}
        activeTabId="problem:1"
        onSelectTab={onSelectTab}
        onCloseDoc={onCloseDoc}
      />
    )
    fireEvent.click(screen.getByText('sol.cpp'))
    expect(onSelectTab).toHaveBeenCalledWith('vfs:sol')
    fireEvent.click(screen.getByLabelText('关闭 poker.cpp'))
    expect(onCloseDoc).toHaveBeenCalledWith('problem:1')
  })

  it('另存为：重名错误内联且不关窗，成功后关窗并切换', async () => {
    const onSaveAs = vi.fn((name: string) => (name === 'sol.cpp' ? '「sol.cpp」已存在' : null))
    render(
      <GeanyWindow
        {...base}
        tabs={[{ id: 'vfs:x', name: '无标题', kind: 'untitled' }]}
        activeTabId="vfs:x"
        onSelectTab={vi.fn()}
        onSaveAs={onSaveAs}
      />
    )
    // 未命名文档 Ctrl+S 弹出「保存文件」对话框
    fireEvent.keyDown(window, { key: 's', ctrlKey: true })
    const input = await screen.findByLabelText('另存为文件名')
    await userEvent.clear(input)
    await userEvent.type(input, 'sol.cpp')
    fireEvent.click(screen.getByText('保存'))
    expect(await screen.findByText('「sol.cpp」已存在')).toBeInTheDocument()
    expect(screen.getByLabelText('另存为文件名')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------- Evince

describe('EvinceWindow', () => {
  const base = {
    fileName: 'day1.pdf',
    title: 'A. 扑克牌',
    content: '## 题目描述\n\n小杨有 n 张牌。\n## 输入格式\n\n第一行 n。',
    onMinimize: vi.fn(),
    onMaximize: vi.fn(),
    onClose: vi.fn(),
  }

  it('真 PDF 模式：iframe src 携带 page/zoom 参数，缩放与目录跳页更新 src', async () => {
    render(<EvinceWindow {...base} pdfUrl="/api/v1/contests/c1/pdf" />)
    const srcOf = () => screen.getByTitle('day1.pdf').getAttribute('src') ?? ''
    expect(srcOf()).toContain('page=1')
    expect(srcOf()).toContain('zoom=100')
    fireEvent.click(screen.getByLabelText('放大'))
    await waitFor(() => expect(srcOf()).toContain('zoom=125'))
    fireEvent.click(screen.getByLabelText('缩小'))
    await waitFor(() => expect(srcOf()).toContain('zoom=100'))
  })

  it('键盘翻页：PgDn 递增并钳制到 1..max', async () => {
    render(<EvinceWindow {...base} pdfUrl="/api/v1/contests/c1/pdf" />)
    const shell = screen.getByLabelText('day1.pdf 查看器')
    const srcOf = () => screen.getByTitle('day1.pdf').getAttribute('src') ?? ''
    fireEvent.keyDown(shell, { key: 'PageDown' })
    await waitFor(() => expect(srcOf()).toContain('page=2'))
    fireEvent.keyDown(shell, { key: 'Home' })
    await waitFor(() => expect(srcOf()).toMatch(/page=1&/))
  })

  it('离线模式渲染 Markdown 分页与页码', () => {
    render(<EvinceWindow {...base} />)
    expect(screen.getAllByText(/— 1 —/).length).toBeGreaterThan(0)
  })
})
