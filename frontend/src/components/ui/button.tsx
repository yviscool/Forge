import React from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost' | 'success'
  size?: 'sm' | 'md' | 'lg' | 'icon'
  loading?: boolean
}

const base =
  'inline-flex select-none items-center justify-center whitespace-nowrap rounded-xl font-semibold transition-[background-color,color,border-color,box-shadow,transform] duration-150 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-forge-700/15 disabled:pointer-events-none disabled:opacity-50 active:translate-y-px'

const variants = {
  primary:
    'bg-forge-700 text-white shadow-[0_1px_2px_rgba(16,24,40,0.08),0_8px_18px_-10px_rgba(194,65,12,0.65)] hover:bg-forge-800',
  secondary: 'bg-navy-50 text-navy-900 hover:bg-navy-100',
  outline:
    'border border-[#d0d5dd] bg-white text-slate-700 shadow-[0_1px_1px_rgba(16,24,40,0.03)] hover:border-[#b8c0cc] hover:bg-slate-50',
  danger: 'bg-[#b42318] text-white shadow-sm hover:bg-[#912018]',
  success: 'bg-[#087443] text-white shadow-sm hover:bg-[#075c37]',
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
