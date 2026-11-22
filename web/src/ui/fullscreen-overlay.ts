type Rect = { left: number; top: number; right: number; bottom: number };
type Size = { width: number; height: number };

const valid = (values: readonly number[]) => values.every(Number.isSafeInteger);
const overlaps = (a: Rect, b: Rect) => a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;

export function placeFullscreenOverlay({ bounds, size, obstacles, edge = 'right', inset = 8000, gap = 6000 }: { bounds: Rect; size: Size; obstacles: readonly Rect[]; edge?: 'left' | 'right'; inset?: number; gap?: number }) {
  if (!valid([...Object.values(bounds), size.width, size.height, inset, gap]) || bounds.right <= bounds.left || bounds.bottom <= bounds.top || size.width <= 0 || size.height <= 0 || inset < 0 || gap < 0 || !obstacles.every(rect => valid(Object.values(rect)))) throw Error('Invalid integer fullscreen geometry');
  const minLeft = bounds.left + inset;
  const maxLeft = bounds.right - inset - size.width;
  const minTop = bounds.top + inset;
  const maxTop = bounds.bottom - inset - size.height;
  const xs = [...new Set([minLeft, maxLeft, ...obstacles.flatMap(rect => [rect.left - gap - size.width, rect.right + gap])])].filter(value => Number.isSafeInteger(value) && value >= minLeft && value <= maxLeft).sort((a, b) => edge === 'right' ? b - a : a - b);
  const ys = [...new Set([minTop, ...obstacles.map(rect => rect.bottom + gap)])].filter(value => Number.isSafeInteger(value) && value >= minTop && value <= maxTop).sort((a, b) => a - b);
  for (const top of ys) for (const left of xs) {
    const candidate = { left, top, right: left + size.width, bottom: top + size.height };
    if (valid(Object.values(candidate)) && !obstacles.some(rect => overlaps(candidate, rect))) return { left, top, overlap: false };
  }
  return { left: Math.max(bounds.left, edge === 'right' ? maxLeft : minLeft), top: minTop, overlap: true };
}
