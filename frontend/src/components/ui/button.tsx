import React from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** accent 只给一屏内唯一的主动作（进入考场 / 提交答题卡 / 新建比赛）；
      其余操作一律 primary（墨色）或更轻的变体，橙色不是装饰是信号。 */
  variant?: 'primary' | 'accent' | 'secondary' | 'outline' | 'danger' | 'ghost' | 'success'
  size?: 'sm' | 'md' | 'lg' | 'icon'
  loading?: boolean
}

const base =
  'inline-flex select-none items-center justify-center whitespace-nowrap rounded-[10px] font-semibold transition-[background-color,color,border-color,box-shadow,transform] duration-150 ease-[cubic-bezier(0.25,0,0,1)] focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-forge-700/15 disabled:pointer-events-none disabled:opacity-50 active:scale-[0.985]'

const variants = {
  primary:
    'bg-ink text-white shadow-[0_1px_2px_rgba(19,28,43,0.10)] hover:bg-[#0f2233]',
  accent:
    'bg-forge-700 text-white shadow-[0_1px_2px_rgba(19,28,43,0.10),0_8px_18px_-10px_rgba(194,65,12,0.55)] hover:bg-forge-800',
  secondary: 'bg-navy-50 text-navy-900 hover:bg-navy-100',
  outline:
    'border border-[#ccd5df] bg-white text-slate-700 shadow-[0_1px_1px_rgba(19,28,43,0.03)] hover:border-[#aab6c2] hover:bg-slate-50',
  danger: 'bg-[#b42318] text-white shadow-sm hover:bg-[#912018]',
  success: 'bg-[#0e7a5f] text-white shadow-sm hover:bg-[#0a614c]',
  ghost: 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
}

const sizes = {
  sm: 'h-8 gap-1.5 px-3 text-xs',
  md: 'h-10 gap-2 px-4 text-sm',
  lg: 'h-12 gap-2.5 px-5 text-base',
  icon: 'h-9 w-9 p-0',
}

export function Button({
  className,
  variant = 'primary',
  size = 'md',
  loading = false,
  disabled,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      type={props.type ?? 'button'}
      className={twMerge(clsx(base, variants[variant], sizes[size], className))}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading && (
        <span
          aria-hidden="true"
          className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
        />
      )}
      {children}
    </button>
  )
}
