import { beforeEach, describe, expect, it } from 'vitest'
import { useStore, type UserSession } from '../store/useStore'

const session: UserSession = {
  userId: 'user-1',
  username: 'alice',
  name: 'Alice',
  role: 'student',
}

describe('session store', () => {
  beforeEach(() => {
    localStorage.clear()
    useStore.setState({ session: null, authStatus: 'idle' })
  })

  it('keeps the login session in memory and never persists the credential', () => {
    useStore.getState().setSession(session)

    expect(useStore.getState().session).toEqual(session)
    expect(useStore.getState().authStatus).toBe('authenticated')
    expect(localStorage.getItem('forge_session')).toBeNull()
  })

  it('clears credentials and grants but preserves user-scoped drafts on logout', () => {
    useStore.getState().setSession(session)
    useStore.getState().setDraft('u:user-1:contest:c:problem:p', 'secret source')
    useStore.getState().setContestUnlocked('c', true)
    useStore.getState().setActiveAlert({
      id: 'msg-urgent',
      title: 'urgent',
      content: 'private',
      level: 'urgent',
      senderName: 'teacher',
      createdAt: new Date().toISOString(),
    })

    useStore.getState().logout()

    expect(useStore.getState().session).toBeNull()
    expect(useStore.getState().authStatus).toBe('anonymous')
    expect(useStore.getState().drafts).toHaveProperty(
      'u:user-1:contest:c:problem:p',
      'secret source'
    )
    expect(useStore.getState().unlockedContests).toEqual({})
    expect(useStore.getState().activeAlert).toBeNull()
    expect(localStorage.getItem('forge_session')).toBeNull()
    expect(localStorage.getItem('forge_drafts')).toContain('secret source')
    expect(localStorage.getItem('forge_unlocked')).toBeNull()
  })

  it('allows an explicit anonymous session', () => {
    useStore.getState().setSession(session)

    useStore.getState().setSession(null)

    expect(useStore.getState().session).toBeNull()
    expect(useStore.getState().authStatus).toBe('anonymous')
    expect(localStorage.getItem('forge_session')).toBeNull()
  })
})
