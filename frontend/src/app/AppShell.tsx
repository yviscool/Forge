import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  BookOpenCheck,
  GraduationCap,
  KeyRound,
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

/* 仪器级 chrome：52px 高度、发丝线、无装饰。
   品牌只出现一次（这里），导航用分段控制，页面不再有 footer。 */
function BrandMark() {
  return (
    <span className="grid h-7 w-7 place-items-center rounded-lg bg-navy-900 text-[13px] font-black text-white">
      F
    </span>
  )
}

export function AppShell({ children }: { children: React.ReactNode }) {
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
      <header className="no-print sticky top-0 z-50 border-b border-[#e3e8ee] bg-white/90 backdrop-blur-xl">
        <div className="flex h-13 w-full items-center justify-between gap-4 pl-4 pr-4 sm:pl-6 sm:pr-6 lg:pl-8 lg:pr-8">
          <div className="flex min-w-0 items-center gap-5">
            <Link to="/" className="group flex min-w-0 items-center gap-2.5">
              <BrandMark />
              <span className="font-display truncate text-[17px] font-semibold tracking-[-0.02em] text-navy-900">
                Forge
              </span>
              <span className="sr-only">首页</span>
            </Link>

            <nav
              className="hidden items-center gap-0.5 rounded-xl bg-slate-100/80 p-0.5 md:flex"
              aria-label="主导航"
            >
              <Link
                to="/"
                aria-current={!inTeacher ? 'page' : undefined}
                className={
                  !inTeacher
                    ? 'flex h-8 items-center gap-1.5 rounded-[10px] bg-white px-3.5 text-xs font-bold text-ink shadow-[0_1px_2px_rgba(19,28,43,0.08)]'
                    : 'flex h-8 items-center gap-1.5 rounded-[10px] px-3.5 text-xs font-bold text-slate-500 transition hover:text-slate-900'
                }
              >
                <BookOpenCheck className="h-3.5 w-3.5" />
                选手大厅
              </Link>
              {isTeacher && (
                <Link
                  to="/teacher"
                  aria-current={inTeacher ? 'page' : undefined}
                  className={
                    inTeacher
                      ? 'flex h-8 items-center gap-1.5 rounded-[10px] bg-white px-3.5 text-xs font-bold text-ink shadow-[0_1px_2px_rgba(19,28,43,0.08)]'
                      : 'flex h-8 items-center gap-1.5 rounded-[10px] px-3.5 text-xs font-bold text-slate-500 transition hover:text-slate-900'
                  }
                >
                  <LayoutDashboard className="h-3.5 w-3.5" />
                  教师工作台
                </Link>
              )}
            </nav>
          </div>

          <div className="flex items-center gap-1.5">
            {session ? (
              <>
                <div className="mr-1 hidden items-center gap-2 rounded-[10px] border border-[#e3e8ee] bg-white px-2.5 py-1.5 sm:flex">
                  {isTeacher ? (
                    <ShieldCheck className="h-4 w-4 text-forge-700" />
                  ) : (
                    <GraduationCap className="h-4 w-4 text-teal-700" />
                  )}
                  <span className="max-w-[9rem] truncate text-xs font-bold text-slate-800">
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
                  className="grid h-9 w-9 place-items-center rounded-[10px] text-slate-500 transition hover:bg-slate-100 hover:text-slate-900"
                  title="修改密码"
                  aria-label="修改密码"
                >
                  <KeyRound className="h-4 w-4" />
                </button>
                <button
                  data-testid="logout-button"
                  type="button"
                  onClick={() => void handleLogout()}
                  className="grid h-9 w-9 place-items-center rounded-[10px] text-slate-500 transition hover:bg-rose-50 hover:text-rose-600"
                  title="退出登录"
                  aria-label="退出登录"
                >
                  <LogOut className="h-4 w-4" />
                </button>
              </>
            ) : (
              <Link
                to="/login"
                className="inline-flex h-9 items-center gap-2 rounded-[10px] bg-ink px-4 text-xs font-bold text-white transition hover:bg-[#0f2233]"
              >
                <UserRound className="h-3.5 w-3.5" />
                登录
              </Link>
            )}

            {isTeacher && (
              <Link
                to="/teacher"
                className="grid h-9 w-9 place-items-center rounded-[10px] text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 md:hidden"
                aria-label="教师工作台"
                title="教师工作台"
              >
                <LayoutDashboard className="h-4 w-4" />
              </Link>
            )}
          </div>
        </div>
      </header>

      <main className="flex w-full flex-1 flex-col px-4 py-5 sm:px-6 lg:px-8 lg:py-6">
        {children}
      </main>

      <ChangePasswordDialog open={passwordOpen} onClose={() => setPasswordOpen(false)} />
    </div>
  )
}
