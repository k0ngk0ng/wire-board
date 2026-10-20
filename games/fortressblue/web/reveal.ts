export function shouldRevealBoard(
  rect: { top: number; bottom: number; height: number },
  viewportHeight: number,
  phase: string,
  alreadyRevealed: boolean,
) {
  return (
    phase === "playing" &&
    !alreadyRevealed &&
    rect.height > 0 &&
    rect.height <= viewportHeight &&
    (rect.top < 0 || rect.bottom > viewportHeight)
  );
}
