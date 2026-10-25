// NOI Linux 2.0（Ubuntu 20.04.1 LTS + Yaru 主题）的桌面应用复刻。
// 配色与布局参数取自官方 ISO 内的实测素材：Yaru 图标位图、geany-3.20.css、
// gnome-terminal 默认 profile（紫色 #300A24）以及 NOI 官方壁纸的取色。
// 字体栈回退：目标机装有 Ubuntu 字体时逐像素一致，否则退化到相近字形。
import React, { useEffect, useRef, useState } from 'react'
import {
  ChevronLeft,
  ChevronRight,
  Clock3,
  LayoutGrid,
  List,
  Copy as CopyIcon,
  Download,
  FileText,
  FolderOpen,
  Image as ImageIcon,
  Menu as MenuIcon,
  Music,
  Play,
  Search,
  Trash2,
  Video,
  X,
  PanelLeft,
} from 'lucide-react'
import { MarkdownView } from '../MarkdownView'
import { YARU_ICONS } from './yaruIcons'
import { formatBytes } from '../../features/student/submissionFiles'
import type { CommandLog } from '../UbuntuDesktop'

// ---------------------------------------------------------------- 令牌

export const YARU = {
  accent: '#E95420',
  headerbarBg: '#EBEBEB',
  headerbarBorder: '#CFCFCF',
  headerbarFg: '#2D2D2D',
  ssdBg: '#2C2C2C',
  ssdBorder: '#1B1B1B',
  ssdFg: '#F2F2F2',
  contentBg: '#FFFFFF',
  chromeBg: '#F7F7F6',
  sidebarBg: '#FAFAFA',
  sidebarBorder: '#E4E4E4',
  menuBorder: '#C6C6C6',
  hoverFill: 'rgba(0,0,0,0.07)',
  terminalBg: '#300A24',
  terminalFg: '#FFFFFF',
  promptGreen: '#8AE234',
  promptBlue: '#729FCF',
  errorRed: '#EF2929',
  noiBlue: '#002C7D',
  noiRed: '#D7000F',
}

export const YARU_SANS =
  "'Ubuntu', 'Noto Sans CJK SC', 'Microsoft YaHei UI', 'Segoe UI', sans-serif"
export const YARU_MONO =
  "'Ubuntu Mono', 'Cascadia Mono', Consolas, 'Courier New', monospace"

function IconButton({
  kind,
  onClick,
  dark = false,
}: {
  kind: 'min' | 'max' | 'close'
  onClick?: () => void
  dark?: boolean
}) {
  return (
    <button
      type="button"
      onClick={(event) => {
        event.stopPropagation()
        onClick?.()
      }}
      aria-label={kind}
      className={`grid h-[18px] w-[18px] place-items-center rounded-full transition-colors ${
        dark ? 'bg-white/15 text-[#F2F2F2]' : 'bg-[#BFBFBF]/80 text-[#3D3D3D]'
      } ${
        kind === 'close'
          ? dark
            ? 'hover:bg-[#C01C28] hover:text-white'
            : 'hover:bg-[#E95420] hover:text-white'
          : dark
            ? 'hover:bg-white/30'
            : 'hover:bg-[#9A9A9A]'
      }`}
    >
      {kind === 'min' && (
        <svg width="10" height="10" viewBox="0 0 10 10">
          <rect x="1.5" y="4.5" width="7" height="1.6" fill="currentColor" />
        </svg>
      )}
      {kind === 'max' && (
        <svg width="10" height="10" viewBox="0 0 10 10">
          <rect
            x="1.8"
            y="1.8"
            width="6.4"
            height="6.4"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.4"
          />
        </svg>
      )}
      {kind === 'close' && (
        <svg width="10" height="10" viewBox="0 0 10 10">
          <path
            d="M2 2 L8 8 M8 2 L2 8"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
          />
        </svg>
      )}
    </button>
  )
}

