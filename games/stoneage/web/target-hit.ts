type SpriteHit = {
  id: number;
  depth: number;
  rect: { left: number; top: number; width: number; height: number };
  width: number;
  height: number;
  alpha: (x: number, y: number) => number;
};
export function pickSpriteTarget(
  x: number,
  y: number,
  sprites: readonly SpriteHit[],
) {
  const ordered = [...sprites].sort((a, b) => b.depth - a.depth);
  for (const sprite of ordered) {
    const r = sprite.rect;
    if (
      r.width <= 0 ||
      r.height <= 0 ||
      x < r.left ||
      y < r.top ||
      x >= r.left + r.width ||
      y >= r.top + r.height
    )
      continue;
    const px = Math.min(
      sprite.width - 1,
      Math.floor(((x - r.left) * sprite.width) / r.width),
    );
    const py = Math.min(
      sprite.height - 1,
      Math.floor(((y - r.top) * sprite.height) / r.height),
    );
    if (sprite.alpha(px, py) > 24) return sprite.id;
  }
  return null;
}
