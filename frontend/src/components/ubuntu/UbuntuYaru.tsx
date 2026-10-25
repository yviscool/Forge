// NOI Linux 2.0（Ubuntu 20.04.1 LTS + Yaru 主题）的桌面应用复刻。
// 配色与布局参数取自官方 ISO 内的实测素材：Yaru 图标位图、geany-3.20.css、
// gnome-terminal 默认 profile（紫色 #300A24）以及 NOI 官方壁纸的取色。
// 字体栈回退：目标机装有 Ubuntu 字体时逐像素一致，否则退化到相近字形。
import React, { useEffect, useRef, useState } from 'react'
import {
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
} from 'lucide-react'
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

const PLACES: { id: string; label: string; icon: React.ReactNode }[] = [
  { id: 'recent', label: '最近', icon: <Clock3 className="h-4 w-4" /> },
  { id: 'home', label: '主文件夹', icon: <img src={YARU_ICONS.userHome} alt="" className="h-4 w-4" /> },
  { id: 'desktop', label: '桌面', icon: <img src={YARU_ICONS.userDesktop} alt="" className="h-4 w-4" /> },
  { id: 'documents', label: '文档', icon: <img src={YARU_ICONS.folder} alt="" className="h-4 w-4" /> },
  { id: 'downloads', label: '下载', icon: <Download className="h-4 w-4" /> },
  { id: 'music', label: '音乐', icon: <Music className="h-4 w-4" /> },
  { id: 'pictures', label: '图片', icon: <ImageIcon className="h-4 w-4" /> },
  { id: 'videos', label: '视频', icon: <Video className="h-4 w-4" /> },
  { id: 'trash', label: '回收站', icon: <Trash2 className="h-4 w-4" /> },
]

