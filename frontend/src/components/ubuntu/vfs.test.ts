import { describe, expect, it } from 'vitest'
import {
  findItem,
  uniqueUntitledName,
  listDir,
  mkdir,
  mkfile,
  renameItem,
  restoreItem,
  trashItem,
  writeFile,
  type VfsItem,
} from './vfs'

const base: VfsItem[] = [
  { name: 'poker', kind: 'dir' },
  { name: 'b', kind: 'dir' },
  { name: 'poker.cpp', kind: 'file', content: 'int main(){}' },
]

describe('考场桌面 VFS', () => {
  it('listDir 目录排前、名称排序', () => {
    const listed = listDir([
      { name: 'z.cpp', kind: 'file' },
      { name: '子目录', kind: 'dir' },
      { name: 'a.cpp', kind: 'file' },
    ])
    expect(listed.map((i) => i.name)).toEqual(['子目录', 'a.cpp', 'z.cpp'])
  })

  it('mkdir 拒绝空名、路径分隔符与重名', () => {
    expect(mkdir(base, '新建文件夹').ok).toBe(true)
    expect(mkdir(base, '').ok).toBe(false)
    expect(mkdir(base, 'a/b').ok).toBe(false)
    expect(mkdir(base, 'poker').ok).toBe(false)
    expect(mkdir(base, 'poker.cpp').ok).toBe(false)
  })

  it('mkfile 创建空文档且拒绝重名', () => {
    const result = mkfile(base, '笔记.txt', 'hello')
    expect(result.ok).toBe(true)
    expect(findItem(result.items, '笔记.txt')?.content).toBe('hello')
    expect(mkfile(base, 'poker.cpp').ok).toBe(false)
  })

  it('renameItem 保留内容、拒绝冲突', () => {
    const result = renameItem(base, 'poker.cpp', 'sol.cpp')
    expect(result.ok).toBe(true)
    expect(findItem(result.items, 'sol.cpp')?.content).toBe('int main(){}')
    expect(findItem(result.items, 'poker.cpp')).toBeNull()
    expect(renameItem(base, 'poker.cpp', 'b').ok).toBe(false)
  })

  it('trashItem 摘除整棵子树并可恢复', () => {
    const trashed = trashItem(base, 'poker')
    expect(trashed.removed?.name).toBe('poker')
    expect(findItem(trashed.items, 'poker')).toBeNull()
    expect(trashItem(base, '不存在').removed).toBeNull()

    const restored = restoreItem(trashed.items, trashed.removed!)
    expect(restored.ok).toBe(true)
    expect(findItem(restored.items, 'poker')).not.toBeNull()
    expect(restoreItem(base, { name: 'poker', kind: 'dir' }).ok).toBe(false)
  })

  it('uniqueUntitledName 依次避开现有名称', () => {
    expect(uniqueUntitledName(base)).toBe('无标题.cpp')
    const withUntitled: VfsItem[] = [{ name: '无标题.cpp', kind: 'file' }]
    expect(uniqueUntitledName(withUntitled)).toBe('无标题2.cpp')
    expect(uniqueUntitledName(base, '.txt')).toBe('无标题.txt')
  })

  it('writeFile 更新内容并尊重只读', () => {
    const result = writeFile(base, 'poker.cpp', '// new')
    expect(result.ok).toBe(true)
    expect(findItem(result.items, 'poker.cpp')?.content).toBe('// new')
    const ro: VfsItem[] = [{ name: 'poker.in', kind: 'file', content: '6', readonly: true }]
    expect(writeFile(ro, 'poker.in', 'x').ok).toBe(false)
    expect(writeFile(base, 'ghost', 'x').ok).toBe(false)
  })
})
