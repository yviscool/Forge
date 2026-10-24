import type { Problem } from '../../api/client'

export const MAX_SOURCE_BYTES = 64 * 1024
export const MAX_FOLDER_FILES = 512
export const MAX_FOLDER_BYTES = 8 * 1024 * 1024
export const MAX_FOLDER_DEPTH = 8

export type SupportedLanguage = 'cpp' | 'c' | 'python' | 'go'

export interface LanguageOption {
  value: SupportedLanguage
  label: string
  editor: string
}

export const LANGUAGE_OPTIONS: LanguageOption[] = [
  { value: 'cpp', label: 'C++ 14 (g++)', editor: 'cpp' },
  { value: 'c', label: 'C (gcc)', editor: 'c' },
  { value: 'python', label: 'Python 3', editor: 'python' },
  { value: 'go', label: 'Go', editor: 'go' },
]

const languageAliases: Record<string, SupportedLanguage> = {
  cpp: 'cpp',
  'c++': 'cpp',
  cxx: 'cpp',
  cc: 'cpp',
  c: 'c',
  py: 'python',
  python: 'python',
  python3: 'python',
  go: 'go',
  golang: 'go',
}

const extensionLanguages: Record<string, SupportedLanguage> = {
  '.cpp': 'cpp',
  '.cc': 'cpp',
  '.cxx': 'cpp',
  '.c': 'c',
  '.py': 'python',
  '.go': 'go',
}

export const SOURCE_ACCEPT =
  '.cpp,.cc,.cxx,.c,.py,.go,text/x-c++src,text/x-csrc,text/x-python,text/x-go'

export function normalizeLanguage(value: string | undefined | null): SupportedLanguage | null {
  if (!value) return null
  return languageAliases[value.trim().toLowerCase()] ?? null
}

export function languageLabel(value: string): string {
  const language = normalizeLanguage(value)
  return LANGUAGE_OPTIONS.find((item) => item.value === language)?.label ?? value
}

export function editorLanguage(value: string): string {
  const normalized = normalizeLanguage(value)
  // Monaco's C and C++ modes share the `cpp` language id. Returning `c`
  // leaves the model in plaintext and breaks highlighting for .c sources.
  return normalized === 'c' ? 'cpp' : normalized ?? 'plaintext'
}

export function languageFromFilename(filename: string): SupportedLanguage | null {
  const dot = filename.lastIndexOf('.')
  if (dot < 0) return null
  return extensionLanguages[filename.slice(dot).toLowerCase()] ?? null
}

export function sourceExtension(language: SupportedLanguage): string {
  switch (language) {
    case 'c':
      return 'c'
    case 'python':
      return 'py'
    case 'go':
      return 'go'
    default:
      return 'cpp'
  }
}

export function allowedLanguageOptions(contestLanguages?: string[]): LanguageOption[] {
  if (!contestLanguages?.length) return LANGUAGE_OPTIONS
  const allowed = new Set(
    contestLanguages.map(normalizeLanguage).filter((value): value is SupportedLanguage => !!value)
  )
  // A non-empty server allowlist is authoritative. If it names only
  // unsupported languages, fail closed instead of re-enabling every language.
  return LANGUAGE_OPTIONS.filter((item) => allowed.has(item.value))
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
}

export function byteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

export function stripCodeFence(value: string): string {
  const trimmed = value.trim()
  const match = trimmed.match(/^```[a-zA-Z0-9_+-]*\s*\n([\s\S]*?)\n?```$/)
  return match ? match[1] : value
}

export async function readSourceFile(file: File): Promise<string> {
  if (file.size > MAX_SOURCE_BYTES) {
    throw new Error(
      `源文件 ${formatBytes(file.size)} 超过单文件上限 ${formatBytes(MAX_SOURCE_BYTES)}。`
    )
  }
  const text = stripCodeFence(await file.text())
  if (!text.trim()) throw new Error('源文件为空，请选择包含代码的文件。')
  return text
}

export interface FolderSelection {
  root: string
  sourceCount: number
  totalCount: number
  totalBytes: number
  files: File[]
  warnings: string[]
}

