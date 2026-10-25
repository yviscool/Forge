import React, { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { editorLanguage } from '../features/student/submissionFiles'
import { ensureMonacoLanguage } from './monacoLanguages'

interface MonacoEditorProps {
  value: string
  onChange?: (val: string) => void
  language?: string
  readOnly?: boolean
  className?: string
  height?: string
  ariaLabel?: string
  /** 编辑器实例句柄：Geany 复刻的右键菜单（撤销/剪切/粘贴等）通过它触发命令。 */
  onEditorReady?: (editor: monaco.editor.IStandaloneCodeEditor) => void
  /** 光标位置变化：Geany 状态栏的 行/列 实时显示。 */
  onCursorChange?: (line: number, column: number) => void
  fontFamily?: string
  fontSize?: number
  /** 去掉默认的圆角描边（嵌入 Geany 窗口时使用）。 */
  chromeless?: boolean
}

export function MonacoEditor({
  value,
  onChange,
  language = 'cpp',
  readOnly = false,
  className = '',
  height = '420px',
  ariaLabel = '代码编辑器',
  onEditorReady,
  onCursorChange,
  fontFamily = 'Consolas, "Fira Code", monospace',
  fontSize = 13,
  chromeless = false,
}: MonacoEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  const onChangeRef = useRef(onChange)
  const readyRef = useRef(onEditorReady)
  const cursorRef = useRef(onCursorChange)
  const suppressChangeRef = useRef(false)

  // Create the imperative editor once; subsequent effects keep its value,
  // language, and read-only option in sync with React state.
  useEffect(() => {
    onChangeRef.current = onChange
  }, [onChange])

  useEffect(() => {
    readyRef.current = onEditorReady
    cursorRef.current = onCursorChange
  }, [onEditorReady, onCursorChange])

  useEffect(() => {
    if (!containerRef.current) return

    const editor = monaco.editor.create(containerRef.current, {
      value,
      language: editorLanguage(language),
      theme: 'vs',
      readOnly,
      automaticLayout: true,
      fontSize,
      fontFamily,
      lineNumbers: 'on',
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      tabSize: 4,
      ariaLabel,
    })
    editorRef.current = editor
    readyRef.current?.(editor)
    void ensureMonacoLanguage(language)

    const sub = editor.onDidChangeModelContent(() => {
      if (!suppressChangeRef.current) onChangeRef.current?.(editor.getValue())
    })

    // 测试用的 monaco mock 未必实现光标事件，做可选调用。
    const cursorSub = editor.onDidChangeCursorPosition?.((event) => {
      cursorRef.current?.(event.position.lineNumber, event.position.column)
    })

    return () => {
      sub.dispose()
      cursorSub?.dispose()
      editor.dispose()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Sync external value changes
  useEffect(() => {
    if (editorRef.current && editorRef.current.getValue() !== value) {
      suppressChangeRef.current = true
      editorRef.current.setValue(value)
      queueMicrotask(() => {
        suppressChangeRef.current = false
      })
    }
  }, [value])

  // Sync language changes
  useEffect(() => {
    let cancelled = false
    void ensureMonacoLanguage(language).then((id) => {
      if (cancelled) return
      const model = editorRef.current?.getModel()
      if (model) monaco.editor.setModelLanguage(model, id)
    })
    return () => {
      cancelled = true
    }
  }, [language])

  useEffect(() => {
    editorRef.current?.updateOptions({ readOnly })
  }, [readOnly])

  useEffect(() => {
    editorRef.current?.updateOptions({ fontSize, fontFamily })
  }, [fontSize, fontFamily])

  return (
    <div
      ref={containerRef}
      style={{ height }}
      className={`w-full overflow-hidden ${
        chromeless ? '' : 'rounded-lg border border-slate-200'
      } ${className}`}
    />
  )
}
