import React from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...props }, ref) {
    return (
      <input
        ref={ref}
        className={twMerge(clsx('app-field', className))}
        {...props}
      />
    )
  }
)

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className, ...props }, ref) {
  return <textarea ref={ref} className={twMerge(clsx('app-textarea', className))} {...props} />
})

export const Select = React.forwardRef<
  HTMLSelectElement,
  React.SelectHTMLAttributes<HTMLSelectElement>
>(function Select({ className, children, ...props }, ref) {
  return (
    <select ref={ref} className={twMerge(clsx('app-select', className))} {...props}>
      {children}
    </select>
  )
})

export function Field({
  label,
  hint,
  error,
  children,
  className,
}: {
  label: string
  hint?: string
  error?: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <label className={clsx('block space-y-1.5', className)}>
      <span className="flex items-baseline justify-between gap-3">
        <span className="text-xs font-bold text-slate-700">{label}</span>
        {hint && <span className="text-[11px] font-medium text-slate-400">{hint}</span>}
      </span>
      {children}
      {error && <span className="block text-[11px] font-medium text-rose-600">{error}</span>}
    </label>
  )
}
