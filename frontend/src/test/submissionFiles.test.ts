import { describe, expect, it } from 'vitest'
import {
  MAX_FOLDER_BYTES,
  MAX_FOLDER_DEPTH,
  MAX_FOLDER_FILES,
  MAX_SOURCE_BYTES,
  allowedLanguageOptions,
  folderDepth,
  languageFromFilename,
  matchFolderToProblems,
  normalizeLanguage,
  readSourceFile,
  selectSourceFolder,
  stripCodeFence,
  type FolderSelection,
} from '../features/student/submissionFiles'
import type { Problem } from '../api/client'

function fileWithPath(name: string, path: string, content = 'int main(){}'): File {
  const file = new File([content], name, { type: 'text/plain' })
  Object.defineProperty(file, 'webkitRelativePath', {
    configurable: true,
    value: path,
  })
  return file
}

function problem(id: string, code: string, title = code, slug?: string): Problem {
  return {
    id,
    contestId: 'contest-1',
    code,
    slug,
    title,
    statement: '',
    input: '',
    output: '',
    examples: '',
    constraints: '',
    timeLimitMs: 1000,
    memoryLimitMib: 256,
  }
}

function selection(files: File[]): FolderSelection {
  return {
    root: 'exam',
    sourceCount: files.length,
    totalCount: files.length,
    totalBytes: files.reduce((sum, file) => sum + file.size, 0),
    files,
    warnings: [],
  }
}

describe('submission file helpers', () => {
  it('normalizes languages and detects extensions case-insensitively', () => {
    expect(normalizeLanguage('C++')).toBe('cpp')
    expect(normalizeLanguage('python3')).toBe('python')
    expect(normalizeLanguage('golang')).toBe('go')
    expect(normalizeLanguage('rust')).toBeNull()
    expect(languageFromFilename('SOLUTION.CPP')).toBe('cpp')
    expect(languageFromFilename('main.py')).toBe('python')
    expect(languageFromFilename('README.md')).toBeNull()
  })

  it('honors the contest language whitelist and falls back safely', () => {
    expect(allowedLanguageOptions(['cpp', 'python']).map((item) => item.value)).toEqual([
      'cpp',
      'python',
    ])
    expect(allowedLanguageOptions(['rust'])).toEqual([])
  })

  it('rejects oversize, empty and fence-wrapped source files', async () => {
    await expect(
      readSourceFile(new File([new Uint8Array(MAX_SOURCE_BYTES + 1)], 'big.cpp'))
    ).rejects.toThrow(/超过单文件上限/)
    await expect(readSourceFile(new File(['   '], 'empty.cpp'))).rejects.toThrow(/为空/)
    await expect(
      readSourceFile(new File(['```cpp\nint main(){}\n```'], 'fenced.cpp'))
    ).resolves.toBe('int main(){}')
    expect(stripCodeFence('plain code')).toBe('plain code')
  })

  it('validates folder root, limits, depth and source membership', () => {
    expect(
      selectSourceFolder([
        fileWithPath('a.cpp', 'exam/A/a.cpp'),
        fileWithPath('b.py', 'exam/B/b.py'),
      ])
    ).toMatchObject({ root: 'exam', sourceCount: 2, totalCount: 2 })

    expect(() => selectSourceFolder([fileWithPath('a.cpp', 'a.cpp')])).toThrow(
      /未返回文件夹结构/
    )
    expect(() =>
      selectSourceFolder([
        fileWithPath('a.cpp', 'one/a.cpp'),
        fileWithPath('b.cpp', 'two/b.cpp'),
      ])
    ).toThrow(/多个顶层文件夹/)
    expect(() =>
      selectSourceFolder([
        fileWithPath('a.cpp', 'exam/a.cpp'),
        fileWithPath('notes.txt', 'exam/notes.txt'),
      ])
    ).not.toThrow()
    expect(() =>
      selectSourceFolder([fileWithPath('notes.txt', 'exam/notes.txt')])
    ).toThrow(/没有可识别的源码文件/)
  })

  it('computes path depth relative to the selected root', () => {
    expect(folderDepth('exam/a.cpp')).toBe(0)
    expect(folderDepth('exam/A/sub/a.cpp')).toBe(2)
    expect(MAX_FOLDER_DEPTH).toBe(8)
    expect(MAX_FOLDER_FILES).toBe(512)
    expect(MAX_FOLDER_BYTES).toBe(8 * 1024 * 1024)
  })

  it('matches problem codes from directories before falling back to filenames', () => {
    const problems = [problem('p-a', 'A'), problem('p-b', 'B'), problem('p-c', 'C')]
    const matched = matchFolderToProblems(
      selection([
        fileWithPath('solution.cpp', 'exam/B/solution.cpp'),
        fileWithPath('A.cpp', 'exam/random/A.cpp'),
        fileWithPath('z.cpp', 'exam/misc/unmatched.cpp'),
      ]),
      problems
    )

    expect(matched[0].problem?.id).toBe('p-b')
    expect(matched[1].problem?.id).toBe('p-a')
    expect(matched[2].problem).toBeNull()
    expect(matched.every((entry) => entry.language === 'cpp')).toBe(true)
  })

  it('matches CSP slug directories and slug-named source files', () => {
    // 真实考点按 <准考证号>/<英文名>/<英文名>.cpp 组织源码：
    // 目录段和文件名都要能按题面英文名认出题目。
    const problems = [
      problem('p-poker', 'A', '扑克牌', 'poker'),
      problem('p-struct', 'B', '结构体', 'struct'),
    ]
    const matched = matchFolderToProblems(
      selection([
        fileWithPath('poker.cpp', 'exam/HA-J00001/poker/poker.cpp'),
        fileWithPath('struct.cpp', 'exam/HA-J00001/struct/struct.cpp'),
        fileWithPath('miss.cpp', 'exam/HA-J00001/typo/typo.cpp'),
      ]),
      problems
    )

    expect(matched[0].problem?.id).toBe('p-poker')
    expect(matched[1].problem?.id).toBe('p-struct')
    expect(matched[2].problem).toBeNull()
  })
})
