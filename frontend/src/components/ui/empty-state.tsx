import React from 'react'
import { AlertCircle, Loader2, type LucideIcon } from 'lucide-react'

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon?: LucideIcon
  title: string
  description?: string
  action?: React.ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-[#ccd5df] bg-white/60 px-6 py-14 text-center">
      {Icon && (
        <div className="mb-4 grid h-12 w-12 place-items-center rounded-2xl bg-forge-50 text-forge-700">
          <Icon className="h-6 w-6" />
        </div>
      )}
      <h3 className="text-base font-extrabold text-[#0f2233]">{title}</h3>
      {description && <p className="mt-1.5 max-w-md text-sm leading-6 text-slate-500">{description}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}

export function LoadingBlock({ label = '正在加载...' }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2.5 rounded-2xl border border-[#e3e8ee] bg-white/70 py-14 text-sm font-medium text-slate-500">
      <Loader2 className="h-4 w-4 animate-spin text-forge-700" />
      {label}
    </div>
  )
}

export function ErrorState({ message, action }: { message: string; action?: React.ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-2xl border border-rose-200 bg-rose-50/60 px-6 py-12 text-center">
      <AlertCircle className="mb-3 h-7 w-7 text-rose-500" />
      <p className="max-w-lg text-sm font-semibold text-rose-800">{message}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export function InlineError({
  title = '数据加载失败',
  message,
  action,
  className = '',
}: {
  title?: string
  message: string
  action?: React.ReactNode
  className?: string
}) {
  return (
    <div
      role="alert"
      className={`flex flex-col gap-3 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-rose-800 sm:flex-row sm:items-center ${className}`}
    >
      <AlertCircle className="h-4 w-4 shrink-0 text-rose-500" />
      <div className="min-w-0 flex-1">
        <p className="text-xs font-extrabold">{title}</p>
        <p className="mt-0.5 break-words text-xs leading-5 text-rose-700">{message}</p>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