export function NautilusWindow({
  homeName,
  dirName,
  items,
  selectedId,
  onSelect,
  onOpen,
  onNavigate,
  onNewFolder,
  onOpenTerminal,
  onRename,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  homeName: string
  dirName: string
  items: NautilusItem[]
  selectedId: string | null
  onSelect: (id: string | null) => void
  onOpen: (item: NautilusItem) => void
  onNavigate: (level: 0 | 1) => void
  onNewFolder: () => void
  onOpenTerminal: () => void
  onRename: (item: NautilusItem) => void
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  const [menu, setMenu] = useState<{ x: number; y: number; item: NautilusItem | null } | null>(null)
  const [activePlace, setActivePlace] = useState('home')
  const [view, setView] = useState<'grid' | 'list'>('grid')
  // GNOME 属性对话框：null 关闭，'root' 表示主目录自身。
  const [properties, setProperties] = useState<NautilusItem | 'root' | null>(null)

  const openMenu = (event: React.MouseEvent, item: NautilusItem | null) => {
    event.preventDefault()
    event.stopPropagation()
    if (item) onSelect(item.id)
    setMenu({ x: event.clientX, y: event.clientY, item })
  }

  const menuItems = (item: NautilusItem | null): MenuItem[] =>
    item
      ? [
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
          { label: '剪切', shortcut: 'Ctrl+X', onClick: () => {} },
          { label: '复制', shortcut: 'Ctrl+C', onClick: () => {} },
          { separator: true },
          { label: '重命名…', shortcut: 'F2', onClick: () => onRename(item) },
          { label: '移动到回收站', shortcut: 'Delete', danger: true, onClick: () => onRename(item) },
          { separator: true },
          { label: '压缩…', onClick: () => {} },
          { label: '属性', shortcut: 'Ctrl+I', onClick: () => setProperties(item) },
        ]
      : [
          { label: '新建文件夹', shortcut: 'Shift+Ctrl+N', onClick: onNewFolder },
          { separator: true },
          { label: '粘贴', shortcut: 'Ctrl+V', disabled: true },
          { separator: true },
          { label: '在终端中打开', onClick: onOpenTerminal },
          { separator: true },
          { label: '属性', onClick: () => setProperties('root') },
        ]

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
          <div className="flex items-center gap-0.5">
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation()
                onNavigate(0)
              }}
              className="rounded-md px-2.5 py-0.5 text-[12px] font-semibold text-[#3D3D3D] hover:bg-black/10"
              style={dirName === '' ? { background: 'rgba(0,0,0,0.09)' } : undefined}
            >
              {homeName}
            </button>
            {dirName !== '' && (
              <>
                <ChevronRight className="h-3 w-3 opacity-50" />
                <button
                  type="button"
                  onClick={(event) => {
                    event.stopPropagation()
                    onNavigate(1)
                  }}
                  className="rounded-md px-2.5 py-0.5 text-[12px] font-semibold text-[#3D3D3D] hover:bg-black/10"
                  style={{ background: 'rgba(0,0,0,0.09)' }}
                >
                  {dirName}
                </button>
              </>
            )}
          </div>
        }
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
          {PLACES.map((place) => {
            const active = place.id === activePlace
            return (
              <button
                key={place.id}
                type="button"
                onClick={() => {
                  setActivePlace(place.id)
                  onSelect(null)
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

        {/* 图标视图 */}
        <div
          className="relative min-h-0 flex-1 overflow-y-auto"
          style={{ background: YARU.contentBg }}
          onMouseDown={() => onSelect(null)}
          onContextMenu={(event) => openMenu(event, null)}
        >
          <div
            className={
              view === 'grid'
                ? `grid grid-cols-[repeat(auto-fill,minmax(104px,1fr))] gap-2 p-4${activePlace !== 'home' ? ' hidden' : ''}`
                : `flex flex-col p-2${activePlace !== 'home' ? ' hidden' : ''}`
            }
          >
            {items.map((item) => {
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
          {view === 'list' && (
            <div className="mx-1 border-b" style={{ borderColor: '#EDEDED' }}>
              <div
                className="flex items-center gap-2 px-2 py-1 text-[11px] font-bold text-[#8F8F8F]"
              >
                <span className="w-5" />
                <span className="flex-1">名称</span>
                <span className="w-20 text-right">大小</span>
              </div>
              {items.map((item) => {
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
                      {item.kind === 'source' ? formatBytes(item.sizeBytes ?? 0) : `${items.length} 项`}
                    </span>
                  </button>
                )
              })}
            </div>
          )}
          {(activePlace !== 'home' || items.length === 0) && (
            <p className="grid place-items-center py-16 text-[13px] text-slate-400">
              {activePlace !== 'home' ? '此文件夹为空（模拟桌面仅挂载考生主目录）' : '此文件夹为空'}
            </p>
          )}
        </div>
      </div>
      {menu && (
        <ContextMenu x={menu.x} y={menu.y} items={menuItems(menu.item)} onClose={() => setMenu(null)} />
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
                      ['位置', `/home/${homeName}${dirName ? `/${dirName}` : ''}`],
                      ['编码', (properties as NautilusItem).kind === 'source' ? 'UTF-8' : undefined],
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
  onDoctor,
  onSave,
  tabs,
  activeTabId,
  onSelectTab,
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
  onDoctor: () => void
  onSave: () => void
  /** 每题一个文件标签（真实 Geany 的多文档形态）。 */
  tabs: { id: string; name: string }[]
  activeTabId: string
  onSelectTab: (id: string) => void
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
  const [sidebarTab, setSidebarTab] = useState<'symbols' | 'docs'>('symbols')
  const editorRef = useRef<GeanyEditorHandle | null>(null)
  const [caret, setCaret] = useState({ line: 1, column: 1 })
  const symbols = parseGeanySymbols(code)

  const runEditor = (cmd: string) => editorRef.current?.trigger('geany', cmd, null)

  // 真实 Geany 的肌肉记忆：F8 编译、F5 运行、Ctrl+S 显式保存（对本窗口全局生效）。
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'F8') {
        event.preventDefault()
        doCompile()
      } else if (event.key === 'F5') {
        event.preventDefault()
        onRun()
      } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
        event.preventDefault()
        onSave()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // doCompile 随渲染重建，按引用入依赖会造成每次渲染重挂监听；语义上只依赖这三个回调。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onCompile, onRun, onSave])

  const doCompile = () => {
    setMessageTab('compiler')
    onCompile()
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
    { label: '打开所在文件夹', onClick: onDoctor },
    { separator: true },
    { label: '首选项…', onClick: onDoctor },
  ]

  // 菜单栏下拉（Geany 真实菜单结构；未接后端的项保持视觉一致）。
  const menubarMenus: Record<string, MenuItem[]> = {
    文件: [
      { label: '新建', shortcut: 'Ctrl+N' },
      { label: '打开…', shortcut: 'Ctrl+O', onClick: onPickUploadFile },
      { label: '保存', shortcut: 'Ctrl+S' },
      { label: '另存为…', shortcut: 'Shift+Ctrl+S' },
      { separator: true },
      { label: '关闭', shortcut: 'Ctrl+W' },
      { label: '退出', shortcut: 'Ctrl+Q', onClick: onClose },
    ],
    编辑: menuItems,
    搜索: [
      { label: '查找…', shortcut: 'Ctrl+F', onClick: () => runEditor('actions.find') },
      { label: '替换…', shortcut: 'Ctrl+H', onClick: () => runEditor('editor.action.startFindReplaceAction') },
    ],
    查看: [
      { label: '切换消息窗口', shortcut: 'Ctrl+J' },
      { label: '切换侧边栏', shortcut: 'Ctrl+B' },
      { label: '全屏', shortcut: 'F11' },
    ],
    文档: [{ label: `${fileName} (UTF-8)`, disabled: true }],
    项目: [{ label: '未打开项目', disabled: true }],
    构建: [
      { label: '编译', shortcut: 'F8', onClick: doCompile },
      { label: '构建', shortcut: 'F9', onClick: doCompile },
      { separator: true },
      { label: '运行', shortcut: 'F5', onClick: onRun },
    ],
    工具: [
      { label: '考场体检', onClick: onDoctor },
      { label: '插入日期', disabled: readOnly, onClick: insertDate },
    ],
    帮助: [{ label: '关于 Geany' }],
  }

  return (
    <div
      className="relative flex h-full w-full flex-col overflow-hidden rounded-xl border border-black/40 shadow-[0_14px_48px_rgba(0,0,0,0.5)]"
      style={{ fontFamily: YARU_SANS }}
    >
      <YaruTitleBar
        title={`${fileName} - /home/${homeName}/${dirName} - Geany`}
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
          { icon: <FileText className="h-4 w-4" />, label: '新建' },
          { icon: <FolderOpen className="h-4 w-4" />, label: '打开', onClick: onPickUploadFile },
          { icon: <Download className="h-4 w-4 rotate-180" />, label: '保存' },
          { separator: true },
          { icon: <Play className="h-4 w-4 text-emerald-700" />, label: '编译 (F8)', onClick: doCompile },
          { icon: <Play className="h-4 w-4 text-blue-700" />, label: '在终端中运行 (F5)', onClick: onRun },
          { separator: true },
          { icon: <Search className="h-4 w-4" />, label: '查找' },
        ].map((btn, index) =>
          btn.separator ? (
            <div key={index} className="mx-1.5 h-5 w-px bg-[#D9D9D6]" />
          ) : (
            <button
              key={index}
              type="button"
              title={btn.label}
              onClick={btn.onClick}
              className="grid h-7 w-7 place-items-center rounded text-[#3D3D3D] hover:bg-black/10"
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
            onClick={onDoctor}
            className="h-6 rounded border border-amber-300 bg-amber-50 px-2 text-[10px] font-bold text-amber-800 hover:bg-amber-100"
          >
            考场体检
          </button>
          <button
            type="button"
            onClick={onSubmit}
            disabled={submitting}
            className="h-6 rounded bg-[#2C64C5] px-2.5 text-[10px] font-bold text-white hover:bg-[#1F4FA5] disabled:opacity-60"
          >
            {submitting ? '提交中...' : '正式提交'}
          </button>
        </div>
      </div>

      {/* 主体：侧栏 + 编辑器 */}
      <div className="flex min-h-0 flex-1">
        <div className="flex w-44 shrink-0 flex-col border-r border-[#D9D9D6]" style={{ background: YARU.sidebarBg }}>
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
              symbols.length ? (
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
            ) : (
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
            )}
          </div>
        </div>

        <div className="flex min-w-0 flex-1 flex-col" style={{ background: YARU.contentBg }}>
          {/* 编辑器标签页：一题一个文件标签，与真实 Geany 的多文档一致 */}
          <div className="flex h-7 shrink-0 items-stretch overflow-x-auto border-b border-[#D9D9D6]" style={{ background: '#EFEFED' }}>
            {tabs.map((entry) => {
              const active = entry.id === activeTabId
              return (
                <button
                  key={entry.id}
                  type="button"
                  onClick={() => onSelectTab(entry.id)}
                  className="flex shrink-0 items-center gap-1.5 border-r border-[#D9D9D6] px-3 text-[12px]"
                  style={{ background: active ? YARU.contentBg : 'transparent', color: '#2D2D2D' }}
                >
                  <img src={YARU_ICONS.textGeneric} alt="" className="h-3.5 w-3.5" />
                  <span style={{ fontFamily: YARU_MONO }}>{entry.name}</span>
                  <X className="h-3 w-3 opacity-50" />
                </button>
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
            <MonacoSlot
              code={code}
              onCode={onCode}
              readOnly={readOnly}
              onReady={(editor) => {
                editorRef.current = editor
              }}
              onCursor={(line, column) => setCaret({ line, column })}
            />
          </div>
        </div>
      </div>

      {/* 消息窗口 */}
      <div className="h-36 shrink-0 border-t border-[#D9D9D6]" style={{ background: YARU.contentBg }}>
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
              {fileName} 已载入。{readOnly ? '（只读预览：考试未开始或已结束）' : ''}
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
          {messageTab === 'notes' && <p className="text-slate-400">此窗口用于记录临时笔记。</p>}
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
          '文件类型: C++',
        ].map((part, index) => (
          <span key={index} className="border-l border-[#D9D9D6] px-2 first:border-l-0">
            {part}
          </span>
        ))}
      </div>

      {uploadMismatch && uploadFile && (
        <div className="absolute bottom-[10.2rem] left-0 right-0 border-t border-amber-300 bg-amber-50 px-3 py-1 text-[11px] font-semibold text-amber-900">
          文件名与题面英文名不一致：本题目录为 <span style={{ fontFamily: YARU_MONO }}>{uploadMismatch}</span>，当前选择{' '}
          <span style={{ fontFamily: YARU_MONO }}>{uploadFile.name}</span>。
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
      ? ['clear', 'help', 'pwd', 'ls', 'cd', 'cat', 'mkdir', 'g++', 'doctor']
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

export function EvinceWindow({
  fileName,
  title,
  children,
  onMouseDown,
  onMinimize,
  onMaximize,
  onClose,
}: {
  fileName: string
  title: string
  children: React.ReactNode
  onMouseDown?: (e: React.MouseEvent) => void
  onMinimize?: () => void
  onMaximize?: () => void
  onClose?: () => void
}) {
  return (
    <div
      className="flex h-full w-full flex-col overflow-hidden rounded-xl border border-black/40 shadow-[0_14px_48px_rgba(0,0,0,0.5)]"
      style={{ fontFamily: YARU_SANS }}
    >
      <YaruHeaderBar
        title={`${fileName} — ${title}`}
        onMouseDown={onMouseDown}
        onMinimize={onMinimize}
        onMaximize={onMaximize}
        onClose={onClose}
        actions={
          <div className="flex items-center gap-1">
            <button type="button" className="grid h-7 w-7 place-items-center rounded-md text-[#3D3D3D] hover:bg-black/10" aria-label="缩小">
              <span className="text-[13px] font-bold leading-none">−</span>
            </button>
            <span className="min-w-[38px] text-center text-[11px] font-semibold text-[#3D3D3D]">100%</span>
            <button type="button" className="grid h-7 w-7 place-items-center rounded-md text-[#3D3D3D] hover:bg-black/10" aria-label="放大">
              <span className="text-[13px] font-bold leading-none">+</span>
            </button>
          </div>
        }
      />
      <div className="min-h-0 flex-1 overflow-y-auto bg-white">{children}</div>
    </div>
  )
}

export { FileText, CopyIcon, MenuIcon }
