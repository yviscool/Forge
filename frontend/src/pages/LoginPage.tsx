import React, { useState } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  ArrowRight,
  CheckCircle2,
  Command,
  Eye,
  EyeOff,
  GraduationCap,
  LockKeyhole,
  ShieldCheck,
  Sparkles,
  UserRound,
} from 'lucide-react'
import { api, ApiError } from '../api/client'
import { useStore } from '../store/useStore'
import { Button } from '../components/ui/button'
import { Field, Input } from '../components/ui/input'
import { Badge } from '../components/ui/badge'

function BrandMark() {
  return (
    <span className="relative grid h-10 w-10 place-items-center overflow-hidden rounded-xl bg-forge-700 text-white shadow-[0_14px_30px_-14px_rgba(194,65,12,0.9)]">
      <Command className="h-4 w-4" strokeWidth={2.5} />
      <span className="absolute -right-2 -top-2 h-6 w-6 rounded-full bg-amber-300/80 blur-[1px]" />
      <span className="absolute bottom-1.5 left-1.5 h-1 w-1 rounded-full bg-emerald-200" />
    </span>
  )
}

function safeRedirect(value: unknown): string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) return '/'
  return value
}

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const { session, authStatus, setSession } = useStore()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const destination = safeRedirect((location.state as { from?: unknown } | null)?.from)

  if (session && authStatus === 'authenticated') {
    return <Navigate to={destination} replace />
  }

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const username = login.trim()
    if (!username || !password) {
      setError('请输入账号和密码后继续。')
      return
    }

    setLoading(true)
    setError('')
    try {
      await api.login(username, password)
      // The browser receives an HttpOnly session cookie. Read the profile back
      // from the server so the shell never trusts a client-side identity.
      const me = await api.me()
      // Never allow another account's cached contests, source, or ranking to
      // survive an account switch on a shared exam machine.
      queryClient.clear()
      setSession({
        userId: me.id,
        username: me.username,
        name: me.name,
        role: me.role,
        examTicket: me.examTicket,
      })
      navigate(destination, { replace: true })
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) {
        setError('账号或密码不正确，请核对后重试。')
      } else {
        setError(cause instanceof Error ? cause.message : '登录失败，请稍后重试。')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="grid min-h-[calc(100vh-13rem)] overflow-hidden rounded-[2rem] border border-white/60 bg-[#fffefa] shadow-[0_30px_90px_-42px_rgba(20,40,61,0.42)] lg:grid-cols-[1.08fr_0.92fr]">
      <section className="login-showcase relative hidden overflow-hidden bg-[#14283d] p-10 text-white lg:flex lg:flex-col lg:justify-between xl:p-14">
        <div className="absolute inset-0 forge-grid opacity-20" aria-hidden="true" />
        <div
          className="absolute -left-24 top-1/4 h-80 w-80 rounded-full bg-forge-500/20 blur-3xl"
          aria-hidden="true"
        />
        <div
          className="absolute -bottom-32 right-0 h-96 w-96 rounded-full bg-teal-400/10 blur-3xl"
          aria-hidden="true"
        />

        <div className="relative">
          <Link to="/login" className="inline-flex items-center gap-3" aria-label="Forge 登录">
            <BrandMark />
            <span>
              <span className="font-display block text-2xl font-semibold leading-none tracking-[-0.04em]">
                Forge
              </span>
              <span className="mt-1 block text-[9px] font-bold uppercase tracking-[0.28em] text-white/45">
                Contest atelier
              </span>
            </span>
          </Link>

          <div className="login-intro mt-20 max-w-xl">
            <Badge variant="accent" dot className="border-forge-400/30 bg-forge-400/10 text-forge-100">
              为严肃竞赛而作
            </Badge>
            <h1 className="font-display mt-6 text-[clamp(2.8rem,5vw,4.8rem)] font-semibold leading-[0.98] tracking-[-0.045em]">
              把每一次评测，
              <span className="block text-forge-200">变成可见的成长。</span>
            </h1>
            <p className="mt-6 max-w-md text-sm leading-7 text-white/62">
              从出题、考试安全到实时榜单与代码演进，Forge
              把信息学竞赛的完整流程收进一个克制、可靠的工作台。
            </p>
          </div>
        </div>

        <div className="login-features relative mt-12 grid gap-3 sm:grid-cols-2">
          {[
            { icon: ShieldCheck, title: '考场级安全', text: '时间窗口、机位绑定与即时广播。' },
            { icon: Sparkles, title: '沉浸式作答', text: '题面、编辑器、草稿与评测闭环。' },
            { icon: GraduationCap, title: '学情可见', text: '版本演进、榜单和成绩单一目了然。' },
            { icon: CheckCircle2, title: '本地优先', text: '数据留在自己的基础设施里。' },
          ].map(({ icon: Icon, title, text }) => (
            <div
              key={title}
              className="login-feature rounded-2xl border border-white/10 bg-white/[0.055] p-4 backdrop-blur-sm"
            >
              <Icon className="h-4 w-4 shrink-0 text-forge-200" />
              <p className="login-feature-title mt-3 text-xs font-bold text-white">{title}</p>
              <p className="login-feature-text mt-1 text-[11px] leading-5 text-white/45">{text}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="login-form-col relative flex items-center justify-center px-5 py-12 sm:px-10 lg:px-14">
        <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-forge-600 via-amber-400 to-teal-600 lg:hidden" />
        <div className="w-full max-w-md animate-rise">
          <div className="mb-9 lg:hidden">
            <div className="flex items-center gap-3">
              <BrandMark />
              <div>
                <p className="font-display text-2xl font-semibold tracking-[-0.04em] text-[#14283d]">
                  Forge
                </p>
                <p className="text-[9px] font-bold uppercase tracking-[0.24em] text-slate-400">
                  Contest atelier
                </p>
              </div>
            </div>
          </div>

          <p className="app-eyebrow">Welcome back</p>
          <h2 className="font-display mt-3 text-4xl font-semibold tracking-[-0.045em] text-[#14283d]">
            返回你的工作台
          </h2>
          <p className="mt-3 text-sm leading-6 text-slate-500">
            使用教师分配的账号登录。选手账号即考场身份，请不要共用。
          </p>

          <form onSubmit={handleSubmit} className="mt-8 space-y-5" noValidate>
            <Field label="账号 / 学号" hint="必填">
              <div className="relative">
                <UserRound className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                <Input
                  id="login-username"
                  name="username"
                  autoComplete="username"
                  autoFocus
                  value={login}
                  onChange={(event) => {
                    setLogin(event.target.value)
                    if (error) setError('')
                  }}
                  placeholder="请输入账号或学号"
                  className="pl-10"
                  aria-invalid={Boolean(error)}
                  aria-describedby={error ? 'login-error' : undefined}
                />
              </div>
            </Field>

            <Field label="密码" hint="必填">
              <div className="relative">
                <LockKeyhole className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                <Input
                  id="login-password"
                  name="password"
                  type={showPassword ? 'text' : 'password'}
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => {
                    setPassword(event.target.value)
                    if (error) setError('')
                  }}
                  placeholder="请输入登录密码"
                  className="pl-10 pr-11"
                  aria-invalid={Boolean(error)}
                  aria-describedby={error ? 'login-error' : undefined}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((visible) => !visible)}
                  className="absolute right-2.5 top-1/2 grid h-8 w-8 -translate-y-1/2 place-items-center rounded-lg text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
                  aria-label={showPassword ? '隐藏密码' : '显示密码'}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </Field>

            {error && (
              <div
                id="login-error"
                role="alert"
                className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs font-semibold leading-5 text-rose-700"
              >
                {error}
              </div>
            )}

            <Button type="submit" size="lg" className="w-full" loading={loading}>
              {loading ? '正在验证身份...' : '进入 Forge'}
              {!loading && <ArrowRight className="h-4 w-4" />}
            </Button>
          </form>

          {import.meta.env.DEV && (
            <div className="mt-6 rounded-2xl border border-amber-200 bg-amber-50/70 px-4 py-3.5">
              <div className="flex items-start gap-3">
                <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
                <div className="min-w-0 text-[11px] leading-5 text-amber-900">
                  <p className="font-bold">开发环境管理员凭据</p>
                  <p className="mt-0.5 text-amber-800">
                    `FORGE_ADMIN_PASSWORD` 未设置时，首次启动会随机生成管理员密码并打印在服务端日志中；不存在公开默认口令。
                  </p>
                  <p className="mt-1 text-amber-700/75">
                    该密码只用于首次登录，请立即通过 `/api/v1/auth/password` 或在账号设置中轮换。
                  </p>
                </div>
              </div>
            </div>
          )}

          <p className="mt-8 text-center text-[11px] leading-5 text-slate-400">
            无法登录？请联系监考教师重置密码。
            <span className="mx-2 text-slate-300">·</span>
            <Link to="/" className="font-semibold text-slate-500 hover:text-forge-700">
              返回大厅
            </Link>
          </p>
        </div>
      </section>
    </div>
  )
}
