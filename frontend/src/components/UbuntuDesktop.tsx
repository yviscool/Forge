import React, { useState, useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Contest, Problem, api } from '../api/client'
import { useStore } from '../store/useStore'
import {
  ContextMenu,
  EvinceWindow,
  GeanyWindow,
  NautilusWindow,
  NoiWallpaper,
  TerminalWindow,
  YARU_SANS,
  YaruHeaderBar,
  registerMonacoSlot,
  type GeanyEditorHandle,
  type MenuItem,
} from './ubuntu/UbuntuYaru'
import { YARU_ICONS } from './ubuntu/yaruIcons'
import {
  dirKey,
  findItem,
  mkfile,
  listDir,
  mkdir as vfsMkdir,
  renameItem as vfsRename,
  restoreItem as vfsRestore,
  trashItem as vfsTrash,
  writeFile as vfsWrite,
  type VfsItem,
} from './ubuntu/vfs'
import { ContestTimer } from './ContestTimer'
import { Button } from './ui/button'
import { useToast } from './ui/toast'
import { byteLength } from '../features/student/submissionFiles'
import {
  LayoutGrid,
  Terminal as TerminalIcon,
  FileText,
  ShieldCheck,
  ShieldAlert,
  Lock,
  Sparkles,
  HardDrive,
  Wifi,
  Volume2,
  Power,
} from 'lucide-react'

const MonacoEditor = React.lazy(() =>
  import('./MonacoEditor').then((module) => ({ default: module.MonacoEditor }))
)

// Geany 窗口内的编辑器实例：UbuntuYaru 通过该槽位触发撤销/剪切/粘贴等真实命令。
registerMonacoSlot(function GeanyMonacoSlot({ code, onCode, readOnly, onReady, onCursor }) {
  return (
    <React.Suspense fallback={<div className="h-full bg-white" />}>
      <MonacoEditor
        value={code}
        onChange={onCode}
        language="cpp"
        readOnly={readOnly}
        height="100%"
        ariaLabel="Geany 源代码编辑器"
        fontFamily="'Ubuntu Mono', 'Cascadia Mono', Consolas, monospace"
        fontSize={14}
        chromeless
        onEditorReady={(editor) =>
          onReady({
            trigger: (srcCmd, cmd) => editor.trigger(srcCmd, cmd, null),
            getPosition: () => editor.getPosition() ?? { lineNumber: 1, column: 1 },
            reveal: (line) => {
              editor.revealLineInCenter(line)
              editor.setPosition({ lineNumber: line, column: 1 })
              editor.focus()
            },
          } as GeanyEditorHandle)
        }
        onCursorChange={onCursor}
      />
    </React.Suspense>
  )
})

interface UbuntuDesktopProps {
  contest: Contest
  problems: Problem[]
  activeProblem: Problem | null
  setActiveProblem: (p: Problem) => void
  code: string
  setCode: (c: string) => void
  onExitDesktop: () => void
  onSubmit: () => void
  submitting: boolean
  /** Ctrl+S 显式保存：把当前草稿立即写入本机（Geany 快捷键链路）。 */
  onSave?: () => void
  /** 已选择待上传的源文件（真实考场方式：选择文件 → 上传）。 */
  uploadFile: { name: string; size: number } | null
  /** 文件名与题面英文名不一致时的提示（本题期望的目录名）。 */
  uploadMismatch: string | null
  onPickUploadFile: () => void
  onClearUploadFile: () => void
}

type WindowKey = 'files' | 'editor' | 'terminal' | 'pdf' | 'doctor' | 'unlock'

interface WindowState {
  isOpen: boolean
  isMinimized: boolean
  isMaximized: boolean
  x: number
  y: number
  w: number
  h: number
  zIndex: number
}

export interface CommandLog {
  type: 'input' | 'output' | 'error' | 'success' | 'warn'
  text: string
  /** 编译诊断对应的源码行号：Geany 编译器窗格点击跳行用。 */
  line?: number
}

