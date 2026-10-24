import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  ArrowUpRight,
  BookOpenCheck,
  Command,
  GraduationCap,
  KeyRound,
  Landmark,
  LayoutDashboard,
  LogOut,
  ShieldCheck,
  UserRound,
} from 'lucide-react'
import { useStore } from '../store/useStore'
import { Badge } from '../components/ui/badge'
import { api } from '../api/client'
import { ChangePasswordDialog } from '../components/ChangePasswordDialog'

const educatorRoles = new Set(['admin', 'teacher'])

function BrandMark() {
  return (
    <span className="relative grid h-9 w-9 place-items-center overflow-hidden rounded-xl bg-[#14283d] text-white shadow-[0_10px_24px_-12px_rgba(20,40,61,0.9)]">
      <Command className="h-4 w-4" strokeWidth={2.4} />
      <span className="absolute -right-2 -top-2 h-5 w-5 rounded-full bg-forge-500/80 blur-[1px]" />
      <span className="absolute bottom-1 left-1 h-1 w-1 rounded-full bg-emerald-300" />
    </span>
  )
}

export function AppShell({ children }: { children: React.ReactNode }) {
  // 桌面壳（Wails 主窗/托盘面板）内：跨区导航交给原生窗口指令，
  // 主窗始终停留在教师台——web 浏览器里保持普通 SPA 路由。
  const inDesktopShell = () => {
    try {
      return localStorage.getItem('forge_desktop') === '1'
    } catch {
      return false
    }
  }
  const desktopGoto = (action: 'open-student' | 'show-main', fallback: string) => {
    if (!inDesktopShell()) {
      window.location.assign(fallback)
      return
    }
    void api.uiCommand(action).catch(() => {
      window.location.assign(fallback)
    })
  }

  const location = useLocation()
  const queryClient = useQueryClient()
  const { session, logout } = useStore()
  const [passwordOpen, setPasswordOpen] = React.useState(false)
  const isTeacher = educatorRoles.has(session?.role || '')
  const inTeacher = location.pathname.startsWith('/teacher')

  const handleLogout = async () => {
    try {
      // The bearer token must still exist while revoking the server session.
      await api.logout()
    } catch {
      // Local logout remains reliable when the server is unreachable.
    } finally {
      queryClient.clear()
      logout()
    }
  }

  return (
    <div className="flex min-h-screen flex-col">
      <header className="no-print sticky top-0 z-50 border-b border-[#e4e7ec]/80 bg-[#fffefa]/88 backdrop-blur-xl">
        <div className="mx-auto flex h-16 w-full max-w-[1440px] items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
          <div className="flex min-w-0 items-center gap-6">
            <Link to="/" className="group flex min-w-0 items-center gap-3">
              <BrandMark />
              <span className="min-w-0">
                <span className="font-display block truncate text-[21px] font-semibold leading-none tracking-[-0.03em] text-[#14283d]">
                  Forge
                </span>
                <span className="mt-1 hidden text-[9px] font-extrabold uppercase tracking-[0.22em] text-slate-400 sm:block">
                  Contest atelier
                </span>
              </span>
              <span className="sr-only">首页</span>
            </Link>

            <nav className="hidden items-center gap-1 md:flex" aria-label="主导航">
              <button
                type="button"
                onClick={() => desktopGoto('open-student', '/')}
                className={
                  !inTeacher
                    ? 'flex h-9 items-center gap-1.5 rounded-xl bg-[#14283d] px-3 text-xs font-bold text-white'
                    : 'flex h-9 items-center gap-1.5 rounded-xl px-3 text-xs font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-900'
                }
              >
                <BookOpenCheck className="h-3.5 w-3.5" />
                选手大厅
              </button>
              {isTeacher && (
                <button
                  type="button"
                  onClick={() => desktopGoto('show-main', '/teacher')}
                  className={
                    inTeacher
                      ? 'flex h-9 items-center gap-1.5 rounded-xl bg-[#14283d] px-3 text-xs font-bold text-white'
                      : 'flex h-9 items-center gap-1.5 rounded-xl px-3 text-xs font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-900'
                  }
                >
                  <LayoutDashboard className="h-3.5 w-3.5" />
                  教师工作台
                </button>
              )}
            </nav>
          </div>

          <div className="flex items-center gap-2">
            {session ? (
              <>
                <div className="hidden items-center gap-2 rounded-xl border border-[#e4e7ec] bg-white/70 px-3 py-1.5 sm:flex">
                  {isTeacher ? (
                    <ShieldCheck className="h-4 w-4 text-forge-700" />
                  ) : (
                    <GraduationCap className="h-4 w-4 text-teal-700" />
                  )}
                  <span className="max-w-[10rem] truncate text-xs font-bold text-slate-800">
                    {session.name || session.username}
                  </span>
                  <Badge variant={isTeacher ? 'accent' : 'info'}>
                    {session.role === 'admin' ? '管理员' : isTeacher ? '教师' : '选手'}
                  </Badge>
                </div>
                <button
                  data-testid="change-password-button"
                  type="button"
                  onClick={() => setPasswordOpen(true)}
                  className="grid h-9 w-9 place-items-center rounded-xl text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
                  title="修改密码"
                  aria-label="修改密码"
                >
                  <KeyRound className="h-4 w-4" />
                </button>
                <button
                  data-testid="logout-button"
                  type="button"
                  onClick={() => void handleLogout()}
                  className="grid h-9 w-9 place-items-center rounded-xl text-slate-400 transition hover:bg-rose-50 hover:text-rose-600"
                  title="退出登录"
                  aria-label="退出登录"
                >
                  <LogOut className="h-4 w-4" />
                </button>
              </>
            ) : (
              <Link
                to="/login"
                className="inline-flex h-9 items-center gap-2 rounded-xl bg-[#14283d] px-4 text-xs font-bold text-white transition hover:bg-[#1f3a52]"
              >
                <UserRound className="h-3.5 w-3.5" />
                登录
              </Link>
            )}

            {isTeacher && (
              <Link
                to="/teacher"
                className="grid h-9 w-9 place-items-center rounded-xl text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 md:hidden"
                aria-label="教师工作台"
                title="教师工作台"
              >
                <LayoutDashboard className="h-4 w-4" />
              </Link>
            )}

            {isTeacher && (
              <button
                type="button"
                onClick={() =>
                  inTeacher
                    ? desktopGoto('open-student', '/')
                    : desktopGoto('show-main', '/teacher')
                }
                className="hidden h-9 items-center gap-1.5 rounded-xl border border-[#e4e7ec] bg-white px-3 text-xs font-bold text-slate-600 transition hover:border-[#d0d5dd] hover:text-slate-900 sm:inline-flex"
              >
                <Landmark className="h-3.5 w-3.5" />
                {inTeacher ? '选手视角' : '教师工作台'}
                <ArrowUpRight className="h-3 w-3 opacity-50" />
              </button>
            )}
          </div>
        </div>
      </header>

      <main className="mx-auto flex w-full max-w-[1440px] flex-1 flex-col px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
        {children}
      </main>

      <footer className="no-print border-t border-[#e4e7ec]/80 bg-transparent">
        <div className="mx-auto flex w-full max-w-[1440px] flex-col gap-2 px-4 py-5 text-[11px] text-slate-400 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8">
          <p>Forge Contest Atelier · Built for serious practice.</p>
          <p className="font-mono tracking-wide">CSP / NOIP / GESP · local-first judging</p>
        </div>
      </footer>
      <ChangePasswordDialog open={passwordOpen} onClose={() => setPasswordOpen(false)} />
    </div>
  )
}
