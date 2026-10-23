import React, { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { clsx } from 'clsx'

interface DialogProps {
  open: boolean
  onClose: () => void
  title: string
  eyebrow?: string
  children: React.ReactNode
  footer?: React.ReactNode
  width?: 'sm' | 'md' | 'lg' | 'xl'
  dismissible?: boolean
}

const widths = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-3xl',
  xl: 'max-w-5xl',
}

export function Dialog({
  open,
  onClose,
  title,
  eyebrow,
  children,
  footer,
  width = 'md',
  dismissible = true,
}: DialogProps) {
  const titleId = useId()
  const panelRef = useRef<HTMLElement>(null)
  const onCloseRef = useRef(onClose)
  const dismissibleRef = useRef(dismissible)
  onCloseRef.current = onClose
  dismissibleRef.current = dismissible

  useEffect(() => {
    if (!open) return
    const previousOverflow = document.body.style.overflow
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const appRoot = document.getElementById('root')
    document.body.style.overflow = 'hidden'
    appRoot?.setAttribute('inert', '')

    const focusableSelector = [
      'a[href]',
      'button:not([disabled])',
      'input:not([disabled])',
      'select:not([disabled])',
      'textarea:not([disabled])',
      '[tabindex]:not([tabindex="-1"])',
    ].join(',')
    const focusable = () =>
      Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) || []).filter(
        (element) => element.getAttribute('aria-hidden') !== 'true'
      )

    const timer = window.setTimeout(() => {
      const first = focusable()[0]
      ;(first || panelRef.current)?.focus()
    }, 0)
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && dismissibleRef.current) {
        event.preventDefault()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const elements = focusable()
      if (!elements.length) {
        event.preventDefault()
        panelRef.current?.focus()
        return
      }
      const first = elements[0]
      const last = elements[elements.length - 1]
      if (event.shiftKey && (document.activeElement === first || !panelRef.current?.contains(document.activeElement))) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (document.activeElement === last || !panelRef.current?.contains(document.activeElement))) {
        event.preventDefault()
        first.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.clearTimeout(timer)
      document.body.style.overflow = previousOverflow
      appRoot?.removeAttribute('inert')
      window.removeEventListener('keydown', onKey)
      if (previousFocus?.isConnected) previousFocus.focus()
    }
  }, [open])

  if (!open || typeof document === 'undefined') return null

  return createPortal(
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center overflow-y-auto bg-[#14283d]/45 p-4 backdrop-blur-[3px] animate-fade"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && dismissible) onClose()
      }}
    >
      <section
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={clsx(
          'my-auto w-full overflow-hidden rounded-3xl border border-white/50 bg-[#fffefa] shadow-[0_32px_80px_-24px_rgba(20,40,61,0.5)] animate-rise',
          widths[width]
        )}
      >
        <header className="flex items-start justify-between gap-4 border-b border-[#eaecf0] px-6 py-5">
          <div>
            {eyebrow && <p className="app-eyebrow mb-1.5">{eyebrow}</p>}
            <h2 id={titleId} className="text-xl font-extrabold tracking-tight text-[#14283d]">
              {title}
            </h2>
          </div>
          {dismissible && (
            <button
              type="button"
              aria-label="关闭"
              onClick={onClose}
              className="rounded-xl p-2 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
            >
              <X className="h-5 w-5" />
            </button>
          )}
        </header>
        <div className="max-h-[calc(100vh-14rem)] overflow-y-auto px-6 py-5">{children}</div>
        {footer && (
          <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-[#eaecf0] bg-[#fbfaf6] px-6 py-4">
            {footer}
          </footer>
        )}
      </section>
    </div>,
    document.body
  )
}