export const UbuntuDesktop: React.FC<UbuntuDesktopProps> = ({
  contest,
  problems,
  activeProblem,
  setActiveProblem,
  code,
  setCode,
  onExitDesktop,
  onSubmit,
  submitting,
  onSave,
  uploadFile,
  uploadMismatch,
  onPickUploadFile,
  onClearUploadFile,
}) => {
  const { session } = useStore()
  const queryClient = useQueryClient()
  const toast = useToast()
  // Only the signed grant reflected by the server is authoritative. A stale
  // localStorage flag must not hide the unlock prompt after a server restart.
  const isUnlocked = !contest.zipProtected || Boolean(contest.zipUnlocked)
  // 真实考点的主目录名是准考证号（HA-J00001）；未发号时退回登录名。
  const homeName = session?.examTicket?.trim() || session?.username || 'student'
  // CSP 复赛要求按「准考证号/题面英文名/英文名.cpp」组织源码：
  // 模拟桌面的目录与文件名跟收题包同一口径，slug 缺省时退回题号小写。
  const taskDirOf = (problem: Problem | null | undefined): string => {
    if (!problem) return 'a'
    const slug = problem.slug?.trim()
    return slug || problem.code.toLowerCase()
  }

  const { serverTimeOffset } = useStore()

  // Dynamic system time (server-corrected so an exam cannot be extended by
  // changing the candidate machine's clock).
  const [currentTimeStr, setCurrentTimeStr] = useState('')
  useEffect(() => {
    const updateTime = () => {
      const d = new Date(Date.now() + serverTimeOffset)
      const m = String(d.getMonth() + 1).padStart(2, '0')
      const day = String(d.getDate()).padStart(2, '0')
      const h = String(d.getHours()).padStart(2, '0')
      const min = String(d.getMinutes()).padStart(2, '0')
      const weekday = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
      setCurrentTimeStr(`${weekday} ${m}月${day}日 ${h}:${min}`)
    }
    updateTime()
    const t = setInterval(updateTime, 1000)
    return () => clearInterval(t)
  }, [serverTimeOffset])

  type DirEntry = {
    name: string
    kind: 'dir' | 'file'
    content?: string
    readonly?: boolean
    problemId?: string
    sizeBytes?: number
  }

  // Window manager state
  const [zCounter, setZCounter] = useState(20)
  const bumpZ = () => {
    const next = zCounter + 1
    setZCounter(next)
    return next
  }

  const [windows, setWindows] = useState<Record<WindowKey, WindowState>>({
    files: { isOpen: false, isMinimized: false, isMaximized: false, x: 80, y: 46, w: 880, h: 560, zIndex: 10 },
    editor: { isOpen: false, isMinimized: false, isMaximized: false, x: 210, y: 84, w: 920, h: 620, zIndex: 12 },
    terminal: { isOpen: false, isMinimized: false, isMaximized: false, x: 170, y: 220, w: 660, h: 420, zIndex: 14 },
    pdf: { isOpen: false, isMinimized: false, isMaximized: false, x: 120, y: 60, w: 680, h: 580, zIndex: 11 },
    doctor: { isOpen: false, isMinimized: false, isMaximized: false, x: 320, y: 120, w: 560, h: 480, zIndex: 15 },
    unlock: { isOpen: !isUnlocked, isMinimized: false, isMaximized: false, x: 380, y: 160, w: 440, h: 300, zIndex: 30 },
  })

  // Bring window to focus
  const focusWindow = (key: WindowKey) => {
    const newZ = bumpZ()
    setWindows((prev) => ({
      ...prev,
      [key]: { ...prev[key], isOpen: true, isMinimized: false, zIndex: newZ },
    }))
  }

  // Toggle window from Dock
  const toggleWindow = (key: WindowKey) => {
    const w = windows[key]
    if (!w.isOpen) {
      focusWindow(key)
    } else if (w.isMinimized) {
      focusWindow(key)
    } else {
      // If it's already top, minimize it; otherwise bring it to top
      const isTop = Object.values(windows).every((other) => other.zIndex <= w.zIndex || other.isMinimized || !other.isOpen)
      if (isTop) {
        setWindows((prev) => ({ ...prev, [key]: { ...w, isMinimized: true } }))
      } else {
        focusWindow(key)
      }
    }
  }

  // Dragging support
  const [draggingKey, setDraggingKey] = useState<WindowKey | null>(null)
  const dragOffsetRef = useRef<{ ox: number; oy: number }>({ ox: 0, oy: 0 })

  const handleTitleMouseDown = (key: WindowKey, e: React.MouseEvent) => {
    if (windows[key].isMaximized) return
    focusWindow(key)
    setDraggingKey(key)
    dragOffsetRef.current = {
      ox: e.clientX - windows[key].x,
      oy: e.clientY - windows[key].y,
    }
  }

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!draggingKey) return
      const nx = Math.max(0, e.clientX - dragOffsetRef.current.ox)
      const ny = Math.max(30, e.clientY - dragOffsetRef.current.oy)
      setWindows((prev) => ({
        ...prev,
        [draggingKey]: { ...prev[draggingKey], x: nx, y: ny },
      }))
    }
    const handleMouseUp = () => setDraggingKey(null)

    if (draggingKey) {
      window.addEventListener('mousemove', handleMouseMove)
      window.addEventListener('mouseup', handleMouseUp)
    }
    return () => {
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }
  }, [draggingKey])

  // Zip Password Unlock State
  const [zipInputPassword, setZipInputPassword] = useState('')
  const [unlockError, setUnlockError] = useState<string | null>(null)
  const [unlocking, setUnlocking] = useState(false)

  const handleUnlock = async (e: React.FormEvent) => {
    e.preventDefault()
    setUnlocking(true)
    setUnlockError(null)
    try {
      await api.unlockZip(contest.id, zipInputPassword)
      const fresh = await api.getContest(contest.id)
      if (!fresh.zipUnlocked) {
        throw new Error('服务端未确认解锁，请重新登录后再试。')
      }
      queryClient.setQueryData(['contest', contest.id], fresh)
      setWindows((prev) => ({ ...prev, unlock: { ...prev.unlock, isOpen: false } }))
      toast.success('试卷包已解锁', '现在可以查看考场文件与题面资料。')
      focusWindow('files')
    } catch (err: unknown) {
      setUnlockError(err instanceof Error ? err.message : '密码错误，解锁失败')
    } finally {
      setUnlocking(false)
    }
  }

  // Nautilus 选中项与 Geany 编译器输出。
  const [selectedNautilus, setSelectedNautilus] = useState<string | null>(null)
  // 虚拟文件系统：考生自建的文件夹/文件按目录路径存放；回收站记录来源目录。
  // 与真实考点一致：文件「在磁盘上」，刷新/重启不丢，按考生账号隔离。
  const vfsKey = session ? `forge_desktop_vfs_${session.userId}` : null
  const loadVfs = (): Record<string, VfsItem[]> => {
    if (!vfsKey) return {}
    try {
      return JSON.parse(localStorage.getItem(vfsKey) ?? '{}') as Record<string, VfsItem[]>
    } catch {
      return {}
    }
  }
  const [userFiles, setUserFiles] = useState<Record<string, VfsItem[]>>(loadVfs)
  const [trash, setTrash] = useState<{ item: VfsItem; fromKey: string }[]>(() => {
    if (!vfsKey) return []
    try {
      return JSON.parse(localStorage.getItem(`${vfsKey}_trash`) ?? '[]')
    } catch {
      return []
    }
  })
  const [trashView, setTrashView] = useState(false)
  // Geany 文档模型：标签 = 打开的文档。untitled = 真实 Geany 的「无标题 N」：
  // 不在磁盘上，首次保存弹出命名对话框；关机未保存即丢失（这就是要练的肌肉记忆）。
  type GeanyDoc = {
    id: string
    kind: 'problem' | 'vfs' | 'untitled'
    name: string
    problemId?: string
    dirKey?: string
    readonly?: boolean
    content?: string
  }
  const docsKey = session ? `forge_geany_docs_${session.userId}` : null
  const loadDocs = (): GeanyDoc[] => {
    if (!docsKey) return []
    try {
      const docs = JSON.parse(localStorage.getItem(docsKey) ?? '[]') as GeanyDoc[]
      // 未保存的无标题文档不跨会话存活（真实行为）；
      // VFS 文档若已被删除也不再恢复。
      return docs.filter(
        (d) =>
          d.kind === 'problem' ||
          (d.kind === 'vfs' && findItem(userFiles[d.dirKey ?? ''], d.name) != null)
      )
    } catch {
      return []
    }
  }
  const [openDocs, setOpenDocs] = useState<GeanyDoc[]>(loadDocs)
  const untitledCounter = useRef(0)
  const [activeDocId, setActiveDocId] = useState<string | null>(() => {
    if (!docsKey) return null
    try {
      return JSON.parse(localStorage.getItem(`${docsKey}_active`) ?? 'null') as string | null
    } catch {
      return null
    }
  })
  const [untitledContents, setUntitledContents] = useState<Record<string, string>>({})

  // 持久化：任何变更立即落 localStorage（考试机刷新 = 重新登录，不丢文件）。
  useEffect(() => {
    if (!vfsKey) return
    localStorage.setItem(vfsKey, JSON.stringify(userFiles))
  }, [userFiles, vfsKey])
  useEffect(() => {
    if (!vfsKey) return
    localStorage.setItem(`${vfsKey}_trash`, JSON.stringify(trash))
  }, [trash, vfsKey])
  useEffect(() => {
    if (!docsKey) return
    localStorage.setItem(
      docsKey,
      JSON.stringify(openDocs.filter((d) => d.kind !== 'untitled'))
    )
    localStorage.setItem(`${docsKey}_active`, JSON.stringify(activeDocId))
  }, [openDocs, activeDocId, docsKey])
  // Alt+Tab 窗口切换器（GNOME 行为：按住 Alt 连按 Tab 轮选，松开 Alt 落点）。
  const [switcher, setSwitcher] = useState<{ index: number } | null>(null)
  // 桌面空白处右键菜单。
  const [desktopMenu, setDesktopMenu] = useState<{ x: number; y: number } | null>(null)
  const [geanyCompilerLog, setGeanyCompilerLog] = useState<CommandLog[]>([])

  // 静态预检（浏览器内无真实编译器）：终端 g++ 与 Geany 编译按钮共用同一份输出。
  const buildStaticCheckLog = (
    source: string,
    target: string,
    problem: Problem | null
  ): CommandLog[] => {
    if (!source || source.trim().length === 0) {
      return [
        {
          type: 'error',
          text: `${target}: fatal error: 0 bytes input file\ncompilation terminated.`,
        },
      ]
    }
    const hasMain = /\bmain\s*\(/.test(source)
    const openBraces = (source.match(/{/g) || []).length
    const closeBraces = (source.match(/}/g) || []).length
    if (!hasMain) {
      return [
        {
          type: 'error',
          text: `/usr/lib/gcc/x86_64-linux-gnu/9/../../../x86_64-linux-gnu/Scrt1.o: In function '_start':\n(.text+0x20): undefined reference to 'main'\ncollect2: error: ld returned 1 exit status`,
        },
      ]
    }
    if (openBraces !== closeBraces) {
      return [
        {
          type: 'error',
          text: `${target}: error: expected '}' at end of input (unmatched curly braces: ${openBraces} open, ${closeBraces} close)\ncompilation terminated.`,
        },
      ]
    }

    // 软警告（-Wall 口径）：浏览器做不了真编译，但这几类是考场高频失分点，
    // 用正则扫得出来就值得当场念给考生听。
    const srcLines = source.split('\n')
    const warnings: CommandLog[] = []
    if (/\bint\s+main\s*\(/.test(source) && !/\breturn\s+0\s*;/.test(source)) {
      const lastLine = srcLines.length
      warnings.push({
        type: 'warn',
        line: lastLine,
        text: `${target}:${lastLine}: warning: control reaches end of non-void function [-Wreturn-type]`,
      })
    }
    for (const match of source.matchAll(/\bint\s+([a-zA-Z_]\w*)\s*(?:=\s*[^;]+)?;|\bdouble\s+([a-zA-Z_]\w*)\s*(?:=\s*[^;]+)?;/g)) {
      const name = match[1] ?? match[2]
      if (!name) continue
      const occurrences = source.split(name).length - 1
      if (occurrences <= 1) {
        const line = srcLines.findIndex((l) => l.includes(name)) + 1
        warnings.push({
          type: 'warn',
          line,
          text: `${target}:${line}: warning: unused variable '${name}' [-Wunused-variable]`,
        })
      }
    }
    if (/\bgets\s*\(/.test(source)) {
      const line = srcLines.findIndex((l) => /\bgets\s*\(/.test(l)) + 1
      warnings.push({
        type: 'warn',
        line,
        text: `${target}:${line}: warning: the \`gets' function is dangerous and should not be used. [-Wdeprecated-declarations]`,
      })
    }
    // freopen 拼错文件名：本机能跑、评测读不到输入直接 0 分——最值得练的坑。
    if (problem?.ioMode === 'file') {
      const inFile = problem.inFile || `${taskDirOf(problem)}.in`
      const outFile = problem.outFile || `${taskDirOf(problem)}.out`
      const opened = Array.from(source.matchAll(/freopen\s*\(\s*"([^"]+)"/g)).map((m) => m[1])
      if (opened.length > 0 && !(opened.includes(inFile) && opened.includes(outFile))) {
        const line = srcLines.findIndex((l) => l.includes('freopen')) + 1
        warnings.push({
          type: 'warn',
          line,
          text: `${target}:${line}: warning: freopen 的文件名与题目要求不符（应为 ${inFile} / ${outFile}）——本机能跑，评测时读不到输入将直接 0 分！`,
        })
      }
    }

    const out: CommandLog[] = [{ type: 'output', text: '静态检查：检测到 main()，花括号数量平衡。' }]
    out.push(...warnings)
    if (warnings.length > 0) {
      out.push({
        type: 'output',
        text: `${warnings.length} 条警告，0 个错误。带下划线的行可点击跳转；这不代表源码可以编译通过。`,
      })
    }
    out.push({ type: 'output', text: '注意：浏览器预览没有执行真实编译器，最终以服务端评测为准。' })
    return out
  }

  // 题面样例：cat poker.in / poker.out 直接给官方样例（学生考前看样例是真实习惯）。
  const sampleIO = (problem: Problem | null, kind: 'in' | 'out'): string => {
    if (!problem?.examples) return ''
    const text = problem.examples.replace(/\r/g, '')
    const label = kind === 'in' ? '输入样例' : '输出样例'
    const seg = text.split(new RegExp(`${label}[^：:\n]*[：:]?`))[1] ?? ''
    if (kind === 'in') {
      const end = seg.indexOf('输出样例')
      return (end >= 0 ? seg.slice(0, end) : seg).trim()
    }
    return seg.trim()
  }



  // File Manager navigation state：文件管理器的路径独立于编辑器，初始落在主目录。
  const currentTaskDir = taskDirOf(activeProblem)
  const [currentPath, setCurrentPath] = useState<string[]>(['home', homeName])

  // 当前目录的可见条目：题目派生（源文件/样例）+ 考生自建（VFS）。
  const homeKey = dirKey(currentPath.slice(2))
  const dirEntries: DirEntry[] = (() => {
    if (currentPath.length === 2) {
      return [
        ...problems.map((p) => ({ name: taskDirOf(p), kind: 'dir' as const })),
        ...listDir(userFiles['']).map(
          (item): DirEntry => ({
            name: item.name,
            kind: item.kind,
            content: item.content,
            readonly: item.readonly,
            sizeBytes: item.kind === 'file' ? byteLength(item.content ?? '') : undefined,
          })
        ),
      ]
    }
    const base: DirEntry[] = []
    const dirName = currentPath[2]
    const problem = currentPath.length === 3 ? problems.find((p) => taskDirOf(p) === dirName) : undefined
    if (problem) {
      base.push({
        name: `${taskDirOf(problem)}.cpp`,
        kind: 'file',
        content: problem.id === activeProblem?.id ? code : undefined,
        problemId: problem.id,
      })
      if (problem.ioMode === 'file') {
        base.push({
          name: problem.inFile || `${taskDirOf(problem)}.in`,
          kind: 'file',
          readonly: true,
          content: sampleIO(problem, 'in'),
        })
        base.push({
          name: problem.outFile || `${taskDirOf(problem)}.out`,
          kind: 'file',
          readonly: true,
          content: sampleIO(problem, 'out'),
        })
      }
    }
    return [
      ...base,
      ...listDir(userFiles[homeKey]).map(
        (item): DirEntry => ({
          name: item.name,
          kind: item.kind,
          content: item.content,
          readonly: item.readonly,
          sizeBytes: item.kind === 'file' ? byteLength(item.content ?? '') : undefined,
        })
      ),
    ]
  })()

  const protectedNames = dirEntries
    .filter((entry) => entry.problemId != null || entry.readonly)
    .map((entry) => entry.name)
    .concat(currentPath.length === 2 ? [] : [])
    .filter((name, index, all) => all.indexOf(name) === index)

  const terminalCompletions = dirEntries.map((entry) => entry.name)

  // ---- Geany 文档模型 ----
  const activeDoc = openDocs.find((d) => d.id === activeDocId) ?? null
  const activeVfsItem =
    activeDoc?.kind === 'vfs'
      ? findItem(userFiles[activeDoc.dirKey ?? ''], activeDoc.name)
      : null
  const docCode =
    activeDoc?.kind === 'problem'
      ? code
      : activeDoc?.kind === 'untitled'
        ? (untitledContents[activeDoc.id] ?? '')
        : (activeVfsItem?.content ?? '')
  const docReadOnly = activeDoc?.kind === 'vfs' ? !!activeVfsItem?.readonly : false
  const docPath = activeDoc ? (activeDoc.dirKey ?? '') : currentPath.slice(2).join('/')
  const activeDocProblem =
    activeDoc?.kind === 'problem'
      ? problems.find((p) => p.id === activeDoc.problemId) ?? null
      : null

  const setDocCode = (value: string) => {
    if (!activeDoc) return
    if (activeDoc.kind === 'problem') {
      setCode(value)
      return
    }
    if (activeDoc.kind === 'untitled') {
      setUntitledContents((prev) => ({ ...prev, [activeDoc.id]: value }))
      return
    }
    const key = activeDoc.dirKey ?? ''
    setUserFiles((prev) => {
      const result = vfsWrite(prev[key] ?? [], activeDoc.name, value)
      return result.ok ? { ...prev, [key]: result.items } : prev
    })
  }

  const openProblemDoc = (problem: Problem) => {
    const id = `problem:${problem.id}`
    setOpenDocs((prev) =>
      prev.some((d) => d.id === id)
        ? prev
        : [
            ...prev,
            {
              id,
              kind: 'problem' as const,
              name: `${taskDirOf(problem)}.cpp`,
              problemId: problem.id,
              dirKey: taskDirOf(problem),
            },
          ]
    )
    setActiveDocId(id)
    setActiveProblem(problem)
    focusWindow('editor')
  }

  const openVfsDoc = (key: string, item: VfsItem) => {
    const id = `vfs:${key}/${item.name}`
    setOpenDocs((prev) =>
      prev.some((d) => d.id === id)
        ? prev
        : [
            ...prev,
            {
              id,
              kind: 'vfs' as const,
              name: item.name,
              dirKey: key,
              readonly: item.readonly,
            },
          ]
    )
    setActiveDocId(id)
    focusWindow('editor')
  }

  const closeDoc = (id: string) => {
    setOpenDocs((prev) => {
      const index = prev.findIndex((d) => d.id === id)
      if (index < 0) return prev
      const next = prev.filter((d) => d.id !== id)
      if (activeDocId === id) {
        setActiveDocId(next.length ? next[Math.max(0, index - 1)].id : null)
      }
      return next
    })
  }

  // Interactive Terminal state
  const [terminalLogs, setTerminalLogs] = useState<CommandLog[]>([
    { type: 'output', text: 'Welcome to Ubuntu 20.04.1 LTS (GNU/Linux 5.4.0-42-generic x86_64)' },
    { type: 'output', text: '  * NOI Linux 2.0  --  China National Olympiad in Informatics' },
    { type: 'output', text: '' },
    { type: 'output', text: `Last login: exam session for ${homeName} on pts/0` },
    { type: 'output', text: 'Type "help" to see available exam room commands (g++, ls, cat, cd, etc.).' },
  ])

  // Alt+Tab：真实考点里窗口切换全靠它；Windows 浏览器可能被系统截获，Linux 下可用。
  useEffect(() => {
    const openKeys = (Object.keys(windows) as WindowKey[]).filter(
      (key) => windows[key].isOpen && !windows[key].isMinimized
    )
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Tab' && event.altKey) {
        event.preventDefault()
        setSwitcher((current) => {
          if (!openKeys.length) return null
          if (!current) return { index: openKeys.length > 1 ? 1 : 0 }
          return { index: (current.index + 1) % openKeys.length }
        })
      } else if (event.key === 'Escape') {
        setSwitcher(null)
      }
    }
    const onKeyUp = (event: KeyboardEvent) => {
      if (event.key === 'Alt') {
        setSwitcher((current) => {
          if (current && openKeys.length) {
            focusWindow(openKeys[current.index % openKeys.length])
          }
          return null
        })
      }
    }
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [windows])

  // Terminal Command Execution Logic
  const handleTerminalCommand = (cmd: string) => {
    const trimmed = cmd.trim()
    if (!trimmed) return

    setTerminalLogs((prev) => [...prev, { type: 'input', text: trimmed }])

    const parts = trimmed.split(/\s+/)
    const mainCmd = parts[0]

    switch (mainCmd) {
      case 'clear':
        setTerminalLogs([])
        break

      case 'help':
        setTerminalLogs((prev) => [
          ...prev,
          { type: 'output', text: 'Available commands in NOI Linux emulator:' },
          { type: 'output', text: '  ls [-l]                     List files in current exam directory' },
          { type: 'output', text: '  pwd                         Print current working directory path' },
          { type: 'output', text: '  cd <dir>                    Change directory (cd .., cd <task>)' },
          { type: 'output', text: '  cat <file>                  Display contents of source file' },
          { type: 'output', text: '  mkdir <name>                Create a new subdirectory' },
          { type: 'output', text: '  g++ -O2 <file> -o <bin>     Static source preflight only (no compiler in browser)' },
          { type: 'output', text: '  ./<bin>                     Execute binary with problem sample input' },
          { type: 'output', text: '  doctor                      Launch CCF Exam Inspector and run health-check' },
        ])
        break

      case 'pwd':
        setTerminalLogs((prev) => [
          ...prev,
          { type: 'output', text: `/home/${homeName}/${currentPath.slice(2).join('/') || ''}` },
        ])
        break

      case 'ls': {
        const isDetailed = parts.includes('-l')
        if (isDetailed) {
          const rows = dirEntries.map((entry) => {
            const size = entry.kind === 'dir' ? 4096 : byteLength(entry.content ?? '')
            const mode = entry.kind === 'dir' ? 'drwxr-xr-x' : '-rw-r--r--'
            return `${mode} 1 noilinux noilinux ${size.toString().padStart(5, ' ')} Oct 08 18:50 ${entry.name}`
          })
          setTerminalLogs((prev) => [
            ...prev,
            { type: 'output', text: `total ${dirEntries.length}` },
            ...rows.map((text) => ({ type: 'output' as const, text })),
          ])
        } else {
          setTerminalLogs((prev) => [
            ...prev,
            { type: 'output', text: dirEntries.map((entry) => entry.name).join('  ') || '(空目录)' },
          ])
        }
        break
      }

      case 'mkdir': {
        const name = parts[1]
        if (!name) {
          setTerminalLogs((prev) => [...prev, { type: 'error', text: 'mkdir: missing operand' }])
          break
        }
        const result = vfsMkdir(userFiles[homeKey] ?? [], name)
        if (result.ok) {
          setUserFiles((prev) => ({ ...prev, [homeKey]: result.items }))
        } else {
          setTerminalLogs((prev) => [...prev, { type: 'error', text: `mkdir: ${result.error}` }])
        }
        break
      }

      case 'cd': {
        const target = parts[1]
        if (!target || target === '~' || target === `/home/${homeName}`) {
          setCurrentPath(['home', homeName])
          setTerminalLogs((prev) => [...prev, { type: 'output', text: '' }])
        } else if (target === '..') {
          if (currentPath.length > 2) {
            setCurrentPath((prev) => prev.slice(0, prev.length - 1))
          }
        } else {
          const dirEntry = dirEntries.find(
            (entry) => entry.kind === 'dir' && entry.name.toLowerCase() === target.toLowerCase()
          )
          if (dirEntry) {
            setCurrentPath((prev) => [...prev, dirEntry.name])
            const problem = problems.find((p) => taskDirOf(p) === dirEntry.name)
            if (problem) setActiveProblem(problem)
          } else {
            setTerminalLogs((prev) => [...prev, { type: 'error', text: `cd: no such file or directory: ${target}` }])
          }
        }
        break
      }

      case 'cat': {
        const fileToCat = parts[1]
        if (!fileToCat) {
          setTerminalLogs((prev) => [...prev, { type: 'error', text: 'cat: missing filename' }])
        } else if (dirEntries.some((entry) => entry.name === fileToCat && entry.kind === 'dir')) {
          setTerminalLogs((prev) => [...prev, { type: 'error', text: `cat: ${fileToCat}: Is a directory` }])
        } else {
          const entry = dirEntries.find((e) => e.name === fileToCat)
          if (!entry) {
            setTerminalLogs((prev) => [...prev, { type: 'error', text: `cat: ${fileToCat}: No such file` }])
          } else {
            const content =
              entry.content ??
              (entry.problemId && entry.problemId !== activeProblem?.id
                ? '(先在 Geany 中打开该题文档后，内容才会同步到终端)'
                : '// Empty file (0 bytes)')
            setTerminalLogs((prev) => [...prev, { type: 'output', text: content }])
          }
        }
        break
      }

      case 'doctor':
        focusWindow('doctor')
        setTerminalLogs((prev) => [
          ...prev,
          { type: 'success', text: 'Opening CCF NOI Exam Inspector...' },
        ])
        break

      default:
        if (trimmed.startsWith('g++')) {
          // This desktop is a browser-side exam simulator. It can run a few
          // static source checks, but it must never pretend to be a compiler.
          const sourceMatch = trimmed.match(/(\w+\.cpp)/)
          if (!sourceMatch) {
            setTerminalLogs((prev) => [
              ...prev,
              { type: 'error', text: 'g++: fatal error: no input files\ncompilation terminated.' },
            ])
            return
          }
          const targetSource = sourceMatch[1]
          if (targetSource !== `${currentTaskDir}.cpp`) {
            setTerminalLogs((prev) => [
              ...prev,
              { type: 'error', text: `g++: error: ${targetSource}: No such file or directory` },
            ])
            return
          }
          setTerminalLogs((prev) => [...prev, ...buildStaticCheckLog(code, targetSource, activeProblem)])
        } else if (trimmed.startsWith('./')) {
          setTerminalLogs((prev) => [
            ...prev,
            {
              type: 'output',
              text:
                '浏览器预览不会执行本地二进制，也不会生成伪造的样例输出。\n' +
                '正式提交后，服务端会记录代码；若部署了隔离评测 worker，将返回真实逐点结果。',
            },
          ])
        } else {
          setTerminalLogs((prev) => [
            ...prev,
            { type: 'error', text: `bash: ${mainCmd}: command not found. Type "help" for list.` },
          ])
        }
        break
    }
  }

  // Static preflight checks for the browser preview. These deliberately avoid
  // claiming compiler or filesystem guarantees that the browser cannot prove.
  const runFileCheck = () => {
    if (!activeProblem) return []
    const results = []
    const expectedDir = taskDirOf(activeProblem)
    const expectedFile = `${expectedDir}.cpp`

    // 1. Directory Structure Check
    const inCorrectDir = currentPath[currentPath.length - 1] === expectedDir
    results.push({
      item: '模拟工作目录与题面英文名',
      status: inCorrectDir ? 'pass' : 'fail',
      msg: inCorrectDir
        ? `界面路径匹配: ~/${homeName}/${expectedDir}/${expectedFile}（与收题包同名，仅为模拟路径）`
        : `当前模拟路径为 ~/${currentPath.join('/')}，本题目录对应 ~/${homeName}/${expectedDir}/。`,
    })

    // 2. 0-byte source check
    if (!code || code.trim().length === 0) {
      results.push({
        item: '源文件非空检查 (防0字节提交悲剧)',
        status: 'fail',
        msg: '危险：当前代码为 0 字节！在真实考场中提交 0 字节文件将直接被判 0 分！',
      })
    } else {
      results.push({
        item: '源文件非空检查',
        status: 'pass',
        msg: `代码大小: ${new Blob([code]).size} 字节 (${code.split('\n').length} 行)`,
      })
    }

    // 3. File IO declaration check
    if (activeProblem.ioMode === 'file') {
      const inFile = activeProblem.inFile || `${expectedDir}.in`
      const outFile = activeProblem.outFile || `${expectedDir}.out`
      const hasFreopenIn = code.includes(inFile)
      const hasFreopenOut = code.includes(outFile)

      if (!hasFreopenIn || !hasFreopenOut) {
        results.push({
          item: '考场文件重定向 (freopen 专项体检)',
          status: 'fail',
          msg: `题目开启了文件 IO！需包含 freopen("${inFile}", "r", stdin); 与 freopen("${outFile}", "w", stdout);`,
        })
      } else {
        results.push({
          item: '考场文件重定向 (freopen)',
          status: 'pass',
          msg: `已正确包含 ${inFile} 与 ${outFile} 输入输出重定向语句`,
        })
      }
    } else {
      results.push({
        item: 'IO 模式规范',
        status: 'pass',
        msg: '当前试题采用标准输入输出 (stdio)，无需 freopen。',
      })
    }

    // 4. Static shape checks. Keep this wording explicit: it is not a
    // compiler and cannot prove that the program builds or runs.
    const hasMain = /\bmain\s*\(/.test(code)
    const openBraces = (code.match(/{/g) || []).length
    const closeBraces = (code.match(/}/g) || []).length
    results.push({
      item: '源码静态结构检查（非编译器）',
      status: hasMain && openBraces === closeBraces ? 'pass' : 'fail',
      msg: !hasMain
        ? '未检测到 main() 函数文本。'
        : openBraces !== closeBraces
          ? `花括号数量不平衡：${openBraces} 个左括号，${closeBraces} 个右括号。`
          : '检测到 main() 且花括号数量平衡；这不等于编译通过，最终以服务端评测为准。',
    })

    return results
  }

  const checkResults = runFileCheck()
  const passCount = checkResults.filter((r) => r.status === 'pass').length

  // Alt+Tab 切换器的候选项：只列打开且未最小化的窗口。
  const switcherEntries = [
    { key: 'files' as WindowKey, label: '文件', icon: <img src={YARU_ICONS.nautilus} alt="" className="h-12 w-12 object-contain" draggable={false} /> },
    { key: 'editor' as WindowKey, label: 'Geany', icon: <img src={YARU_ICONS.geany} alt="" className="h-12 w-12 object-contain" draggable={false} /> },
    { key: 'terminal' as WindowKey, label: '终端', icon: <img src={YARU_ICONS.terminalApp} alt="" className="h-12 w-12 object-contain" draggable={false} /> },
    { key: 'pdf' as WindowKey, label: 'Evince', icon: <img src={YARU_ICONS.evince} alt="" className="h-12 w-12 object-contain" draggable={false} /> },
    { key: 'doctor' as WindowKey, label: '考场体检', icon: <Sparkles className="h-12 w-12 text-amber-400" /> },
  ].filter((entry) => windows[entry.key].isOpen && !windows[entry.key].isMinimized)

  return (
    <div
      className="fixed inset-0 z-50 flex flex-col bg-black text-slate-100 select-none overflow-hidden"
      style={{ fontFamily: YARU_SANS }}
    >
      {/* ============================================================ */}
      {/* 1. GNOME Shell 顶栏（Ubuntu 20.04 / NOI Linux 2.0）          */}
      {/* ============================================================ */}
      <div className="relative h-7 bg-black px-3 flex items-center justify-between text-xs text-slate-200 z-40">
        {/* Left: Activities & Focused App Title */}
        <div className="flex items-center gap-3">
          <button
            onClick={() => focusWindow('files')}
            className="font-bold text-white transition-colors px-1.5 py-0.5 rounded hover:bg-white/10"
          >
            活动
          </button>
          <span className="text-slate-400 font-medium text-[11px] hidden sm:inline">
            {windows.editor.zIndex >= windows.files.zIndex && !windows.editor.isMinimized
              ? 'Geany'
              : windows.terminal.zIndex >= windows.files.zIndex && !windows.terminal.isMinimized
              ? '终端'
              : '文件'}
          </span>
        </div>

        {/* Center: GNOME Clock */}
        <div className="absolute left-1/2 -translate-x-1/2 font-semibold text-[12px] text-white">
          {currentTimeStr}
        </div>

        {/* Right: Exam Timer, System Icons & Exit */}
        <div className="flex items-center gap-2.5">
          {contest && (
            <div className="scale-90 origin-right">
              <ContestTimer contest={contest} userId={session?.userId} />
            </div>
          )}

          <div className="flex items-center gap-1.5 text-slate-400 text-xs px-2 py-0.5 rounded bg-white/5">
            <Wifi className="w-3.5 h-3.5 text-slate-300" />
            <Volume2 className="w-3.5 h-3.5 text-slate-300" />
            <Power className="w-3.5 h-3.5 text-emerald-400" />
            <span className="text-[11px] text-slate-300 ml-1 font-mono">{homeName}</span>
          </div>

          <Button
            size="sm"
            variant="outline"
            onClick={onExitDesktop}
            className="h-5 px-2 text-[10px] bg-red-600/30 border-red-500/50 text-red-200 hover:bg-red-600/50 hover:text-white"
          >
            退出桌面
          </Button>
        </div>
      </div>

      {/* ============================================================ */}
      {/* 2. Main Desktop Body: Ubuntu Dock (Left) + Wallpaper Area    */}
      {/* ============================================================ */}
      <div className="flex-1 flex relative overflow-hidden bg-black">
        <NoiWallpaper />
        {/* Ubuntu Dock (Left) */}
        <div className="w-14 bg-black/60 backdrop-blur-md border-r border-white/10 flex flex-col items-center py-3 gap-3 z-30 shrink-0">
          {/* Nautilus Files */}
          <button
            onClick={() => toggleWindow('files')}
            className={`w-10 h-10 rounded-xl flex items-center justify-center relative transition-transform hover:scale-150 ${
              windows.files.isOpen && !windows.files.isMinimized ? 'bg-orange-500/20 text-orange-400' : 'text-slate-300 hover:bg-white/10'
            }`}
            title="Files (Nautilus 文件管理器)"
          >
            <img src={YARU_ICONS.nautilus} alt="" className="w-6 h-6 object-contain" draggable={false} />
            {windows.files.isOpen && (
              <span className="w-1.5 h-1.5 rounded-full bg-orange-400 absolute left-0.5" />
            )}
          </button>

          {/* Gedit / Code Editor */}
          <button
            onClick={() => toggleWindow('editor')}
            className={`w-10 h-10 rounded-xl flex items-center justify-center relative transition-transform hover:scale-150 ${
              windows.editor.isOpen && !windows.editor.isMinimized ? 'bg-blue-500/20 text-blue-400' : 'text-slate-300 hover:bg-white/10'
            }`}
            title="Geany 代码编辑器"
          >
            <img src={YARU_ICONS.geany} alt="" className="w-6 h-6 object-contain" draggable={false} />
            {windows.editor.isOpen && (
              <span className="w-1.5 h-1.5 rounded-full bg-blue-400 absolute left-0.5" />
            )}
          </button>

          {/* Terminal */}
          <button
            onClick={() => toggleWindow('terminal')}
            className={`w-10 h-10 rounded-xl flex items-center justify-center relative transition-transform hover:scale-150 ${
              windows.terminal.isOpen && !windows.terminal.isMinimized ? 'bg-emerald-500/20 text-emerald-400' : 'text-slate-300 hover:bg-white/10'
            }`}
            title="Terminal (GNOME 终端)"
          >
            <img src={YARU_ICONS.terminalApp} alt="" className="w-6 h-6 object-contain" draggable={false} />
            {windows.terminal.isOpen && (
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 absolute left-0.5" />
            )}
          </button>

          {/* PDF Viewer */}
          <button
            onClick={() => toggleWindow('pdf')}
            className={`w-10 h-10 rounded-xl flex items-center justify-center relative transition-transform hover:scale-150 ${
              windows.pdf.isOpen && !windows.pdf.isMinimized ? 'bg-red-500/20 text-red-400' : 'text-slate-300 hover:bg-white/10'
            }`}
            title="Evince (试卷题面查看器)"
          >
            <img src={YARU_ICONS.evince} alt="" className="w-6 h-6 object-contain" draggable={false} />
            {windows.pdf.isOpen && (
              <span className="w-1.5 h-1.5 rounded-full bg-red-400 absolute left-0.5" />
            )}
          </button>

          {/* Exam Doctor */}
          <button
            onClick={() => toggleWindow('doctor')}
            className={`w-10 h-10 rounded-xl flex items-center justify-center relative transition-transform hover:scale-150 ${
              windows.doctor.isOpen && !windows.doctor.isMinimized ? 'bg-amber-500/20 text-amber-400' : 'text-slate-300 hover:bg-white/10'
            }`}
            title="CCF 考场防踩坑体检医生"
          >
            <Sparkles className="w-5 h-5 text-amber-400" />
            {windows.doctor.isOpen && (
              <span className="w-1.5 h-1.5 rounded-full bg-amber-400 absolute left-0.5" />
            )}
          </button>

          {/* 应用网格（Show Applications，视觉还原） */}
          <button
            className="w-10 h-10 rounded-xl flex items-center justify-center text-slate-300 hover:bg-white/10 transition-transform"
            title="显示应用"
          >
            <LayoutGrid className="w-5 h-5" />
          </button>

          <div className="w-8 h-px bg-white/10 my-auto" />

          {/* Trash */}
          <button
            onClick={() =>
              toast.notify('info', '回收站为空', '考试桌面不会保留临时文件。')
            }
            className="w-10 h-10 rounded-xl flex items-center justify-center text-slate-400 hover:bg-white/10 transition-transform"
            title="Trash (废纸篓)"
          >
            <img src={YARU_ICONS.userTrash} alt="" className="w-6 h-6 object-contain" draggable={false} />
          </button>
        </div>

        {/* Desktop Canvas (Wallpaper + Desktop Icons) */}
        <div
          className="flex-1 relative p-6 overflow-hidden"
          onContextMenu={(event) => {
            event.preventDefault()
            setDesktopMenu({ x: event.clientX, y: event.clientY })
          }}
        >
          {/* Desktop Icons Grid */}
          <div className="grid grid-cols-1 gap-6 w-24">
            {/* 1. User Home Folder */}
            <div
              onDoubleClick={() => focusWindow('files')}
              className="flex flex-col items-center gap-1.5 p-2 rounded-xl hover:bg-white/10 cursor-pointer group text-center"
            >
              <img
                src={YARU_ICONS.userHome}
                alt=""
                draggable={false}
                className="w-12 h-12 object-contain drop-shadow-lg group-hover:scale-105 transition-transform"
              />
              <span className="text-[11px] font-medium text-slate-100 group-hover:underline drop-shadow-sm line-clamp-1">
                {homeName}
              </span>
            </div>

            {/* 2. Exam Zip (Locked or Unlocked) */}
            <div
              onDoubleClick={() => (!isUnlocked ? focusWindow('unlock') : focusWindow('files'))}
              className="flex flex-col items-center gap-1.5 p-2 rounded-xl hover:bg-white/10 cursor-pointer group text-center relative"
            >
              <div className="w-12 h-12 rounded-lg bg-amber-500/20 border border-amber-400/40 flex items-center justify-center text-amber-300 shadow-lg group-hover:scale-105 transition-transform">
                <HardDrive className="w-6 h-6" />
                {!isUnlocked && (
                  <span className="absolute -top-1 -right-1 w-5 h-5 rounded-full bg-rose-600 text-white flex items-center justify-center text-[10px] shadow">
                    <Lock className="w-3 h-3" />
                  </span>
                )}
              </div>
              <span className="text-[11px] font-medium text-slate-100 group-hover:underline drop-shadow-sm line-clamp-1">
                {contest.name || 'exam'}.zip
              </span>
            </div>

            {/* 3. Official PDF */}
            <div
              onDoubleClick={() => focusWindow('pdf')}
              className="flex flex-col items-center gap-1.5 p-2 rounded-xl hover:bg-white/10 cursor-pointer group text-center"
            >
              <div className="w-12 h-12 rounded-lg bg-red-500/20 border border-red-400/40 flex items-center justify-center text-red-300 shadow-lg group-hover:scale-105 transition-transform">
                <FileText className="w-6 h-6" />
              </div>
              <span className="text-[11px] font-medium text-slate-100 group-hover:underline drop-shadow-sm line-clamp-1">
                day1.pdf
              </span>
            </div>

            {/* 4. Terminal Launcher */}
            <div
              onDoubleClick={() => focusWindow('terminal')}
              className="flex flex-col items-center gap-1.5 p-2 rounded-xl hover:bg-white/10 cursor-pointer group text-center"
            >
              <div className="w-12 h-12 rounded-lg bg-emerald-500/20 border border-emerald-400/40 flex items-center justify-center text-emerald-300 shadow-lg group-hover:scale-105 transition-transform">
                <TerminalIcon className="w-6 h-6" />
              </div>
              <span className="text-[11px] font-medium text-slate-100 group-hover:underline drop-shadow-sm line-clamp-1">
                Terminal
              </span>
            </div>
          </div>

          {/* ============================================================ */}
          {/* WINDOW 1: Nautilus 文件管理器（Yaru 复刻）                       */}
          {/* ============================================================ */}
          {windows.files.isOpen && !windows.files.isMinimized && (
            <div
              onClick={() => focusWindow('files')}
              style={{
                left: windows.files.isMaximized ? 0 : windows.files.x,
                top: windows.files.isMaximized ? 0 : windows.files.y,
                width: windows.files.isMaximized ? '100%' : windows.files.w,
                height: windows.files.isMaximized ? '100%' : windows.files.h,
                zIndex: windows.files.zIndex,
              }}
              className="absolute flex flex-col overflow-hidden"
            >
              <NautilusWindow
                homeName={homeName}
                pathSegments={currentPath.slice(2)}
                trashMode={trashView}
                protectedNames={protectedNames}
                items={
                  trashView
                    ? trash.map((entry) => ({
                        id: entry.item.name,
                        name: entry.item.name,
                        kind: entry.item.kind === 'dir' ? ('folder' as const) : ('source' as const),
                        sizeBytes: byteLength(entry.item.content ?? ''),
                      }))
                    : dirEntries.map((entry) => ({
                        id: entry.name,
                        name: entry.name,
                        kind: entry.kind === 'dir' ? ('folder' as const) : ('source' as const),
                        sizeBytes: entry.sizeBytes,
                      }))
                }
                selectedId={selectedNautilus}
                onSelect={setSelectedNautilus}
                onNavigateDepth={(level) => {
                  if (level === -1) {
                    setTrashView(true)
                    return
                  }
                  setTrashView(false)
                  if (level === 0) setCurrentPath(['home', homeName])
                  else setCurrentPath(['home', homeName, ...currentPath.slice(2).slice(0, level)])
                }}
                onOpen={(item) => {
                  if (trashView) return
                  if (item.kind === 'folder') {
                    setCurrentPath((prev) => [...prev, item.name])
                  } else {
                    const entry = dirEntries.find((e) => e.name === item.name)
                    if (entry?.problemId) {
                      const problem = problems.find((p) => p.id === entry.problemId)
                      if (problem) openProblemDoc(problem)
                    } else {
                      openVfsDoc(homeKey, {
                        name: item.name,
                        kind: 'file',
                        content: entry?.content,
                        readonly: entry?.readonly,
                      })
                    }
                  }
                }}
                onNewFolder={(name) => {
                  const result = vfsMkdir(userFiles[homeKey] ?? [], name)
                  if (result.ok) setUserFiles((prev) => ({ ...prev, [homeKey]: result.items }))
                  return result.ok ? null : (result.error ?? '无法创建')
                }}
                onRenamePath={(oldName, newName) => {
                  const result = vfsRename(userFiles[homeKey] ?? [], oldName, newName)
                  if (result.ok) setUserFiles((prev) => ({ ...prev, [homeKey]: result.items }))
                  return result.ok ? null : (result.error ?? '无法重命名')
                }}
                onTrash={(name) => {
                  const result = vfsTrash(userFiles[homeKey] ?? [], name)
                  if (result.removed) {
                    setUserFiles((prev) => ({ ...prev, [homeKey]: result.items }))
                    setTrash((prev) => [...prev, { item: result.removed!, fromKey: homeKey }])
                  }
                }}
                onRestore={(name) => {
                  const entry = trash.find((t) => t.item.name === name)
                  if (!entry) return
                  const result = vfsRestore(userFiles[entry.fromKey] ?? [], entry.item)
                  if (result.ok) {
                    setUserFiles((prev) => ({ ...prev, [entry.fromKey]: result.items }))
                    setTrash((prev) => prev.filter((t) => t.item.name !== name))
                  }
                }}
                onEmptyTrash={() => {
                  setTrash([])
                }}
                onOpenTerminal={() => toggleWindow('terminal')}
                onMouseDown={(e) => handleTitleMouseDown('files', e)}
                onMinimize={() => setWindows((p) => ({ ...p, files: { ...p.files, isMinimized: true } }))}
                onMaximize={() => setWindows((p) => ({ ...p, files: { ...p.files, isMaximized: !p.files.isMaximized } }))}
                onClose={() => setWindows((p) => ({ ...p, files: { ...p.files, isOpen: false } }))}
              />
            </div>
          )}

          {/* ============================================================ */}
          {/* WINDOW 2: Geany 代码编辑器（Yaru 复刻）                         */}
          {/* ============================================================ */}
          {windows.editor.isOpen && !windows.editor.isMinimized && (
            <div
              onClick={() => focusWindow('editor')}
              style={{
                left: windows.editor.isMaximized ? 0 : windows.editor.x,
                top: windows.editor.isMaximized ? 0 : windows.editor.y,
                width: windows.editor.isMaximized ? '100%' : windows.editor.w,
                height: windows.editor.isMaximized ? '100%' : windows.editor.h,
                zIndex: windows.editor.zIndex,
              }}
              className="absolute flex flex-col overflow-hidden"
            >
              <GeanyWindow
                dirName={docPath}
                fileName={activeDoc?.name ?? `${currentTaskDir}.cpp`}
                homeName={homeName}
                code={docCode}
                onCode={setDocCode}
                readOnly={docReadOnly}
                compilerLog={geanyCompilerLog}
                onCompile={() => {
                  setGeanyCompilerLog(
                    buildStaticCheckLog(
                      docCode,
                      activeDoc?.name ?? `${currentTaskDir}.cpp`,
                      activeDoc?.kind === 'problem' ? activeDocProblem : null
                    )
                  )
                  focusWindow('editor')
                }}
                onRun={() => toggleWindow('terminal')}
                onSave={() => {
                  if (activeDoc?.kind === 'problem') onSave?.()
                }}
                tabs={openDocs.map((d) => ({ id: d.id, name: d.name, kind: d.kind }))}
                activeTabId={activeDocId}
                onSelectTab={(id) => {
                  const doc = openDocs.find((d) => d.id === id)
                  if (!doc) return
                  setActiveDocId(id)
                  if (doc.kind === 'problem') {
                    const next = problems.find((x) => x.id === doc.problemId)
                    if (next) {
                      setActiveProblem(next)
                      setCurrentPath(['home', homeName, taskDirOf(next)])
                    }
                  }
                }}
                onCloseDoc={closeDoc}
                canSubmit={activeDoc?.kind === 'problem'}
                onNewDoc={() => {
                  untitledCounter.current += 1
                  const n = untitledCounter.current
                  const id = `untitled:${n}`
                  setOpenDocs((prev) => [
                    ...prev,
                    {
                      id,
                      kind: 'untitled' as const,
                      name: n > 1 ? `无标题 ${n}` : '无标题',
                    },
                  ])
                  setUntitledContents((prev) => ({ ...prev, [id]: '' }))
                  setActiveDocId(id)
                  focusWindow('editor')
                }}
                onSaveAs={(newName) => {
                  if (!activeDoc || activeDoc.kind === 'problem') return '未命名文档已失效'
                  const key = activeDoc.kind === 'vfs' ? (activeDoc.dirKey ?? '') : ''
                  const result = mkfile(userFiles[key] ?? [], newName, docCode)
                  if (!result.ok) return (result.error ?? '保存失败')
                  setUserFiles((prev) => ({ ...prev, [key]: result.items }))
                  // 真实 Geany：保存后文档就地变成已命名文件，无任何提示框。
                  setOpenDocs((prev) =>
                    prev.map((d) =>
                      d.id === activeDoc.id
                        ? { ...d, id: `vfs:${key}/${newName}`, kind: 'vfs' as const, name: newName, dirKey: key }
                        : d
                    )
                  )
                  setActiveDocId(`vfs:${key}/${newName}`)
                  setUntitledContents((prev) => {
                    const next = { ...prev }
                    delete next[activeDoc.id]
                    return next
                  })
                  return null
                }}
                onDoctor={() => focusWindow('doctor')}
                onPickUploadFile={onPickUploadFile}
                uploadFile={uploadFile}
                uploadMismatch={uploadMismatch}
                onClearUploadFile={onClearUploadFile}
                onSubmit={onSubmit}
                submitting={submitting}
                onMouseDown={(e) => handleTitleMouseDown('editor', e)}
                onMinimize={() => setWindows((p) => ({ ...p, editor: { ...p.editor, isMinimized: true } }))}
                onMaximize={() => setWindows((p) => ({ ...p, editor: { ...p.editor, isMaximized: !p.editor.isMaximized } }))}
                onClose={() => setWindows((p) => ({ ...p, editor: { ...p.editor, isOpen: false } }))}
              />
            </div>
          )}

          {/* ============================================================ */}
          {/* WINDOW 3: GNOME Terminal（Yaru 复刻）                          */}
          {/* ============================================================ */}
          {windows.terminal.isOpen && !windows.terminal.isMinimized && (
            <div
              onClick={() => focusWindow('terminal')}
              style={{
                left: windows.terminal.isMaximized ? 0 : windows.terminal.x,
                top: windows.terminal.isMaximized ? 0 : windows.terminal.y,
                width: windows.terminal.isMaximized ? '100%' : windows.terminal.w,
                height: windows.terminal.isMaximized ? '100%' : windows.terminal.h,
                zIndex: windows.terminal.zIndex,
              }}
              className="absolute flex flex-col overflow-hidden"
            >
              <TerminalWindow
                homeName={homeName}
                path={currentPath.slice(2).join('/')}
                logs={terminalLogs}
                onCommand={handleTerminalCommand}
                onHint={(text) => setTerminalLogs((prev) => [...prev, { type: 'output', text }])}
                onClearScreen={() => setTerminalLogs([])}
                completions={terminalCompletions}
                onMouseDown={(e) => handleTitleMouseDown('terminal', e)}
                onMinimize={() => setWindows((p) => ({ ...p, terminal: { ...p.terminal, isMinimized: true } }))}
                onMaximize={() => setWindows((p) => ({ ...p, terminal: { ...p.terminal, isMaximized: !p.terminal.isMaximized } }))}
                onClose={() => setWindows((p) => ({ ...p, terminal: { ...p.terminal, isOpen: false } }))}
              />
            </div>
          )}

          {/* ============================================================ */}
          {/* WINDOW 4: Evince 文档查看器（Yaru 复刻）                       */}
          {/* ============================================================ */}
          {windows.pdf.isOpen && !windows.pdf.isMinimized && activeProblem && (
            <div
              onClick={() => focusWindow('pdf')}
              style={{
                left: windows.pdf.isMaximized ? 0 : windows.pdf.x,
                top: windows.pdf.isMaximized ? 0 : windows.pdf.y,
                width: windows.pdf.isMaximized ? '100%' : windows.pdf.w,
                height: windows.pdf.isMaximized ? '100%' : windows.pdf.h,
                zIndex: windows.pdf.zIndex,
              }}
              className="absolute flex flex-col overflow-hidden"
            >
              <EvinceWindow
                fileName={`${currentTaskDir}.pdf`}
                title={`${activeProblem.code}. ${activeProblem.title}`}
                content={activeProblem.statement}
                onMouseDown={(e) => handleTitleMouseDown('pdf', e)}
                onMinimize={() => setWindows((p) => ({ ...p, pdf: { ...p.pdf, isMinimized: true } }))}
                onMaximize={() => setWindows((p) => ({ ...p, pdf: { ...p.pdf, isMaximized: !p.pdf.isMaximized } }))}
                onClose={() => setWindows((p) => ({ ...p, pdf: { ...p.pdf, isOpen: false } }))}
              />
            </div>
          )}

          {/* WINDOW 5: CCF Exam Doctor (Inspector)                        */}
          {/* ============================================================ */}
          {windows.doctor.isOpen && !windows.doctor.isMinimized && (
            <div
              onClick={() => focusWindow('doctor')}
              style={{
                left: windows.doctor.isMaximized ? 0 : windows.doctor.x,
                top: windows.doctor.isMaximized ? 0 : windows.doctor.y,
                width: windows.doctor.isMaximized ? '100%' : windows.doctor.w,
                height: windows.doctor.isMaximized ? '100%' : windows.doctor.h,
                zIndex: windows.doctor.zIndex,
              }}
              className="absolute bg-white rounded-xl shadow-2xl border border-slate-300 text-slate-800 flex flex-col overflow-hidden animate-in fade-in-50 duration-100"
            >
              {/* Header */}
              <YaruHeaderBar
                title="CCF 考场静态检查器（非编译器）"
                onMouseDown={(e) => handleTitleMouseDown('doctor', e)}
                onMinimize={() => setWindows((p) => ({ ...p, doctor: { ...p.doctor, isMinimized: true } }))}
                onMaximize={() => setWindows((p) => ({ ...p, doctor: { ...p.doctor, isMaximized: !p.doctor.isMaximized } }))}
                onClose={() => setWindows((p) => ({ ...p, doctor: { ...p.doctor, isOpen: false } }))}
                actions={<Sparkles className="h-4 w-4 text-amber-500" />}
              />

              {/* Diagnostic Score Card */}
              <div className="p-4 bg-amber-50/70 border-b border-amber-100 flex items-center justify-between">
                <div>
                  <div className="text-sm font-bold text-amber-950 flex items-center gap-2">
                    {passCount === checkResults.length ? (
                      <ShieldCheck className="w-5 h-5 text-emerald-600" />
                    ) : (
                      <ShieldAlert className="w-5 h-5 text-rose-600" />
                    )}
                    <span>静态检查: {passCount} / {checkResults.length} 项通过</span>
                  </div>
                  <p className="text-[11px] text-amber-800 mt-0.5">
                    仅检查界面内可验证的源码文本与题目配置；浏览器不执行编译、运行或本地文件扫描，不能替代服务端评测。
                  </p>
                </div>
              </div>

              {/* Check Items List */}
              <div className="flex-1 p-4 overflow-y-auto space-y-2.5 bg-slate-50">
                {checkResults.map((r, i) => (
                  <div
                    key={i}
                    className={`p-3 rounded-lg border text-xs flex items-start gap-2.5 bg-white ${
                      r.status === 'pass'
                        ? 'border-emerald-200 text-slate-700'
                        : 'border-rose-300 bg-rose-50/40 text-rose-900 font-medium'
                    }`}
                  >
                    {r.status === 'pass' ? (
                      <ShieldCheck className="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
                    ) : (
                      <ShieldAlert className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
                    )}
                    <div className="space-y-0.5 flex-1">
                      <div className="font-bold">{r.item}</div>
                      <div className="text-[11px] opacity-90">{r.msg}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* ============================================================ */}
          {/* WINDOW 6: Zip Password Unlock Modal                          */}
          {/* ============================================================ */}
          {windows.unlock.isOpen && !windows.unlock.isMinimized && (
            <div
              onClick={() => focusWindow('unlock')}
              style={{
                left: windows.unlock.isMaximized ? 0 : windows.unlock.x,
                top: windows.unlock.isMaximized ? 0 : windows.unlock.y,
                width: windows.unlock.isMaximized ? '100%' : windows.unlock.w,
                height: windows.unlock.isMaximized ? '100%' : windows.unlock.h,
                zIndex: windows.unlock.zIndex,
              }}
              className="absolute bg-white rounded-2xl shadow-2xl border border-slate-300 text-slate-800 flex flex-col overflow-hidden animate-in fade-in-50 duration-100"
            >
              <YaruHeaderBar
                title="考场试题包密码解锁"
                onMouseDown={(e) => handleTitleMouseDown('unlock', e)}
                onClose={() => setWindows((p) => ({ ...p, unlock: { ...p.unlock, isOpen: false } }))}
              />

              <form onSubmit={handleUnlock} className="p-6 space-y-4">
                <div className="text-center space-y-1">
                  <h3 className="font-bold text-slate-900 text-base">解密考场题目压缩包</h3>
                  <p className="text-xs text-slate-500">
                    监考口播或系统下发口令后在此输入以就地解包题面
                  </p>
                </div>

                {unlockError && (
                  <div className="p-2.5 rounded-lg bg-rose-50 border border-rose-200 text-rose-700 text-xs">
                    {unlockError}
                  </div>
                )}

                <input
                  type="password"
                  placeholder="输入开考解锁口令"
                  value={zipInputPassword}
                  onChange={(e) => setZipInputPassword(e.target.value)}
                  className="w-full text-center tracking-widest text-base font-mono py-2 rounded-lg border border-slate-300 focus:ring-2 focus:ring-blue-500 outline-none"
                  autoFocus
                />

                <Button className="w-full" disabled={unlocking}>
                  {unlocking ? '正在解密并展开试题...' : '立即解锁考场题面'}
                </Button>
              </form>
            </div>
          )}
        </div>
      </div>
        {switcher && switcherEntries.length > 0 && (
          <div className="absolute inset-0 z-[80] grid place-items-center bg-black/40">
            <div className="flex max-w-[82%] flex-wrap items-center justify-center gap-3 rounded-2xl border border-white/15 bg-[#1C1C1C]/95 p-4 shadow-2xl">
              {switcherEntries.map((entry, index) => (
                <div
                  key={entry.key}
                  className={`flex w-24 flex-col items-center gap-1.5 rounded-xl p-3 ${
                    index === switcher.index ? 'bg-white/15 ring-2 ring-[#E95420]' : ''
                  }`}
                >
                  {entry.icon}
                  <span className="text-[11px] font-semibold text-white">{entry.label}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        {desktopMenu && (
          <ContextMenu
            x={desktopMenu.x}
            y={desktopMenu.y}
            items={[
              { label: '新建文件夹', shortcut: 'Shift+Ctrl+N', disabled: true },
              { label: '粘贴', shortcut: 'Ctrl+V', disabled: true },
              { separator: true },
              { label: '在终端中打开', onClick: () => toggleWindow('terminal') },
              { separator: true },
              { label: '更改背景…', disabled: true },
              { label: '显示设置', disabled: true },
              { separator: true },
              {
                label: '属性',
                onClick: () =>
                  toast.notify('info', '桌面属性', 'NOI Linux 2.0 模拟桌面 · Ubuntu 20.04.1 LTS (Yaru)'),
              },
            ] as MenuItem[]}
            onClose={() => setDesktopMenu(null)}
          />
        )}
    </div>
  )
}
