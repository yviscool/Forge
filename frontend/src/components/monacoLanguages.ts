import 'monaco-editor/esm/vs/basic-languages/cpp/cpp.contribution'
import { editorLanguage } from '../features/student/submissionFiles'

const languageLoaders: Record<string, () => Promise<unknown>> = {
  go: () => import('monaco-editor/esm/vs/basic-languages/go/go.contribution'),
  python: () => import('monaco-editor/esm/vs/basic-languages/python/python.contribution'),
}

const loaded = new Set<string>(['cpp', 'plaintext'])
const pending = new Map<string, Promise<void>>()

/**
 * Loads only the Monaco grammar needed by the selected source language. This
 * keeps Python's multi-megabyte contribution off the initial editor path.
 */
export async function ensureMonacoLanguage(language: string): Promise<string> {
  const id = editorLanguage(language)
  if (loaded.has(id)) return id
  const loader = languageLoaders[id]
  if (!loader) return id
  let promise = pending.get(id)
  if (!promise) {
    promise = loader().then(() => {
      loaded.add(id)
      pending.delete(id)
    })
    pending.set(id, promise)
  }
  await promise
  return id
}