// GNOME 头栏（CSD，Yaru 浅色）：标题绝对居中（GNOME 规范），left 放路径栏/
// 标签页等左侧内容；未提供最小化/最大化回调时不渲染对应圆钮（如解锁弹窗）。
export function YaruHeaderBar({
  title,
  left,
  actions,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  title?: string
  left?: React.ReactNode
  actions?: React.ReactNode
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  return (
    <div
      onMouseDown={onMouseDown}
      onDoubleClick={onMaximize}
      className="relative flex h-10 shrink-0 items-center px-2.5"
      style={{ background: YARU.headerbarBg, borderBottom: `1px solid ${YARU.headerbarBorder}` }}
    >
      {left && <div className="z-10 flex min-w-0 items-center">{left}</div>}
      {title && (
        <span
          className="pointer-events-none absolute left-1/2 max-w-[45%] -translate-x-1/2 truncate text-[13px] font-bold"
          style={{ color: YARU.headerbarFg, fontFamily: YARU_SANS }}
        >
          {title}
        </span>
      )}
      <div className="ml-auto flex shrink-0 items-center gap-2.5">
        {actions}
        <div className="flex items-center gap-2">
          {onMinimize && <IconButton kind="min" onClick={onMinimize} />}
          {onMaximize && <IconButton kind="max" onClick={onMaximize} />}
          <IconButton kind="close" onClick={onClose} />
        </div>
      </div>
    </div>
  )
}

// 传统标题栏（SSD，深色）：Geany 等未走 CSD 的应用用。
export function YaruTitleBar({
  title,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  title: string
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  return (
    <div
      onMouseDown={onMouseDown}
      onDoubleClick={onMaximize}
      className="flex h-8 shrink-0 items-center px-2"
      style={{ background: YARU.ssdBg, borderBottom: `1px solid ${YARU.ssdBorder}` }}
    >
      <span
        className="mx-auto truncate text-[12px] font-bold"
        style={{ color: YARU.ssdFg, fontFamily: YARU_SANS }}
      >
        {title}
      </span>
      <div className="absolute right-2.5 flex items-center gap-2">
        <IconButton kind="min" dark onClick={onMinimize} />
        <IconButton kind="max" dark onClick={onMaximize} />
        <IconButton kind="close" dark onClick={onClose} />
      </div>
    </div>
  )
}

// ---------------------------------------------------------------- 右键菜单

export interface MenuItem {
  label?: string
  shortcut?: string
  disabled?: boolean
  danger?: boolean
  separator?: boolean
  submenu?: boolean
  icon?: React.ReactNode
  onClick?: () => void
}

export function ContextMenu({
  x,
  y,
  items,
  onClose,
}: {
  x: number
  y: number
  items: MenuItem[]
  onClose: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const dismiss = (event: MouseEvent) => {
      if (ref.current && event.target instanceof Node && ref.current.contains(event.target)) {
        return
      }
      onClose()
    }
    window.addEventListener('mousedown', dismiss, true)
    window.addEventListener('contextmenu', dismiss, true)
    return () => {
      window.removeEventListener('mousedown', dismiss, true)
      window.removeEventListener('contextmenu', dismiss, true)
    }
  }, [onClose])

  const left = Math.min(x, window.innerWidth - 260)
  const top = Math.min(y, window.innerHeight - items.length * 28 - 24)

  return (
    <div
      ref={ref}
      className="fixed z-[90] min-w-[210px] overflow-hidden rounded-lg py-1 shadow-[0_10px_36px_rgba(0,0,0,0.4)]"
      style={{ left, top, background: YARU.contentBg, border: `1px solid ${YARU.menuBorder}` }}
    >
      {items.map((item, index) =>
        item.separator ? (
          <div key={index} className="my-1 h-px" style={{ background: '#E8E8E8' }} />
        ) : (
          <button
            key={index}
            type="button"
            disabled={item.disabled}
            onClick={() => {
              if (item.disabled) return
              item.onClick?.()
              onClose()
            }}
            className={`flex h-7 w-full items-center gap-2 px-3 text-left text-[13px] ${
              item.disabled ? 'opacity-40' : 'hover:text-white'
            }`}
            style={{ fontFamily: YARU_SANS, color: item.danger ? '#C01C28' : YARU.headerbarFg }}
            onMouseEnter={(event) => {
              if (!item.disabled) {
                event.currentTarget.style.background = item.danger ? '#FADBD9' : YARU.accent
                if (!item.danger) event.currentTarget.style.color = '#FFFFFF'
              }
            }}
            onMouseLeave={(event) => {
              event.currentTarget.style.background = 'transparent'
              if (!item.danger) event.currentTarget.style.color = YARU.headerbarFg
            }}
          >
            {item.icon && <span className="grid w-4 place-items-center opacity-80">{item.icon}</span>}
            <span className="flex-1 truncate">{item.label}</span>
            {item.shortcut && (
              <span className="text-[11px] opacity-55">{item.shortcut}</span>
            )}
            {item.submenu && <ChevronRight className="h-3.5 w-3.5 opacity-60" />}
          </button>
        )
      )}
    </div>
  )
}

// ---------------------------------------------------------------- NOI 壁纸

// 官方壁纸 noi_wallpaper_00.png 的右下角原样裁片（徽标 + 标语一起内嵌），
// 字形与官方逐像素一致；上一版的矢量描摹已废弃。
export function NoiWallpaper() {
  return (
    <div className="absolute inset-0 bg-black">
      <img
        src={YARU_ICONS.noiLogo}
        alt=""
        draggable={false}
        className="absolute bottom-[2.5%] right-[1.5%] w-[34%] max-w-[600px] min-w-[300px] select-none"
      />
    </div>
  )
}

// ---------------------------------------------------------------- Nautilus

export interface NautilusItem {
  id: string
  name: string
  kind: 'folder' | 'source'
  sizeBytes?: number
}


export function NautilusWindow({
  homeName,
  pathSegments,
  items,
  trashMode = false,
  protectedNames = [],
  selectedId,
  onSelect,
  onOpen,
  onNavigateDepth,
  onNavigatePath,
  breadcrumbChildren,
  onNewFolder,
  onOpenTerminal,
  onRenamePath,
  onTrash,
  onRestore,
  onEmptyTrash,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  homeName: string
  /** 相对主目录的层级段：[] = 主目录，['poker'] = 题面目录。 */
  pathSegments: string[]
  items: NautilusItem[]
  /** 回收站视图：菜单切换为 恢复 / 清空回收站。 */
  trashMode?: boolean
  /** 派生条目（题面文件夹/源文件/样例）：重命名与回收站对其禁用。 */
  protectedNames?: string[]
  selectedId: string | null
  onSelect: (id: string | null) => void
  onOpen: (item: NautilusItem) => void
  onNavigateDepth: (depth: number) => void
  /** 侧栏位置导航：segments 相对主目录（[] = 主目录）。 */
  onNavigatePath: (segments: string[]) => void
  /** 面包屑段下拉：列出某层的子目录。 */
  breadcrumbChildren?: (segments: string[]) => string[]
  /** 返回 null = 成功；字符串 = 内联显示在对话框里的错误（重名等）。 */
  onNewFolder: (name: string) => string | null
  onOpenTerminal: () => void
  onRenamePath: (oldName: string, newName: string) => string | null
  onTrash: (name: string) => void
  onRestore: (name: string) => void
  onEmptyTrash: () => void
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  const [menu, setMenu] = useState<{ x: number; y: number; item: NautilusItem | null } | null>(null)
  const [view, setView] = useState<'grid' | 'list'>('grid')
  // 排序：真实 Nautilus 头栏菜单里的排序方式与方向。
  const [sortBy, setSortBy] = useState<'name' | 'size' | 'type'>('name')
  const [sortAsc, setSortAsc] = useState(true)
  const [crumbMenu, setCrumbMenu] = useState<{ x: number; y: number; depth: number } | null>(null)
  const [appMenu, setAppMenu] = useState<{ x: number; y: number } | null>(null)
  // GNOME 属性对话框：null 关闭，'root' 表示主目录自身。
  const [properties, setProperties] = useState<NautilusItem | 'root' | null>(null)
  const [dialog, setDialog] = useState<{
    kind: 'mkdir' | 'rename'
    value: string
    target?: string
    error?: string
  } | null>(null)

  const openMenu = (event: React.MouseEvent, item: NautilusItem | null) => {
    event.preventDefault()
    event.stopPropagation()
    if (item) onSelect(item.id)
    setMenu({ x: event.clientX, y: event.clientY, item })
  }

  const isProtected = (item: NautilusItem | null) =>
    !!item && protectedNames.includes(item.name)

  // 排序：目录优先于文件（Nautilus 默认），再按所选键。
  const sortedItems = [...items].sort((a, b) => {
    const dirFirst = (x: NautilusItem) => (x.kind === 'folder' ? 0 : 1)
    if (dirFirst(a) !== dirFirst(b)) return dirFirst(a) - dirFirst(b)
    const cmp =
      sortBy === 'name'
        ? a.name.localeCompare(b.name, 'zh-CN')
        : sortBy === 'size'
          ? (a.sizeBytes ?? 0) - (b.sizeBytes ?? 0)
          : a.kind.localeCompare(b.kind) || a.name.localeCompare(b.name, 'zh-CN')
    return sortAsc ? cmp : -cmp
  })

  const menuItems = (item: NautilusItem | null): MenuItem[] => {
    if (trashMode) {
      return item
        ? [
            { label: '恢复', onClick: () => onRestore(item.name) },
            { separator: true },
            { label: '属性', onClick: () => setProperties(item) },
          ]
        : [
            { label: '清空回收站', danger: true, onClick: () => onEmptyTrash() },
            { separator: true },
            { label: '属性', onClick: () => setProperties('root') },
          ]
    }
    if (item) {
      const movable = !isProtected(item)
      const common: MenuItem[] = [
        { label: '剪切', shortcut: 'Ctrl+X', disabled: true },
        { label: '复制', shortcut: 'Ctrl+C', disabled: true },
      ]
      return [
        ...(item.kind === 'folder'
          ? ([
              { label: '打开', onClick: () => onOpen(item) },
              { label: '在终端中打开', onClick: onOpenTerminal },
              { separator: true },
            ] as MenuItem[])
          : ([
              { label: '用 Geany 打开', onClick: () => onOpen(item) },
              { separator: true },
            ] as MenuItem[])),
        ...common,
        { separator: true },
        {
          label: '重命名…',
          shortcut: 'F2',
          disabled: !movable,
          onClick: () => setDialog({ kind: 'rename', value: item.name, target: item.name }),
        },
        {
          label: '移动到回收站',
          shortcut: 'Delete',
          danger: true,
          disabled: !movable,
          onClick: () => onTrash(item.name),
        },
        { separator: true },
        { label: '压缩…', disabled: true },
        { label: '属性', shortcut: 'Ctrl+I', onClick: () => setProperties(item) },
      ]
    }
    return [
      { label: '新建文件夹', shortcut: 'Shift+Ctrl+N', onClick: () => setDialog({ kind: 'mkdir', value: '' }) },
      { separator: true },
      { label: '粘贴', shortcut: 'Ctrl+V', disabled: true },
      { separator: true },
      { label: '在终端中打开', onClick: onOpenTerminal },
      { separator: true },
      { label: '更改背景…', disabled: true },
      { label: '属性', onClick: () => setProperties('root') },
    ]
  }

  const pathbar = (
    <div className="flex items-center gap-0.5">
      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation()
          onNavigateDepth(0)
        }}
        className="rounded-md px-2.5 py-0.5 text-[12px] font-semibold text-[#3D3D3D] hover:bg-black/10"
        style={pathSegments.length === 0 && !trashMode ? { background: 'rgba(0,0,0,0.09)' } : undefined}
      >
        {trashMode ? '回收站' : homeName}
      </button>
      {!trashMode &&
        pathSegments.map((segment, index) => (
          <React.Fragment key={index}>
            <ChevronRight className="h-3 w-3 opacity-50" />
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation()
                onNavigateDepth(index + 1)
              }}
              className="flex items-center gap-0.5 rounded-md px-2.5 py-0.5 text-[12px] font-semibold text-[#3D3D3D] hover:bg-black/10"
              style={index === pathSegments.length - 1 ? { background: 'rgba(0,0,0,0.09)' } : undefined}
            >
              {segment}
              <span
                role="button"
                tabIndex={0}
                aria-label={`${segment} 的子目录`}
                onClick={(event) => {
                  event.stopPropagation()
                  const rect = (event.target as HTMLElement).getBoundingClientRect()
                  setCrumbMenu({ x: rect.left, y: rect.bottom + 4, depth: index + 1 })
                }}
                className="ml-0.5 text-[9px] opacity-60 hover:opacity-100"
              >
                ▾
              </span>
            </button>
          </React.Fragment>
        ))}
    </div>
  )

  // 与真实 xdg 目录一致的考生主目录位置；segments 即 VFS 路径。
  const places: { id: string; label: string; icon: React.ReactNode; segments: string[] | null }[] = [
    { id: 'recent', label: '最近', icon: <Clock3 className="h-4 w-4" />, segments: [] },
    { id: 'home', label: '主文件夹', icon: <img src={YARU_ICONS.userHome} alt="" className="h-4 w-4" />, segments: [] },
    { id: 'desktop', label: '桌面', icon: <img src={YARU_ICONS.userDesktop} alt="" className="h-4 w-4" />, segments: ['Desktop'] },
    { id: 'documents', label: '文档', icon: <img src={YARU_ICONS.folder} alt="" className="h-4 w-4" />, segments: ['Documents'] },
    { id: 'downloads', label: '下载', icon: <Download className="h-4 w-4" />, segments: ['Downloads'] },
    { id: 'music', label: '音乐', icon: <Music className="h-4 w-4" />, segments: ['Music'] },
    { id: 'pictures', label: '图片', icon: <ImageIcon className="h-4 w-4" />, segments: ['Pictures'] },
    { id: 'videos', label: '视频', icon: <Video className="h-4 w-4" />, segments: ['Videos'] },
    { id: 'trash', label: '回收站', icon: <Trash2 className="h-4 w-4" />, segments: null },
  ]
  const activePlaceId = trashMode
    ? 'trash'
    : places.find((pl) => pl.segments?.length === pathSegments.length && pl.segments?.every((s, i) => s === pathSegments[i]))?.id ?? 'home'

  return (
    <div
      className="flex h-full w-full flex-col overflow-hidden rounded-xl border border-black/40 shadow-[0_14px_48px_rgba(0,0,0,0.5)]"
      style={{ fontFamily: YARU_SANS }}
    >
      <YaruHeaderBar
        onMouseDown={onMouseDown}
        onMinimize={onMinimize}
        onMaximize={onMaximize}
        onClose={onClose}
        left={pathbar}
        actions={
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setView(view === 'grid' ? 'list' : 'grid')}
              className="grid h-7 w-7 place-items-center rounded-md text-[#3D3D3D] hover:bg-black/10"
              aria-label={view === 'grid' ? '切换到列表视图' : '切换到网格视图'}
              title={view === 'grid' ? '列表视图' : '网格视图'}
            >
              {view === 'grid' ? <List className="h-4 w-4" /> : <LayoutGrid className="h-4 w-4" />}
            </button>
            <button
              type="button"
              className="grid h-7 w-7 place-items-center rounded-md text-[#3D3D3D] hover:bg-black/10"
              aria-label="搜索"
            >
              <Search className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={(event) => {
                const rect = event.currentTarget.getBoundingClientRect()
                setAppMenu({ x: Math.min(rect.left - 120, window.innerWidth - 240), y: rect.bottom + 4 })
              }}
              className="grid h-7 w-7 place-items-center rounded-md text-[#3D3D3D] hover:bg-black/10"
              aria-label="菜单"
            >
              <MenuIcon className="h-4 w-4" />
            </button>
          </div>
        }
      />
      <div className="flex min-h-0 flex-1">
        {/* Places 侧栏 */}
        <div
          className="w-44 shrink-0 overflow-y-auto border-r py-2"
          style={{ background: YARU.sidebarBg, borderColor: YARU.sidebarBorder }}
        >
          {places.map((place) => {
            const active = place.id === activePlaceId
            return (
              <button
                key={place.id}
                type="button"
                onClick={() => {
                  onSelect(null)
                  if (place.segments === null) onNavigateDepth(-1)
                  else onNavigatePath(place.segments)
                }}
                className="mx-1.5 flex h-[30px] w-[calc(100%-12px)] items-center gap-2 rounded-md px-2.5 text-left text-[13px]"
                style={
                  active
                    ? { background: YARU.accent, color: '#FFFFFF' }
                    : { color: '#3D3D3D' }
                }
                onMouseEnter={(event) => {
                  if (!active) event.currentTarget.style.background = YARU.hoverFill
                }}
                onMouseLeave={(event) => {
                  if (!active) event.currentTarget.style.background = 'transparent'
                }}
              >
                <span className="grid w-4 place-items-center">{place.icon}</span>
                {place.label}
              </button>
            )
          })}
        </div>

        {/* 图标/列表视图 */}
        <div
          className="relative min-h-0 flex-1 overflow-y-auto"
          style={{ background: YARU.contentBg }}
          onMouseDown={() => onSelect(null)}
          onContextMenu={(event) => openMenu(event, null)}
        >
          <div
            className={
              view === 'grid'
                ? `grid grid-cols-[repeat(auto-fill,minmax(104px,1fr))] gap-2 p-4${pathSegments.length !== 0 && !trashMode && sortedItems.length === 0 ? ' hidden' : ''}`
                : `flex flex-col p-2${pathSegments.length !== 0 && !trashMode && sortedItems.length === 0 ? ' hidden' : ''}`
            }
          >
            {sortedItems.map((item) => {
              const selected = item.id === selectedId
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={(event) => {
                    event.stopPropagation()
                    onSelect(item.id)
                  }}
                  onDoubleClick={() => onOpen(item)}
                  onContextMenu={(event) => openMenu(event, item)}
                  className="flex flex-col items-center gap-1 rounded-lg px-1.5 py-2.5 outline-none"
                  style={selected ? { background: 'rgba(233,84,32,0.13)' } : undefined}
                >
                  <img
                    src={item.kind === 'folder' ? YARU_ICONS.folder : YARU_ICONS.textGeneric}
                    alt=""
                    className="h-[52px] w-[52px] object-contain"
                    draggable={false}
                  />
                  <span
                    className={
                      view === 'grid'
                        ? 'max-w-[96px] truncate rounded px-1.5 py-0.5 text-center text-[12px] leading-4'
                        : 'hidden'
                    }
                    style={
                      selected && view === 'grid'
                        ? { background: YARU.accent, color: '#FFFFFF' }
                        : { color: '#2D2D2D' }
                    }
                  >
                    {item.name}
                  </span>
                </button>
              )
            })}
          </div>
          {view === 'list' && (sortedItems.length > 0 || trashMode) && (
            <div className="mx-1 border-b" style={{ borderColor: '#EDEDED' }}>
              <div className="flex items-center gap-2 px-2 py-1 text-[11px] font-bold text-[#8F8F8F]">
                <span className="w-5" />
                <span className="flex-1">名称</span>
                <span className="w-20 text-right">大小</span>
              </div>
              {sortedItems.map((item) => {
                const selected = item.id === selectedId
                return (
                  <button
                    key={`row-${item.id}`}
                    type="button"
                    onClick={(event) => {
                      event.stopPropagation()
                      onSelect(item.id)
                    }}
                    onDoubleClick={() => onOpen(item)}
                    onContextMenu={(event) => openMenu(event, item)}
                    className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-[13px]"
                    style={
                      selected
                        ? { background: YARU.accent, color: '#FFFFFF' }
                        : { color: '#2D2D2D' }
                    }
                  >
                    <img
                      src={item.kind === 'folder' ? YARU_ICONS.folder : YARU_ICONS.textGeneric}
                      alt=""
                      className="h-5 w-5 object-contain"
                      draggable={false}
                    />
                    <span className="flex-1 truncate" style={{ fontFamily: item.kind === 'source' ? YARU_MONO : undefined }}>
                      {item.name}
                    </span>
                    <span className={`w-20 text-right text-[11px] ${selected ? 'text-white/80' : 'text-[#8F8F8F]'}`}>
                      {item.kind === 'source' ? formatBytes(item.sizeBytes ?? 0) : ''}
                    </span>
                  </button>
                )
              })}
            </div>
          )}
          {sortedItems.length === 0 && (
            <p className="grid place-items-center py-16 text-[13px] text-slate-400">
              {trashMode
                ? '回收站是空的'
                : '此文件夹为空'}
            </p>
          )}
        </div>
      </div>

      {/* 新建文件夹 / 重命名 对话框 */}
      {dialog && (
        <div
          className="absolute inset-0 z-20 grid place-items-center bg-black/30"
          onMouseDown={() => setDialog(null)}
        >
          <div
            className="w-80 overflow-hidden rounded-xl border border-black/30 shadow-2xl"
            style={{ background: YARU.contentBg, fontFamily: YARU_SANS }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div
              className="flex h-9 items-center justify-center text-[13px] font-bold text-[#2D2D2D]"
              style={{ background: YARU.headerbarBg, borderBottom: `1px solid ${YARU.headerbarBorder}` }}
            >
              {dialog.kind === 'mkdir' ? '新建文件夹' : `重命名「${dialog.target}」`}
            </div>
            <form
              className="space-y-3 px-4 py-4"
              onSubmit={(event) => {
                event.preventDefault()
                const value = dialog.value.trim()
                const error =
                  dialog.kind === 'mkdir'
                    ? onNewFolder(value)
                    : dialog.target
                      ? onRenamePath(dialog.target, value)
                      : '名称不能为空'
                if (error) {
                  setDialog({ ...dialog, error })
                  return
                }
                setDialog(null)
              }}
            >
              <input
                autoFocus
                value={dialog.value}
                onChange={(event) => setDialog({ ...dialog, value: event.target.value, error: undefined })}
                placeholder="文件夹名称"
                aria-label={dialog.kind === 'mkdir' ? '文件夹名称' : '新名称'}
                className="w-full rounded-lg border border-[#CFCFCF] bg-white px-3 py-2 text-[13px] text-[#2D2D2D] outline-none focus:border-[#E95420]"
              />
              {dialog.error && (
                <p className="text-[11px] font-semibold text-rose-600">{dialog.error}</p>
              )}
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setDialog(null)}
                  className="h-7 rounded-md border border-[#CFCFCF] bg-white px-3 text-[12px] font-bold text-[#3D3D3D] hover:bg-[#F0F0EE]"
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="h-7 rounded-md px-3 text-[12px] font-bold text-white hover:opacity-90"
                  style={{ background: YARU.accent }}
                >
                  {dialog.kind === 'mkdir' ? '创建' : '重命名'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {properties !== null && (
        <div
          className="absolute inset-0 z-20 grid place-items-center bg-black/30"
          onMouseDown={() => setProperties(null)}
        >
          <div
            className="w-80 overflow-hidden rounded-xl border border-black/30 shadow-2xl"
            style={{ background: YARU.contentBg, fontFamily: YARU_SANS }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div
              className="flex h-9 items-center justify-center text-[13px] font-bold text-[#2D2D2D]"
              style={{ background: YARU.headerbarBg, borderBottom: `1px solid ${YARU.headerbarBorder}` }}
            >
              {properties === 'root' ? '属性 — 主文件夹' : `属性 — ${(properties as NautilusItem).name}`}
            </div>
            <div className="space-y-2 px-4 py-3 text-[12px] text-[#3D3D3D]">
              {(
                properties === 'root'
                  ? [
                      ['名称', homeName],
                      ['类型', '文件夹'],
                      ['位置', '/home'],
                    ]
                  : [
                      ['名称', (properties as NautilusItem).name],
                      [
                        '类型',
                        (properties as NautilusItem).kind === 'folder'
                          ? '文件夹'
                          : 'C++ 源文件 (text/x-c++src)',
                      ],
                      [
                        '大小',
                        (properties as NautilusItem).kind === 'source'
                          ? `${(properties as NautilusItem).sizeBytes ?? 0} 字节`
                          : '—',
                      ],
                      ['位置', `/home/${homeName}${pathSegments.length ? `/${pathSegments.join('/')}` : ''}`],
                    ]
              )
                .filter(([, value]) => value !== undefined)
                .map(([label, value]) => (
                  <div key={label} className="flex gap-3">
                    <span className="w-14 shrink-0 text-[#8F8F8F]">{label}</span>
                    <span className="min-w-0 flex-1 break-all" style={{ fontFamily: YARU_MONO }}>
                      {value}
                    </span>
                  </div>
                ))}
            </div>
            <div className="flex justify-end px-4 pb-3">
              <button
                type="button"
                onClick={() => setProperties(null)}
                className="h-7 rounded-md border border-[#CFCFCF] bg-white px-3 text-[12px] font-bold text-[#3D3D3D] hover:bg-[#F0F0EE]"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}

      {menu && (
        <ContextMenu x={menu.x} y={menu.y} items={menuItems(menu.item)} onClose={() => setMenu(null)} />
      )}
      {appMenu && (
        <ContextMenu
          x={appMenu.x}
          y={appMenu.y}
          items={[
            { label: '网格视图', onClick: () => setView('grid') },
            { label: '列表视图', onClick: () => setView('list') },
            { separator: true },
            { label: '按名称排序', onClick: () => setSortBy('name') },
            { label: '按大小排序', onClick: () => setSortBy('size') },
            { label: '按类型排序', onClick: () => setSortBy('type') },
            { separator: true },
            { label: sortAsc ? '升序' : '降序', onClick: () => setSortAsc((asc) => !asc) },
          ]}
          onClose={() => setAppMenu(null)}
        />
      )}
      {crumbMenu && (
        <ContextMenu
          x={crumbMenu.x}
          y={crumbMenu.y}
          items={(breadcrumbChildren?.(pathSegments.slice(0, crumbMenu.depth)) ?? []).map(
            (child): MenuItem => ({
              label: child,
              onClick: () => onNavigatePath([...pathSegments.slice(0, crumbMenu.depth), child]),
            })
          )}
          onClose={() => setCrumbMenu(null)}
        />
      )}
    </div>
  )
}

// ---------------------------------------------------------------- Geany

export interface GeanySymbol {
  name: string
  line: number
}

// 粗粒度符号解析：与 Geany 的 Ctags 不同，这里只抓函数/类名用于侧栏演示。
export function parseGeanySymbols(code: string): GeanySymbol[] {
  const out: GeanySymbol[] = []
  code.split('\n').forEach((line, index) => {
    const fn = line.match(/^\s*(?:[\w:*&]+\s+)+?(\w+)\s*\([^;]*\)\s*(?:const\s*)?\{?\s*$/)
    const cls = line.match(/^\s*(?:class|struct)\s+(\w+)/)
    if (cls) out.push({ name: `class ${cls[1]}`, line: index + 1 })
    else if (fn && !/\b(if|for|while|switch|return|catch)\b/.test(fn[0])) {
      out.push({ name: fn[1] + '()', line: index + 1 })
    }
  })
  return out
}

export function GeanyWindow({
  dirName,
  fileName,
  homeName,
  code,
  onCode,
  readOnly,
  compilerLog,
  onCompile,
  onRun,
  onOpenFiles,
  onSave,
  onNewDoc,
  onSaveAs,
  tabs,
  activeTabId,
  onSelectTab,
  onCloseDoc,
  canSubmit = true,
  onPickUploadFile,
  uploadFile,
  uploadMismatch,
  onClearUploadFile,
  onSubmit,
  submitting,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  dirName: string
  fileName: string
  homeName: string
  code: string
  onCode: (value: string) => void
  readOnly: boolean
  compilerLog: CommandLog[]
  onCompile: () => void
  onRun: () => void
  onOpenFiles: () => void
  onSave: () => void
  /** 新建未命名文档（落在本场桌面的文件夹里）。 */
  onNewDoc: () => void
  /** 另存为：返回 null = 成功；字符串 = 对话框内联错误（重名等）。 */
  onSaveAs?: (newName: string) => string | null
  /** 打开的文档标签：题目源文件 / 已保存自建文件 / 未保存的 无标题 N。 */
  tabs: { id: string; name: string; kind: 'problem' | 'vfs' | 'untitled' }[]
  activeTabId: string | null
  onSelectTab: (id: string) => void
  onCloseDoc?: (id: string) => void
  /** 提交按钮只对绑定题目的文档生效。 */
  canSubmit?: boolean
  onPickUploadFile: () => void
  uploadFile: { name: string; size: number } | null
  uploadMismatch: string | null
  onClearUploadFile: () => void
  onSubmit: () => void
  submitting: boolean
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)
  const [menubarMenu, setMenubarMenu] = useState<{ x: number; y: number; label: string } | null>(null)
  const [messageTab, setMessageTab] = useState<'status' | 'compiler' | 'msgs' | 'notes'>('status')
  const [saveAsDialog, setSaveAsDialog] = useState<{ value: string; error?: string } | null>(null)
  const [messageHidden, setMessageHidden] = useState(false)
  const [sidebarHidden, setSidebarHidden] = useState(false)
  const [notes, setNotes] = useState('')
  const [aboutOpen, setAboutOpen] = useState(false)
  const activeDocKind = tabs.find((t) => t.id === activeTabId)?.kind
  const activeDocName = tabs.find((t) => t.id === activeTabId)?.name ?? ''
  const [sidebarTab, setSidebarTab] = useState<'symbols' | 'docs'>('symbols')
  const editorRef = useRef<GeanyEditorHandle | null>(null)
  const [caret, setCaret] = useState({ line: 1, column: 1 })
  const symbols = parseGeanySymbols(code)
  const hasDoc = tabs.length > 0 && activeTabId != null

  const runEditor = (cmd: string) => editorRef.current?.trigger('geany', cmd, null)

  // 真实 Geany 的肌肉记忆：F8 编译、F5 运行、Ctrl+S 显式保存（对本窗口全局生效）。
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'F8') {
        event.preventDefault()
        if (hasDoc) onCompile()
      } else if (event.key === 'F5') {
        event.preventDefault()
        onRun()
      } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
        event.preventDefault()
        doSave()
      } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'n') {
        event.preventDefault()
        onNewDoc()
      } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'w') {
        event.preventDefault()
        if (activeTabId) onCloseDoc?.(activeTabId)
      } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'j') {
        event.preventDefault()
        setMessageHidden((hidden) => !hidden)
      } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'b') {
        event.preventDefault()
        setSidebarHidden((hidden) => !hidden)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // 处理器均随渲染重建；以语义依赖（回调与文档状态）为准，避免每键重挂。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onCompile, onRun, onSave, onNewDoc, onCloseDoc, activeTabId, hasDoc])

  const doCompile = () => {
    setMessageTab('compiler')
    onCompile()
  }

  // 真实 Geany 保存语义：静默，无提示框；未命名文档首次保存弹出命名对话框。
  const doSave = () => {
    if (!hasDoc) return
    if (activeDocKind === 'untitled') {
      setSaveAsDialog({ value: '无标题.cpp' })
      return
    }
    onSave()
  }

  const jumpToLine = (line: number) => {
    editorRef.current?.reveal?.(line)
    editorRef.current?.trigger('geany', 'unfold', null)
  }

  const insertDate = () => {
    const stamp = new Date().toLocaleDateString('zh-CN')
    onCode(code + `\n// ${stamp}\n`)
  }

  // 编辑区右键菜单与「编辑」菜单共用同一组真实命令。
  const menuItems: MenuItem[] = [
    { label: '撤销', shortcut: 'Ctrl+Z', disabled: readOnly, onClick: () => runEditor('undo') },
    { label: '重做', shortcut: 'Ctrl+Y', disabled: readOnly, onClick: () => runEditor('redo') },
    { separator: true },
    { label: '剪切', shortcut: 'Ctrl+X', disabled: readOnly, onClick: () => runEditor('editor.action.clipboardCutAction') },
    { label: '复制', shortcut: 'Ctrl+C', onClick: () => runEditor('editor.action.clipboardCopyAction') },
    { label: '粘贴', shortcut: 'Ctrl+V', disabled: readOnly, onClick: () => runEditor('editor.action.clipboardPasteAction') },
    { separator: true },
    { label: '全选', shortcut: 'Ctrl+A', onClick: () => runEditor('selectAll') },
    { separator: true },
    { label: '插入日期', disabled: readOnly, onClick: insertDate },
    { label: '打开所在文件夹', onClick: onOpenFiles },
    { separator: true },
    { label: '首选项…', disabled: true },
  ]

  // 菜单栏下拉（Geany 真实菜单结构；未接后端的项保持视觉一致）。
  const menubarMenus: Record<string, MenuItem[]> = {
    文件: [
      { label: '新建', shortcut: 'Ctrl+N', onClick: onNewDoc },
      { label: '打开…', shortcut: 'Ctrl+O', onClick: onPickUploadFile },
      { label: '保存', shortcut: 'Ctrl+S', onClick: doSave, disabled: !hasDoc },
      {
        label: '另存为…',
        shortcut: 'Shift+Ctrl+S',
        disabled: !hasDoc || activeDocKind === 'problem',
        onClick: () =>
          setSaveAsDialog({ value: activeDocKind === 'untitled' ? '无标题.cpp' : activeDocName }),
      },
      { separator: true },
      {
        label: '关闭',
        shortcut: 'Ctrl+W',
        onClick: () => {
          if (activeTabId) onCloseDoc?.(activeTabId)
        },
      },
      { label: '退出', shortcut: 'Ctrl+Q', onClick: onClose },
    ],
    编辑: menuItems,
    搜索: [
      { label: '查找…', shortcut: 'Ctrl+F', onClick: () => runEditor('actions.find') },
      { label: '替换…', shortcut: 'Ctrl+H', onClick: () => runEditor('editor.action.startFindReplaceAction') },
    ],
    查看: [
      {
        label: messageHidden ? '显示消息窗口' : '切换消息窗口',
        shortcut: 'Ctrl+J',
        onClick: () => setMessageHidden((hidden) => !hidden),
      },
      {
        label: sidebarHidden ? '显示侧边栏' : '切换侧边栏',
        shortcut: 'Ctrl+B',
        onClick: () => setSidebarHidden((hidden) => !hidden),
      },
      { separator: true },
      { label: '全屏', shortcut: 'F11', onClick: onMaximize },
    ],
    文档: [{ label: hasDoc ? `${fileName} (UTF-8)` : '未打开文档', disabled: true }],
    项目: [{ label: '未打开项目', disabled: true }],
    构建: [
      { label: '编译', shortcut: 'F8', onClick: doCompile, disabled: !hasDoc },
      { label: '构建', shortcut: 'F9', onClick: doCompile, disabled: !hasDoc },
      { separator: true },
      { label: '运行', shortcut: 'F5', onClick: onRun },
    ],
    工具: [{ label: '插入日期', disabled: readOnly, onClick: insertDate }],
    帮助: [{ label: '关于 Geany', onClick: () => setAboutOpen(true) }],
  }

  return (
    <div
      className="relative flex h-full w-full flex-col overflow-hidden rounded-xl border border-black/40 shadow-[0_14px_48px_rgba(0,0,0,0.5)]"
      style={{ fontFamily: YARU_SANS }}
    >
      <YaruTitleBar
        title={
          hasDoc
            ? `${fileName} - /home/${homeName}${dirName ? `/${dirName}` : ''} - Geany`
            : 'Geany'
        }
        onMouseDown={onMouseDown}
        onMinimize={onMinimize}
        onMaximize={onMaximize}
        onClose={onClose}
      />
      {/* 菜单栏 */}
      <div className="flex h-[26px] shrink-0 items-stretch border-b border-[#D9D9D6]" style={{ background: YARU.chromeBg }}>
        {Object.keys(menubarMenus).map((label) => (
          <button
            key={label}
            type="button"
            onClick={(event) => {
              const rect = event.currentTarget.getBoundingClientRect()
              setMenubarMenu({ x: rect.left, y: rect.bottom + 2, label })
            }}
            className="px-2.5 text-[12px] font-semibold text-[#3D3D3D] hover:bg-[#E95420] hover:text-white"
          >
            {label}
          </button>
        ))}
      </div>
      {/* 工具栏 */}
      <div className="flex h-9 shrink-0 items-center gap-0.5 border-b border-[#D9D9D6] px-1.5" style={{ background: YARU.chromeBg }}>
        {[
          { icon: <FileText className="h-4 w-4" />, label: '新建 (Ctrl+N)', onClick: onNewDoc },
          { icon: <FolderOpen className="h-4 w-4" />, label: '打开', onClick: onPickUploadFile },
          { icon: <Download className="h-4 w-4 rotate-180" />, label: '保存 (Ctrl+S)', onClick: doSave, disabled: !hasDoc },
          { separator: true },
          { icon: <Play className="h-4 w-4 text-emerald-700" />, label: '编译 (F8)', onClick: doCompile, disabled: !hasDoc },
          { icon: <Play className="h-4 w-4 text-blue-700" />, label: '在终端中运行 (F5)', onClick: onRun },
          { separator: true },
          { icon: <Search className="h-4 w-4" />, label: '查找 (Ctrl+F)', onClick: () => runEditor('actions.find') },
        ].map((btn, index) =>
          btn.separator ? (
            <div key={index} className="mx-1.5 h-5 w-px bg-[#D9D9D6]" />
          ) : (
            <button
              key={index}
              type="button"
              title={btn.label}
              onClick={btn.onClick}
              disabled={btn.disabled}
              className="grid h-7 w-7 place-items-center rounded text-[#3D3D3D] hover:bg-black/10 disabled:opacity-40"
            >
              {btn.icon}
            </button>
          )
        )}
        <div className="ml-auto flex items-center gap-1.5">
          {uploadFile && (
            <span
              className="inline-flex h-6 items-center gap-1 rounded border border-emerald-300 bg-emerald-50 px-1.5 text-[10px] font-bold text-emerald-800"
              style={{ fontFamily: YARU_MONO }}
            >
              {uploadFile.name} · {formatBytes(uploadFile.size)}
              <button
                type="button"
                onClick={onClearUploadFile}
                aria-label="取消已选文件"
                className="text-emerald-500 transition hover:text-emerald-800"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          )}
          <button
            type="button"
            onClick={onPickUploadFile}
            className="h-6 rounded border border-[#CFCFCF] bg-white px-2 text-[10px] font-bold text-[#3D3D3D] hover:bg-[#F0F0EE]"
          >
            选择文件
          </button>
          <button
            type="button"
            onClick={onSubmit}
            disabled={submitting || !canSubmit || !hasDoc}
            title={canSubmit ? undefined : '自建文件不参与收题：请打开题面文件夹里的源文件提交'}
            className="h-6 rounded bg-[#2C64C5] px-2.5 text-[10px] font-bold text-white hover:bg-[#1F4FA5] disabled:opacity-50"
          >
            {submitting ? '提交中...' : '正式提交'}
          </button>
        </div>
      </div>

      {/* 主体：侧栏 + 编辑器 */}
      <div className="flex min-h-0 flex-1">
        <div
          className={`flex w-44 shrink-0 flex-col border-r border-[#D9D9D6] ${
            sidebarHidden ? 'hidden' : ''
          }`}
          style={{ background: YARU.sidebarBg }}
        >
          <div className="flex h-7 shrink-0 items-stretch border-b border-[#D9D9D6]">
            {([
              ['symbols', '符号'],
              ['docs', '文档'],
            ] as const).map(([id, label]) => (
              <button
                key={id}
                type="button"
                onClick={() => setSidebarTab(id)}
                className="px-3 text-[11px] font-bold"
                style={{
                  background: sidebarTab === id ? YARU.contentBg : 'transparent',
                  color: '#3D3D3D',
                  borderRight: '1px solid #D9D9D6',
                  borderBottom: sidebarTab === id ? 'none' : '1px solid #D9D9D6',
                }}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto py-1 text-[12px] text-[#3D3D3D]">
            {sidebarTab === 'symbols' ? (
              hasDoc && symbols.length ? (
                symbols.map((symbol, index) => (
                  <button
                    key={index}
                    type="button"
                    onClick={() => jumpToLine(symbol.line)}
                    className="flex w-full items-center gap-1.5 px-3 py-0.5 text-left hover:bg-black/5"
                  >
                    <span className="text-[10px] font-bold text-purple-700">ƒ</span>
                    <span className="truncate" style={{ fontFamily: YARU_MONO }}>
                      {symbol.name}
                    </span>
                    <span className="ml-auto text-[10px] text-slate-400">:{symbol.line}</span>
                  </button>
                ))
              ) : (
                <p className="px-3 py-2 text-[11px] text-slate-400">没有标签</p>
              )
            ) : tabs.length ? (
              tabs.map((entry) => (
                <button
                  key={entry.id}
                  type="button"
                  onClick={() => onSelectTab(entry.id)}
                  className="flex w-full items-center gap-1.5 px-3 py-0.5 text-left hover:bg-black/5"
                  style={
                    entry.id === activeTabId
                      ? { background: 'rgba(233,84,32,0.14)' }
                      : undefined
                  }
                >
                  <img src={YARU_ICONS.textGeneric} alt="" className="h-3.5 w-3.5" />
                  <span className="truncate" style={{ fontFamily: YARU_MONO }}>
                    {entry.name}
                  </span>
                </button>
              ))
            ) : (
              <p className="px-3 py-2 text-[11px] text-slate-400">没有打开的文档</p>
            )}
          </div>
        </div>

        <div className="flex min-w-0 flex-1 flex-col" style={{ background: YARU.contentBg }}>
          {/* 编辑器标签页：一题或一个自建文件一个标签 */}
          <div className="flex h-7 shrink-0 items-stretch overflow-x-auto border-b border-[#D9D9D6]" style={{ background: '#EFEFED' }}>
            {tabs.map((entry) => {
              const active = entry.id === activeTabId
              return (
                <div
                  key={entry.id}
                  className={`flex shrink-0 items-center gap-1.5 border-r border-[#D9D9D6] px-3 text-[12px] ${
                    active ? '' : 'hover:bg-black/5'
                  }`}
                  style={{ background: active ? YARU.contentBg : 'transparent', color: '#2D2D2D' }}
                >
                  <button type="button" onClick={() => onSelectTab(entry.id)} className="flex items-center gap-1.5">
                    <img src={YARU_ICONS.textGeneric} alt="" className="h-3.5 w-3.5" />
                    <span style={{ fontFamily: YARU_MONO }}>{entry.name}</span>
                  </button>
                  <button
                    type="button"
                    aria-label={`关闭 ${entry.name}`}
                    onClick={(event) => {
                      event.stopPropagation()
                      onCloseDoc?.(entry.id)
                    }}
                    className="opacity-50 transition hover:opacity-100"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </div>
              )
            })}
          </div>
          <div
            className="min-h-0 flex-1"
            onContextMenu={(event) => {
              event.preventDefault()
              setMenu({ x: event.clientX, y: event.clientY })
            }}
          >
            {hasDoc ? (
              <MonacoSlot
                code={code}
                onCode={onCode}
                readOnly={readOnly}
                onReady={(editor) => {
                  editorRef.current = editor
                }}
                onCursor={(line, column) => setCaret({ line, column })}
              />
            ) : (
              <div className="grid h-full place-items-center text-[13px] text-slate-400">
                没有打开的文档 —— 从「文件」打开，或在文件管理器中双击源文件。
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 消息窗口 */}
      <div
        className={`h-36 shrink-0 border-t border-[#D9D9D6] ${messageHidden ? 'hidden' : ''}`}
        style={{ background: YARU.contentBg }}
      >
        <div className="flex h-7 items-stretch border-b border-[#D9D9D6]" style={{ background: '#EFEFED' }}>
          {([
            ['status', '状态栏'],
            ['compiler', '编译器'],
            ['msgs', '消息'],
            ['notes', '便签'],
          ] as const).map(([id, label]) => (
            <button
              key={id}
              type="button"
              onClick={() => setMessageTab(id)}
              className="px-3 text-[11px] font-bold"
              style={{
                background: messageTab === id ? YARU.contentBg : 'transparent',
                color: '#3D3D3D',
                borderRight: '1px solid #D9D9D6',
              }}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="h-[calc(100%-1.75rem)] overflow-y-auto p-2 text-[12px] leading-5" style={{ fontFamily: YARU_MONO }}>
          {messageTab === 'status' && (
            <p className="text-[#3D3D3D]">
              {hasDoc ? `${fileName} 已载入。${readOnly ? '（只读文件）' : ''}` : '没有打开的文档。'}
            </p>
          )}
          {messageTab === 'compiler' && (
            <div>
              {compilerLog.length === 0 ? (
                <p className="text-slate-400">点击工具栏「编译 (F8)」对源文件做静态预检（浏览器内无真实编译器）。</p>
              ) : (
                compilerLog.map((line, index) => (
                  <p
                    key={index}
                    onClick={() => {
                      if (line.line) jumpToLine(line.line)
                    }}
                    className={
                      line.type === 'error'
                        ? 'font-bold'
                        : line.type === 'success'
                          ? 'text-emerald-700'
                          : 'text-[#3D3D3D]'
                    }
                    style={{
                      color: line.type === 'error' ? YARU.errorRed : undefined,
                      cursor: line.line ? 'pointer' : undefined,
                      textDecoration: line.line ? 'underline dotted' : undefined,
                    }}
                  >
                    {line.text}
                  </p>
                ))
              )}
            </div>
          )}
          {messageTab === 'msgs' && <p className="text-slate-400">没有消息。</p>}
          {messageTab === 'notes' && (
            <textarea
              value={notes}
              onChange={(event) => setNotes(event.target.value)}
              placeholder="此窗口用于记录临时笔记（考场公式、思路、卡住的测试点…）"
              spellCheck={false}
              className="h-full min-h-0 w-full resize-none border-0 bg-transparent p-0 font-mono text-[12px] leading-5 text-[#3D3D3D] outline-none placeholder:text-slate-300"
            />
          )}
        </div>
      </div>

      {/* 状态栏 */}
      <div
        className="flex h-[22px] shrink-0 items-center justify-end gap-0 border-t border-[#D9D9D6] px-2 text-[11px] text-[#3D3D3D]"
        style={{ background: YARU.chromeBg, fontFamily: YARU_MONO }}
      >
        {[
          `行: ${caret.line}`,
          `列: ${caret.column}`,
          '缩进: 空格',
          'TAB 宽度: 4',
          '编码: UTF-8',
          '换行: LF',
          `文件类型: ${fileTypeName(fileName)}`,
        ].map((part, index) => (
          <span key={index} className="border-l border-[#D9D9D6] px-2 first:border-l-0">
            {part}
          </span>
        ))}
      </div>

      {uploadMismatch && uploadFile && (
        <div className="absolute bottom-[10.2rem] left-0 right-0 border-t border-amber-300 bg-amber-50 px-3 py-1 text-[11px] font-semibold leading-4 text-amber-800">
          文件名与题面英文名不一致：本题目录为{' '}
          <span className="font-mono">{uploadMismatch}</span>，当前选择{' '}
          <span className="font-mono">{uploadFile.name}</span>
          。真实考场按文件名/目录对应题目，请确认没有选错文件。
        </div>
      )}

      {aboutOpen && (
        <div
          className="absolute inset-0 z-20 grid place-items-center bg-black/30"
          onMouseDown={() => setAboutOpen(false)}
        >
          <div
            className="w-80 overflow-hidden rounded-xl border border-black/30 shadow-2xl"
            style={{ background: YARU.contentBg, fontFamily: YARU_SANS }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div
              className="flex h-9 items-center justify-center text-[13px] font-bold text-[#2D2D2D]"
              style={{ background: YARU.headerbarBg, borderBottom: `1px solid ${YARU.headerbarBorder}` }}
            >
              关于 Geany
            </div>
            <div className="flex flex-col items-center gap-2 px-6 py-5 text-center">
              <img src={YARU_ICONS.geany} alt="" className="h-14 w-14 object-contain" />
              <p className="text-[15px] font-extrabold text-[#2D2D2D]">Geany 1.36</p>
              <p className="text-[11px] leading-5 text-slate-500">
                NOI Linux 2.0 模拟环境 · 以 Yaru 主题在浏览器中复刻。
                <br />
                静态预检不等于真实编译；最终评测以服务端为准。
              </p>
            </div>
            <div className="flex justify-end px-4 pb-3">
              <button
                type="button"
                onClick={() => setAboutOpen(false)}
                className="h-7 rounded-md border border-[#CFCFCF] bg-white px-3 text-[12px] font-bold text-[#3D3D3D] hover:bg-[#F0F0EE]"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
      {saveAsDialog !== null && (
        <div
          className="absolute inset-0 z-20 grid place-items-center bg-black/30"
          onMouseDown={() => setSaveAsDialog(null)}
        >
          <div
            className="w-80 overflow-hidden rounded-xl border border-black/30 shadow-2xl"
            style={{ background: YARU.contentBg, fontFamily: YARU_SANS }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div
              className="flex h-9 items-center justify-center text-[13px] font-bold text-[#2D2D2D]"
              style={{ background: YARU.headerbarBg, borderBottom: `1px solid ${YARU.headerbarBorder}` }}
            >
              {activeDocKind === 'untitled' ? '保存文件' : '另存为'}
            </div>
            <form
              className="space-y-3 px-4 py-4"
              onSubmit={(event) => {
                event.preventDefault()
                const name = saveAsDialog.value.trim()
                if (!name) return
                const error = onSaveAs?.(name) ?? null
                if (error) {
                  setSaveAsDialog({ value: saveAsDialog.value, error })
                  return
                }
                setSaveAsDialog(null)
              }}
            >
              <input
                autoFocus
                value={saveAsDialog.value}
                onChange={(event) => setSaveAsDialog({ value: event.target.value, error: undefined })}
                placeholder="文件名（如 sol.cpp）"
                aria-label="另存为文件名"
                className="w-full rounded-lg border border-[#CFCFCF] bg-white px-3 py-2 text-[13px] text-[#2D2D2D] outline-none focus:border-[#E95420]"
              />
              {saveAsDialog.error && (
                <p className="text-[11px] font-semibold text-rose-600">{saveAsDialog.error}</p>
              )}
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setSaveAsDialog(null)}
                  className="h-7 rounded-md border border-[#CFCFCF] bg-white px-3 text-[12px] font-bold text-[#3D3D3D] hover:bg-[#F0F0EE]"
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="h-7 rounded-md px-3 text-[12px] font-bold text-white hover:opacity-90"
                  style={{ background: YARU.accent }}
                >
                  保存
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
      {menubarMenu && (
        <ContextMenu
          x={menubarMenu.x}
          y={menubarMenu.y}
          items={menubarMenus[menubarMenu.label]}
          onClose={() => setMenubarMenu(null)}
        />
      )}
    </div>
  )
}

// Monaco 由 UbuntuDesktop 的懒加载模块注入，避免重复打包。
let MonacoSlotImpl: React.FC<{
  code: string
  onCode: (value: string) => void
  readOnly: boolean
  onReady: (editor: GeanyEditorHandle) => void
  onCursor: (line: number, column: number) => void
}> | null = null

export interface GeanyEditorHandle {
  trigger: (src: string, cmd: string, payload?: unknown) => void
  getPosition?: () => { lineNumber: number; column: number }
  /** 跳到指定源码行（符号面板 / 编译诊断点击）。 */
  reveal?: (line: number) => void
}

export function registerMonacoSlot(
  impl: React.FC<{
    code: string
    onCode: (value: string) => void
    readOnly: boolean
    onReady: (editor: GeanyEditorHandle) => void
    onCursor: (line: number, column: number) => void
  }>
) {
  MonacoSlotImpl = impl
}

function MonacoSlot(props: {
  code: string
  onCode: (value: string) => void
  readOnly: boolean
  onReady: (editor: GeanyEditorHandle) => void
  onCursor: (line: number, column: number) => void
}) {
  if (!MonacoSlotImpl) return <div className="h-full w-full bg-white" />
  const Impl = MonacoSlotImpl
  return <Impl {...props} />
}

// ---------------------------------------------------------------- 终端

export function TerminalWindow({
  homeName,
  path,
  logs,
  onCommand,
  onHint,
  onClearScreen,
  completions = [],
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  homeName: string
  path: string
  logs: CommandLog[]
  onCommand: (cmd: string) => void
  /** Tab 补全出现多义匹配时向终端回显候选行（bash 行为）。 */
  onHint?: (text: string) => void
  /** 右键菜单「清屏」。 */
  onClearScreen?: () => void
  /** 当前目录可补全的文件名（目录/源文件/样例文件）。 */
  completions?: string[]
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  const [input, setInput] = useState('')
  const [history, setHistory] = useState<string[]>([])
  const [historyIdx, setHistoryIdx] = useState(-1)
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const endRef = useRef<HTMLDivElement>(null)
  const promptPath = `~/${path}`.replace(/~\/$/, '~')

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  const submit = () => {
    if (!input.trim()) return
    setHistory((prev) => [...prev, input.trim()])
    setHistoryIdx(-1)
    onCommand(input)
    setInput('')
  }

  // ↑/↓ 命令历史（与真实 bash 相同：↑ 逐条回溯，↓ 回到最新，到底后回空行）。
  const recallHistory = (direction: -1 | 1) => {
    if (history.length === 0) return
    let idx = historyIdx
    if (direction === -1) {
      idx = idx === -1 ? history.length - 1 : Math.max(0, idx - 1)
    } else {
      idx = idx === -1 ? -1 : idx + 1
      if (idx >= history.length) {
        setHistoryIdx(-1)
        setInput('')
        return
      }
    }
    setHistoryIdx(idx)
    setInput(history[idx])
  }

  // Tab 补全：首个 token 补命令名，其余 token 补当前目录文件名；多义时回显候选。
  const complete = () => {
    const parts = input.split(/\s+/)
    const last = parts[parts.length - 1] ?? ''
    const candidates = parts.length === 1
      ? ['clear', 'help', 'pwd', 'ls', 'cd', 'cat', 'mkdir', 'g++']
      : completions
    const matches = candidates.filter((item) => item.startsWith(last) && last !== '')
    if (matches.length === 1) {
      parts[parts.length - 1] = matches[0]
      setInput(parts.join(' ') + (parts.length === 1 ? ' ' : ''))
    } else if (matches.length > 1) {
      onHint?.(matches.join('  '))
    }
  }

  const menuItems: MenuItem[] = [
    {
      label: '复制',
      shortcut: 'Shift+Ctrl+C',
      disabled: !(window.getSelection()?.toString() ?? ''),
      onClick: () => {
        const text = window.getSelection()?.toString()
        if (text) void navigator.clipboard.writeText(text).catch(() => {})
      },
    },
    {
      label: '粘贴',
      shortcut: 'Shift+Ctrl+V',
      onClick: () => {
        void navigator.clipboard
          .readText()
          .then((text) => setInput((current) => current + text))
          .catch(() => onHint?.('（浏览器拒绝了剪贴板读取权限）'))
      },
    },
    { separator: true },
    { label: '全选', onClick: () => { if (contentRef.current) window.getSelection()?.selectAllChildren(contentRef.current) } },
    { separator: true },
    { label: '清屏', onClick: () => onClearScreen?.() },
  ]

  const prompt = (
    <span>
      <span className="font-bold" style={{ color: YARU.promptGreen }}>
        {homeName}@ubuntu
      </span>
      <span>:</span>
      <span className="font-bold" style={{ color: YARU.promptBlue }}>
        {promptPath}
      </span>
      <span>$ </span>
    </span>
  )

  return (
    <div
      className="flex h-full w-full flex-col overflow-hidden rounded-xl border border-black/40 shadow-[0_14px_48px_rgba(0,0,0,0.5)]"
      style={{ fontFamily: YARU_SANS }}
    >
      <YaruHeaderBar
        onMouseDown={onMouseDown}
        onMinimize={onMinimize}
        onMaximize={onMaximize}
        onClose={onClose}
        left={
          <div className="flex items-end gap-1">
            <button
              type="button"
              className="grid h-7 w-7 place-items-center rounded-md text-[13px] font-bold text-[#3D3D3D] hover:bg-black/10"
              aria-label="新建标签页"
              title="新建标签页"
            >
              +
            </button>
            <div
              className="flex h-[26px] items-center gap-2 rounded-t-md px-3 text-[12px] font-semibold"
              style={{ background: YARU.terminalBg, color: '#FFFFFF' }}
            >
              <span style={{ fontFamily: YARU_MONO }}>{homeName}@ubuntu: {promptPath}</span>
              <X className="h-3 w-3 opacity-70" />
            </div>
          </div>
        }
        actions={
          <button
            type="button"
            className="grid h-7 w-7 place-items-center rounded-md text-[#3D3D3D] hover:bg-black/10"
            aria-label="菜单"
          >
            <MenuIcon className="h-4 w-4" />
          </button>
        }
      />
      <div
        ref={contentRef}
        className="min-h-0 flex-1 overflow-y-auto px-3 py-2 text-[13px] leading-[1.45] selection:bg-[#5E4A63] selection:text-white"
        style={{ background: YARU.terminalBg, color: YARU.terminalFg, fontFamily: YARU_MONO }}
        onContextMenu={(event) => {
          event.preventDefault()
          setMenu({ x: event.clientX, y: event.clientY })
        }}
        onMouseDown={(event) => {
          const target = event.currentTarget
          if (window.getSelection()?.isCollapsed) {
            requestAnimationFrame(() => {
              const inputEl = target.parentElement?.querySelector('input')
              inputEl?.focus()
            })
          }
        }}
      >
        {logs.map((line, index) => (
          <div key={index} className="whitespace-pre-wrap break-words">
            {line.type === 'input' ? (
              <>
                {prompt}
                {line.text}
              </>
            ) : (
              <span
                style={
                  line.type === 'error'
                    ? { color: YARU.errorRed, fontWeight: 700 }
                    : line.type === 'warn'
                      ? { color: '#E9AD0C' }
                      : line.type === 'success'
                        ? { color: YARU.promptGreen }
                        : undefined
                }
              >
                {line.text}
              </span>
            )}
          </div>
        ))}
        <div className="flex items-center">
          {prompt}
          <span
            className="relative flex min-w-0 flex-1 items-center"
            onClick={() => inputRef.current?.focus()}
          >
            <span aria-hidden="true" className="invisible whitespace-pre">
              {input}
            </span>
            <span aria-hidden="true" className="term-cursor -ml-[1px] inline-block">
              █
            </span>
            <input
              ref={inputRef}
              value={input}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') submit()
                else if (event.key === 'ArrowUp') {
                  event.preventDefault()
                  recallHistory(-1)
                } else if (event.key === 'ArrowDown') {
                  event.preventDefault()
                  recallHistory(1)
                } else if (event.key === 'Tab') {
                  event.preventDefault()
                  complete()
                }
              }}
              spellCheck={false}
              autoComplete="off"
              aria-label="终端命令输入"
              className="absolute inset-0 w-full border-0 bg-transparent p-0 font-mono text-[13px] text-transparent caret-transparent outline-none selection:bg-[#5E4A63] selection:text-white"
            />
          </span>
        </div>
        <style>{`
          @keyframes term-blink { 0%, 55% { opacity: 1; } 56%, 100% { opacity: 0; } }
          .term-cursor { animation: term-blink 1.1s steps(1) infinite; }
        `}</style>
        <div ref={endRef} />
      </div>
      {menu && (
        <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />
      )}
    </div>
  )
}

// ---------------------------------------------------------------- Evince 题面

// Evince 文档查看器高仿（GNOME Papers/Evince，Yaru 浅色）：
// 头栏 = 侧栏开关 + 缩放组 + 居中标题；侧栏 = 页面缩略图；正文 = 灰底上的
// 白色 A4 分页。题面内容按行数分页，页脚带页码。
// Evince 文档查看器高仿：窗口里渲染的是**服务端生成的真 PDF**
// （/contests/{cid}/pdf，与教师下载的认证版 A4 同源），由 Chromium 内置
// PDF 引擎光栅化——字体、排版、分页与考点里 Poppler 打开的 day1.pdf 完全
// 一致。跳页/缩放通过 PDF open parameters（#page / #zoom）驱动；
// 无后端（?desktop-preview 离线态）时退回 Markdown 分页模拟。
// Evince 文档查看器 —— 对齐最新版（GNOME Papers 46/47）：
// 头栏 = 历史←→ + 双行标题（文件名 / 第 N 页，共 M 页）+ 侧栏钮 + 汉堡菜单；
// 缩放收进菜单（放大/缩小/适合页面/适合宽度/档位）；侧栏顶部下拉切换
// 缩略图 / 索引；内容区暗灰衬托白页。真 PDF 由 Chromium 光栅化（open
// parameters 控页控缩放），离线预览退回 Markdown 分页。
function evinceOutline(content: string): { title: string; page: number }[] {
  const perPage = 26
  const outline: { title: string; page: number }[] = []
  content.split('\n').forEach((line, index) => {
    const match = line.match(/^#{1,3}\s+(.*)/)
    if (match) {
      outline.push({ title: match[1].trim(), page: Math.floor(index / perPage) + 1 })
    }
  })
  return outline
}

function paginateStatement(markdown: string, linesPerPage = 26): string[] {
  const lines = markdown.replace(/\r/g, '').split('\n')
  const pages: string[] = []
  for (let i = 0; i < lines.length; i += linesPerPage) {
    pages.push(lines.slice(i, i + linesPerPage).join('\n'))
  }
  return pages.length ? pages : ['（本题暂无题面内容）']
}

function fileTypeName(fileName: string): string {
  const ext = fileName.slice(fileName.lastIndexOf('.') + 1).toLowerCase()
  switch (ext) {
    case 'cpp':
    case 'cc':
    case 'cxx':
      return 'C++'
    case 'c':
      return 'C'
    case 'h':
      return 'C/C++ 头文件'
    case 'py':
      return 'Python'
    default:
      return '无'
  }
}

const EVINCE_ZOOM_STEPS = [0.6, 0.8, 1, 1.25, 1.5, 2]

export function EvinceWindow({
  fileName,
  title,
  pdfUrl,
  content,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  fileName: string
  title: string
  /** 服务端生成的题面 PDF 地址；缺省时退回 Markdown 分页模拟。 */
  pdfUrl?: string
  content: string
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  const [zoomIndex, setZoomIndex] = useState(2)
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [sidebarView, setSidebarView] = useState<'thumbnails' | 'index'>('index')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState<number | null>(null)
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)
  const pageRefs = useRef<(HTMLDivElement | null)[]>([])

  // 真 PDF 模式：从响应头读总页数（标题副标题与翻页钳制用）。
  useEffect(() => {
    if (!pdfUrl) return
    let cancelled = false
    void fetch(pdfUrl, { credentials: 'same-origin' })
      .then((response) => {
        const pages = Number(response.headers.get('X-Pdf-Pages') ?? '')
        if (!cancelled && Number.isFinite(pages) && pages > 0) setTotalPages(pages)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [pdfUrl])

  const pdfMode = Boolean(pdfUrl)
  const pages = paginateStatement(content)
  const outline = evinceOutline(content)
  const maxPage = totalPages ?? (pdfMode ? null : pages.length)
  const zoom = EVINCE_ZOOM_STEPS[zoomIndex]
  const pageCount = totalPages ?? pages.length

  const gotoPageClamped = (target: number) => {
    const bounded = Math.max(1, Math.min(maxPage ?? Infinity, target))
    setPage(bounded)
    if (!pdfMode) pageRefs.current[bounded - 1]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const zoomTo = (index: number) => {
    setZoomIndex(Math.max(0, Math.min(EVINCE_ZOOM_STEPS.length - 1, index)))
  }

  const menuItems: MenuItem[] = [
    { label: '上一页', shortcut: 'PgUp', disabled: page <= 1, onClick: () => gotoPageClamped(page - 1) },
    { label: '下一页', shortcut: 'PgDn', disabled: maxPage != null && page >= maxPage, onClick: () => gotoPageClamped(page + 1) },
    { separator: true },
    { label: '放大', shortcut: 'Ctrl++', disabled: zoomIndex >= EVINCE_ZOOM_STEPS.length - 1, onClick: () => zoomTo(zoomIndex + 1) },
    { label: '缩小', shortcut: 'Ctrl+-', disabled: zoomIndex <= 0, onClick: () => zoomTo(zoomIndex - 1) },
    { label: `${Math.round(zoom * 100)}%`, disabled: true },
    { label: '适合页面', onClick: () => zoomTo(2) },
    { separator: true },
    { label: '首页', shortcut: 'Home', onClick: () => gotoPageClamped(1) },
    { label: '末页', shortcut: 'End', onClick: () => gotoPageClamped(maxPage ?? page) },
  ]

  return (
    <div
      tabIndex={0}
      aria-label={`${fileName} 查看器`}
      onKeyDown={(event) => {
        const keys = ['PageDown', 'PageUp', 'ArrowRight', 'ArrowLeft', 'Home', 'End']
        if (!keys.includes(event.key)) return
        event.preventDefault()
        if (event.key === 'PageDown' || event.key === 'ArrowRight') gotoPageClamped(page + 1)
        else if (event.key === 'PageUp' || event.key === 'ArrowLeft') gotoPageClamped(page - 1)
        else if (event.key === 'Home') gotoPageClamped(1)
        else if (event.key === 'End') gotoPageClamped(maxPage ?? page)
      }}
      className="flex h-full w-full flex-col overflow-hidden rounded-xl border border-black/40 shadow-[0_14px_48px_rgba(0,0,0,0.5)] outline-none"
      style={{ fontFamily: YARU_SANS }}
    >
      {/* Papers 头栏：历史 ← → | 双行标题 | 侧栏 + 汉堡 */}
      <div
        onMouseDown={onMouseDown}
        onDoubleClick={onMaximize}
        className="relative flex h-12 shrink-0 items-center gap-1 px-2"
        style={{ background: YARU.headerbarBg, borderBottom: `1px solid ${YARU.headerbarBorder}` }}
      >
        <button
          type="button"
          disabled
          aria-label="后退"
          className="grid h-8 w-8 place-items-center rounded-full text-slate-400 disabled:opacity-40"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>
        <button
          type="button"
          disabled
          aria-label="前进"
          className="grid h-8 w-8 place-items-center rounded-full text-slate-400 disabled:opacity-40"
        >
          <ChevronRight className="h-4 w-4" />
        </button>

        {/* 双行标题：文件名 / 第 N 页，共 M 页 */}
        <div className="pointer-events-none absolute left-1/2 top-1/2 flex -translate-x-1/2 -translate-y-1/2 flex-col items-center">
          <span className="max-w-[45vw] truncate text-[13px] font-bold" style={{ color: YARU.headerbarFg }}>
            {fileName}
          </span>
          <span className="text-[10px] font-semibold text-slate-500">
            第 {page} 页{maxPage != null ? `，共 ${maxPage} 页` : ''}
          </span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <button
            type="button"
            onClick={() => setSidebarOpen((open) => !open)}
            className={`grid h-8 w-8 place-items-center rounded-full text-[#3D3D3D] hover:bg-black/10 ${
              sidebarOpen ? 'bg-black/10' : ''
            }`}
            aria-label="侧栏"
            title="切换侧栏"
          >
            <PanelLeft className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={(event) => {
              const rect = event.currentTarget.getBoundingClientRect()
              setMenu({ x: Math.min(rect.right - 210, window.innerWidth - 230), y: rect.bottom + 4 })
            }}
            className="grid h-8 w-8 place-items-center rounded-full text-[#3D3D3D] hover:bg-black/10"
            aria-label="主菜单"
          >
            <MenuIcon className="h-4 w-4" />
          </button>
          <div className="ml-1.5 flex items-center gap-2">
            {onMinimize && (
              <button
                type="button"
                onClick={onMinimize}
                aria-label="min"
                className="grid h-[18px] w-[18px] place-items-center rounded-full bg-[#BFBFBF]/80 text-[#3D3D3D] hover:bg-[#9A9A9A]"
              >
                <svg width="10" height="10" viewBox="0 0 10 10">
                  <rect x="1.5" y="4.5" width="7" height="1.6" fill="currentColor" />
                </svg>
              </button>
            )}
            {onMaximize && (
              <button
                type="button"
                onClick={onMaximize}
                aria-label="max"
                className="grid h-[18px] w-[18px] place-items-center rounded-full bg-[#BFBFBF]/80 text-[#3D3D3D] hover:bg-[#9A9A9A]"
              >
                <svg width="10" height="10" viewBox="0 0 10 10">
                  <rect x="1.8" y="1.8" width="6.4" height="6.4" fill="none" stroke="currentColor" strokeWidth="1.4" />
                </svg>
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              aria-label="close"
              className="grid h-[18px] w-[18px] place-items-center rounded-full bg-[#BFBFBF]/80 text-[#3D3D3D] hover:bg-[#E95420] hover:text-white"
            >
              <svg width="10" height="10" viewBox="0 0 10 10">
                <path d="M2 2 L8 8 M8 2 L2 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* 侧栏：顶部下拉（缩略图/索引）+ 内容 */}
        {sidebarOpen && (
          <div
            className="flex w-40 shrink-0 flex-col border-r"
            style={{ background: YARU.sidebarBg, borderColor: YARU.sidebarBorder }}
          >
            <div className="p-1.5">
              <select
                value={sidebarView}
                onChange={(event) => setSidebarView(event.target.value as 'thumbnails' | 'index')}
                aria-label="侧栏视图"
                className="h-7 w-full rounded-md border border-[#CFCFCF] bg-white px-1.5 text-[11px] font-semibold text-[#3D3D3D]"
              >
                <option value="thumbnails">缩略图</option>
                <option value="index">索引</option>
              </select>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto py-1">
              {sidebarView === 'thumbnails'
                ? Array.from({ length: pageCount }).map((_, index) => (
                    <button
                      key={index}
                      type="button"
                      onClick={() => gotoPageClamped(index + 1)}
                      className="mx-auto mb-2 flex flex-col items-center gap-1"
                    >
                      <div
                        className={`h-[96px] w-[68px] rounded-sm border bg-white p-1.5 shadow-sm ${
                          index + 1 === page ? 'border-[#E95420] ring-1 ring-[#E95420]' : 'border-[#C9C9C9]'
                        }`}
                      >
                        {Array.from({ length: 7 }).map((_, bar) => (
                          <div
                            key={bar}
                            className="mb-1 h-[3px] rounded-sm bg-slate-300"
                            style={{ width: `${88 - bar * 9}%` }}
                          />
                        ))}
                      </div>
                      <span className="text-[10px] font-semibold text-[#575757]">{index + 1}</span>
                    </button>
                  ))
                : outline.length > 0
                  ? outline.map((entry, index) => (
                      <button
                        key={index}
                        type="button"
                        onClick={() => gotoPageClamped(entry.page)}
                        className="mx-1.5 flex w-[calc(100%-12px)] items-center gap-1 rounded-md px-2 py-1 text-left text-[12px] text-[#3D3D3D] hover:bg-black/10"
                      >
                        <span className="truncate">{entry.title}</span>
                      </button>
                    ))
                  : (
                      <p className="px-3 py-2 text-[11px] text-slate-400">此文档没有索引</p>
                    )}
            </div>
          </div>
        )}

        {/* 内容区：暗灰衬托白页（Papers 观感） */}
        {pdfMode ? (
          <div className="min-h-0 flex-1" style={{ background: '#383838' }}>
            <iframe
              key={`${pdfUrl}#${page}#${Math.round(zoom * 100)}`}
              src={`${pdfUrl}#page=${page}&zoom=${Math.round(zoom * 100)}&toolbar=0&navpanes=0&view=Fit`}
              title={fileName}
              className="h-full w-full border-0"
            />
          </div>
        ) : (
          <div className="min-h-0 flex-1 overflow-y-auto p-6" style={{ background: '#383838' }}>
            <div className="mx-auto flex flex-col items-center gap-6" style={{ zoom }}>
              {pages.map((pageText, index) => (
                <div
                  key={index}
                  ref={(el) => {
                    if (!pdfMode) pageRefs.current[index] = el
                  }}
                  className={`relative w-full max-w-[720px] bg-white px-10 py-12 shadow-[0_2px_12px_rgba(0,0,0,0.45)] ${
                    index + 1 === page ? 'outline outline-2 outline-[#E95420]' : ''
                  }`}
                  style={{ minHeight: '920px' }}
                >
                  <MarkdownView source={pageText} />
                  <span
                    className="absolute bottom-4 left-1/2 -translate-x-1/2 text-[11px] font-semibold text-slate-500"
                    style={{ fontFamily: YARU_MONO }}
                  >
                    — {index + 1} —
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
    </div>
  )
}

export { FileText, CopyIcon, MenuIcon }
