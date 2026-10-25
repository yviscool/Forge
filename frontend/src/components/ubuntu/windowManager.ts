// 考场桌面的窗口几何（GNOME 行为子集，纯函数便于测试）：
// 8 向边缘调整大小、贴边（顶=最大化 / 左右=半屏）、拖离还原。

export interface WindowRect {
  x: number
  y: number
  w: number
  h: number
}

export const MIN_WINDOW_W = 420
export const MIN_WINDOW_H = 320
/** GNOME 顶栏高度（贴顶判定阈值 = 指针进入顶栏）。 */
export const TOP_BAR_H = 28
/** 贴边判定：指针距屏幕左右边缘的像素阈值。 */
export const SNAP_EDGE_PX = 10

export type ResizeDir = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'

/** 从手柄方向与指针位移计算新矩形（含最小尺寸约束与屏幕上界）。 */
export function applyResize(
  start: WindowRect,
  dir: ResizeDir,
  dx: number,
  dy: number,
  viewport: { w: number; h: number }
): WindowRect {
  let { x, y, w, h } = start
  if (dir.includes('e')) w = Math.max(MIN_WINDOW_W, start.w + dx)
  if (dir.includes('s')) h = Math.max(MIN_WINDOW_H, start.h + dy)
  if (dir.includes('w')) {
    const right = start.x + start.w
    x = Math.min(start.x + dx, right - MIN_WINDOW_W)
    w = right - x
  }
  if (dir.includes('n')) {
    const bottom = start.y + start.h
    y = Math.max(TOP_BAR_H, Math.min(start.y + dy, bottom - MIN_WINDOW_H))
    h = bottom - y
  }
  // 不越出屏幕右/下缘。
  w = Math.min(w, viewport.w - x)
  h = Math.min(h, viewport.h - y)
  return { x: Math.round(x), y: Math.round(y), w: Math.round(w), h: Math.round(h) }
}

export type SnapZone = 'max' | 'left' | 'right' | null

/** 拖动时指针所在贴边区：顶栏→最大化，左右边缘→半屏。 */
export function detectSnapZone(
  pointer: { x: number; y: number },
  viewport: { w: number }
): SnapZone {
  if (pointer.y <= TOP_BAR_H) return 'max'
  if (pointer.x <= SNAP_EDGE_PX) return 'left'
  if (pointer.x >= viewport.w - SNAP_EDGE_PX) return 'right'
  return null
}

/** 贴边落位后的目标矩形（顶栏下方铺满；半屏占左右一半）。 */
export function snapRect(zone: Exclude<SnapZone, null>, viewport: { w: number; h: number }): WindowRect {
  if (zone === 'max') {
    return { x: 0, y: TOP_BAR_H, w: viewport.w, h: viewport.h - TOP_BAR_H }
  }
  if (zone === 'left') {
    return { x: 0, y: TOP_BAR_H, w: Math.round(viewport.w / 2), h: viewport.h - TOP_BAR_H }
  }
  return {
    x: Math.round(viewport.w / 2),
    y: TOP_BAR_H,
    w: viewport.w - Math.round(viewport.w / 2),
    h: viewport.h - TOP_BAR_H,
  }
}

/** 是否处于贴边形态（最大化或半屏）。 */
export function isSnapped(rect: WindowRect, viewport: { w: number; h: number }): boolean {
  const max = snapRect('max', viewport)
  const left = snapRect('left', viewport)
  const right = snapRect('right', viewport)
  return [max, left, right].some((r) => r.x === rect.x && r.y === rect.y && r.w === rect.w && r.h === rect.h)
}
