import { create } from 'zustand'

export type Locale = 'zh-CN' | 'en-US'

export interface UserSession {
  userId: string
  username: string
  name: string
  role: string
  /** 准考证号（如 HA-J00001）：考场桌面主目录名与考场身份展示用；未发号则缺省。 */
  examTicket?: string
}

export interface BroadcastAlert {
  id: string
  title: string
  content: string
  level: 'info' | 'warning' | 'urgent'
  senderName: string
  createdAt: string
}

interface AppState {
  locale: Locale
  setLocale: (l: Locale) => void
  toggleLocale: () => void

  session: UserSession | null
  setSession: (s: UserSession | null) => void
  logout: () => void

  // Session lifecycle: a persisted token is optimistic until /auth/me says
  // otherwise. Protected routes render a loading state while this is pending.
  authStatus: 'idle' | 'checking' | 'authenticated' | 'anonymous' | 'unavailable'
  setAuthStatus: (status: AppState['authStatus']) => void

  activeContestId: string | null
  setActiveContestId: (id: string | null) => void

  // Code drafts by problem ID
  drafts: Record<string, string>
  setDraft: (problemId: string, code: string) => void

  // 消除时钟差：服务端时间偏置 (ms) = serverTime - localClientTime
  serverTimeOffset: number
  setServerTimeOffset: (offset: number) => void

  // Ubuntu 18.04 考场桌面仿真模式
  desktopMode: boolean
  setDesktopMode: (val: boolean) => void

  // 考场 Zip 密码解锁状态：contestId -> unlocked
  unlockedContests: Record<string, boolean>
  setContestUnlocked: (cid: string, unlocked: boolean) => void

  // 考场突发广播未读数与即时弹窗告警
  unreadBroadcasts: number
  setUnreadBroadcasts: (cnt: number) => void
  activeAlert: BroadcastAlert | null
  setActiveAlert: (alert: BroadcastAlert | null) => void

  /** 本地持久化失败（配额满/私密模式）：UI 顶部红色横幅，考生可见。 */
  persistenceError: string | null
  clearPersistenceError: () => void
}

// Local exam artifacts must never cross account boundaries on a shared exam
// machine. Every key carries the owning user, so an interrupted logout cannot
// expose one candidate's work to the next person who signs in.
export function scopedKey(userId: string | undefined | null, ...parts: string[]): string {
  return ['u', userId || 'anonymous', ...parts].join(':')
}

export function contestProblemDraftKey(
  userId: string | undefined | null,
  contestId: string,
  problemId: string
): string {
  return scopedKey(userId, 'contest', contestId, 'problem', problemId)
}

export function choiceDraftKey(
  userId: string | undefined | null,
  contestId: string,
  problemId: string
): string {
  return scopedKey(userId, 'contest', contestId, 'problem', problemId, 'choice-sheet')
}

const getBrowserStorage = (): Storage | null => {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null
  } catch {
    return null
  }
}

const getStorageItem = (k: string) => {
  try {
    return getBrowserStorage()?.getItem(k) ?? null
  } catch {
    return null
  }
}

// persistenceBroken：一次失败后持续告警，直到成功写入复位。
let persistenceBroken = false

const setStorageItem = (k: string, v: string) => {
  try {
    getBrowserStorage()?.setItem(k, v)
    return true
  } catch {
    // 私密模式/配额满：不抛异常打断 UI，但必须让上层知道写入失败——
    // 草稿是考生答卷，静默丢失等于丢卷（持久化失败横幅由 UI 呈现）。
    persistenceBroken = true
    queueMicrotask(() => {
      try {
        useStore.setState({ persistenceError: '本地存储写入失败（可能已满）：草稿与考场文件未保存！请立即举手联系监考教师。' })
      } catch {
        /* store 未初始化时忽略 */
      }
    })
    return false
  }
}

const removeStorageItem = (k: string) => {
  try {
    getBrowserStorage()?.removeItem(k)
  } catch {
    // Storage is a cache, not the source of truth.
  }
}

