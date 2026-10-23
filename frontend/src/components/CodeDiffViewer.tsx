import React, { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { editorLanguage } from '../features/student/submissionFiles'
import { ensureMonacoLanguage } from './monacoLanguages'

interface CodeDiffViewerProps {
  original: string
  modified: string
  originalTitle?: string
  modifiedTitle?: string
  language?: string
  height?: string
}

export function CodeDiffViewer({
  original,
  modified,
  originalTitle = '上一版本',
  modifiedTitle = '当前版本',
  language = 'cpp',
  height = '480px',
}: CodeDiffViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const diffEditorRef = useRef<monaco.editor.IStandaloneDiffEditor | null>(null)

  // The editor is constructed once. Dedicated effects below synchronize the
  // model contents and language without tearing down the editor on each edit.
  useEffect(() => {
    if (!containerRef.current) return

    const diffEditor = monaco.editor.createDiffEditor(containerRef.current, {
      readOnly: true,
      automaticLayout: true,
      fontSize: 13,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      renderSideBySide: true,
      fontFamily: 'Consolas, "Fira Code", monospace',
    })
    diffEditorRef.current = diffEditor

    const origModel = monaco.editor.createModel(original, editorLanguage(language))
    const modModel = monaco.editor.createModel(modified, editorLanguage(language))
    diffEditor.setModel({ original: origModel, modified: modModel })
    void ensureMonacoLanguage(language)

    return () => {
      origModel.dispose()
      modModel.dispose()
      diffEditor.dispose()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (diffEditorRef.current) {
      const model = diffEditorRef.current.getModel()
      if (model) {
        if (model.original.getValue() !== original) {
          model.original.setValue(original)
        }
        if (model.modified.getValue() !== modified) {
          model.modified.setValue(modified)
        }
      }
    }
  }, [original, modified])

  useEffect(() => {
    let cancelled = false
    void ensureMonacoLanguage(language).then((id) => {
      if (cancelled) return
      const model = diffEditorRef.current?.getModel()
      if (!model) return
      monaco.editor.setModelLanguage(model.original, id)
      monaco.editor.setModelLanguage(model.modified, id)
    })
    return () => {
      cancelled = true
    }
  }, [language])

  return (
    <div className="flex flex-col border border-slate-200 rounded-lg overflow-hidden bg-white shadow-sm">
      <div className="flex items-center justify-between px-4 py-2 bg-slate-100 border-b border-slate-200 text-xs font-semibold text-slate-600">
        <span className="flex items-center gap-1.5 text-rose-700">
          <span className="w-2 h-2 rounded-full bg-rose-500 inline-block"></span>
          {originalTitle} (旧)
        </span>
        <span className="flex items-center gap-1.5 text-emerald-700">
          <span className="w-2 h-2 rounded-full bg-emerald-500 inline-block"></span>
          {modifiedTitle} (新)
        </span>
      </div>
      <div ref={containerRef} style={{ height }} className="w-full" />
    </div>
  )
}
