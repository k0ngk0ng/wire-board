export function motionPhase(phase: string) {
  return phase === 'live' || phase === 'round_end';
}

export function zoomAfterShot(weapon: string, zoom: number, shot: { weapon: string; at: number }, now: number) {
  const elapsed = Math.round(now) - Math.round(shot.at);
  const duration = weapon === 'awp' ? 1450 : weapon === 'scout' ? 1250 : 0;
  return shot.weapon === weapon && elapsed >= 0 && elapsed < duration ? 0 : zoom;
}
