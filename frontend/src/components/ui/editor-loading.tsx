import { LoaderCircle } from 'lucide-react'

export function EditorLoading({ fullscreen = false }: { fullscreen?: boolean }) {
  return (
    <div
      role="status"
      aria-live="polite"
      className={`grid place-items-center bg-[#ffffff] text-slate-500 ${
        fullscreen ? 'min-h-screen' : 'h-full min-h-[22rem] w-full'
      }`}
    >
      <div className="flex items-center gap-2.5 rounded-xl border border-[#e3e8ee] bg-white px-4 py-3 text-xs font-bold shadow-sm">
        <LoaderCircle className="h-4 w-4 animate-spin text-forge-700" />
        正在加载代码编辑器...
      </div>
    </div>
  )
}
