import React from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function Card({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={twMerge(
        clsx(
          'rounded-2xl border border-[#e3e8ee] bg-white shadow-[0_1px_2px_rgba(19,28,43,0.04),0_14px_36px_-28px_rgba(19,28,43,0.22)]',
          className
        )
      )}
      {...props}
    >
      {children}
    </div>
  )
}

export function CardHeader({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={twMerge(clsx('flex flex-col gap-1 p-5', className))} {...props}>
      {children}
    </div>
  )
}

export function CardTitle({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3
      className={twMerge(clsx('text-base font-extrabold tracking-tight text-[#0f2233]', className))}
      {...props}
    >
      {children}
    </h3>
  )
}

export function CardContent({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={twMerge(clsx('p-5', className))} {...props}>
      {children}
    </div>
  )
}

export function CardFooter({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={twMerge(
        clsx('flex items-center gap-2 border-t border-[#e6ebf0] px-5 py-3.5', className)
      )}
      {...props}
    >
      {children}
    </div>
  )
}