// Remove credentials persisted by pre-launch builds. Authentication now lives
// exclusively in an HttpOnly cookie and must never be readable by JavaScript.
removeStorageItem('forge_session')

// 草稿持久化：打字高频，全量序列化走 debounce（300ms）；页面卸载/隐藏时
// 同步 flush，保证「保存」语义在任何退出路径下成立。
let draftsPending: Record<string, string> | null = null
let draftsTimer = 0
const flushDrafts = () => {
  if (draftsTimer) {
    clearTimeout(draftsTimer)
    draftsTimer = 0
  }
  if (!draftsPending) return
  const snapshot = draftsPending
  draftsPending = null
  if (setStorageItem('forge_drafts', JSON.stringify(snapshot)) && persistenceBroken) {
    persistenceBroken = false
    try {
      useStore.setState({ persistenceError: null })
    } catch {
      /* noop */
    }
  }
}
const scheduleDraftsFlush = (drafts: Record<string, string>) => {
  draftsPending = drafts
  if (draftsTimer) return
  draftsTimer = window.setTimeout(() => {
    draftsTimer = 0
    flushDrafts()
  }, 300)
}
if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', flushDrafts)
  window.addEventListener('pagehide', flushDrafts)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushDrafts()
  })
}

export const useStore = create<AppState>((set) => ({
  locale: (getStorageItem('forge_locale') as Locale) || 'zh-CN',
  setLocale: (locale) => {
    setStorageItem('forge_locale', locale)
    set({ locale })
  },
  toggleLocale: () =>
    set((state) => {
      const next = state.locale === 'zh-CN' ? 'en-US' : 'zh-CN'
      setStorageItem('forge_locale', next)
      return { locale: next }
    }),

  session: null,
  authStatus: 'idle',
  setAuthStatus: (authStatus) => set({ authStatus }),
  setSession: (session) => {
    set({ session, authStatus: session ? 'authenticated' : 'anonymous' })
  },
  logout: () => {
    flushDrafts()
    removeStorageItem('forge_session')
    removeStorageItem('forge_unlocked')
    set({
      session: null,
      authStatus: 'anonymous',
      unlockedContests: {},
      activeAlert: null,
      unreadBroadcasts: 0,
      activeContestId: null,
      serverTimeOffset: 0,
    })
  },

  activeContestId: null,
  setActiveContestId: (activeContestId) => set({ activeContestId }),

  drafts: (() => {
    try {
      const saved = getStorageItem('forge_drafts')
      return saved ? JSON.parse(saved) : {}
    } catch {
      return {}
    }
  })(),
  setDraft: (problemId, code) => {
    set((state) => {
      const nextDrafts = { ...state.drafts, [problemId]: code }
      scheduleDraftsFlush(nextDrafts)
      return { drafts: nextDrafts }
    })
  },

  serverTimeOffset: 0,
  setServerTimeOffset: (serverTimeOffset) => set({ serverTimeOffset }),

  desktopMode: getStorageItem('forge_desktop_mode') === 'true',
  setDesktopMode: (desktopMode) => {
    setStorageItem('forge_desktop_mode', desktopMode ? 'true' : 'false')
    set({ desktopMode })
  },

  unlockedContests: (() => {
    try {
      const saved = getStorageItem('forge_unlocked')
      return saved ? JSON.parse(saved) : {}
    } catch {
      return {}
    }
  })(),
  setContestUnlocked: (cid, unlocked) =>
    set((state) => {
      const next = { ...state.unlockedContests, [cid]: unlocked }
      setStorageItem('forge_unlocked', JSON.stringify(next))
      return { unlockedContests: next }
    }),

  unreadBroadcasts: 0,
  setUnreadBroadcasts: (unreadBroadcasts) => set({ unreadBroadcasts }),
  activeAlert: null,
  setActiveAlert: (activeAlert) => set({ activeAlert }),
  persistenceError: null,
  clearPersistenceError: () => set({ persistenceError: null }),
}))
