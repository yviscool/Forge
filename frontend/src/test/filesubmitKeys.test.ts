import { beforeEach, describe, expect, it } from 'vitest'
import {
  MESSAGE_SEEN_KEY,
  answerModeKey,
  choiceDraftKey,
  codeDraftKey,
  readNumber,
  writeNumber,
} from '@/features/filesubmit/keys'

// 文件提交模式（Lemon）本地存储键：与 LemonNext 键方案一致的格式契约。

describe('filesubmit storage keys', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('builds per-user keys with the LemonNext shape', () => {
    expect(codeDraftKey('HA-1', 3)).toBe('forge.lemon.draft.HA-1.3')
    expect(choiceDraftKey('HA-1', 4)).toBe('forge.lemon.choice.HA-1.4')
    expect(answerModeKey('HA-1')).toBe('forge.lemon.answerMode.HA-1')
  })

  it('falls back to the persisted session user', () => {
    localStorage.setItem('forge.session.user', JSON.stringify('HA-J00001'))
    expect(codeDraftKey('', 5)).toBe('forge.lemon.draft.HA-J00001.5')
    expect(answerModeKey('')).toBe('forge.lemon.answerMode.HA-J00001')
  })

  it('falls back to anonymous when no session user exists', () => {
    expect(choiceDraftKey('', 2)).toBe('forge.lemon.choice.anonymous.2')
  })

  it('keeps a corrupted session user from breaking key building', () => {
    localStorage.setItem('forge.session.user', '{not-json')
    expect(codeDraftKey('', 1)).toBe('forge.lemon.draft.anonymous.1')
  })

  it('round-trips numbers and honours fallbacks', () => {
    expect(readNumber(MESSAGE_SEEN_KEY)).toBe(0)
    expect(readNumber(MESSAGE_SEEN_KEY, 7)).toBe(7)
    writeNumber(MESSAGE_SEEN_KEY, 42)
    expect(readNumber(MESSAGE_SEEN_KEY)).toBe(42)
    localStorage.setItem(MESSAGE_SEEN_KEY, 'not-a-number')
    expect(readNumber(MESSAGE_SEEN_KEY, 3)).toBe(3)
  })
})
