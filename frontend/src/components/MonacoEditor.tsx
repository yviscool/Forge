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
}

export function MonacoEditor({
  value,
  onChange,
  language = 'cpp',
  readOnly = false,
  className = '',
  height = '420px',
  ariaLabel = '代码编辑器',
}: MonacoEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  const onChangeRef = useRef(onChange)
  const suppressChangeRef = useRef(false)

  // Create the imperative editor once; subsequent effects keep its value,
  // language, and read-only option in sync with React state.
  useEffect(() => {
    onChangeRef.current = onChange
  }, [onChange])

  useEffect(() => {
    if (!containerRef.current) return

    const editor = monaco.editor.create(containerRef.current, {
      value,
      language: editorLanguage(language),
      theme: 'vs',
      readOnly,
      automaticLayout: true,
      fontSize: 13,
      lineNumbers: 'on',
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      tabSize: 4,
      fontFamily: 'Consolas, "Fira Code", monospace',
      ariaLabel,
    })
    editorRef.current = editor
    void ensureMonacoLanguage(language)

    const sub = editor.onDidChangeModelContent(() => {
      if (!suppressChangeRef.current) onChangeRef.current?.(editor.getValue())
    })

    return () => {
      sub.dispose()
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

  return (
    <div
      ref={containerRef}
      style={{ height }}
      className={`w-full rounded-lg border border-slate-200 overflow-hidden ${className}`}
    />
  )
}
