type Bounds = { left: number; top: number; right: number; bottom: number };
type Point = { x: number; y: number };

export function defaultChatAnchor(bounds: Bounds, width: number, height: number, obstacles: readonly Bounds[], desired?: Point): Point {
  const pixelScale = 64;
  const fixed = (value: number) => Math.round(value * pixelScale);
  const edge = { left: fixed(bounds.left), top: fixed(bounds.top), right: fixed(bounds.right), bottom: fixed(bounds.bottom) };
  const w = Math.max(1, fixed(width)), h = Math.max(1, fixed(height)), gap = 8 * pixelScale;
  const right = Math.max(edge.left, edge.right - w), bottom = Math.max(edge.top, edge.bottom - h);
  const clamp = (x: number, y: number) => ({ x: Math.max(edge.left, Math.min(right, x)), y: Math.max(edge.top, Math.min(bottom, y)) });
  const preferred = desired ? clamp(fixed(desired.x), fixed(desired.y)) : { x: right, y: bottom };
  const rectangles = obstacles.map(rect => ({ left: fixed(rect.left), top: fixed(rect.top), right: fixed(rect.right), bottom: fixed(rect.bottom) }));
  const occupied = (point: Point) => rectangles.some(rect => point.x < rect.right + gap && point.x + w > rect.left - gap && point.y < rect.bottom + gap && point.y + h > rect.top - gap);
  const candidates = [preferred, { x: edge.left, y: bottom }, { x: right, y: edge.top }, { x: edge.left, y: edge.top }];
  for (const rect of rectangles) {
    const left = rect.left - w - gap, after = rect.right + gap, above = rect.top - h - gap, below = rect.bottom + gap;
    candidates.push(clamp(left, preferred.y), clamp(after, preferred.y), clamp(preferred.x, above), clamp(preferred.x, below), clamp(left, above), clamp(after, above), clamp(left, below), clamp(after, below));
  }
  let selected = preferred, distance = Number.MAX_SAFE_INTEGER;
  for (const candidate of candidates) {
    if (occupied(candidate)) continue;
    const dx = candidate.x - preferred.x, dy = candidate.y - preferred.y, score = dx * dx + dy * dy;
    if (score < distance) { selected = candidate; distance = score; }
  }
  return { x: selected.x / pixelScale, y: selected.y / pixelScale };
}