function relativePath(file: File): string {
  const candidate = (file as File & { webkitRelativePath?: string }).webkitRelativePath
  return candidate || file.name
}

export function folderDepth(path: string): number {
  // The first segment is the selected root and the last is the file itself;
  // depth counts only directories nested below the root.
  return Math.max(0, path.split('/').filter(Boolean).length - 2)
}

export function selectSourceFolder(fileList: FileList | File[]): FolderSelection {
  const files = Array.from(fileList)
  if (!files.length) throw new Error('文件夹为空，请重新选择。')

  const paths = files.map(relativePath)
  const withoutPaths = paths.every((path) => !path.includes('/'))
  if (withoutPaths) {
    throw new Error('当前浏览器未返回文件夹结构，请使用 Chrome / Edge 重新选择。')
  }

  const roots = new Set(paths.map((path) => path.split('/')[0]))
  if (roots.size !== 1) {
    throw new Error('检测到多个顶层文件夹，请只选择本次考试对应的文件夹。')
  }

  if (files.length > MAX_FOLDER_FILES) {
    throw new Error(`文件夹包含 ${files.length} 个文件，超过 ${MAX_FOLDER_FILES} 个上限。`)
  }

  const totalBytes = files.reduce((sum, file) => sum + file.size, 0)
  if (totalBytes > MAX_FOLDER_BYTES) {
    throw new Error(
      `文件夹共 ${formatBytes(totalBytes)}，超过 ${formatBytes(MAX_FOLDER_BYTES)} 上限。`
    )
  }

  const deepest = paths.reduce((max, path) => Math.max(max, folderDepth(path)), 0)
  if (deepest > MAX_FOLDER_DEPTH) {
    throw new Error(`文件夹层级过深，最多支持 ${MAX_FOLDER_DEPTH} 层目录。`)
  }

  const warnings: string[] = []
  const sourceFiles = files.filter((file) => {
    const language = languageFromFilename(file.name)
    if (!language) return false
    if (file.size > MAX_SOURCE_BYTES) {
      warnings.push(`${relativePath(file)} 超过 64 KB，已跳过。`)
      return false
    }
    return true
  })

  if (!sourceFiles.length) {
    throw new Error('文件夹中没有可识别的源码文件（.cpp / .c / .py / .go）。')
  }

  return {
    root: [...roots][0],
    sourceCount: sourceFiles.length,
    totalCount: files.length,
    totalBytes,
    files: sourceFiles,
    warnings,
  }
}

export interface FolderFileMatch {
  file: File
  path: string
  problem: Problem | null
  language: SupportedLanguage | null
}

function basename(path: string): string {
  return path.split('/').pop() || path
}

function stem(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return (dot > 0 ? filename.slice(0, dot) : filename).toLowerCase()
}

export function matchFolderToProblems(
  selection: FolderSelection,
  problems: Problem[]
): FolderFileMatch[] {
  const byCode = new Map<string, Problem>()
  problems.forEach((problem) => {
    byCode.set(problem.code.trim().toLowerCase(), problem)
    byCode.set(stem(problem.code), problem)
    // CSP 收题口径：目录与源文件名用题面英文名（poker/poker.cpp），
    // 学生按 <准考证号>/<英文名>/ 整理的真实文件夹要能按 slug 认出题目。
    const slug = problem.slug?.trim().toLowerCase()
    if (slug) {
      byCode.set(slug, problem)
    }
  })

  return selection.files.map((file) => {
    const path = relativePath(file)
    const parts = path.split('/').map((part) => part.toLowerCase())
    const name = basename(path)
    const fileStem = stem(name)
    let problem: Problem | null = null

    // Prefer an exact problem code in the path, then the source filename.
    for (let index = parts.length - 2; index >= 0; index -= 1) {
      const candidate = byCode.get(parts[index])
      if (candidate) {
        problem = candidate
        break
      }
    }
    problem ??= byCode.get(fileStem) ?? null

    return {
      file,
      path,
      problem,
      language: languageFromFilename(name),
    }
  })
}

