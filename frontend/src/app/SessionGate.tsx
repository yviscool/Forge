import React, { useEffect, useRef } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ServerCrash } from 'lucide-react'
import { api, ApiError } from '../api/client'
import { Button } from '../components/ui/button'
import { useStore } from '../store/useStore'

/**
 * Hydrates identity from the HttpOnly session cookie on boot. No credential is
 * ever read from or written to localStorage.
 */
export function SessionGate({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient()
  const { authStatus, setSession, setAuthStatus, logout } = useStore()
  const lastUserIdRef = useRef<string | null>(null)

  useEffect(() => {
    const onUnauthorized = () => {
      queryClient.clear()
      logout()
    }
    window.addEventListener('forge:unauthorized', onUnauthorized)
    return () => window.removeEventListener('forge:unauthorized', onUnauthorized)
  }, [logout, queryClient])

  // 桌面壳一次性登录令牌（?dt=）必须与会话查询串行：先换取会话再查 /auth/me。
  // 早期版本把交换放在独立 effect 里，/auth/me 的 401 先到导致页面先跳去
  // 登录页，令牌兑换完成后也不回头——主窗因此停在登录页。
  const sessionQuery = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: async () => {
      const params = new URLSearchParams(window.location.search)
      const token = params.get('dt')
      if (token) {
        params.delete('dt')
        const rest = params.toString()
        window.history.replaceState(
          {},
          '',
          window.location.pathname + (rest ? `?${rest}` : '') + window.location.hash
        )
        try {
          const res = await fetch('/api/v1/auth/desktop-exchange', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token }),
          })
          if (res.ok) {
            // 桌面壳主窗标记：AppShell 跨区导航改走原生窗口指令。
            try {
              localStorage.setItem('forge_desktop', '1')
            } catch {
              /* 隐私模式下忽略 */
            }
          }
        } catch {
          // 令牌失效（60s 窗口错过/重放）时按常规匿名流程走。
        }
      }
      return api.me()
    },
    enabled: authStatus === 'idle' || authStatus === 'checking',
    retry: false,
    staleTime: 5 * 60 * 1000,
    meta: { silent: true },
  })

  useEffect(() => {
    if (authStatus === 'idle') setAuthStatus('checking')
  }, [authStatus, setAuthStatus])


  useEffect(() => {
    const user = sessionQuery.data
    if (!user || (authStatus !== 'idle' && authStatus !== 'checking')) return
    if (lastUserIdRef.current && lastUserIdRef.current !== user.id) {
      queryClient.removeQueries({
        predicate: (query) => query.queryKey[0] !== 'auth',
      })
    }
    lastUserIdRef.current = user.id
    setSession({
      userId: user.id,
      username: user.username,
      name: user.name,
      role: user.role,
      examTicket: user.examTicket,
    })
  }, [authStatus, queryClient, sessionQuery.data, setSession])

  useEffect(() => {
    const error = sessionQuery.error
    if (!error) return
    if (error instanceof ApiError && error.status === 401) {
      lastUserIdRef.current = null
      queryClient.clear()
      logout()
      return
    }
    setAuthStatus('unavailable')
  }, [queryClient, sessionQuery.error, logout, setAuthStatus])

  if (authStatus === 'unavailable') {
    return (
      <div className="mx-auto flex min-h-[60vh] max-w-xl items-center justify-center px-4">
        <section
          role="alert"
          className="w-full rounded-3xl border border-amber-200 bg-[#fffefa] p-8 text-center shadow-[0_24px_80px_-48px_rgba(20,40,61,0.45)]"
        >
          <span className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-amber-50 text-amber-700">
            <ServerCrash className="h-6 w-6" />
          </span>
          <h1 className="font-display mt-5 text-2xl font-semibold tracking-[-0.03em] text-[#14283d]">
            暂时无法确认登录状态
          </h1>
          <p className="mt-2 text-sm leading-6 text-slate-500">
            服务可能正在重启或网络暂时中断。你的本机草稿仍然保留，请勿关闭考场页面。
          </p>
          <Button
            className="mt-6"
            loading={sessionQuery.isFetching}
            onClick={() => {
              setAuthStatus('idle')
              void queryClient.resetQueries({ queryKey: ['auth', 'me'] })
              void sessionQuery.refetch()
            }}
          >
            重新连接
          </Button>
        </section>
      </div>
    )
  }

  return <>{children}</>
}
