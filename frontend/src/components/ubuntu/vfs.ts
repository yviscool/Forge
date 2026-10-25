// 考场桌面的虚拟文件系统（纯函数）：Nautilus 的文件夹操作、Geany 的文档、
// 终端的 ls/cat 都消费同一份 VFS 状态。题目目录与其源文件是「派生视图」
// （绑定题目草稿），不在 VFS 里存储；VFS 只持有考生自建的文件夹与文件。
export interface VfsItem {
  name: string
  kind: 'dir' | 'file'
  /** 文件内容（仅 file）。 */
  content?: string
  /** 只读文件（样例输入输出）：Geany 以只读标签打开。 */
  readonly?: boolean
}

export type VfsDirMap = Record<string, VfsItem[]>

export function listDir(items: VfsItem[] | undefined): VfsItem[] {
  const list = items ? [...items] : []
  list.sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1
    return a.name.localeCompare(b.name, 'zh-CN')
  })
  return list
}

export function findItem(items: VfsItem[] | undefined, name: string): VfsItem | null {
  return items?.find((item) => item.name === name) ?? null
}

export interface VfsResult {
  items: VfsItem[]
  ok: boolean
  error?: string
}

function done(items: VfsItem[]): VfsResult {
  return { items, ok: true }
}
function fail(items: VfsItem[], error: string): VfsResult {
  return { items, ok: false, error }
}

/** 新建文件夹：同名（文件或文件夹）冲突拒绝。 */
export function mkdir(items: VfsItem[], name: string): VfsResult {
  const trimmed = name.trim()
  if (!trimmed) return fail(items, '名称不能为空')
  if (trimmed.includes('/') || trimmed.includes('\\')) return fail(items, '名称不能包含路径分隔符')
  if (items.some((item) => item.name === trimmed)) return fail(items, `「${trimmed}」已存在`)
  return done([...items, { name: trimmed, kind: 'dir' }])
}

/** 新建空文档。 */
export function mkfile(items: VfsItem[], name: string, content = ''): VfsResult {
  const trimmed = name.trim()
  if (!trimmed) return fail(items, '名称不能为空')
  if (items.some((item) => item.name === trimmed)) return fail(items, `「${trimmed}」已存在`)
  return done([...items, { name: trimmed, kind: 'file', content }])
}

/** 重命名：保留原内容；冲突拒绝。 */
export function renameItem(items: VfsItem[], name: string, newName: string): VfsResult {
  const trimmed = newName.trim()
  if (!trimmed) return fail(items, '名称不能为空')
  if (items.some((item) => item.name === trimmed && item.name !== name)) {
    return fail(items, `「${trimmed}」已存在`)
  }
  const index = items.findIndex((item) => item.name === name)
  if (index < 0) return fail(items, `「${name}」不存在`)
  const next = [...items]
  next[index] = { ...next[index], name: trimmed }
  return done(next)
}

/**
 * 移到回收站：摘除节点本身；若为目录，其子树（所有以 `${dirKey}/${name}/`
 * 为前缀的 VFS 键）由调用方一并摘除挂到 trash 记录上——恢复时原样放回，
 * 清空时真正删除。否则子树会成为不可见也清不掉的孤儿数据。
 */
export function trashItem(items: VfsItem[], name: string): { items: VfsItem[]; removed: VfsItem | null } {
  const removed = items.find((item) => item.name === name) ?? null
  return { items: items.filter((item) => item.name !== name), removed }
}

/** 摘除某目录的整棵子树：返回摘除后的表与被摘下的子树键值对。 */
export function detachSubtree(
  files: VfsDirMap,
  dirKey: string,
  name: string
): { files: VfsDirMap; subtree: VfsDirMap } {
  // 目录的直接内容键 = `${dirKey}/${name}`（无尾斜杠）；更深子键带 `/` 前缀。
  const nodeKey = dirKey ? `${dirKey}/${name}` : name
  const subtree: VfsDirMap = {}
  const next: VfsDirMap = {}
  for (const [key, value] of Object.entries(files)) {
    if (key === nodeKey) {
      subtree[''] = value
    } else if (key.startsWith(`${nodeKey}/`)) {
      subtree[key.slice(nodeKey.length + 1)] = value
    } else {
      next[key] = value
    }
  }
  return { files: next, subtree }
}

/** 把子树挂回某目录之下（恢复用）。 */
export function attachSubtree(files: VfsDirMap, dirKey: string, name: string, subtree: VfsDirMap): VfsDirMap {
  const nodeKey = dirKey ? `${dirKey}/${name}` : name
  const next: VfsDirMap = { ...files }
  for (const [key, value] of Object.entries(subtree)) {
    next[key === '' ? nodeKey : `${nodeKey}/${key}`] = value
  }
  return next
}

/** 从回收站恢复。 */
export function restoreItem(items: VfsItem[], removed: VfsItem): VfsResult {
  if (items.some((item) => item.name === removed.name)) {
    return fail(items, `「${removed.name}」已存在，无法恢复`)
  }
  return done([...items, removed])
}

/** 写文件内容（Geany 保存 VFS 文档）。 */
export function writeFile(items: VfsItem[], name: string, content: string): VfsResult {
  const index = items.findIndex((item) => item.name === name)
  if (index < 0) return fail(items, `「${name}」不存在`)
  if (items[index].readonly) return fail(items, `「${name}」是只读文件`)
  const next = [...items]
  next[index] = { ...next[index], content }
  return done(next)
}

/** 未命名文档命名：无标题.cpp、无标题2.cpp… 直到不与现有条目冲突。 */
export function uniqueUntitledName(items: VfsItem[], ext = '.cpp'): string {
  let index = 1
  let name = `无标题${index > 1 ? index : ''}${ext}`
  while (items.some((item) => item.name === name)) {
    index += 1
    name = `无标题${index}${ext}`
  }
  return name
}

/** 路径 key（相对考生主目录）：'' 表示主目录本身，'poker/sub' 表示嵌套。 */
export function dirKey(path: string[]): string {
  return path.join('/')
}
