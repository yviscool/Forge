import React, { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from 'lucide-react'
import { clsx } from 'clsx'

type ToastTone = 'success' | 'error' | 'info' | 'warning'

interface ToastItem {
  id: number
  tone: ToastTone
  title: string
  message?: string
}

interface ToastApi {
  notify: (tone: ToastTone, title: string, message?: string) => void
  success: (title: string, message?: string) => void
  error: (title: string, message?: string) => void
  warning: (title: string, message?: string) => void
}

const ToastContext = createContext<ToastApi | null>(null)

const toneStyle: Record<ToastTone, string> = {
  success: 'border-emerald-200 bg-emerald-50 text-emerald-900',
  error: 'border-rose-200 bg-rose-50 text-rose-900',
  info: 'border-sky-200 bg-sky-50 text-sky-900',
  warning: 'border-amber-200 bg-amber-50 text-amber-900',
}

function ToneIcon({ tone }: { tone: ToastTone }) {
  const className = 'h-5 w-5 shrink-0'
  if (tone === 'success') return <CheckCircle2 className={clsx(className, 'text-emerald-600')} />
  if (tone === 'error') return <XCircle className={clsx(className, 'text-rose-600')} />
  if (tone === 'warning') return <AlertTriangle className={clsx(className, 'text-amber-600')} />
  return <Info className={clsx(className, 'text-sky-600')} />
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])

  const remove = useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id))
  }, [])

  const notify = useCallback(
    (tone: ToastTone, title: string, message?: string) => {
      const id = Date.now() + Math.random()
      setItems((current) => [...current.slice(-3), { id, tone, title, message }])
      window.setTimeout(() => remove(id), tone === 'error' ? 6500 : 4200)
    },
    [remove]
  )

  const value = useMemo<ToastApi>(
    () => ({
      notify,
      success: (title, message) => notify('success', title, message),
      error: (title, message) => notify('error', title, message),
      warning: (title, message) => notify('warning', title, message),
    }),
    [notify]
  )

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        aria-live="polite"
        aria-atomic="false"
        className="pointer-events-none fixed bottom-5 right-5 z-[200] flex w-[min(92vw,24rem)] flex-col gap-2"
      >
        {items.map((item) => (
          <div
            key={item.id}
            className={clsx(
              'pointer-events-auto flex items-start gap-3 rounded-2xl border px-4 py-3.5 shadow-lift animate-rise',
              toneStyle[item.tone]
            )}
          >
            <ToneIcon tone={item.tone} />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-bold">{item.title}</p>
              {item.message && <p className="mt-0.5 text-xs leading-5 opacity-80">{item.message}</p>}
            </div>
            <button
              type="button"
              aria-label="关闭提示"
              onClick={() => remove(item.id)}
              className="rounded-md p-0.5 opacity-50 transition hover:bg-black/5 hover:opacity-100"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
