import React from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { Button } from '../components/ui/button'

interface State {
  failed: boolean
}

export class ErrorBoundary extends React.Component<React.PropsWithChildren, State> {
  state: State = { failed: false }

  static getDerivedStateFromError(): State {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // Keep diagnostics in the browser console without exposing them to users.
    console.error('Forge UI crashed', error, info.componentStack)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <main className="grid min-h-[70vh] place-items-center px-4 py-12">
        <section className="w-full max-w-xl rounded-3xl border border-rose-200 bg-[#fffefa] p-7 text-center shadow-[0_24px_60px_-38px_rgba(20,40,61,0.45)]">
          <div className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-rose-50 text-rose-700">
            <AlertTriangle className="h-7 w-7" />
          </div>
          <h1 className="mt-5 text-2xl font-extrabold tracking-tight text-[#14283d]">
            页面遇到意外错误
          </h1>
          <p className="mt-2 text-sm leading-6 text-slate-500">
            当前操作尚未确认成功。请刷新页面恢复；若问题持续出现，请联系考场管理员并提供浏览器控制台信息。
          </p>
          <Button className="mt-6 gap-2" onClick={() => window.location.reload()}>
            <RefreshCw className="h-4 w-4" />
            刷新并恢复
          </Button>
        </section>
      </main>
    )
  }
}
