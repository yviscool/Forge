import { describe, expect, it } from 'vitest'
import {
  MIN_WINDOW_H,
  MIN_WINDOW_W,
  TOP_BAR_H,
  applyResize,
  detectSnapZone,
  isSnapped,
  snapRect,
  type WindowRect,
} from './windowManager'

const VP = { w: 1440, h: 900 }
const base: WindowRect = { x: 100, y: 80, w: 800, h: 560 }

describe('窗口几何：边缘调整大小', () => {
  it('东/南把手向右下放大', () => {
    expect(applyResize(base, 'e', 120, 0, VP)).toEqual({ ...base, w: 920 })
    expect(applyResize(base, 's', 0, 90, VP)).toEqual({ ...base, h: 650 })
  })

  it('西/北把手移动对边并保持尺寸语义', () => {
    const west = applyResize(base, 'w', 60, 0, VP)
    expect(west.x).toBe(160)
    expect(west.w).toBe(740) // 右缘不动
    const north = applyResize(base, 'n', 0, -40, VP)
    expect(north.y).toBe(40)
    expect(north.h).toBe(600) // 下缘不动
  })

  it('最小尺寸约束与顶栏上界', () => {
    expect(applyResize(base, 'e', -5000, 0, VP).w).toBe(MIN_WINDOW_W)
    expect(applyResize(base, 's', 0, -5000, VP).h).toBe(MIN_WINDOW_H)
    expect(applyResize(base, 'n', 0, -5000, VP).y).toBe(0)
    // 西把手收过头：右缘固定，宽被钳到最小
    const west = applyResize(base, 'w', 5000, 0, VP)
    expect(west.w).toBe(MIN_WINDOW_W)
  })

  it('不越出屏幕右/下缘', () => {
    const far: WindowRect = { x: 1200, y: 700, w: 200, h: 150 }
    const se = applyResize(far, 'se', 2000, 2000, VP)
    expect(se.x + se.w).toBeLessThanOrEqual(VP.w)
    expect(se.y + se.h).toBeLessThanOrEqual(VP.h)
  })
})

describe('窗口几何：贴边', () => {
  it('检测：顶栏→最大化，左右边缘→半屏，中央→无', () => {
    expect(detectSnapZone({ x: 700, y: 10 }, VP)).toBe('max')
    expect(detectSnapZone({ x: 4, y: 500 }, VP)).toBe('left')
    expect(detectSnapZone({ x: 1436, y: 500 }, VP)).toBe('right')
    expect(detectSnapZone({ x: 700, y: 500 }, VP)).toBeNull()
  })

  it('落位：最大化铺满顶栏以下，半屏各占一半', () => {
    const max = snapRect('max', VP)
    expect(max).toEqual({ x: 0, y: 0, w: 1440, h: 900 })
    const left = snapRect('left', VP)
    expect(left.x).toBe(0)
    expect(left.w).toBe(720)
    const right = snapRect('right', VP)
    expect(right.x).toBe(720)
    expect(right.w).toBe(720)
  })

  it('isSnapped 识别贴边形态', () => {
    expect(isSnapped(snapRect('left', VP), VP)).toBe(true)
    expect(isSnapped(snapRect('max', VP), VP)).toBe(true)
    expect(isSnapped(base, VP)).toBe(false)
  })
})
