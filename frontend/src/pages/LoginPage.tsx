import React, { useState } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  ArrowRight,
  Eye,
  EyeOff,
  LockKeyhole,
  ShieldCheck,
  UserRound,
} from 'lucide-react'
import { api, ApiError } from '../api/client'
import { useStore } from '../store/useStore'
import { Button } from '../components/ui/button'
import { Field, Input } from '../components/ui/input'

function BrandMark() {
  return (
    <span className="grid h-9 w-9 place-items-center rounded-[10px] bg-white/10 text-sm font-black text-white ring-1 ring-inset ring-white/15">
      F
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
    <div className="grid min-h-[calc(100vh-13rem)] overflow-hidden rounded-2xl border border-[#e3e8ee] bg-white shadow-[0_1px_2px_rgba(19,28,43,0.04),0_24px_64px_-40px_rgba(19,28,43,0.35)] lg:grid-cols-[1.04fr_0.96fr]">
      {/* 左栏：仪器的出厂铭牌 —— 只说产品真话，不喊口号 */}
      <section className="login-showcase relative hidden overflow-hidden bg-navy-900 p-10 text-white lg:flex lg:flex-col lg:justify-between xl:p-14">
        <div className="absolute inset-0 forge-grid opacity-[0.12]" aria-hidden="true" />
        <div
          className="absolute -left-28 top-1/3 h-80 w-80 rounded-full bg-forge-600/15 blur-3xl"
          aria-hidden="true"
        />

        <div className="relative">
          <Link to="/login" className="inline-flex items-center gap-3" aria-label="Forge 登录">
            <BrandMark />
            <span className="font-display block text-xl font-semibold tracking-[-0.02em]">
              Forge
            </span>
          </Link>

          <div className="login-intro mt-16 max-w-lg">
            <h1 className="font-display text-[clamp(1.75rem,2.4vw,2.35rem)] leading-[1.28] text-white">
              出题、监考、评测、交付，
              <span className="block text-forge-200">一台仪器，全部在场。</span>
            </h1>
            <p className="mt-5 max-w-md text-sm leading-7 text-white/55">
              为学校与集训队打造的竞赛平台：单机部署、离线可用，数据不出机房。
            </p>
          </div>
        </div>

        <div className="login-features relative mt-12 grid gap-x-8 gap-y-4 border-t border-white/10 pt-8 sm:grid-cols-2">
          {[
            { icon: ShieldCheck, title: '考场级安全', text: '时间窗口、机位绑定与即时广播。' },
            { icon: ArrowRight, title: '实时评测', text: '逐点结果与榜单，十秒内可见。' },
            { icon: LockKeyhole, title: '本地优先', text: 'SQLite 单文件，数据留在机房。' },
            { icon: UserRound, title: '全流程交付', text: 'CCF 试卷、成绩与比赛包导出。' },
          ].map(({ icon: Icon, title, text }) => (
            <div key={title} className="login-feature">
              <Icon className="h-4 w-4 shrink-0 text-forge-200" />
              <div className="min-w-0">
                <p className="login-feature-title text-xs font-bold text-white">{title}</p>
                <p className="login-feature-text mt-0.5 text-[11px] leading-5 text-white/40">
                  {text}
                </p>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="login-form-col relative flex items-center justify-center px-5 py-12 sm:px-10 lg:px-14">
        <div className="w-full max-w-sm animate-rise">
          <div className="mb-8 lg:hidden">
            <div className="flex items-center gap-2.5">
              <span className="grid h-9 w-9 place-items-center rounded-[10px] bg-navy-900 text-sm font-black text-white">
                F
              </span>
              <span className="font-display text-xl font-semibold tracking-[-0.02em] text-navy-900">
                Forge
              </span>
            </div>
          </div>

          <h2 className="font-display text-[26px] leading-tight text-navy-900">登录工作台</h2>
          <p className="mt-2 text-sm leading-6 text-slate-500">
            使用教师分配的账号。选手账号即考场身份，请不要共用。
          </p>

          <form onSubmit={handleSubmit} className="mt-7 space-y-4" noValidate>
            <Field label="账号 / 学号">
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

            <Field label="密码">
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
                className="animate-fade rounded-[10px] border border-rose-200 bg-rose-50 px-4 py-3 text-xs font-semibold leading-5 text-rose-700"
              >
                {error}
              </div>
            )}

            <Button type="submit" size="lg" variant="accent" className="w-full" loading={loading}>
              {loading ? '正在验证身份...' : '进入 Forge'}
              {!loading && <ArrowRight className="h-4 w-4" />}
            </Button>
          </form>

          {import.meta.env.DEV && (
            <p className="mt-4 rounded-[10px] border border-amber-200 bg-amber-50/70 px-3.5 py-2.5 text-[11px] leading-5 text-amber-800">
              <span className="font-bold">开发环境：</span>
              `FORGE_ADMIN_PASSWORD` 未设置时首次启动会生成随机密码，写入服务端日志；
              首次登录后请立即轮换。
            </p>
          )}

          <p className="mt-7 text-center text-[11px] leading-5 text-slate-400">
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
