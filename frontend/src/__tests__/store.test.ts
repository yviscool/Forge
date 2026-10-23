import { describe, it, expect, beforeEach } from 'vitest'
import { useStore } from '../store/useStore'

describe('useStore', () => {
  beforeEach(() => {
    useStore.setState({
      locale: 'zh-CN',
      activeContestId: null,
      drafts: {},
      serverTimeOffset: 0,
      desktopMode: false,
      unlockedContests: {},
      unreadBroadcasts: 0,
      activeAlert: null,
    })
  })

  it('toggles locale correctly', () => {
    expect(useStore.getState().locale).toBe('zh-CN')
    useStore.getState().toggleLocale()
    expect(useStore.getState().locale).toBe('en-US')
    useStore.getState().toggleLocale()
    expect(useStore.getState().locale).toBe('zh-CN')
  })

  it('updates draft correctly', () => {
    useStore.getState().setDraft('prob-1', '#include <iostream>')
    expect(useStore.getState().drafts['prob-1']).toBe('#include <iostream>')
  })

  it('manages desktopMode and zip unlocks correctly', () => {
    expect(useStore.getState().desktopMode).toBe(false)
    useStore.getState().setDesktopMode(true)
    expect(useStore.getState().desktopMode).toBe(true)

    useStore.getState().setContestUnlocked('contest-101', true)
    expect(useStore.getState().unlockedContests['contest-101']).toBe(true)
  })

  it('handles server time offset and broadcast alerts', () => {
    useStore.getState().setServerTimeOffset(1250)
    expect(useStore.getState().serverTimeOffset).toBe(1250)

    useStore.getState().setActiveAlert({
      id: 'msg-1',
      title: '紧急更正',
      content: 'T2 样例数据已修复',
      level: 'urgent',
      senderName: '监考主任',
      createdAt: new Date().toISOString(),
    })
    expect(useStore.getState().activeAlert?.level).toBe('urgent')
  })
})
