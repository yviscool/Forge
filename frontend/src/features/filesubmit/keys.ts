// 文件提交模式（Lemon）——本地存储键。
//
// 与 LemonNext 的键方案一致（草稿按 用户+题目 隔离、消息已读水位、
// 双模式下记忆学生上次选择的答题方式），前缀换成 forge 以免互相污染。

const seenUser = (): string => {
  try {
    const raw = localStorage.getItem('forge.session.user')
    if (raw) return JSON.parse(raw) as string
  } catch {
    // ignore
  }
  return 'anonymous'
}

/** 在线编辑代码草稿：forge.lemon.draft.<user>.<taskId> */
export const codeDraftKey = (user: string, taskId: number) =>
  `forge.lemon.draft.${user || seenUser()}.${taskId}`

/** 选择题答案草稿（JSON 答案表）：forge.lemon.choice.<user>.<taskId> */
export const choiceDraftKey = (user: string, taskId: number) =>
  `forge.lemon.choice.${user || seenUser()}.${taskId}`

/** 双模式（editor/file）下学生上次的选择：forge.lemon.answerMode.<user> */
export const answerModeKey = (user: string) =>
  `forge.lemon.answerMode.${user || seenUser()}`

/** 消息已读水位（最新已读消息 id）：forge.lemon.msgseen */
export const MESSAGE_SEEN_KEY = 'forge.lemon.msgseen'

export const readNumber = (key: string, fallback = 0): number => {
  try {
    const value = parseInt(localStorage.getItem(key) || '', 10)
    return Number.isFinite(value) ? value : fallback
  } catch {
    return fallback
  }
}

export const writeNumber = (key: string, value: number) => {
  try {
    localStorage.setItem(key, String(value))
  } catch {
    // ignore
  }
}
